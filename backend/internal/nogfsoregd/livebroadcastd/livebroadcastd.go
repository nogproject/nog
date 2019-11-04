package livebroadcastd

import (
	"context"
	"sync"

	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const ChannelAllAggregateSignals = "allaggsig"
const ConfigUpdatesQueueSize = 100
const ConfigMaxPendingSignalsLen = 4 * 1024

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

type Journals struct {
	Main               *events.Journal
	Registry           *events.Journal
	Repos              *events.Journal
	Workflows          *events.Journal
	EphemeralWorkflows *events.Journal
}

type Server struct {
	ctx   context.Context
	lg    Logger
	authn auth.Authenticator
	authz auth.Authorizer

	mainJ         *events.Journal
	registryJ     *events.Journal
	reposJ        *events.Journal
	workflowsJ    *events.Journal
	ephWorkflowsJ *events.Journal
}

func New(
	ctx context.Context,
	lg Logger,
	authn auth.Authenticator,
	authz auth.Authorizer,
	journals *Journals,
) *Server {
	return &Server{
		ctx:           ctx,
		lg:            lg,
		authn:         authn,
		authz:         authz,
		mainJ:         journals.Main,
		registryJ:     journals.Registry,
		reposJ:        journals.Repos,
		workflowsJ:    journals.Workflows,
		ephWorkflowsJ: journals.EphemeralWorkflows,
	}
}

var selectAggregatesAll = []pb.AggregateSignalsI_AggregateSelector{
	pb.AggregateSignalsI_AS_MAIN,
	pb.AggregateSignalsI_AS_REGISTRY,
	pb.AggregateSignalsI_AS_REPO,
	pb.AggregateSignalsI_AS_WORKFLOW,
	pb.AggregateSignalsI_AS_EPHEMERAL_WORKFLOW,
}

func (srv *Server) AggregateSignals(
	req *pb.AggregateSignalsI,
	stream pb.LiveBroadcast_AggregateSignalsServer,
) error {
	// `ctx.Done()` indicates client close, see
	// <https://groups.google.com/d/msg/grpc-io/C0rAhtCUhSs/SzFDLGqiCgAJ>.
	ctx := stream.Context()
	if err := srv.authName(
		ctx, AABroadcastRead, ChannelAllAggregateSignals,
	); err != nil {
		return err
	}

	selMap := make(map[pb.AggregateSignalsI_AggregateSelector]struct{})
	sel := req.SelectAggregates
	if len(sel) == 0 {
		sel = selectAggregatesAll
	}
	for _, s := range sel {
		selMap[s] = struct{}{}
	}

	// XXX There is no reliable way to detect if update notifications got
	// lost.  Maybe some mechanism should be added to `Subscribe()` to
	// communicate that update notifications could not be written.
	//
	// It probably does not matter in practice, because the goroutine below
	// should drain the channels fast enough.

	var mainCh <-chan uuid.I
	if _, ok := selMap[pb.AggregateSignalsI_AS_MAIN]; ok {
		ch := make(chan uuid.I, ConfigUpdatesQueueSize)
		srv.mainJ.Subscribe(ch, events.WildcardTopic)
		defer srv.mainJ.Unsubscribe(ch)
		mainCh = ch
	}

	var registryCh <-chan uuid.I
	if _, ok := selMap[pb.AggregateSignalsI_AS_REGISTRY]; ok {
		ch := make(chan uuid.I, ConfigUpdatesQueueSize)
		srv.registryJ.Subscribe(ch, events.WildcardTopic)
		defer srv.registryJ.Unsubscribe(ch)
		registryCh = ch
	}

	var repoCh <-chan uuid.I
	if _, ok := selMap[pb.AggregateSignalsI_AS_REPO]; ok {
		ch := make(chan uuid.I, ConfigUpdatesQueueSize)
		srv.reposJ.Subscribe(ch, events.WildcardTopic)
		defer srv.reposJ.Unsubscribe(ch)
		repoCh = ch
	}

	var workflowCh <-chan uuid.I
	if _, ok := selMap[pb.AggregateSignalsI_AS_WORKFLOW]; ok {
		ch := make(chan uuid.I, ConfigUpdatesQueueSize)
		srv.workflowsJ.Subscribe(ch, events.WildcardTopic)
		defer srv.workflowsJ.Unsubscribe(ch)
		workflowCh = ch
	}

	var ephWorkflowCh <-chan uuid.I
	if _, ok := selMap[pb.AggregateSignalsI_AS_EPHEMERAL_WORKFLOW]; ok {
		ch := make(chan uuid.I, ConfigUpdatesQueueSize)
		srv.ephWorkflowsJ.Subscribe(ch, events.WildcardTopic)
		defer srv.ephWorkflowsJ.Unsubscribe(ch)
		ephWorkflowCh = ch
	}

	var wg sync.WaitGroup
	done := make(chan struct{})
	defer func() {
		close(done)
		wg.Wait()
	}()

	sigCh := make(chan *pendingSignals)
	errCh := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		// `sigs` contains the pending signals, which will be sent to
		// the gRPC stream via `sigCh`.  `sigCh`, however, is not
		// directly used.  `ch` is used instead, which is initially nil
		// to deactivate sending until the first signal is set.
		sigs := &pendingSignals{}
		var ch chan<- *pendingSignals
		for {
			if sigs.Len() > ConfigMaxPendingSignalsLen {
				errCh <- status.Errorf(
					codes.ResourceExhausted,
					"too many pending signals",
				)
				return
			}
			select {
			case <-done:
				return
			case ch <- sigs:
				sigs = &pendingSignals{}
				ch = nil
			case id := <-mainCh:
				sigs.SetMainUpdate(id)
				ch = sigCh
			case id := <-registryCh:
				sigs.SetRegistryUpdate(id)
				ch = sigCh
			case id := <-repoCh:
				sigs.SetRepoUpdate(id)
				ch = sigCh
			case id := <-workflowCh:
				sigs.SetWorkflowUpdate(id)
				ch = sigCh
			case id := <-ephWorkflowCh:
				sigs.SetEphemeralWorkflowUpdate(id)
				ch = sigCh
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-srv.ctx.Done():
			err := status.Errorf(codes.Unavailable, "shutdown")
			return err
		case err := <-errCh:
			return err
		case sigs := <-sigCh:
			if err := stream.Send(sigs.AsPb()); err != nil {
				return err
			}
		}
	}
}

type pendingSignals struct {
	main        idSet
	registry    idSet
	repo        idSet
	workflow    idSet
	ephWorkflow idSet
}

type idSet map[uuid.I]struct{}

func (sigs *pendingSignals) Len() int {
	return len(sigs.main) +
		len(sigs.registry) +
		len(sigs.repo) +
		len(sigs.workflow)
}

func (sigs *pendingSignals) SetMainUpdate(id uuid.I) {
	if sigs.main == nil {
		sigs.main = make(map[uuid.I]struct{})
	}
	sigs.main[id] = struct{}{}
}

func (sigs *pendingSignals) SetRegistryUpdate(id uuid.I) {
	if sigs.registry == nil {
		sigs.registry = make(map[uuid.I]struct{})
	}
	sigs.registry[id] = struct{}{}
}

func (sigs *pendingSignals) SetRepoUpdate(id uuid.I) {
	if sigs.repo == nil {
		sigs.repo = make(map[uuid.I]struct{})
	}
	sigs.repo[id] = struct{}{}
}

func (sigs *pendingSignals) SetWorkflowUpdate(id uuid.I) {
	if sigs.workflow == nil {
		sigs.workflow = make(map[uuid.I]struct{})
	}
	sigs.workflow[id] = struct{}{}
}

func (sigs *pendingSignals) SetEphemeralWorkflowUpdate(id uuid.I) {
	if sigs.ephWorkflow == nil {
		sigs.ephWorkflow = make(map[uuid.I]struct{})
	}
	sigs.ephWorkflow[id] = struct{}{}
}

func (sigs *pendingSignals) AsPb() *pb.AggregateSignalsO {
	s := make([]*pb.AggregateSignal, 0, sigs.Len())
	s = append(s, sigs.main.AsPb(pb.AggregateSignal_AT_MAIN)...)
	s = append(s, sigs.registry.AsPb(pb.AggregateSignal_AT_REGISTRY)...)
	s = append(s, sigs.repo.AsPb(pb.AggregateSignal_AT_REPO)...)
	s = append(s, sigs.workflow.AsPb(pb.AggregateSignal_AT_WORKFLOW)...)
	s = append(s, sigs.ephWorkflow.AsPb(pb.AggregateSignal_AT_EPHEMERAL_WORKFLOW)...)
	return &pb.AggregateSignalsO{Signals: s}
}

func (m idSet) AsPb(
	typ pb.AggregateSignal_AggregateType,
) []*pb.AggregateSignal {
	if len(m) == 0 {
		return nil
	}

	a := make([]*pb.AggregateSignal, 0, len(m))
	for id, _ := range m {
		a = append(a, &pb.AggregateSignal{
			AggregateType: typ,
			Signals:       uint32(pb.AggregateSignal_S_UPDATED),
			EntityId:      id[:],
		})
	}
	return a
}
