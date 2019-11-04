// Server `nogfsostasvsd`.
//
// Test with:
//
// First terminal:
//
// ```
// ddev nogfsostasvsd --userspec=daemon --group-prefix=org_ --group-prefix=srv_ -- bash -c 'trap "exit 0" TERM INT; id; while read; do :; done'
// ```
//
// Second terminal:
//
// ```
// docker ps
// cid=1c5...
//
// docker exec ${cid} addgroup --system org_bar
// docker exec ${cid} addgroup --system org_foo
// ```
//
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	docopt "github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/pkg/getent"
	"github.com/nogproject/nog/backend/pkg/mulog"
	"github.com/nogproject/nog/backend/pkg/zap"
)

// `xVersion` and `xBuild` are injected by the `Makefile`.
var (
	xVersion string
	xBuild   string
	version  = fmt.Sprintf("nogfsostasvsd-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogfsostasvsd [options] --userspec=<spec> --group-prefix=<prefix>... -- <cmd>...

Options:
  --shutdown-timeout=<duration>  [default: 20s]
        Maximum time to wait before forced shutdown.
  --log=<logger>  [default: prod]
        Specify logger: prod, dev, or mu.
  --userspec=<spec>
        The ''--userspec'' for ''chroot''.
  --group-prefix=<prefix>
        Prefix to select groups from ''getent group''

''nogfsostasvsd'' runs a command with a list of numeric supplementary groups.
Specifically, it runs ''chroot --userspec=<spec> --groups=<gids> / <cmd>...'',
where ''<gids>'' is a list of GIDs for the groups whose names start with one of
the ''--group-prefix'' arguments.  ''nogfsostasvsd'' checks the GIDs at regular
intervals and restarts the command when the GIDs change.
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

	lg.Infow("nogfsostasvsd started.")

	userspec := args["--userspec"].(string)
	prefixes := args["--group-prefix"].([]string)
	cmd := args["<cmd>"].([]string)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	signal.Notify(sigs, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	childCh := make(chan int)

	go func() {
		childCh <- runChild(ctx, userspec, prefixes, cmd)
	}()

	select {
	case code := <-childCh:
		lg.Infow(
			"Child exit.",
			"code", code,
		)
		os.Exit(code)

	case sig := <-sigs:
		cancel()

		d := args["--shutdown-timeout"].(time.Duration)
		timeout := time.NewTimer(d)
		lg.Infow(
			"Started graceful shutdown.",
			"sig", sig,
			"timeout", d,
		)

		select {
		case code := <-childCh:
			lg.Infow(
				"Completed graceful shutdown with child exit.",
				"code", code,
			)
			os.Exit(code)
		case <-timeout.C:
			lg.Fatalw("Timeout; forced shutdown.")
		}
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

func runChild(ctx context.Context, userspec string, prefixes, cmd []string) int {
	gids, err := getGIDs(ctx, prefixes)
	if err != nil {
		lg.Warnw("Failed to get initial GIDs.", "err", err)
		gids = nil // Begin without supplementary groups.
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		lg.Infow(
			"Starting child.",
			"gids", gids,
		)
		args := make([]string, 0, len(cmd)+3)
		args = append(args, fmt.Sprintf("--userspec=%s", userspec))
		if len(gids) > 0 {
			args = append(args,
				fmt.Sprintf("--groups=%s", gids.String()),
			)
		}
		args = append(args, "/")
		args = append(args, cmd...)
		cmd := exec.Command("chroot", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			lg.Fatalw("Failed to start child.", "err", err)
		}

		childCh := make(chan int)
		go func() {
			err := cmd.Wait()
			if err == nil {
				childCh <- 0
			} else if err2, ok := err.(*exec.ExitError); ok {
				childCh <- err2.ExitCode()
			} else {
				lg.Errorw(
					"Unexpected child exec error.",
					"err", err,
				)
				childCh <- 1
			}
		}()

	CheckGids:
		for {
			select {
			case code := <-childCh:
				return code

			case <-ctx.Done():
				cmd.Process.Signal(syscall.SIGTERM)
				return <-childCh

			case <-ticker.C:
				newGids, err := getGIDs(ctx, prefixes)
				if err != nil {
					lg.Warnw(
						"Failed to get GIDs.",
						"err", err,
					)
					continue
				}

				if newGids.Equal(gids) {
					continue CheckGids
				}

				lg.Infow("GIDs changed; restarting child.")

				cmd.Process.Signal(syscall.SIGTERM)
				code := <-childCh
				if code != 0 {
					lg.Warnw("Child exit != 0 on SIGTERM.")
					return code
				}

				gids = newGids
				break CheckGids // Restart child with new GIDs.
			}
		}
	}
}

type GIDs []uint32

func getGIDs(ctx context.Context, prefixes []string) (GIDs, error) {
	groups, err := getent.Groups(ctx)
	if err != nil {
		return nil, err
	}
	groups = getent.SelectGroups(groups, prefixes)
	groups, err = getent.DedupGroups(groups)
	if err != nil {
		return nil, err
	}

	gids := make(GIDs, 0, len(groups))
	for _, g := range groups {
		gids = append(gids, g.Gid)
	}
	sort.Slice(gids, func(i, j int) bool { return gids[i] < gids[j] })

	return gids, nil
}

func (a GIDs) Equal(b GIDs) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (gids GIDs) String() string {
	bs := make([]string, 0, len(gids))
	for _, g := range gids {
		bs = append(bs,
			strconv.FormatUint(uint64(g), 10),
		)
	}
	return strings.Join(bs, ",")
}
