// vim: sw=8

// Server `nogfsorstd`; see NOE-24.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	docopt "github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/internal/grpcjwt"
	"github.com/nogproject/nog/backend/internal/nogfsorstd/workflowproc"
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
	version  = fmt.Sprintf("nogfsorstd-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogfsorstd [options] --host=<host>... --prefix=<path>... <registry>...

Options:
  --tls-cert=<pem>  [default: /nog/ssl/certs/nogfsorstd/combined.pem]
        TLS certificate and corresponding private key.  PEM files can be
        concatenated ''cat cert.pem privkey.pem > combined.pem''.
  --tls-ca=<pem>  [default: /nog/ssl/certs/nogfsorstd/ca.pem]
        TLS certificates that are accepted as CA for client certs.  Multiple
        PEM files can be concatenated.
  --sys-jwt=<path>  [default: /nog/jwt/tokens/nogfsorstd.jwt]
        Path of the JWT for system GRPCs.
  --nogfsoregd=<addr>  [default: localhost:7550]
  --shutdown-timeout=<duration>  [default: 20s]
        Maximum time to wait before forced shutdown.
  --prefix=<path>
        Limits processing to repos whose global path is below one of the
        prefixes.
  --host=<host>
        Repos that pass the prefix filter must be on one of the hosts.
  --cap-path=<dir>  [default: /usr/local/lib/nogfsorstd]
        The directory will be prepended to the environment variable ''PATH''
        when executing ''tartt''.  See below for details.
  --log=<logger>  [default: prod]
        Specify logger: prod, dev, or mu.

If ''--cap-path'' is specified, it must contain a tar program with capabilities
that allow it to restore file ownership and permissions, usually:

    setcap cap_chown,cap_dac_override,cap_fowner=ep tar

Use ''--cap-path='' to disable using a special tar program.

`)

var (
	ConfigClientAliveInterval      = 40 * time.Second
	ConfigClientAliveWithoutStream = true
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

	var err error
	switch args["--log"].(string) {
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

	lg.Infow("nogfsorstd started.")

	conn, err := grpc.Dial(
		args["--nogfsoregd"].(string),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      ca,
		})),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                ConfigClientAliveInterval,
			PermitWithoutStream: ConfigClientAliveWithoutStream,
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

	workflowProc := workflowproc.New(lg, &workflowproc.Config{
		Registries:  args["<registry>"].([]string),
		Prefixes:    args["--prefix"].([]string),
		Hosts:       args["--host"].([]string),
		Conn:        conn,
		SysRPCCreds: sysRPCCreds,
		CapPath:     args["--cap-path"].(string),
	})
	wg.Add(1)
	go func() {
		err := workflowProc.Run(ctx)
		if err != context.Canceled {
			lg.Fatalw(
				"Unexpected workflow processor error.",
				"err", err,
			)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected workflow processor shutdown.")
		}
		wg.Done()
	}()

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
		lg.Warnw("Timeout; forced shutdown.")
	case <-done:
		lg.Infow("Completed graceful shutdown.")
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
