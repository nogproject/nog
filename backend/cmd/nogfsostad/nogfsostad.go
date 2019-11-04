// vim: sw=8

// Server `nogfsostad`; see NOE-13.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/internal/fsoauthz"
	"github.com/nogproject/nog/backend/internal/grpcjwt"
	"github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad"
	"github.com/nogproject/nog/backend/internal/nogfsostad/acls"
	"github.com/nogproject/nog/backend/internal/nogfsostad/discoveryd"
	"github.com/nogproject/nog/backend/internal/nogfsostad/gits"
	"github.com/nogproject/nog/backend/internal/nogfsostad/observer6"
	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/daemons"
	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/dialsududod"
	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/dialudod"
	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/sudoudod"
	"github.com/nogproject/nog/backend/internal/nogfsostad/shadows"
	"github.com/nogproject/nog/backend/internal/nogfsostad/statd"
	"github.com/nogproject/nog/backend/internal/nogfsostad/tarttd"
	"github.com/nogproject/nog/backend/internal/nogfsostad/testudod"
	"github.com/nogproject/nog/backend/internal/nogfsostad/workflowproc"
	"github.com/nogproject/nog/backend/pkg/mulog"
	"github.com/nogproject/nog/backend/pkg/regexpx"
	"github.com/nogproject/nog/backend/pkg/unixauth"
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
	version  = fmt.Sprintf("nogfsostad-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogfsostad [options] [--prefix-init-limit=<limit>...]
                       [--repo-init-limit=<limit>...]
                       [--trim-host-root=<path>] [--shadow-root=<path>]
                       [--shadow-root-alt=<path>...]
                       --host=<host>... --prefix=<path>... <registry>...

Options:
  --tls-cert=<pem>  [default: /nog/ssl/certs/nogfsostad/combined.pem]
        TLS certificate and corresponding private key, which is used when
        connecting as a client to ''nogfsoregd'' and when listening as a
        server with ''--bind-grpc''.  PEM files can be concatenated like
        ''cat cert.pem privkey.pem > combined.pem''.
  --tls-ca=<pem>  [default: /nog/ssl/certs/nogfsostad/ca.pem]
        Certificates that are accepted as CA for TLS connections.  Multiple
        PEM files can be concatenated.
  --jwt-ca=<pem>  [default: /nog/ssl/certs/nogfsostad/ca.pem]
        X.509 CA for JWTs.  Multiple PEM files can be concatenated.
  --jwt-ou=<ou>  [default: nogfsoiam]
        Required OU of JWT signing key X.509 Subject.
  --sys-jwt=<path>  [default: /nog/jwt/tokens/nogfsostad.jwt]
        Path to JWT that is used for system gRPCs.
  --jwt-unix-domain=<domain>
        The domain that is expected in a JWT ''xcrd'' claim.  If unset,
        services that require a local Unix user will be disabled.
  --udod-socket-dir=<dir>
        If set, ''nogfsostad'' will connect to ''nogfsostaudod-path'' daemons
        via Unix domain sockets in the specified directory, instead of using
        ''sudo'' to start ''nogfsostaudod-fd'' processes when needed.
  --sududod-socket=<path>
        If set, ''nogfsostad'' will connect to ''nogfsostasududod'' via a Unix
        domain socket at the specified path to start ''nogfsostaudod-fd''
        processes when needed, instead of starting them directly via ''sudo''.
  --session-name=<hostname>  [default: localhost]
        The hostname used during TLS handshake when establishing a callback
        session from ''nogfsoregd''.  The name must be an X.509 Subject
        Alternative Name of ''--tls-cert''.
  --nogfsoregd=<addr>  [default: localhost:7550]
  --shutdown-timeout=<duration>  [default: 20s]
        Time to wait after receiving a shutdown signal to give clients a chance
        to gracefully disconnect and background tasks a chance to gracefully
        quit before forcing exit.  The shutdown signals are SIGTERM and SIGINT.
  --prefix=<path>
        Limits processing to repos whose global paths are below one of the
        prefixes.
  --host=<host>
        Repos that pass the prefix filter must be on one of the hosts.
  --trim-host-root=<path>
        Directory to remove from the start of a host path when mapping it to a
        shadow path.
  --shadow-root=<path>  [default: /nogfso/shadow]
        Directory for new shadow repositories.
  --shadow-root-alt=<path>
        Alternative shadow root directories.  ''nogfsostad'' initializes new
        shadow repositories below ''--shadow-root''.  But it accepts existing
        repositories also below alternative shadow roots.
  --archive-repo-spool=<path>
        Spool directory for archive repo processing.  The directory must exist,
        it must be writable by ''nogfsostad'', and it must be on the same
        filesystem as the realdirs, so that ''rename()'' can be used to swap
        placeholders and realdirs.
  --unarchive-repo-spool=<path>
        Spool directory for unarchive repo processing.  The directory must
        exist, it must be writable by ''nogfsostad'', and it must be on the
        same filesystem as the realdirs, so that ''rename()'' can be used to
        swap placeholders and realdirs.
  --git-fso-program=<path>  [default: /go/src/github.com/nogproject/nog/backend/bin/git-fso]
  --gitlab=<addr>  [default: http://localhost:80]
        Use ''no'' to disable publishing shadow repos to GitLab.
  --gitlab-token=<path>  [default: /etc/gitlab/root.token]
  --log=<logger>  [default: prod]
        Specifies the logger: ''prod'', ''dev'', or ''mu''.
  --observer=<version>  [default: v6]
        The implementation for monitoring the registry:
          * ''v6'': the default since 2018-12.
	Older versions have been removed:
          * ''v5'' (removed): an experiment in 2018-11.
          * ''v4'' (removed): the default from 2018-07 to 2018-11.
          * ''v3'' (removed): an experiment in 2018-07.
          * ''v2'' (removed): an experiment in 2018-07.
          * ''v1'' (removed): was used until 2018-07.
        See source Git history for details.
  --init-limit-max-files=<count>  [default: 2k]
        Limits number of files during repo init.  If the host path contains
        more files, init will be refused.  Suffixes ''k'', ''m'', ''g'', ''t''.
  --init-limit-max-size=<size>  [default: 5G]
        Limits data size during repo init.  If the host path contains more
        data, init will be refused.  Suffixes ''k'', ''m'', ''g'', ''t''.
  --prefix-init-limit=<limit>
        Init limits for repos below a prefix, overriding the global limits.
        The prefix must be a directory.  ''<limit>'' is specified as
        ''<global-path-prefix>:<max-files>:<max-size>''.  The option can be
        repeated to specify limits for multiple prefixes.
  --repo-init-limit=<limit>
        Per-repo limits, overriding prefix and global limits.  ''<limit>''
        is specified as ''<global-path>:<max-files>:<max-size>''.  The option
        can be repeated to specify limits for multiple repos.
  --bind-grpc=<addr>
        Enables a gRPC server on ''<addr>'', which may be useful for debugging.
        The recommended address is ''0.0.0.0:7552''.
  --git-gc-scan-start=<wait-duration>  [default: 20m]
        Enables ''git gc'' on the shadow repos at startup after a wait
        duration.  Use ''0'' to disable.
  --git-gc-scan-every=<interval>  [default: 240h]
        Enables ''git gc'' at regular intervals in the background on the shadow
        repos.  Use ''0'' to disable.
  --stat-author=<author>
        Git author for background stat commits.
        Example: ''A U Thor <author@example.org>''.
  --git-committer=<author>
        Git committer for shadow repo commits.
        Example: ''nogfsostad <nogfsostad@example.org>''.
  --stat-scan-start=<wait-duration>  [default: 10m]
        Enables ''git-fso stat --mtime-range-only'' on all repos at startup
        after a wait duration.  Use ''0'' to disable.
  --stat-scan-every=<interval>  [default: 24h]
        Enables ''git-fso stat --mtime-range-only'' on all repos at regular
        intervals in the background.  Use ''0'' to disable.
  --stdtools-projects-root=<path>
        Host path to Stdtools projects root.
`)

var (
	clientAliveInterval             = 40 * time.Second
	clientAliveWithoutStream        = true
	enforceMinAliveInterval         = 30 * time.Second
	enforcePermitAliveWithoutStream = true
	serverAliveInterval             = 40 * time.Second
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

	jwtCa, err := x509io.LoadCABundle(args["--jwt-ca"].(string))
	if err != nil {
		lg.Fatalw("Failed to load --jwt-ca.", "err", err)
	}
	authn := grpcjwt.NewRSAAuthn(jwtCa, args["--jwt-ou"].(string))
	var domain string
	var authnUnix *unixauth.UserAuthn
	if arg, ok := args["--jwt-unix-domain"].(string); ok {
		domain = arg
		authnUnix = &unixauth.UserAuthn{
			ContextAuthenticator: authn,
			Domain:               arg,
		}
	}
	authz := fsoauthz.CreateScopeAuthz(lg)
	sysRPCCreds, err := grpcjwt.Load(args["--sys-jwt"].(string))
	if err != nil {
		lg.Fatalw("Failed to load --sys-jwt", "err", err)
	}

	lg.Infow("nogfsostad started.")

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

	cfg := shadows.Config{
		ShadowRoot:             args["--shadow-root"].(string),
		ShadowRootAlternatives: args["--shadow-root-alt"].([]string),
		GitFsoProgram:          args["--git-fso-program"].(string),
	}
	if arg, ok := args["--trim-host-root"].(string); ok {
		cfg.TrimHostRoot = arg
	}
	if arg, ok := args["--git-committer"].(statd.User); ok {
		cfg.GitCommitter = shadows.User(arg)
	}
	shadow, err := shadows.New(lg, cfg)
	if err != nil {
		lg.Fatalw(
			"Failed to create shadow module.",
			"err", err,
		)
	}

	var gitlab *gits.Gitlab
	if args["--gitlab"] == nil {
		lg.Infow("GitLab disabled.")
	} else {
		gl, err := gits.NewGitlab(&gits.GitlabConfig{
			Addr:      args["--gitlab"].(string),
			TokenPath: args["--gitlab-token"].(string),
		})
		if err != nil {
			lg.Fatalw(
				"Failed to create GitLab module.",
				"err", err,
			)
		}
		gitlab = gl
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	daemons := daemons.New(lg)
	wg.Add(1)
	go func() {
		err := daemons.Run(ctx)
		if err != context.Canceled {
			lg.Fatalw("daemons.Run() failed.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected daemons.Run() cancel.")
		}
		wg.Done()
	}()

	var nogfsostadPrivileges nogfsostad.Privileges
	var testudodPrivileges testudod.Privileges
	var aclsPrivileges acls.UdoBashPrivileges
	var workflowprocPrivileges workflowproc.Privileges
	if sock, ok := args["--sududod-socket"].(string); ok {
		lg.Infow(
			"Using nogfsostaudo-fd via nogfsostasududod.",
			"socket", sock,
		)
		privs := dialsududod.New(daemons, sock)
		nogfsostadPrivileges = privs
		testudodPrivileges = privs
		aclsPrivileges = privs
		workflowprocPrivileges = privs
	} else if sockDir, ok := args["--udod-socket-dir"].(string); ok {
		lg.Infow(
			"Using nogfsostaudod-path.",
			"socketDir", sockDir,
		)
		privs := dialudod.New(daemons, sockDir)
		nogfsostadPrivileges = privs
		testudodPrivileges = privs
		aclsPrivileges = privs
		workflowprocPrivileges = privs
	} else {
		lg.Infow(
			"Using Sudo nogfsostaudod-fd.",
		)
		privs := sudoudod.New(daemons)
		nogfsostadPrivileges = privs
		testudodPrivileges = privs
		aclsPrivileges = privs
		workflowprocPrivileges = privs
	}

	initLimits := nogfsostad.NewInitLimits(&nogfsostad.InitLimitsConfig{
		MaxFiles:     args["--init-limit-max-files"].(uint64),
		MaxBytes:     args["--init-limit-max-size"].(uint64),
		PrefixLimits: args["--prefix-init-limit"].([]nogfsostad.PathInitLimit),
		RepoLimits:   args["--repo-init-limit"].([]nogfsostad.PathInitLimit),
	})
	useUdo := nogfsostad.UseUdo{
		// archive-repo and unarchive-repo use udo(root) for rename.
		// The configuration is currently hard-coded.  We would expose
		// it as a command argument if we wanted to use direct rename
		// in the future.
		Rename: true,
	}
	broadcaster := nogfsostad.NewBroadcaster(lg, conn, sysRPCCreds)
	proc := nogfsostad.NewProcessor(
		lg, initLimits, shadow, broadcaster,
		nogfsostadPrivileges, useUdo,
	)

	switch args["--observer"] {
	case "v6":
		lg.Infow("Started observer v6.")
		// Observer v6 uses initializer v4!
		initializer4 := nogfsostad.NewRepoInitializer4(
			lg,
			proc,
			conn, sysRPCCreds,
			args["--host"].([]string),
			shadow, broadcaster, gitlab,
		)
		obs6 := observer6.New(lg, &observer6.Config{
			Registries:  args["<registry>"].([]string),
			Prefixes:    args["--prefix"].([]string),
			Conn:        conn,
			SysRPCCreds: sysRPCCreds,
			Initializer: initializer4,
			Processor:   proc,
		})
		wg.Add(1)
		go func() {
			err := obs6.Watch(ctx)
			if err != context.Canceled {
				lg.Fatalw(
					"Observer quit with unexpected error.",
					"err", err,
				)
			}
			if atomic.LoadInt32(&isShutdown) == 0 {
				lg.Fatalw("Unexpected observer shutdown.")
			}
			wg.Done()
		}()
	}

	archiveRepoSpool := ""
	unarchiveRepoSpool := ""
	if arg, ok := args["--archive-repo-spool"].(string); ok {
		archiveRepoSpool = arg
		lg.Infow(
			"Enabled archive-repo spool dir.",
			"archiveRepoSpool", archiveRepoSpool,
		)
	}
	if arg, ok := args["--unarchive-repo-spool"].(string); ok {
		unarchiveRepoSpool = arg
		lg.Infow(
			"Enabled unarchive-repo spool dir.",
			"unarchiveRepoSpool", unarchiveRepoSpool,
		)
	}
	aclPropagator := acls.NewUdoBash(aclsPrivileges)
	workflowProc := workflowproc.New(lg, &workflowproc.Config{
		Registries:         args["<registry>"].([]string),
		Prefixes:           args["--prefix"].([]string),
		Conn:               conn,
		SysRPCCreds:        sysRPCCreds,
		RepoProcessor:      proc,
		Privileges:         workflowprocPrivileges,
		AclPropagator:      aclPropagator,
		ArchiveRepoSpool:   archiveRepoSpool,
		UnarchiveRepoSpool: unarchiveRepoSpool,
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

	stasrv := nogfsostad.NewStatServer(lg, authn, authz, proc, sysRPCCreds)

	// DEPRECATED: See comment at `listener` below.
	gsrv := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    ca,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		})),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             enforceMinAliveInterval,
			PermitWithoutStream: enforcePermitAliveWithoutStream,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time: serverAliveInterval,
		}),
	)
	nogfsopb.RegisterStatServer(gsrv, stasrv)
	wg.Add(1)
	go func() {
		err := stasrv.Process(ctx, conn)
		if err != context.Canceled {
			lg.Fatalw("Statd process failed.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected statd process cancel.")
		}
		wg.Done()
	}()

	// Don't register with `gsrv` but only with session.
	gitnogd := nogfsostad.NewGitNogServer(lg, authn, authz, proc)
	tarttd := tarttd.New(lg, authn, authz, proc)

	var testUdoD nogfsopb.TestUdoServer
	if authnUnix == nil || testudodPrivileges == nil {
		lg.Infow("Disabled nogfsostaudod.")
	} else {
		testUdoD = testudod.New(
			lg,
			authn, domain, authnUnix, authz,
			proc, testudodPrivileges,
		)
	}

	stdtoolsProjectsRoot, ok := args["--stdtools-projects-root"].(string)
	if ok {
		lg.Infow(
			"Enabled Stdtools project discovery.",
			"projectsRoot", stdtoolsProjectsRoot,
		)
	} else {
		lg.Infow("Disabled Stdtools project discovery.")
	}
	discoveryd := discoveryd.New(lg, conn, &discoveryd.Config{
		Authenticator:        authn,
		Authorizer:           authz,
		SysRPCCreds:          sysRPCCreds,
		Registries:           args["<registry>"].([]string),
		Prefixes:             args["--prefix"].([]string),
		Hosts:                args["--host"].([]string),
		StdtoolsProjectsRoot: stdtoolsProjectsRoot,
	})
	wg.Add(1)
	go func() {
		err := discoveryd.Watch(ctx)
		if err != context.Canceled {
			lg.Fatalw("discoveryd.Watch() failed.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected discoveryd.Watch() cancel.")
		}
		wg.Done()
	}()

	// DEPRECATED: The gRPC listener is disabled by default.  All gRPCs use
	// reverse gRPC via `nogfsoregd`.  The listener may be useful for
	// debugging, so we keep it for now.  But we could remove it at any
	// time.
	var listener net.Listener
	if addr, ok := args["--bind-grpc"].(string); ok {
		addrType := "tcp"
		if strings.HasPrefix(addr, "/") {
			addrType = "unix"
			_ = os.Remove(addr)
		}
		lis, err := net.Listen(addrType, addr)
		if err != nil {
			lg.Fatalw("Listen failed.", "family", addrType, "addr", addr)
		}
		lg.Infow("gRPC listening.", "family", addrType, "addr", addr)
		listener = lis
	} else {
		lg.Infow("gRPC listening disabled.")
	}

	var wg2 sync.WaitGroup
	ctx2, cancel2 := context.WithCancel(context.Background())

	sessionCfg := &nogfsostad.SessionConfig{
		Prefixes:    args["--prefix"].([]string),
		Hosts:       args["--host"].([]string),
		InitLimits:  initLimits,
		SessionName: args["--session-name"].(string),
		TransportCredentials: credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    ca,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}),
		Authenticator: authn,
		Authorizer:    authz,
		SysRPCCreds:   sysRPCCreds,
	}
	session := nogfsostad.NewSession(
		lg,
		stasrv, gitnogd, gitnogd, discoveryd, tarttd, testUdoD,
		sessionCfg,
	)
	wg2.Add(1)
	go func() {
		err := session.Process(ctx2, conn)
		if err != context.Canceled {
			lg.Fatalw("Session process failed.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected session process cancel.")
		}
		wg2.Done()
	}()

	if listener != nil {
		wg2.Add(1)
		go func() {
			err := gsrv.Serve(listener)
			if atomic.LoadInt32(&isShutdown) > 0 {
				wg2.Done()
				return
			}
			log.Fatal(err)
		}()
	}

	startGitGcScans(args, &wg2, ctx2, proc)
	startStatScans(args, &wg2, ctx2, proc)

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

func startGitGcScans(
	args map[string]interface{},
	wg *sync.WaitGroup,
	ctx context.Context,
	proc *nogfsostad.Processor,
) {
	start, startYes := args["--git-gc-scan-start"].(time.Duration)
	if startYes && start == 0 {
		startYes = false
	}
	every, everyYes := args["--git-gc-scan-every"].(time.Duration)
	if everyYes && every == 0 {
		everyYes = false
	}
	if !startYes && !everyYes {
		return
	}
	switch {
	case startYes && everyYes:
		lg.Infow(
			"Enabled initial and regular git gc scans.",
			"start", start,
			"every", every,
		)
	case startYes:
		lg.Infow(
			"Enabled initial git gc scan.",
			"start", start,
		)
	case everyYes:
		lg.Infow(
			"Enabled regular git gc scans.",
			"every", every,
		)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		gitGcScan(ctx, proc, start, every)
	}()
}

func gitGcScan(
	ctx context.Context,
	proc *nogfsostad.Processor,
	scanStart, scanEvery time.Duration,
) {
	if scanStart != 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.NewTimer(scanStart).C:
		}
		lg.Infow("Started initial git gc scan.")
		err := proc.GitGcAll(ctx)
		if err == context.Canceled {
			return
		}
		if err != nil {
			lg.Warnw("Initial git gc scan failed.", "err", err)
		} else {
			lg.Infow("Completed initial git gc scan.")
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
			lg.Infow("Started regular git gc scan.")
			err := proc.GitGcAll(ctx)
			if err == context.Canceled {
				continue
			}
			if err != nil {
				lg.Warnw(
					"Regular git gc scan failed.",
					"err", err,
				)
			} else {
				lg.Infow("Completed regular git gc scan.")
			}
		}
	}
}

func startStatScans(
	args map[string]interface{},
	wg *sync.WaitGroup,
	ctx context.Context,
	proc *nogfsostad.Processor,
) {
	start, startYes := args["--stat-scan-start"].(time.Duration)
	if startYes && start == 0 {
		startYes = false
	}
	every, everyYes := args["--stat-scan-every"].(time.Duration)
	if everyYes && every == 0 {
		everyYes = false
	}
	if !startYes && !everyYes {
		return
	}
	author, ok := args["--stat-author"].(statd.User)
	if !ok {
		lg.Warnw("Stat scans disabled: missing --stat-author.")
		return
	}
	switch {
	case startYes && everyYes:
		lg.Infow(
			"Enabled initial and regular stat scans.",
			"start", start,
			"every", every,
		)
	case startYes:
		lg.Infow(
			"Enabled initial stat scan.",
			"start", start,
		)
	case everyYes:
		lg.Infow(
			"Enabled regular stat scans.",
			"every", every,
		)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		statScan(ctx, proc, start, every, author)
	}()
}

func statScan(
	ctx context.Context,
	proc *nogfsostad.Processor,
	scanStart, scanEvery time.Duration,
	author statd.User,
) {
	if scanStart != 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.NewTimer(scanStart).C:
		}
		lg.Infow("Started initial stat scan.")
		err := proc.StatMtimeRangeOnlyAllRepos(ctx, author)
		if err == context.Canceled {
			return
		}
		if err != nil {
			lg.Warnw("Initial stat scan failed.", "err", err)
		} else {
			lg.Infow("Completed initial stat scan.")
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
			lg.Infow("Started regular stat scan.")
			err := proc.StatMtimeRangeOnlyAllRepos(ctx, author)
			if err == context.Canceled {
				continue
			}
			if err != nil {
				lg.Warnw(
					"Regular stat scan failed.",
					"err", err,
				)
			} else {
				lg.Infow("Completed regular stat scan.")
			}
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
		"--git-gc-scan-start",
		"--git-gc-scan-every",
		"--stat-scan-start",
		"--stat-scan-every",
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
		"--init-limit-max-files",
		"--init-limit-max-size",
	} {
		if v, err := parseUint64Si(args[k].(string)); err != nil {
			msg := fmt.Sprintf("Invalid %s.", k)
			lg.Fatalw(msg, "err", err)
		} else {
			args[k] = v
		}
	}

	for _, k := range []string{
		"--stat-author",
		"--git-committer",
	} {
		if arg, ok := args[k].(string); ok {
			name, email, err := parseUser(arg)
			if err != nil {
				lg.Fatalw(
					fmt.Sprintf("Invalid %s", k),
					"err", err,
				)
			}
			args[k] = statd.User{
				Name:  name,
				Email: email,
			}
		}
	}

	if args["--repo-init-limit"], err = parsePathInitLimits(
		args["--repo-init-limit"].([]string),
	); err != nil {
		lg.Fatalw("Failed to parse --repo-init-limit.", "err", err)
	}

	if args["--prefix-init-limit"], err = parsePathInitLimits(
		args["--prefix-init-limit"].([]string),
	); err != nil {
		lg.Fatalw("Failed to parse --prefix-init-limit.", "err", err)
	}

	if args["--gitlab"].(string) == "no" {
		args["--gitlab"] = nil
	}

	if arg := args["--observer"].(string); !isValidObserverVersion(arg) {
		lg.Fatalw("Invalid --observer.")
	}

	return args
}

func isValidObserverVersion(v string) bool {
	return v == "v6"
}

func parsePathInitLimits(args []string) ([]nogfsostad.PathInitLimit, error) {
	limits := make([]nogfsostad.PathInitLimit, 0)
	for _, arg := range args {
		lim, err := parsePathInitLimit(arg)
		if err != nil {
			return nil, err
		}
		limits = append(limits, lim)
	}
	return limits, nil
}

func parsePathInitLimit(arg string) (lim nogfsostad.PathInitLimit, err error) {
	fields := strings.Split(arg, ":")
	if len(fields) != 3 {
		err := errors.New("wrong number of fields")
		return lim, err
	}

	lim.Path = fields[0]

	lim.MaxFiles, err = parseUint64Si(fields[1])
	if err != nil {
		return lim, err
	}

	lim.MaxBytes, err = parseUint64Si(fields[2])
	if err != nil {
		return lim, err
	}

	return lim, err
}

var siMap = map[string]uint64{
	"k": 1 << 10,
	"m": 1 << 20,
	"g": 1 << 30,
	"t": 1 << 40,
}

func parseUint64Si(s string) (uint64, error) {
	s = strings.ToLower(s)

	m := uint64(1)
	for suf, mult := range siMap {
		if strings.HasSuffix(s, suf) {
			m = mult
			s = s[0 : len(s)-len(suf)]
			break
		}
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if v < 0 {
		err := fmt.Errorf("must be positive, got %d", v)
		return 0, err
	}

	return uint64(v) * m, nil
}

// Example:
// `A U Thor <author@example.com>` -> (`A U Thor`, `author@example.com`).
var rgxUser = regexp.MustCompile(regexpx.Verbose(`
	^
	( [^<]+ )
	\s
	< ( [^>]+ ) >
	$
`))

func parseUser(user string) (name, email string, err error) {
	m := rgxUser.FindStringSubmatch(user)
	if m == nil {
		err := fmt.Errorf("does not match `%s`", rgxUser)
		return "", "", err
	}
	return m[1], m[2], nil
}
