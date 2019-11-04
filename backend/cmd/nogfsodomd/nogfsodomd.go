// vim: sw=8

// Server `nogfsodomd`.
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
	"github.com/nogproject/nog/backend/internal/nogfsodomd"
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
	version  = fmt.Sprintf("nogfsodomd-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogfsodomd [options] --group-prefix=<prefix>... <domain>

Options:
  --tls-cert=<pem>  [default: /nog/ssl/certs/nogfsodomd/combined.pem]
        TLS certificate and corresponding private key.  PEM files can be
        concatenated ''cat cert.pem privkey.pem > combined.pem''.
  --tls-ca=<pem>  [default: /nog/ssl/certs/nogfsodomd/ca.pem]
        TLS certificates that are accepted as CA for client certs.  Multiple
        PEM files can be concatenated.
  --sys-jwt=<path>  [default: /nog/jwt/tokens/nogfsodomd.jwt]
        Path of the JWT for system GRPCs.
  --nogfsoregd=<addr>  [default: localhost:7550]
  --shutdown-timeout=<duration>  [default: 20s]
        Maximum time to wait before forced shutdown.
  --group-prefix=<prefix>
        Prefix to select groups from ''getent group''
  --sync-domain-start=<wait-duration>  [default: 10m]
        Run first ''getent'' at startup after a wait duration.
        Use ''0'' to disable.
  --sync-domain-every=<interval>  [default: 1h]
        Run ''getent'' at regular intervals.  Use ''0'' to disable.
  --log=<logger>  [default: prod]
        Specify logger: prod, dev, or mu.

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

	lg.Infow("nogfsodomd started.")

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

	syncer := nogfsodomd.New(lg, &nogfsodomd.Config{
		Domain:        args["<domain>"].(string),
		GroupPrefixes: args["--group-prefix"].([]string),
		Conn:          conn,
		SysRPCCreds:   sysRPCCreds,
	})
	startSyncs(args, &wg, ctx, syncer)

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
		"--sync-domain-start",
		"--sync-domain-every",
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

func startSyncs(
	args map[string]interface{},
	wg *sync.WaitGroup,
	ctx context.Context,
	syncer *nogfsodomd.Syncer,
) {
	start, startYes := args["--sync-domain-start"].(time.Duration)
	if startYes && start == 0 {
		startYes = false
	}
	every, everyYes := args["--sync-domain-every"].(time.Duration)
	if everyYes && every == 0 {
		everyYes = false
	}
	if !startYes && !everyYes {
		return
	}
	switch {
	case startYes && everyYes:
		lg.Infow(
			"Enabled initial and regular domain sync.",
			"start", start,
			"every", every,
		)
	case startYes:
		lg.Infow(
			"Enabled initial domain sync.",
			"start", start,
		)
	case everyYes:
		lg.Infow(
			"Enabled regular domain sync.",
			"every", every,
		)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		runSyncs(ctx, syncer, start, every)
	}()
}

func runSyncs(
	ctx context.Context,
	syncer *nogfsodomd.Syncer,
	scanStart, scanEvery time.Duration,
) {
	if scanStart != 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.NewTimer(scanStart).C:
		}
		lg.Infow("Started initial domain sync.")
		err := syncer.Sync(ctx)
		if err == context.Canceled {
			return
		}
		if err != nil {
			lg.Warnw("Initial domain sync failed.", "err", err)
		} else {
			lg.Infow("Completed initial domain sync.")
		}
	}

	if scanEvery == 0 {
		return
	}
	tick := time.NewTicker(scanEvery)
	for {
		select {
		case <-ctx.Done():
			tick.Stop()
			return
		case <-tick.C:
			lg.Infow("Started regular domain sync.")
			err := syncer.Sync(ctx)
			if err == context.Canceled {
				continue
			}
			if err != nil {
				lg.Warnw(
					"Regular domain sync failed.",
					"err", err,
				)
			} else {
				lg.Infow("Completed regular domain sync.")
			}
		}
	}
}
