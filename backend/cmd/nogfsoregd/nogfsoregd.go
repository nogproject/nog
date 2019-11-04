// vim: sw=8

// Server `nogfsoregd`; see NOE-13.
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
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/internal/broadcast"
	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsoauthz"
	"github.com/nogproject/nog/backend/internal/fsomain"
	"github.com/nogproject/nog/backend/internal/fsoregistry"
	"github.com/nogproject/nog/backend/internal/fsorepos"
	"github.com/nogproject/nog/backend/internal/grpcjwt"
	"github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsoregd"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/livebroadcastd"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/registryd"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/replicate"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/unixdomainsd"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/workflowproc"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/internal/unixdomains"
	"github.com/nogproject/nog/backend/internal/unixdomainspb"
	"github.com/nogproject/nog/backend/internal/workflows/archiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/durootwf"
	"github.com/nogproject/nog/backend/internal/workflows/freezerepowf"
	"github.com/nogproject/nog/backend/internal/workflows/moverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/moveshadowwf"
	"github.com/nogproject/nog/backend/internal/workflows/pingregistrywf"
	"github.com/nogproject/nog/backend/internal/workflows/splitrootwf"
	"github.com/nogproject/nog/backend/internal/workflows/unarchiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/unfreezerepowf"
	"github.com/nogproject/nog/backend/internal/workflows/wfgc"
	"github.com/nogproject/nog/backend/internal/workflows/wfindexes"
	"github.com/nogproject/nog/backend/pkg/mgo"
	"github.com/nogproject/nog/backend/pkg/mulog"
	"github.com/nogproject/nog/backend/pkg/netx"
	"github.com/nogproject/nog/backend/pkg/uuid"
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
	version  = fmt.Sprintf("nogfsoregd-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(`Usage:
  nogfsoregd [options] [--proc-registry=<registry>...]

Options:
  --bind-grpc=<addr>  [default: 0.0.0.0:7550]
  --bind-rgrpc=<addr>  [default: 0.0.0.0:7551]
  --advertise-rgrpc=<addr>  [default: localhost:7551]
	The address nogfsostad uses to connect to ''--bind-rgrpc''.
  --tls-cert=<pem>  [default: /nog/ssl/certs/nogfsoregd/combined.pem]
        TLS certificate and corresponding private key.  PEM files can be
        concatenated ''cat cert.pem privkey.pem > combined.pem''.
  --tls-ca=<pem>  [default: /nog/ssl/certs/nogfsoregd/ca.pem]
        TLS certificates that are accepted as CA for client certs.  Multiple
        PEM files can be concatenated.
  --jwt-ca=<pem>  [default: /nog/ssl/certs/nogfsoregd/ca.pem]
	X.509 CA for JWTs.  Multiple PEMs can be concatenated.
  --jwt-ou=<ou>  [default: nogfsoiam]
	OU of JWT signing key X.509 Subject.
  --mongodb=<url>  [default: localhost:27017/nogfsoreg]
  --mongodb-ca=<pem>
        Path of file with CA certificates to use when connecting to MongoDB.
  --mongodb-cert=<pem>
        Path of file with certificate and private key to use when connecting to
        MongoDB.  Concatenate PEM files like
        ''cat cert.pem privkey.pem > combined.pem''.
  --names-collection=<ns>  [default: names]
  --names-prefix=<code>  [default: F]
  --shutdown-timeout=<duration>  [default: 20s]
        Maximum time to wait before forced shutdown.
  --log=<logger>  [default: prod]
        Specifies logger: prod, dev, or mu.
  --proc-registry=<registry>
        Enable workflow processing for a registry.
  --proc-registry-jwt=<path>  [default: /nog/jwt/tokens/nogfsoregd.jwt]
        Path of the JWT for in-process system GRPCs.
  --events-gc-scan-start=<wait-duration>  [default: 20m]
        Run events garbage collection at startup after wait duration.
        Use ''0'' to disable.
  --events-gc-scan-every=<interval>  [default: 240h]
        Run events garbage collection at regular intervals.
        Use ''0'' to disable.
  --events-gc-scan-jitter=<duration>  [default: 10m]
        Max random wait duration before each events garbage collection.
  --history-trim-scan-start=<wait-duration>  [default: 0]
        Trim histories at startup after wait duration.
        Use ''0'' to disable.
  --history-trim-scan-every=<interval>  [default: 0]
        Trim histories at regular intervals.
        Use ''0'' to disable.
  --history-trim-scan-jitter=<duration>  [default: 10m]
        Max random wait duration before each history trim scan.
  --workflows-gc-scan-start=<wait-duration>  [default: 0]
        Run workflows garbage collection at startup after wait duration.
        Use ''0'' to disable.
  --workflows-gc-scan-every=<interval>  [default: 0]
        Run workflows garbage collection at regular intervals.
        Use ''0'' to disable.
  --workflows-gc-scan-jitter=<duration>  [default: 10m]
        Max random wait duration before each workflows garbage collection.
`)

var ErrDialedTwice = errors.New("dialed more than once")

var (
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

const (
	NsFsoMain   = "fsomain"
	FsoMainName = "main"

	NsBroadcast  = "broadcast"
	BroadcastAll = "all"
)

type initRepoAllower struct {
	regstatdsd fsoregistry.InitRepoAllower
}

func (a *initRepoAllower) IsInitRepoAllowed(
	ctx context.Context,
	repo, host, hostPath string,
	subdirTracking nogfsopb.SubdirTracking,
) (deny string, err error) {
	if a.regstatdsd == nil {
		return "default deny", nil
	}
	return a.regstatdsd.IsInitRepoAllowed(
		ctx,
		repo, host, hostPath,
		subdirTracking,
	)
}

type idChecker struct {
	journals []*events.Journal

	mu    sync.Mutex
	seen  map[uuid.I]struct{}
	seen2 map[uuid.I]struct{}
}

func (c *idChecker) IsUnusedId(
	id uuid.I,
) (decision string, err error) {
	const accept = ""
	const deny = "ID has been used before"

	// Every ID can be used only once, even if it does not enter the
	// journal.  This also avoids a race condition when the same ID is
	// immediately used again before the aggregate is initialized.
	if !c.addNewId(id) {
		return deny, nil
	}

	for _, j := range c.journals {
		vid, err := j.Head(id)
		if err != nil {
			return "", err
		}
		if vid != events.EventEpoch {
			return deny, nil
		}
	}
	return accept, nil
}

func (c *idChecker) IsUnusedWorkflowId(
	id uuid.I,
) (decision string, err error) {
	return c.IsUnusedId(id)
}

func (c *idChecker) addNewId(id uuid.I) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Manage limited size ID set.
	if len(c.seen) > 100 {
		c.seen2 = c.seen
		c.seen = nil
	}
	if c.seen == nil {
		c.seen = make(map[uuid.I]struct{})
	}

	// Reject IDs that have been seen before.
	if _, ok := c.seen[id]; ok {
		return false
	}
	if c.seen2 != nil {
		if _, ok := c.seen2[id]; ok {
			return false
		}
	}

	// It's a new ID.
	c.seen[id] = struct{}{}
	return true
}

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
	authz := fsoauthz.CreateScopeAuthz(lg)

	lg.Infow("nogfsoregd started.")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	signal.Notify(sigs, syscall.SIGINT)
	var isShutdown int32

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	lg.Infow("Begin connecting to mongo.")
	var mgs *mgo.Session
	{
		uri := args["--mongodb"].(string)
		ca, caOk := args["--mongodb-ca"].(string)
		cert, certOk := args["--mongodb-cert"].(string)
		if caOk || certOk {
			lg.Infow("Using MongoDB SSL.", "ca", ca, "cert", cert)
			s, err := mgo.DialCACert(uri, ca, cert)
			if err != nil {
				lg.Fatalw(
					"Failed to SSL dial mongo.",
					"err", err,
				)
			}
			mgs = s
		} else {
			s, err := mgo.Dial(uri)
			if err != nil {
				lg.Fatalw("Failed to dial mongo.", "err", err)
			}
			mgs = s
		}
	}
	defer mgs.Close()
	lg.Infow("Connected to mongo.")

	// All Mongo requests use the same `mgo.Session` in `Strong` mode, so
	// that they all see each others effects.  A strong session must be
	// reset using `Refresh()` before it can be used again after a
	// connection problem.  See:
	//
	// - <https://github.com/night-codes/mgo-wrapper/blob/master/mongo.go>.
	// - GitHub issue mgo-49,
	//   <https://github.com/go-mgo/mgo/issues/49#issuecomment-65122720>.
	//
	// Using multiple sessions would be an alternative.  It is not
	// immediately obvious at which level to `Copy()` the session.  Each
	// journal could use its own session, maybe even each request.
	//
	// For now, `Refresh()` is handled here.  If a ping fails, a refresh is
	// scheduled for the next tick.  Refresh is not called immediately, so
	// that users of the session get a chance to see the error and fail.
	// It seems safer to give them a chance to fail instead of hiding
	// session refreshs, which may give a false sense of consistency.
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		needsRefresh := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if needsRefresh {
					mgs.Refresh()
				}
				if err := mgs.Ping(); err != nil {
					if !needsRefresh {
						lg.Infow("Ping mongo failed.")
					}
					needsRefresh = true
				} else {
					if needsRefresh {
						lg.Infow("Mongo recovered.")
					}
					needsRefresh = false
				}
			}
		}
	}()

	names := shorteruuid.NewNogNames()

	mainJ, err := events.NewJournal(mgs, "evjournal.fsomain")
	if err != nil {
		lg.Fatalw("Failed to create main journal.", "err", err)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := mainJ.Serve(ctx)
		if err != context.Canceled {
			lg.Fatalw("Main journal serve failed.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected main journal serve cancel.")
		}
	}()

	workflowsJ, err := events.NewJournal(mgs, "evjournal.workflows")
	if err != nil {
		lg.Fatalw("Failed to create workflows journal.", "err", err)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := workflowsJ.Serve(ctx)
		if err != context.Canceled {
			lg.Fatalw(
				"Workflows journal serve failed.",
				"err", err,
			)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected journal serve cancel.")
		}
	}()

	// The ephemeral workflows state, which may be reset at any time.
	ephWorkflowsJ, err := events.NewJournal(mgs, "ephevj.ephworkflows")
	if err != nil {
		lg.Fatalw(
			"Failed to create ephemeral workflows journal.",
			"err", err,
		)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := ephWorkflowsJ.Serve(ctx)
		if err != context.Canceled {
			lg.Fatalw(
				"Ephemeral workflows journal serve error.",
				"err", err,
			)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw(
				"Unexpected ephemeral workflows journal cancel.",
			)
		}
	}()

	moveRepoWorkflows := moverepowf.New(workflowsJ)
	moveShadowWorkflows := moveshadowwf.New(workflowsJ)

	registryWorkflowIndexes := wfindexes.New(ephWorkflowsJ)
	duRootWorkflows := durootwf.New(ephWorkflowsJ)
	pingRegistryWorkflows := pingregistrywf.New(ephWorkflowsJ)
	splitRootWorkflows := splitrootwf.New(ephWorkflowsJ)
	freezeRepoWorkflows := freezerepowf.New(ephWorkflowsJ)
	unfreezeRepoWorkflows := unfreezerepowf.New(ephWorkflowsJ)
	archiveRepoWorkflows := archiverepowf.New(ephWorkflowsJ)
	unarchiveRepoWorkflows := unarchiverepowf.New(ephWorkflowsJ)

	registryJ, err := events.NewJournal(mgs, "evjournal.fsoregistry")
	if err != nil {
		lg.Fatalw("Failed to create fsoregistry journal.", "err", err)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := registryJ.Serve(ctx)
		if err != context.Canceled {
			lg.Fatalw("Registry journal serve error.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected registry journal cancel.")
		}
	}()

	reposJ, err := events.NewJournal(mgs, "evjournal.fsorepos")
	if err != nil {
		lg.Fatalw("Failed to create fsorepos journal.", "err", err)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := reposJ.Serve(ctx)
		if err != context.Canceled {
			lg.Fatalw("Repos journal serve failed.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected journal serve cancel.")
		}
	}()

	broadcastJ, err := events.NewJournal(mgs, "evjournal.fsobroadcast")
	if err != nil {
		lg.Fatalw("Failed to create fsobroadcast journal.", "err", err)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := broadcastJ.Serve(ctx)
		if err != context.Canceled {
			lg.Fatalw(
				"Broadcast journal serve failed.", "err", err,
			)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected journal serve cancel.")
		}
	}()

	domainsJ, err := events.NewJournal(mgs, "evjournal.unixdomains")
	if err != nil {
		lg.Fatalw("Failed to create Unix domains journal.", "err", err)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := domainsJ.Serve(ctx)
		if err != context.Canceled {
			lg.Fatalw(
				"Unix domains journal serve failed.",
				"err", err,
			)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected journal serve cancel.")
		}
	}()

	allJournalsIdChecker := &idChecker{journals: []*events.Journal{
		mainJ,
		registryJ,
		reposJ,
		workflowsJ,
		ephWorkflowsJ,
		broadcastJ,
		domainsJ,
	}}

	main := fsomain.New(mainJ)

	// Until `initRepoAllower.regstatdsd` is set below, init is disabled.
	var initRepoAllower initRepoAllower
	registry := fsoregistry.New(registryJ, fsoregistry.Preconditions{
		InitRepo:        &initRepoAllower,
		WorkflowIdCheck: allJournalsIdChecker,
	})

	repos := fsorepos.New(reposJ)
	domains := unixdomains.New(domainsJ)

	var wg2 sync.WaitGroup
	ctx2, cancel2 := context.WithCancel(context.Background())

	broadcastId := names.UUID(NsBroadcast, BroadcastAll)
	broadcaster := broadcast.New(
		lg,
		broadcastJ,
		broadcastId,
		broadcast.WatchConfig{
			MainJ:     mainJ,
			RegistryJ: registryJ,
			ReposJ:    reposJ,
		},
	)
	wg2.Add(1)
	go func() {
		err := broadcaster.Process(ctx2)
		if err != context.Canceled {
			lg.Fatalw("Process broadcast error.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected broadcast process cancel.")
		}
		wg2.Done()
		return
	}()

	mainId := names.UUID(NsFsoMain, FsoMainName)
	_, err = main.Init(mainId, FsoMainName)
	if err != nil {
		lg.Fatalw("Failed to init main.", "err", err)
	}

	wg2.Add(1)
	go func() {
		err := nogfsoregd.ProcessRegistryInit(
			ctx2, lg, names, mainJ, main, mainId, registry,
		)
		if err != context.Canceled {
			lg.Fatalw("Process registry init error.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected registry init cancel.")
		}
		wg2.Done()
		return
	}()

	wg2.Add(1)
	go func() {
		err := nogfsoregd.ProcessRepoInit(
			ctx2,
			lg,
			names,
			mainJ, main, mainId,
			registryJ, registry,
			repos,
		)
		if err != context.Canceled {
			lg.Fatalw("Repo init process failed.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected repo init process cancel.")
		}
		wg2.Done()
		return
	}()

	replProc := replicate.NewProcessor(
		lg,
		names,
		mainJ, registryJ, reposJ, workflowsJ,
		registry, repos, moveRepoWorkflows, moveShadowWorkflows,
		mainId,
	)
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		err := replProc.Process(ctx2)
		if err != context.Canceled {
			lg.Fatalw("Event replication failed.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Event replication quit unexpectedly.")
		}
	}()

	// The default `grpc.keepalive` parameters allow connections to persist
	// forever.
	inprocGrpcD := grpc.NewServer()

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

	mainD := nogfsoregd.NewMainServer(
		ctx2, authn, authz, main, mainId, FsoMainName,
	)
	nogfsopb.RegisterMainServer(gsrv, mainD)

	registryD := registryd.New(
		ctx2, lg, authn, authz,
		names, allJournalsIdChecker,
		main, mainId,
		registryJ, registry,
		repos,
		ephWorkflowsJ, registryWorkflowIndexes,
		duRootWorkflows,
		pingRegistryWorkflows,
		splitRootWorkflows,
		freezeRepoWorkflows, unfreezeRepoWorkflows,
		archiveRepoWorkflows, unarchiveRepoWorkflows,
	)
	nogfsopb.RegisterRegistryServer(inprocGrpcD, registryD)
	nogfsopb.RegisterRegistryServer(gsrv, registryD)
	nogfsopb.RegisterEphemeralRegistryServer(inprocGrpcD, registryD)
	nogfsopb.RegisterEphemeralRegistryServer(gsrv, registryD)
	nogfsopb.RegisterDiskUsageServer(gsrv, registryD)
	nogfsopb.RegisterPingRegistryServer(inprocGrpcD, registryD)
	nogfsopb.RegisterPingRegistryServer(gsrv, registryD)
	nogfsopb.RegisterSplitRootServer(inprocGrpcD, registryD)
	nogfsopb.RegisterSplitRootServer(gsrv, registryD)
	nogfsopb.RegisterFreezeRepoServer(inprocGrpcD, registryD)
	nogfsopb.RegisterFreezeRepoServer(gsrv, registryD)
	nogfsopb.RegisterRegistryFreezeRepoServer(inprocGrpcD, registryD)
	// Do not `nogfsopb.RegisterRegistryFreezeRepoServer(gsrv, registryD)`,
	// because the service is only used by nogfsoregd internally.
	nogfsopb.RegisterUnfreezeRepoServer(inprocGrpcD, registryD)
	nogfsopb.RegisterUnfreezeRepoServer(gsrv, registryD)
	nogfsopb.RegisterRegistryUnfreezeRepoServer(inprocGrpcD, registryD)
	// Do not `nogfsopb.RegisterRegistryUnfreezeRepoServer(gsrv, registryD)`,
	// because the service is only used by nogfsoregd internally.
	nogfsopb.RegisterArchiveRepoServer(inprocGrpcD, registryD)
	nogfsopb.RegisterArchiveRepoServer(gsrv, registryD)
	nogfsopb.RegisterRegistryArchiveRepoServer(inprocGrpcD, registryD)
	// Do not `nogfsopb.RegisterRegistryArchiveRepoServer(gsrv, registryD)`,
	// because the service is only used by nogfsoregd internally.
	nogfsopb.RegisterUnarchiveRepoServer(inprocGrpcD, registryD)
	nogfsopb.RegisterUnarchiveRepoServer(gsrv, registryD)
	nogfsopb.RegisterExecUnarchiveRepoServer(inprocGrpcD, registryD)
	nogfsopb.RegisterExecUnarchiveRepoServer(gsrv, registryD)
	nogfsopb.RegisterRegistryUnarchiveRepoServer(inprocGrpcD, registryD)
	// Do not `nogfsopb.RegisterRegistryUnarchiveRepoServer(gsrv, registryD)`,
	// because the service is only used by nogfsoregd internally.

	reposD := nogfsoregd.NewReposServer(
		ctx2, lg, authn, authz,
		names,
		reposJ, repos,
		workflowsJ, moveRepoWorkflows, moveShadowWorkflows,
	)
	nogfsopb.RegisterReposServer(gsrv, reposD)
	nogfsopb.RegisterReposFreezeRepoServer(inprocGrpcD, reposD)
	// Do not `nogfsopb.RegisterReposFreezeRepoServer(gsrv, reposD)`,
	// because the service is only used by nogfsoregd internally.
	nogfsopb.RegisterReposUnfreezeRepoServer(inprocGrpcD, reposD)
	// Do not `nogfsopb.RegisterReposUnfreezeRepoServer(gsrv, reposD)`,
	// because the service is only used by nogfsoregd internally.
	nogfsopb.RegisterReposArchiveRepoServer(inprocGrpcD, reposD)
	// Do not `nogfsopb.RegisterReposArchiveRepoServer(gsrv, reposD)`,
	// because the service is only used by nogfsoregd internally.
	nogfsopb.RegisterReposUnarchiveRepoServer(inprocGrpcD, reposD)
	// Do not `nogfsopb.RegisterReposUnarchiveRepoServer(gsrv, reposD)`,
	// because the service is only used by nogfsoregd internally.

	broadcastd := nogfsoregd.NewBroadcastServer(
		ctx2, lg, authn, authz, names, broadcastJ,
	)
	wg2.Add(1)
	go func() {
		err := broadcastd.Serve()
		if err != context.Canceled {
			lg.Fatalw("Repo init process failed.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected repo init process cancel.")
		}
		wg2.Done()
		return
	}()
	nogfsopb.RegisterBroadcastServer(gsrv, broadcastd)
	nogfsopb.RegisterGitBroadcasterServer(gsrv, broadcastd)

	liveBroadcastD := livebroadcastd.New(
		ctx2, lg, authn, authz, &livebroadcastd.Journals{
			Main:               mainJ,
			Registry:           registryJ,
			Repos:              reposJ,
			Workflows:          workflowsJ,
			EphemeralWorkflows: ephWorkflowsJ,
		},
	)
	nogfsopb.RegisterLiveBroadcastServer(inprocGrpcD, liveBroadcastD)
	nogfsopb.RegisterLiveBroadcastServer(gsrv, liveBroadcastD)

	domainsD := unixdomainsd.New(
		ctx2, lg, authn, authz,
		main, mainId,
		domains, domainsJ,
	)
	unixdomainspb.RegisterUnixDomainsServer(gsrv, domainsD)

	addrType := "tcp"
	addr := args["--bind-rgrpc"].(string)
	if strings.HasPrefix(addr, "/") {
		addrType = "unix"
		_ = os.Remove(addr)
	}
	rLis, err := net.Listen(addrType, addr)
	if err != nil {
		lg.Fatalw("Listen failed.", "family", addrType, "addr", addr)
	}
	sessionTls := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      ca,
	})
	statdsd := nogfsoregd.NewStatdsServer(
		lg,
		args["--advertise-rgrpc"].(string),
		sessionTls, authn, authz,
		repos,
	)
	wg2.Add(1)
	go func() {
		err := statdsd.Serve(ctx2, rLis)
		if err != context.Canceled {
			lg.Fatalw("statdsd Serve error.", "err", err)
		}
		if atomic.LoadInt32(&isShutdown) == 0 {
			lg.Fatalw("Unexpected statdsd cancel.")
		}
		wg2.Done()
		return
	}()
	nogfsopb.RegisterStatdsServer(gsrv, statdsd)
	lg.Infow("Reverse GRPC listening.", "family", addrType, "addr", addr)
	initRepoAllower.regstatdsd = statdsd
	nogfsopb.RegisterStatServer(gsrv, statdsd)
	nogfsopb.RegisterGitNogServer(gsrv, statdsd)
	nogfsopb.RegisterGitNogTreeServer(gsrv, statdsd)
	nogfsopb.RegisterDiscoveryServer(gsrv, statdsd)
	nogfsopb.RegisterTarttServer(gsrv, statdsd)
	nogfsopb.RegisterTestUdoServer(gsrv, statdsd)

	inprocSocks, err := netx.UnixSocketpair()
	if err != nil {
		lg.Fatalw("Failed to create socket pair.", "err", err)
	}
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		err := inprocGrpcD.Serve(
			netx.ListenConnectedConn(inprocSocks[0]),
		)
		if atomic.LoadInt32(&isShutdown) > 0 {
			return
		}
		lg.Fatalw("gsrv error.", "err", err)
	}()
	lg.Infow("Started in-process GRPC server.")

	// `grpc.Dial()` should not retry with `WithBlock(),
	// FailOnNonTempDialError()`.  Nonetheless, return the socket only for
	// the first dial, and return errors for further dials.
	first := make(chan struct{}, 1)
	first <- struct{}{}
	inprocConn, err := grpc.DialContext(
		ctx,
		"",
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithDisableRetry(),
		grpc.WithDialer(func(string, time.Duration) (net.Conn, error) {
			select {
			case <-first:
				c := inprocSocks[1]
				inprocSocks[1] = nil
				return c, nil
			default:
				return nil, ErrDialedTwice
			}
		}),
	)
	if err != nil {
		lg.Fatalw(
			"Failed to dial in-process GRPC server.",
			"err", err,
		)
	}
	if inprocSocks[1] != nil {
		panic("dial did not use inprocSocks[1]")
	}
	lg.Infow("Established in-process GRPC connection.")

	addrType = "tcp"
	addr = args["--bind-grpc"].(string)
	if strings.HasPrefix(addr, "/") {
		addrType = "unix"
		_ = os.Remove(addr)
	}
	lis, err := net.Listen(addrType, addr)
	if err != nil {
		lg.Fatalw(
			"Listen failed.",
			"family", addrType,
			"addr", addr,
			"err", err,
		)
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
	lg.Infow("GRPC listening.", "family", addrType, "addr", addr)

	var wg3 sync.WaitGroup
	ctx3, cancel3 := context.WithCancel(context.Background())

	// XXX Consider automatic discovery of registries from main.
	if regs := args["--proc-registry"].([]string); len(regs) > 0 {
		sysRPCCreds, err := grpcjwt.Load(
			args["--proc-registry-jwt"].(string),
		)
		if err != nil {
			lg.Fatalw(
				"Failed to load --proc-registry-jwt",
				"err", err,
			)
		}
		// A socket pair is secure without TLS.
		sysRPCCreds.AllowInsecureTransport = true

		lg.Infow(
			"Started registry workflow processing.",
			"registries", regs,
		)
		workflowProc := workflowproc.New(lg, &workflowproc.Config{
			Registries:  regs,
			Conn:        inprocConn,
			SysRPCCreds: sysRPCCreds,
		})
		wg3.Add(1)
		go func() {
			defer wg3.Done()
			err := workflowProc.Run(ctx3)
			if err != context.Canceled {
				lg.Fatalw(
					"Registry workflow processing error.",
					"err", err,
				)
			}
			if atomic.LoadInt32(&isShutdown) == 0 {
				lg.Fatalw(
					"Unexpected registry workflow processing shutdown.",
				)
			}
		}()
	} else {
		lg.Infow("Disabled registry workflow processing.")
	}

	startEventsGcScans(args, &wg3, ctx3, []*events.Journal{
		mainJ,
		workflowsJ,
		ephWorkflowsJ,
		registryJ,
		reposJ,
		broadcastJ,
	})
	startHistoryTrimScans(args, &wg3, ctx3, []*events.Journal{
		mainJ,
		workflowsJ,
		ephWorkflowsJ,
		registryJ,
		reposJ,
		broadcastJ,
	})

	if regs := args["--proc-registry"].([]string); len(regs) > 0 {
		gc := wfgc.New(
			lg,
			regs,
			names,
			ephWorkflowsJ,
			registryWorkflowIndexes,
			duRootWorkflows,
			pingRegistryWorkflows,
			splitRootWorkflows,
		)
		startWorkflowsGcScans(args, &wg3, ctx3, gc)
	}

	sig := <-sigs
	atomic.StoreInt32(&isShutdown, 1)

	done := make(chan struct{})
	go func() {
		cancel3()
		wg3.Wait()
		lg.Infow("Completed level 3 shutdown.")

		_ = inprocConn.Close()
		cancel2()
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			inprocGrpcD.GracefulStop()
		}()
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

	for _, k := range []string{
		"--shutdown-timeout",
		"--events-gc-scan-start",
		"--events-gc-scan-every",
		"--events-gc-scan-jitter",
		"--history-trim-scan-start",
		"--history-trim-scan-every",
		"--history-trim-scan-jitter",
		"--workflows-gc-scan-start",
		"--workflows-gc-scan-every",
		"--workflows-gc-scan-jitter",
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

	c := args["--names-prefix"].(string)
	if c != strings.ToUpper(c) {
		lg.Fatalw("--names-prefix must be all uppercase.")
	}
	if len(c) > 2 {
		lg.Fatalw("--names-prefix longer than max length 2.")
	}

	return args
}

// `eventsGCP` is an adapter to use `events.EventsGarbageCollector` as a
// `Processor`.
type eventsGCP struct {
	*events.EventsGarbageCollector
}

func (p *eventsGCP) Process(ctx context.Context) error {
	return p.Gc(ctx)
}

func startEventsGcScans(
	args map[string]interface{},
	wg *sync.WaitGroup,
	ctx context.Context,
	journals []*events.Journal,
) {
	what := "events gc"
	start := args["--events-gc-scan-start"]
	every := args["--events-gc-scan-every"]
	jitter := args["--events-gc-scan-jitter"]
	procs := make([]Processor, 0, len(journals))
	for _, j := range journals {
		procs = append(procs, &eventsGCP{
			events.NewEventsGarbageCollector(lg, j),
		})
	}
	StartScans(
		lg, wg, ctx, what, procs, start, every, jitter,
	)
}

// `historyTP` is an adapter to use `events.Trimmer` as a `Processor`.
type historyTP struct {
	*events.Trimmer
}

func (p *historyTP) Process(ctx context.Context) error {
	return p.Trim(ctx)
}

func startHistoryTrimScans(
	args map[string]interface{},
	wg *sync.WaitGroup,
	ctx context.Context,
	journals []*events.Journal,
) {
	what := "history trimming"
	start := args["--history-trim-scan-start"]
	every := args["--history-trim-scan-every"]
	jitter := args["--history-trim-scan-jitter"]
	procs := make([]Processor, 0, len(journals))
	for _, j := range journals {
		procs = append(procs, &historyTP{events.NewTrimmer(lg, j)})
	}
	StartScans(
		lg, wg, ctx, what, procs, start, every, jitter,
	)
}

// `wfgcGCP` is an adapter to use an `wfgc.GarbageCollector` as a `Processor`.
type wfgcGCP struct {
	*wfgc.GarbageCollector
}

func (p *wfgcGCP) Process(ctx context.Context) error {
	return p.Gc(ctx)
}

func startWorkflowsGcScans(
	args map[string]interface{},
	wg *sync.WaitGroup,
	ctx context.Context,
	gc *wfgc.GarbageCollector,
) {
	what := "workflows gc"
	start := args["--workflows-gc-scan-start"]
	every := args["--workflows-gc-scan-every"]
	jitter := args["--workflows-gc-scan-jitter"]
	procs := []Processor{&wfgcGCP{gc}}
	StartScans(
		lg, wg, ctx, what, procs, start, every, jitter,
	)
}
