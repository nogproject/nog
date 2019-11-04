// Package `observer6` contains `Observer` to watch for registry and repo
// events and trigger actions on a `Processor`, specifically
// `nogfsostad.Processor`.  It:
//
//  - triggers shadow repo initialization;
//  - enables repos to receive gRPCs.
//
// `RegistryObserver` uses entity activities to process events.  It uses one
// activity per entity, that is registry, repo, and repo workflow.  Activities
// watch the events streams forever.  The registry activity starts repo
// activities for repos that are below the observed prefixes.  The repo
// activities start workflow activities as needed and wait for their completion
// before continuing to process repo events.
//
// The move-repo workflow is a special case.  If the old location was below an
// observed prefix, the repo activity runs the release workflow part and then
// quits.  The registry activity starts a new repo activity for the acquire
// workflow part and to continue watching the event stream.
//
// Per-repo activities are serialized using a chain of `done -> dep` channels.
// See `DepChainMap`.  Serialization is currently only relevant if the
// move-repo workflow release and acquire parts both run in the same server.
//
// Activities distinguish two phases:
//
//  1. initial loading until the event stream will block;
//  2. watching the event stream, processing event per event.
//
// Desired effects of the two-phase approach are:
//
//  - Initial loading directly enables repos that are already initialized
//    without revisiting the actual initialization work.
//  - Initial loading skips initializing repos with stored errors.
//  - Initial loading naturally supports multiple events that may toggle some
//    state.  It will take action only based on the final state.  Example: A
//    repo could in the future toggle between renaming and not renaming.  If
//    the final state is renaming, it will remain disabled.  If the final state
//    is not renaming, it will be enabled.
//  - The initial batch processing of all events is separated from the ongoing
//    one-by-one processing of new events.
//
// Differently than `observer4`, `observer6` uses `grpclazy.Engine` to manage
// the gRPC streams for the activities.
//
// Differently than `observer5`, `observer6` runs an unbounded number of
// activities in order to concurrently watch all repos that are below the
// observed prefixed.
//
// `observer6` uses `grpclazy.Engine` for all activities.  `grpclazy.Engine`
// uses a single gRPC stream to receive broadcast signals about new events.
// For each activity that has new events, `grpclazy.Engine` starts a goroutine
// and opens a separate gRPC stream to pass the available events to the
// activity.  The activity processes the available events and returns to the
// `grpclazy.Engine`, which puts the activity to sleep until the next signal
// that there are new events.
package observer6

