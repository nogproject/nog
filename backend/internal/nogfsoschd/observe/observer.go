package observe

import (
	"context"
	"errors"
	"fmt"
	slashpath "path"
	"strings"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsoschd/execute"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Processor interface {
	// `Process()` should handle most processing errors.  It may return
	// errors that are likely due to a shutdown, like `context.Canceled` or
	// termination of a child due to a signal.
	ProcessRepo(ctx context.Context, repo *execute.Repo) error
}

// `StateStore` is used to preserve journal locations across restarts.
// Concurrent operations on different keys must be safe.
type StateStore interface {
	// `LoadULID()` loads the journal position for `name` from permanent
	// storage.  If successful, it returns either a valid ULID or
	// `ulid.Nil` to indicate that there is no stored state.  It may return
	// errors, such as file I/O problems.
	LoadULID(name string) (ulid.I, error)
	SaveULID(name string, id ulid.I) error
}

type Config struct {
	Conn       *grpc.ClientConn
	RPCCreds   credentials.PerRPCCredentials
	StateStore StateStore
	Processor  Processor
	Registries []string
	Refs       []string
	Prefixes   []string
	Hosts      []string
}

type Observer struct {
	lg         Logger
	conn       *grpc.ClientConn
	rpcCreds   grpc.CallOption
	state      StateStore
	proc       Processor
	registries map[string]struct{}
	refs       map[string]struct{}
	prefixes   []string
	hosts      map[string]struct{}
}

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

func NewObserver(lg Logger, cfg *Config) *Observer {
	var prefixes []string
	for _, p := range cfg.Prefixes {
		prefixes = append(prefixes, slashpath.Clean(p))
	}

	registries := make(map[string]struct{})
	for _, r := range cfg.Registries {
		registries[r] = struct{}{}
	}

	refs := make(map[string]struct{})
	for _, r := range cfg.Refs {
		refs[r] = struct{}{}
	}

	hosts := make(map[string]struct{})
	for _, h := range cfg.Hosts {
		hosts[h] = struct{}{}
	}

	o := &Observer{
		lg:         lg,
		conn:       cfg.Conn,
		state:      cfg.StateStore,
		proc:       cfg.Processor,
		rpcCreds:   grpc.PerRPCCredentials(cfg.RPCCreds),
		registries: registries,
		refs:       refs,
		prefixes:   prefixes,
		hosts:      hosts,
	}
	if o.state == nil {
		o.state = NewVolatileStateStore()
	}
	return o
}

// `Watch()` currently processes each event separately.  A consequence is that
// the same repo is usually processed many times when starting from the event
// epoch.  The processor needs to ensure that repeated processing of a repo
// without changes is sufficiently efficient.
//
// Repeated events could be de-duplicated here, for example by first reading
// all events until `WillBlock` and gathering a set of repos that need
// processing, and then the repos only once.  But the logic would become more
// complex.  We keep the naive approach unless we observe practical perfomance
// problems.
func (o *Observer) Watch(ctx context.Context) error {
	// Watch broadcast as the only source that indicates that a shadow repo
	// needs attention.
	//
	// In principle, it would be possible to watch the registries for
	// `EV_FSO_REPO_ADDED` and then watch the specific repo for
	// `EV_FSO_SHADOW_REPO_CREATED` to detect a new shadow repo that needs
	// attention.  In practice, it is simpler to ensure that
	// `EV_BC_FSO_GIT_REF_UPDATED` is emitted after the shadow repo has
	// been created, so that watching the broadcast is sufficient.
	return o.watchBroadcastForever(ctx)
}

