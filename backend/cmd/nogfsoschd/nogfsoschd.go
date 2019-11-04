// vim: sw=8

// Nog FSO schedule server `nogfsoschd`.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/internal/grpcjwt"
	"github.com/nogproject/nog/backend/internal/nogfsoschd/execute"
	"github.com/nogproject/nog/backend/internal/nogfsoschd/observe"
	"github.com/nogproject/nog/backend/internal/nogfsoschd/scan"
	"github.com/nogproject/nog/backend/pkg/mulog"
	"github.com/nogproject/nog/backend/pkg/x509io"
	"github.com/nogproject/nog/backend/pkg/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// `xVersion` and `xBuild` are injected by the `Makefile`.
var (
	xVersion string
	xBuild   string
	version  = fmt.Sprintf("nogfsoschd-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogfsoschd [options] [--state=<dir>]
             --registry=<registry>...
             --prefix=<path>... --host=<host>... [--ref=<ref>...]
             [--no-watch] [--scan-start] [--scan-every=<interval>]
             [--] [<cmd> [<cmdargs>...]]

Options:
  --log=<logger>  [default: prod]
        Specify logger: prod, dev, or mu.
  --tls-cert=<pem>  [default: /nog/ssl/certs/nogfsoschd/combined.pem]
        TLS client certificate and corresponding private key.  PEM files can be
        concatenated ''cat cert.pem privkey.pem > combined.pem''.
  --tls-ca=<pem>  [default: /nog/ssl/certs/nogfsoschd/ca.pem]
        X.509 CA for TLS.  Multiple PEM files can be concatenated.
  --sys-jwt=<path>  [default: /nog/jwt/tokens/nogfsoschd.jwt]
        Path of the JWT for system GRPCs.
  --nogfsoregd=<addr>  [default: localhost:7550]
  --shutdown-timeout=<duration>  [default: 1h]
        Maximum time to wait before forced shutdown.
  --state=<dir>
        Directory to which to save state that should be maintained across
        restarts, such as journal locations.
  --no-watch
        Disable watch registry broadcast for changes.
  --registry=<registry>
        Registries to watch.
  --prefix=<path>
        Limits processing to repos whose global paths are equal or below one of
        the prefixes.
  --host=<host>
        Repos that pass the prefix filter must be on one of the hosts.
  --ref=<ref>
        Shadow Git refs to watch; full ref, like ''refs/heads/master-stat''.
  --scan-start
        Scan repos of registries matching prefixes during startup.
  --scan-every=<interval>
        Regularly scan repos of registries matching prefixes.

''nogfsoschd'' watches the registries for changes to repos below the specified
prefixes.  It runs ''<cmd> <cmdargs>... <repojson>'' for each change, where
''<repojson>'' is a JSON object that is encoded as a single line without
whitespace and contains the following information:

    {
        "id": "1076f5d4-ee22-43c8-9efe-c9dae3ec2c1f",
        "vid": "01CB9Y3FB3384YM968RXMTFS3N",
        "registry": "exreg",
        "globalPath": "/example/data",
        "file": "files.example.com:/data",
        "shadow": "files.example.com:/shadow/1076f5d4-ee22-43c8-9efe-c9dae3ec2c1f.fso",
        "archive": "tartt://files.example.com/tartt/1076f5d4-ee22-43c8-9efe-c9dae3ec2c1f.tartt",
        "archiveRecipients": [
                "8080808080808080808080808080808080808080",
                "F0F0F0F0F0F0F0F0F0F0F0F0F0F0F0F0F0F0F0F0"
        ],
        "shadowBackup": "nogfsobak://files.example.com/bak/1076f5d4-ee22-43c8-9efe-c9dae3ec2c1f",
        "shadowBackupRecipients": [
                "5050505050505050505050505050505050505050",
                "C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0C0"
        ],
    }

Empty fields may be omitted.

''archiveRecipients'' and ''shadowBackupRecipients'' are lists of GPG key
fingerprints.  A list is omitted if encryption is not configured.

Unless ''<cmd>'' is interrupted by SIGINT or SIGTERM, ''nogfsoschd'' updates
the event journal cursors in ''--state=<dir>'' and will not process the same
change again after a restart.  If ''<cmd>'' completes with a non-zero exit
code, ''nogfsoschd'' will log the error without retrying the command.
`)

var (
	clientAliveInterval      = 40 * time.Second
	clientAliveWithoutStream = true
)

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
	Fatalw(msg string, kv ...interface{})
}

var lg Logger = mulog.Logger{}

func main() {
	args := argparse()
	initLogging(args["--log"].(string))

	// The scanner uses toplevel rand function.  Init seed to avoid
	// repeating the same scan order after restart.
	rand.Seed(time.Now().UnixNano())

	cert, err := x509io.LoadCombinedCert(args["--tls-cert"].(string))
	if err != nil {
		lg.Fatalw("Failed to load --tls-cert.", "err", err)
	}
	ca, err := x509io.LoadCABundle(args["--tls-ca"].(string))
	if err != nil {
		lg.Fatalw("Failed to load --tls-ca.", "err", err)
	}

	sysRPCCreds, err := grpcjwt.Load(args["--sys-jwt"].(string))
	if err != nil {
		lg.Fatalw("Failed to load --sys-jwt", "err", err)
	}

	lg.Infow("nogfsoschd started.")

	conn, err := grpc.Dial(
		args["--nogfsoregd"].(string),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      ca,
		})),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                clientAliveInterval,
			PermitWithoutStream: clientAliveWithoutStream,
		}),
	)
	if err != nil {
		lg.Fatalw("Failed to dial nogfsoregd.", "err", err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			lg.Errorw(
				"Failed to close nogfsoregd conn.", "err", err,
			)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	signal.Notify(sigs, syscall.SIGINT)
	var isShutdown int32

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	ctxSlow, cancelSlow := context.WithCancel(context.Background())

	procCfg := &execute.Config{
		CmdArgs: args["<cmdargs>"].([]string),
	}
	if a, ok := args["<cmd>"].(string); ok {
		procCfg.Cmd = a
	}
	proc := execute.NewProcessor(ctxSlow, lg, procCfg)

	if args["--no-watch"].(bool) {
		lg.Infow("Watch disabled.")
	} else {
		stateDir, ok := args["--state"].(string)
		if !ok {
			lg.Fatalw("--state required unless --no-watch.")
		}
		state := observe.NewFileStateStore(stateDir)
		obs := observe.NewObserver(lg, &observe.Config{
			Conn:       conn,
			RPCCreds:   sysRPCCreds,
			StateStore: state,
			Processor:  proc,
			Registries: args["--registry"].([]string),
			Refs:       args["--ref"].([]string),
			Prefixes:   args["--prefix"].([]string),
			Hosts:      args["--host"].([]string),
		})
		lg.Infow("Enabled watch registry broadcast.")
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := obs.Watch(ctx)
			if err != context.Canceled {
				lg.Fatalw("Observer failed.", "err", err)
			}
			if atomic.LoadInt32(&isShutdown) == 0 {
				lg.Fatalw("Unexpected observer cancel.")
			}
		}()
	}

	scanner := scan.NewScanner(lg, &scan.Config{
		Conn:       conn,
		RPCCreds:   sysRPCCreds,
		Processor:  proc,
		Registries: args["--registry"].([]string),
		Prefixes:   args["--prefix"].([]string),
		Hosts:      args["--host"].([]string),
	})
	if scanEvery, ok := args["--scan-every"].(time.Duration); ok {
		if args["--scan-start"].(bool) {
			lg.Infow("Enabled initial scan and regular scans.")
		} else {
			lg.Infow("Enabled regular scans.")
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if args["--scan-start"].(bool) {
				err := scanner.Scan(ctx)
				if err != nil {
					lg.Warnw(
						"Initial scan failed.",
						"err", err,
					)
				}
			}
			tick := time.NewTicker(scanEvery)
			for {
				select {
				case <-ctx.Done():
					tick.Stop()
					return
				case <-tick.C:
					err := scanner.Scan(ctx)
					if err != nil {
						lg.Warnw(
							"Regular scan failed.",
							"err", err,
						)
					}
				}
			}
		}()
	} else if args["--scan-start"].(bool) {
		lg.Infow("Enabled initial scan.")
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := scanner.Scan(ctx)
			if err != nil {
				lg.Warnw("Initial scan failed.", "err", err)
			}
		}()
	} else {
		lg.Infow("Scans disabled.")
	}

	sig := <-sigs
	atomic.StoreInt32(&isShutdown, 1)

	done := make(chan struct{})
	go func() {
		cancel()
		wg.Wait()
		lg.Infow("Completed level 1 shutdown.")

		close(done)
	}()

	d := args["--shutdown-timeout"].(time.Duration)
	timeout := time.NewTimer(d)
	lg.Infow("Started graceful shutdown.", "sig", sig, "timeout", d)

	select {
	case <-timeout.C:
		cancelSlow()
		lg.Warnw("Timeout; forced shutdown.")
	case <-done:
		cancelSlow()
		lg.Infow("Completed graceful shutdown.")
	}

}

func initLogging(arg string) {
	var err error
	switch arg {
	case "prod":
		lg, err = zap.NewProduction()
	case "dev":
		lg, err = zap.NewDevelopment()
	case "mu":
		lg = mulog.Logger{}
	default:
		err = fmt.Errorf("Invalid --log option.")
	}
	if err != nil {
		log.Fatal(err)
	}
}

func argparse() map[string]interface{} {
	const autoHelp = true
	const noOptionFirst = false
	args, err := docopt.Parse(
		usage, nil, autoHelp, version, noOptionFirst,
	)
	if err != nil {
		lg.Fatalw("docopt failed", "err", err)
	}

	for _, k := range []string{
		"--shutdown-timeout",
		"--scan-every",
	} {
		if arg, ok := args[k].(string); ok {
			d, err := time.ParseDuration(arg)
			if err != nil {
				lg.Fatalw(
					fmt.Sprintf("Invalid %s", k),
					"err", err,
				)
			}
			args[k] = d
		}
	}

	return args
}
