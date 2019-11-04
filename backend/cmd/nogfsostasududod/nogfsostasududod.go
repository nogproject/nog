// vim: sw=8

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/pkg/mulog"
	"github.com/nogproject/nog/backend/pkg/zap"
)

// `xVersion` and `xBuild` are injected by the `Makefile`.
var (
	xVersion string
	xBuild   string
	version  = fmt.Sprintf("nogfsostasududod-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogfsostasududod [options] [--sududod-socket=<path>] [--stad-uids=<uids>]

Options:
  --log=<logger>  [default: prod]
        Specify logger: prod, dev, or mu.
  --shutdown-timeout=<duration>  [default: 20s]
        Maximum time to wait before forced shutdown.
  --sududod-socket=<path> [default: /nogfso/var/run/nogfsostad/sududod/sock]
        Directory for the Unix socket that ''nogfsostad'' will connect to.
  --stad-uids=<uids>
        ''nogfsostad'' UIDs that are allowed to connect; comma-separated list.
  --stad-gids=<gids>
        ''nogfsostad'' GIDs that are allowed to connect; comma-separated list.
	If neither ''--stad-uids'' nor ''--stad-gids'' is specified, the access
	check is disabled.  If both ''--stad-uids'' and ''--stad-gids'' are
	specified, permission is granted if either the UID or the GID matches.

''nogfsostasududod'' listens on the Unix domain socket ''--sududo-socket'' for
requests by ''nogfsostad'' to start ''nogfsostaudod-fd'' via Sudo.  If
''nogfsostad'' asks to run the daemon as user ''root'', ''nogfsostasududod''
starts a special variant ''nogfsostasuod-fd''.

Assuming ''nogfsostasududod'' runs as user ''daemon'', for example, the
following ''sudoers'' configuration is required to run ''nogfsostaudod-fd'' as
a direct child process, that is ''sudo'' execs ''nogfsostaudod-fd'', as user
''alice'' or users in group ''ag_bob'', for example:

    Defaults:daemon closefrom_override, !pam_session, !pam_setcred
    daemon ALL=(alice) NOPASSWD: /usr/local/bin/nogfsostaudod-fd
    daemon ALL=(%ag_bob) NOPASSWD: /usr/local/bin/nogfsostaudod-fd

To run ''nogfsostasuod-fd'' as user ''root'', the following additional
''sudoers'' configuration is required:

    daemon ALL=(root) NOPASSWD: /usr/local/bin/nogfsostasuod-fd

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

	var auth Auther
	uids, _ := args["--stad-uids"].([]uint32)
	gids, _ := args["--stad-gids"].([]uint32)
	if len(uids) > 0 || len(gids) > 0 {
		auth = &AnyUnixCredsAuther{
			Lg:   lg,
			UIDs: uids,
			GIDs: gids,
		}
		lg.Infow(
			"Enabled SO_PEERCRED auth.",
			"uids", uids,
			"gids", gids,
		)
	} else {
		lg.Infow("Disabled SO_PEERCRED auth.")
	}

	srv, err := NewServer(lg, auth, xVersion)
	if err != nil {
		lg.Fatalw(
			"Failed to create server.",
			"err", err,
		)
	}

	sockPath := args["--sududod-socket"].(string)
	sockMode := os.FileMode(0666)
	_ = os.Remove(sockPath) // Avoid `bind: address already in use`.
	lis, err := net.ListenUnix("unix", &net.UnixAddr{
		Net:  "unix",
		Name: sockPath,
	})
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

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		err := srv.Serve(ctx, lis)
		switch {
		case err != context.Canceled:
			lg.Fatalw(
				"Unexpected server error.",
				"err", err,
			)
		case atomic.LoadInt32(&isShutdown) > 0:
			wg.Done()
		default:
			lg.Fatalw("Unexpected server cancel.")
		}

	}()

	lg.Infow(
		"Server listening.",
		"path", sockPath,
	)

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
	lg.Infow(
		"Started graceful shutdown.",
		"sig", sig,
		"timeout", d,
	)
	select {
	case <-timeout.C:
		lg.Warnw("Forced shutdown after timeout.")
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
		lg.Fatalw(
			"docopt failed",
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
					"err", err,
				)
			}
			args[k] = d
		}
	}

	for _, k := range []string{
		"--stad-uids",
		"--stad-gids",
	} {
		if arg, ok := args[k].(string); ok {
			var uids []uint32
			for _, s := range strings.Split(arg, ",") {
				v, err := strconv.ParseUint(s, 10, 32)
				if err != nil {
					lg.Fatalw(
						"Invalid uint.",
						"arg", k,
						"err", err,
					)
				}
				uids = append(uids, uint32(v))
			}
			args[k] = uids
		}
	}

	return args
}
