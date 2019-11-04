// vim: sw=8

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/internal/nogfsostaudod"
	pb "github.com/nogproject/nog/backend/internal/udopb"
	"github.com/nogproject/nog/backend/pkg/grpc/ucred"
	"github.com/nogproject/nog/backend/pkg/mulog"
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
	version  = fmt.Sprintf("nogfsostaudod-path-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogfsostaudod-path [options] --stad-socket-dir=<dir> --stad-users=<usernames>

Options:
  --log=<logger>  [default: prod]
        Specify logger: prod, dev, or mu.
  --shutdown-timeout=<duration>  [default: 20s]
        Maximum time to wait before forced shutdown.
  --stad-socket-dir=<dir>
        Directory for the Unix socket that ''nogfsostad'' will connect to.
  --stad-users=<usernames>
        ''nogfsostad'' users that are allowed to connect; comma-separated list.

''nogfsostaudod-path'' and ''nogfsostaudod-fd'' both execute commands as a
specific user on behalf of ''nogfsostad''.  ''nogfsostaudod-path'' must be
started in advance; see below.  ''nogfsostaudod-fd'' is started by
''nogfsostad'' when needed; see ''nogfsostaudod-fd --help''.

''nogfsostad'' expects the ''nogfsostaudod-path'' sockets in the directory
''--stad-socket-dir'', which usually has temp-directory-like permissions.
Example:

    # as user root
    mkdir -m a=rwx,o+t /var/run/nogfsostad/udod

    # as user ngfsta
    nogfsostad ... --udod-socket-dir=/var/run/nogfsostad/udod ...

    # as user alice
    nogfsostaudod-path \
        --stad-socket-dir=/var/run/nogfsostad/udod \
        --stad-users=ngfsta

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

	lg.Infow("nogfsostaudod started.")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	signal.Notify(sigs, syscall.SIGINT)
	var isShutdown int32

	stadUsers := args["--stad-users"].([]string)
	stadUids := make([]uint32, len(stadUsers))
	for i, usr := range stadUsers {
		pw := pwd.Getpwnam(usr)
		if pw == nil {
			lg.Fatalw(
				"Unknown user in --stad-users.",
				"user", usr,
			)
		}
		stadUids[i] = pw.UID
	}
	creds := &ucred.SoPeerCred{
		Authorizer: ucred.NewUidAuthorizer(stadUids...),
		Logger:     lg,
	}
	// The default `grpc.keepalive` parameters allow connections to persist
	// forever.
	srv := grpc.NewServer(grpc.Creds(creds))

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

	sockDir := args["--stad-socket-dir"].(string)
	username, err := nogfsostaudod.GetpwuidName(uint32(os.Getuid()))
	if err != nil {
		lg.Fatalw(
			"Failed to determine process username.",
			"err", err,
		)
	}
	sockPath := filepath.Join(
		sockDir, fmt.Sprintf("udod-%s.sock", username),
	)
	sockMode := os.FileMode(0666)
	_ = os.Remove(sockPath) // Avoid `bind: address already in use`.
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		lg.Fatalw(
			"Failed to listen.",
			"path", sockPath,
			"err", err,
		)
	}
	if err := os.Chmod(sockPath, sockMode); err != nil {
		lg.Fatalw(
			"Failed to chmod socket.",
			"path", sockPath,
			"err", err,
		)
	}

	go func() {
		err := srv.Serve(listener)
		if atomic.LoadInt32(&isShutdown) > 0 {
			return
		}
		lg.Fatalw(
			"gRPC server stopped unexpectedly.",
			"module", "nogfsostaudod",
			"err", err,
		)
	}()
	lg.Infow(
		"gRPC server listening.",
		"path", sockPath,
		"stadUsers", stadUsers,
	)

	sig := <-sigs
	atomic.StoreInt32(&isShutdown, 1)

	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		lg.Infow(
			"Completed gRPC server shutdown.",
			"module", "nogfsostaudod",
		)
		close(done)
	}()

	d := args["--shutdown-timeout"].(time.Duration)
	timeout := time.NewTimer(d)
	lg.Infow(
		"Started graceful shutdown.",
		"module", "nogfsostaudod",
		"sig", sig,
		"timeout", d,
	)
	select {
	case <-timeout.C:
		srv.Stop()
		lg.Warnw(
			"Forced shutdown after timeout.",
			"module", "nogfsostaudod",
		)
	case <-done:
		lg.Infow(
			"Completed graceful shutdown.",
			"module", "nogfsostaudod",
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
			"module", "nogfsostaudod",
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
					"module", "nogfsostaudod",
					"err", err,
				)
			}
			args[k] = d
		}
	}

	for _, k := range []string{
		"--stad-users",
	} {
		if arg, ok := args[k].(string); ok {
			args[k] = strings.Split(arg, ",")
		}
	}

	return args
}

var ErrServerUserLookup = status.Error(
	codes.Internal, "failed to lookup server process user",
)

var ErrNoTerminate = status.Error(
	codes.PermissionDenied, "terminate not allowed via gRPC",
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
	return nil, ErrNoTerminate
}
