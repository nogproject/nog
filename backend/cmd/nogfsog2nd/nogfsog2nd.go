// vim: sw=8

// Server `nogfsog2nd`; see NOE-13.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/internal/nogfsog2nd"
	"github.com/nogproject/nog/backend/internal/nogfsog2nd/broadcast"
	"github.com/nogproject/nog/backend/internal/nogfsog2nd/gitlab"
	"github.com/nogproject/nog/backend/internal/nogfsog2nd/gitnogd"
	"github.com/nogproject/nog/backend/internal/nogfsog2nd/gitnogdstateless"
	"github.com/nogproject/nog/backend/internal/nogfsog2nd/gitnogdwatchlist"
	"github.com/nogproject/nog/backend/internal/nogfsopb"
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
	version  = fmt.Sprintf("nogfsog2nd-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogfsog2nd [options] [--gitlab=<spec>...] --prefix=<path>... <registry>...

Options:
  --bind-grpc=<addr>  [default: 0.0.0.0:7554]
  --tls-cert=<pem>  [default: /nog/ssl/certs/nogfsog2nd/combined.pem]
        TLS certificate and corresponding private key.  PEM files can be
        concatenated ''cat cert.pem privkey.pem > combined.pem''.
  --tls-ca=<pem>  [default: /nog/ssl/certs/nogfsog2nd/ca.pem]
        TLS certificates that are accepted as CA for client certs.  Multiple
        PEM files can be concatenated.
  --nogfsoregd=<addr>  [default: localhost:7550]
  --discovery=<mechanism>  [default: watchlist]
        The mechanism to discover repos.  Available mechanisms:
	''watch'': observe registries to maintain state for known repos.
	''watchlist'': observe registries to maintain list of known repos.
	''stateless': get repo details from nogfsoregd during each request.
  --prefix=<path>
        Limits processing to repos whose global path is below one of the
        prefixes.
  --gitlab=<spec>  [default: localhost:/etc/gitlab/root.token:http://localhost:80]
        GitLab config ''<name>:<token-path>:<base-url>''.
  --shutdown-timeout=<duration>  [default: 20s]
        Maximum time to wait before forced shutdown.
  --log=<logger>  [default: prod]
        Specifies logger: prod, dev, or mu.
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

	var gitlabs []*gitlab.Client
	var gitlabHostnames []string
	for _, a := range args["--gitlab"].([]GitlabArg) {
		gl, err := gitlab.New(gitlab.Config(a))
		if err != nil {
			lg.Fatalw(
				"Failed to create GitLab client.", "err", err,
			)
		}
		gitlabs = append(gitlabs, gl)
		gitlabHostnames = append(gitlabHostnames, gl.Hostname)
	}

	lg.Infow("nogfsog2nd started.")

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

	var wg2 sync.WaitGroup
	ctx2, cancel2 := context.WithCancel(context.Background())

	gsrv := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    ca,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	})))

	broadcaster := broadcast.NewBroadcaster(conn)

	switch args["--discovery"] {
	case "stateless":
		nogfsopb.RegisterGitNogServer(gsrv, gitnogdstateless.New(
			ctx2, lg, conn, gitlabs,
			args["--prefix"].([]string),
			broadcaster,
		))
		lg.Infow("Enabled stateless repo discovery.")

	case "watch":
		view := nogfsog2nd.NewRegistryView(
			lg, &nogfsog2nd.RegistryViewConfig{
				Registries: args["<registry>"].([]string),
				Prefixes:   args["--prefix"].([]string),
				Gitlabs:    gitlabHostnames,
			},
		)
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := view.Watch(ctx, conn)
			if err != context.Canceled {
				lg.Fatalw("Watch failed.", "err", err)
			}
			if atomic.LoadInt32(&isShutdown) == 0 {
				lg.Fatalw("Unexpected watch cancel.")
			}
		}()
		nogfsopb.RegisterGitNogServer(gsrv, gitnogd.New(
			ctx2, lg, view, gitlabs, broadcaster,
		))
		lg.Infow("Started watching registries to track repo details.")

	case "watchlist":
		gitnogd := gitnogdwatchlist.New(
			lg, conn, &gitnogdwatchlist.Config{
				Registries:  args["<registry>"].([]string),
				Prefixes:    args["--prefix"].([]string),
				Gitlabs:     gitlabs,
				Broadcaster: broadcaster,
			},
		)
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := gitnogd.Watch(ctx)
			if err != context.Canceled {
				lg.Fatalw("Watch failed.", "err", err)
			}
			if atomic.LoadInt32(&isShutdown) == 0 {
				lg.Fatalw("Unexpected watch cancel.")
			}
		}()
		nogfsopb.RegisterGitNogServer(gsrv, gitnogd)
		lg.Infow("Started watching registries to track list of repos.")
	}

	addrType := "tcp"
	addr := args["--bind-grpc"].(string)
	if strings.HasPrefix(addr, "/") {
		addrType = "unix"
		_ = os.Remove(addr)
	}
	lis, err := net.Listen(addrType, addr)
	if err != nil {
		lg.Fatalw("Listen failed.", "family", addrType, "addr", addr)
	}

	wg2.Add(1)
	go func() {
		err := gsrv.Serve(lis)
		if atomic.LoadInt32(&isShutdown) > 0 {
			wg2.Done()
			return
		}
		lg.Fatalw("gsrv error.", "err", err)
	}()
	lg.Infow("Listening.", "family", addrType, "addr", addr)

	sig := <-sigs
	atomic.StoreInt32(&isShutdown, 1)

	done := make(chan struct{})
	go func() {
		cancel2()
		gsrv.GracefulStop()
		wg2.Wait()
		lg.Infow("Completed level 2 shutdown.")

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
		gsrv.Stop()
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

	if d, err := time.ParseDuration(
		args["--shutdown-timeout"].(string),
	); err != nil {
		lg.Fatalw("Invalid --shutdown-timeout.", "err", err)
	} else {
		args["--shutdown-timeout"] = d
	}

	var gitlabArgs []GitlabArg
	for _, a := range args["--gitlab"].([]string) {
		p, err := parseGitlabArg(a)
		if err != nil {
			lg.Fatalw("Invalid --gitlab.", "err", err)
		}
		gitlabArgs = append(gitlabArgs, p)
	}
	args["--gitlab"] = gitlabArgs

	switch args["--discovery"].(string) {
	case "stateless":
	case "watch":
	case "watchlist":
	default:
		lg.Fatalw("Invalid --discovery.")
	}

	return args
}

type GitlabArg struct {
	Hostname  string
	BaseUrl   string
	TokenPath string
}

func parseGitlabArg(a string) (GitlabArg, error) {
	fields := strings.SplitN(a, ":", 3)
	if len(fields) != 3 {
		err := fmt.Errorf(
			"failed to split `%s` into <host>:<token>:<url>", a,
		)
		return GitlabArg{}, err
	}
	return GitlabArg{
		Hostname:  fields[0],
		BaseUrl:   fields[2],
		TokenPath: fields[1],
	}, nil
}
