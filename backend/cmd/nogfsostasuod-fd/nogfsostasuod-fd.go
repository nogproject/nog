// vim: sw=8

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/internal/nogfsostaudod"
	pb "github.com/nogproject/nog/backend/internal/udopb"
	"github.com/nogproject/nog/backend/pkg/mulog"
	"github.com/nogproject/nog/backend/pkg/netx"
	"github.com/nogproject/nog/backend/pkg/pwd"
	"github.com/nogproject/nog/backend/pkg/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// `xVersion` and `xBuild` are injected by the `Makefile`.
var (
	xVersion string
	xBuild   string
	version  = fmt.Sprintf("nogfsostasuod-fd-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogfsostasuod-fd [options]

Options:
  --log=<logger>  [default: prod]
        Specify logger: prod, dev, or mu.
  --shutdown-timeout=<duration>  [default: 20s]
        Maximum time to wait before forced shutdown.
  --conn-fd=<fd>  [default: 3]
        Wait for a single client on a connected socket file descriptor,
        which is usually one end of a ''socketpair()''.

''nogfsostasuod-fd'' is like ''nogfsostaudod-fd''.  But ''nogfsostasuod-fd'' is
safer to run as root, because it provides a more restricted set of operations
than ''nogfsostaudod-fd'', which should be run only as non-root.
`)

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

	lg.Infow("nogfsostasuod started.")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	signal.Notify(sigs, syscall.SIGINT)
	var isShutdown int32

	// The default `grpc.keepalive` parameters allow connections to persist
	// forever.
	srv := grpc.NewServer()

	daemonD := &daemonServer{}
	pb.RegisterUdoDaemonServer(srv, daemonD)

	udoD, err := nogfsostaudod.New(lg)
	if err != nil {
		lg.Fatalw(
			"Failed to create nogfsostaudod server.",
			"err", err,
		)
	}
	pb.RegisterUdoStatServer(srv, udoD)
	pb.RegisterUdoChattrServer(srv, udoD)
	pb.RegisterUdoAclBashServer(srv, udoD)
	pb.RegisterUdoRenameServer(srv, udoD)

	conn, err := netx.FdConn(uintptr(args["--conn-fd"].(int)))
	if err != nil {
		lg.Fatalw("Failed to use --conn-fd as server connection.")
	}

	go func() {
		err := srv.Serve(netx.ListenConnectedConn(conn))
		if atomic.LoadInt32(&isShutdown) > 0 {
			return
		}
		lg.Fatalw(
			"gRPC server stopped unexpectedly.",
			"module", "nogfsostasuod",
			"err", err,
		)
	}()

	sig := <-sigs
	atomic.StoreInt32(&isShutdown, 1)

	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		lg.Infow(
			"Completed gRPC server shutdown.",
			"module", "nogfsostasuod",
		)
		close(done)
	}()

	d := args["--shutdown-timeout"].(time.Duration)
	timeout := time.NewTimer(d)
	lg.Infow(
		"Started graceful shutdown.",
		"module", "nogfsostasuod",
		"sig", sig,
		"timeout", d,
	)
	select {
	case <-timeout.C:
		srv.Stop()
		lg.Warnw(
			"Forced shutdown after timeout.",
			"module", "nogfsostasuod",
		)
	case <-done:
		lg.Infow(
			"Completed graceful shutdown.",
			"module", "nogfsostasuod",
		)
	}
}

func argparse() map[string]interface{} {
	const autoHelp = true
	const noOptionFirst = false
	args, err := docopt.Parse(
		usage, nil, autoHelp, version, noOptionFirst,
	)
	if err != nil {
		lg.Fatalw(
			"docopt failed",
			"module", "nogfsostasuod",
			"err", err,
		)
	}

	for _, k := range []string{
		"--shutdown-timeout",
	} {
		if arg, ok := args[k].(string); ok {
			d, err := time.ParseDuration(arg)
			if err != nil {
				lg.Fatalw(
					fmt.Sprintf("Invalid %s", k),
					"module", "nogfsostasuod",
					"err", err,
				)
			}
			args[k] = d
		}
	}

	for _, k := range []string{
		"--conn-fd",
	} {
		if v, err := strconv.Atoi(args[k].(string)); err != nil {
			lg.Fatalw(
				fmt.Sprintf("Invalid %s", k),
				"module", "nogfsostasuod",
				"err", err,
			)
		} else {
			args[k] = v
		}
	}

	return args
}

var ErrServerUserLookup = status.Error(
	codes.Internal, "failed to lookup server process user",
)

type daemonServer struct{}

// `Ping()` does not cache lookups, so that every ping checks the process state
// as it is returned by the runtime.
func (srv *daemonServer) Ping(
	ctx context.Context, i *pb.PingI,
) (*pb.PingO, error) {
	uid := uint32(os.Getuid())
	pw := pwd.Getpwuid(uid)
	if pw == nil {
		return nil, ErrServerUserLookup
	}

	return &pb.PingO{
		Username: pw.Name,
		Uid:      uid,
		Pid:      int32(os.Getpid()),
		Ppid:     int32(os.Getppid()),
	}, nil
}

func (srv *daemonServer) Terminate(
	ctx context.Context, i *pb.TerminateI,
) (*pb.TerminateO, error) {
	err := syscall.Kill(os.Getpid(), syscall.SIGTERM)
	if err != nil {
		return nil, err
	}
	return &pb.TerminateO{}, nil
}