import (
	"context"
	"strings"
	"sync"

	"github.com/nogproject/nog/backend/internal/nogfsostad"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	"github.com/nogproject/nog/backend/internal/process/grpclazy"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// We use a general activity stream limit and a separate limit for repo
// activity streams.  Registry and workflow activities never block.  But repo
// activities may block waiting for workflow activities to complete.  Separate
// limits should avoid deadlock.  If we used a single limiter, blocked repo
// activities could consume all available streams waiting for workflows.  But
// workflow progress would be prevented without available streams.
const ConfigMaxStreams = 20
const ConfigMaxRepoStreams = 10

const ConfigMaxRetryWeak = 5
const ConfigCronInvervalSeconds = 1

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

type Processor interface {
	EnableRepo4(ctx context.Context, inf *nogfsostad.RepoInfo) error
	DisableRepo4(ctx context.Context, repoId uuid.I) error
	FreezeRepo(
		ctx context.Context, repoId uuid.I, author nogfsostad.GitUser,
	) error
	UnfreezeRepo(
		ctx context.Context, repoId uuid.I, author nogfsostad.GitUser,
	) error
}

type Initializer interface {
	GetRepo(ctx context.Context, repoId uuid.I) (*nogfsostad.RepoInfo, error)
	InitRepo(ctx context.Context, repoId uuid.I) (*nogfsostad.RepoInfo, error)
	EnableGitlab(ctx context.Context, repoId uuid.I) (*nogfsostad.RepoInfo, error)
	MoveRepo(
		ctx context.Context,
		repoId uuid.I,
		oldHostPath string,
		oldShadowPath string,
		newHostPath string,
	) (newShadowPath string, err error)
}

type repoErrorHandler interface {
	handleRepoError(
		ctx context.Context,
		repoId uuid.I,
		repoError error,
	) (bool, error)
}

type repoErrorStorer interface {
	storeRepoError(
		ctx context.Context,
		repo uuid.I,
		repoError error,
	) error
}

type Config struct {
	Registries  []string
	Prefixes    []string
	Conn        *grpc.ClientConn
	SysRPCCreds credentials.PerRPCCredentials
	Initializer Initializer
	Processor   Processor
}

type Observer struct {
	lg         Logger
	registries []*RegistryObserver
	lazyEngine *grpclazy.Engine
}

type RegistryObserver struct {
	lg          Logger
	registry    string
	prefixes    []string
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	initer      Initializer
	proc        Processor

	// `observedRepos` is the set of repos that this observer watches.
	observedRepos repoSet

	repoEngine     grpcentities.RepoEngine
	workflowEngine grpcentities.RepoWorkflowEngine

	// `repoDepMap` is uses to serialize activities per repo.
	repoDepMap DepChainMap
}

type repoSet map[uuid.I]struct{}

func (ws repoSet) Add(id uuid.I) {
	ws[id] = struct{}{}
}

func (ws repoSet) Delete(id uuid.I) {
	delete(ws, id)
}

func (ws repoSet) Has(id uuid.I) bool {
	_, ok := ws[id]
	return ok
}

// `DepChainMap` is used to serialize activities per repo using a chain of
// `done -> dep` channels.  The upstream activity closes `done` to unlock the
// downstream activity's `dep`.
type DepChainMap map[uuid.I]chan struct{}

type DepChainNode struct {
	Dep  <-chan struct{}
	Done chan<- struct{}
}

func NewDepMap() DepChainMap {
	return make(map[uuid.I]chan struct{})
}

func (m DepChainMap) Next(id uuid.I) DepChainNode {
	dep := m[id]
	done := make(chan struct{})
	m[id] = done // Save `done` as the next `dep`.
	return DepChainNode{
		Dep:  dep,
		Done: done,
	}
}

func (m DepChainMap) Gc() int {
	n := 0
	for id, done := range m {
		select {
		case <-done:
			delete(m, id)
			n++
		default: // non-blocking
		}
	}
	return n
}

// `registryView` is the registry view for all available events.  It is used in
// `init()`.
type registryView struct {
	vid   ulid.I
	repos map[uuid.I]repoView
}

func newRegistryView() *registryView {
	return &registryView{
		repos: make(map[uuid.I]repoView),
	}
}

// `repoView` is the state for one repo.
type repoView struct {
	id                      uuid.I
	moveRepoWorkflowRelease uuid.I
	moveRepoWorkflowAcquire uuid.I
}

func New(lg Logger, cfg *Config) *Observer {
	var prefixes []string
	for _, p := range cfg.Prefixes {
		// Ensure trailing slash.
		p = strings.TrimRight(p, "/") + "/"
		prefixes = append(prefixes, p)
	}

	streamLimiter := semaphore.NewWeighted(ConfigMaxStreams)
	repoStreamLimiter := semaphore.NewWeighted(ConfigMaxRepoStreams)

	lazyEngine := grpclazy.NewEngine(
		lg,
		&grpclazy.EngineConfig{
			Conn:               cfg.Conn,
			SysRPCCreds:        cfg.SysRPCCreds,
			StreamLimiter:      streamLimiter,
			StreamLimiterRepos: repoStreamLimiter,
		},
	)

	registries := make([]*RegistryObserver, 0, len(cfg.Registries))
	for _, r := range cfg.Registries {
		registries = append(registries, &RegistryObserver{
			lg:             lg,
			registry:       r,
			prefixes:       prefixes,
			conn:           cfg.Conn,
			sysRPCCreds:    grpc.PerRPCCredentials(cfg.SysRPCCreds),
			initer:         cfg.Initializer,
			proc:           cfg.Processor,
			repoEngine:     lazyEngine,
			workflowEngine: lazyEngine,
			repoDepMap:     NewDepMap(),
		})
	}

	return &Observer{
		lg:         lg,
		registries: registries,
		lazyEngine: lazyEngine,
	}
}

func (o *Observer) Watch(ctx context.Context) error {
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()
	errCh := make(chan error, 1)

	var wg sync.WaitGroup

	o.lazyEngine.SetContext(ctx2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel2()
		err := o.lazyEngine.Run()
		if err != context.Canceled {
			select {
			case errCh <- err:
			default: // non-blocking
			}
		}
	}()

	for _, r := range o.registries {
		err := o.lazyEngine.StartRegistryActivity(r.registry, r)
		if err != nil {
			cancel2()
			wg.Wait()
			return err
		}
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return ctx.Err()
	}
}

func (o *RegistryObserver) startWatchRepoActivity(
	ctx context.Context,
	repoId uuid.I,
	opts watchRepoActivityOptions,
) error {
	return o.repoEngine.StartRepoActivity(
		repoId,
		&watchRepoActivity{
			chainedRepoActivity: chainedRepoActivity{
				lg:         o.lg,
				chain:      o.repoDepMap.Next(repoId),
				repoId:     repoId,
				errHandler: o.newRepoErrorHandler(),
			},
			initer:         o.initer,
			proc:           o.proc,
			conn:           o.conn,
			sysRPCCreds:    o.sysRPCCreds,
			workflowEngine: o.workflowEngine,
			opts:           opts,
		},
	)
}

func (o *RegistryObserver) startRepoWorkflowActivity(
	ctx context.Context,
	repoId uuid.I,
	workflowId uuid.I,
	act grpcentities.RepoWorkflowActivity,
) error {
	return o.workflowEngine.StartRepoWorkflowActivity(
		repoId, workflowId, act,
	)
}