func (o *Observer) watchBroadcastForever(ctx context.Context) error {
	for {
		err := o.watchBroadcast(ctx)

		// Non-blocking check canceled before logging retry.
		//
		// Sleep a bit to handle signals to child processes gracefully.
		// If shutdown is initiated by SIGINT or SIGTERM, it is likely
		// that the signal has been delivered to child processes, too.
		// `watchBroadcast()` may have returned with an error that
		// indicates that a child has quit due to a signal.  There is a
		// race with canceling the context.  The sleep ensures that
		// main had time to cancel the context and `ctx.Done()` will
		// indicate the cancelation, avoiding a spurious "Will
		// retry..." message.
		time.Sleep(100 * time.Millisecond)
		select {
		default:
		case <-ctx.Done():
			return ctx.Err()
		}

		wait := 20 * time.Second
		o.lg.Errorw(
			"Will retry watch broadcast.",
			"module", "nogfsoschd",
			"err", err,
			"retryIn", wait,
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

func (o *Observer) watchBroadcast(ctx context.Context) error {
	// Cancel stream on return.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c := pb.NewBroadcastClient(o.conn)
	req := &pb.BroadcastEventsI{
		Channel: "all",
		Watch:   true,
	}
	tail, err := o.state.LoadULID("broadcast")
	if err != nil {
		return err
	}
	if tail != ulid.Nil {
		req.After = tail[:]
	}
	stream, err := c.Events(ctx, req, o.rpcCreds)
	if err != nil {
		return err
	}
	o.lg.Infow("Started watch broadcast.", "after", tail.String())

	for {
		rsp, err := stream.Recv()
		if err != nil {
			return err
		}
		for _, ev := range rsp.Events {
			evId, err := ulid.ParseBytes(ev.Id)
			if err != nil {
				return err
			}
			err = o.handleBroadcast(ctx, ev)
			if err != nil {
				return err
			}
			err = o.state.SaveULID("broadcast", evId)
			if err != nil {
				return err
			}
		}
	}
}

func (o *Observer) handleBroadcast(
	ctx context.Context, ev *pb.BroadcastEvent,
) error {
	switch ev.Event {
	case pb.BroadcastEvent_EV_BC_FSO_GIT_REF_UPDATED:
		return o.handleRefUpdated(ctx, ev)

	// Not interested:
	case pb.BroadcastEvent_EV_BC_FSO_MAIN_CHANGED:
	case pb.BroadcastEvent_EV_BC_FSO_REGISTRY_CHANGED:
	case pb.BroadcastEvent_EV_BC_FSO_REPO_CHANGED:
	default:
		o.lg.Warnw("Unknown event type", "ev", ev.Event.String())
	}
	return nil
}

func (o *Observer) handleRefUpdated(
	ctx context.Context, ev *pb.BroadcastEvent,
) error {
	if ev.BcChange == nil {
		return errors.New("invalid event")
	}
	repoId, err := uuid.FromBytes(ev.BcChange.EntityId)
	if err != nil {
		return err
	}

	// Silently ignore other refs.
	ref := ev.BcChange.GitRef
	if len(o.refs) > 0 {
		if _, ok := o.refs[ref]; !ok {
			return nil
		}
	}

	c := pb.NewReposClient(o.conn)
	repo, err := c.GetRepo(ctx, &pb.GetRepoI{Repo: repoId[:]}, o.rpcCreds)
	if err != nil {
		return err
	}

	// Silently ignore other registries.
	if _, ok := o.registries[repo.Registry]; !ok {
		return nil
	}
	// Silently ignore other prefix.
	if !pathIsEqualOrBelowPrefixAny(repo.GlobalPath, o.prefixes) {
		return nil
	}

	// Report host mismatch and ignore.
	host := strings.SplitN(repo.File, ":", 2)[0]
	if _, ok := o.hosts[host]; !ok {
		o.lg.Warnw(
			"Ignored prefix matched but host not.",
			"repoId", repoId.String(),
			"globalPath", repo.GlobalPath,
			"file", repo.File,
		)
		return nil
	}

	repoVid, err := ulid.ParseBytes(repo.Vid)
	if err != nil {
		return err
	}
	return o.proc.ProcessRepo(ctx, &execute.Repo{
		Id:                     repoId,
		Vid:                    repoVid,
		Registry:               repo.Registry,
		GlobalPath:             repo.GlobalPath,
		File:                   repo.File,
		Shadow:                 repo.Shadow,
		Archive:                repo.Archive,
		ArchiveRecipients:      asHexs(repo.ArchiveRecipients),
		ShadowBackup:           repo.ShadowBackup,
		ShadowBackupRecipients: asHexs(repo.ShadowBackupRecipients),
	})
}

func asHexs(ds [][]byte) []string {
	ss := make([]string, 0, len(ds))
	for _, d := range ds {
		ss = append(ss, fmt.Sprintf("%X", d))
	}
	return ss
}

// `prefix` without trailing slash.
func pathIsEqualOrBelowPrefix(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	// Equal or slash right after prefix.
	return len(path) == len(prefix) || path[len(prefix)] == '/'
}

// `prefixes` without trailing slash.
func pathIsEqualOrBelowPrefixAny(path string, prefixes []string) bool {
	for _, pfx := range prefixes {
		if pathIsEqualOrBelowPrefix(path, pfx) {
			return true
		}
	}
	return false
}
