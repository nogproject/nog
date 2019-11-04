package gitnogdwatchlist

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `registryView` watches multiple registries to maintain a set of known repos.
type registryView struct {
	lg         Logger
	registries []string
	prefixes   []string
	origin     pb.RegistryClient

	mu    sync.Mutex
	repos map[uuid.I]bool
}

func newRegistryView(
	lg Logger,
	conn *grpc.ClientConn,
	cfg *Config,
) *registryView {
	var prefixes []string
	for _, p := range cfg.Prefixes {
		// Ensure trailing slash.
		p = strings.TrimRight(p, "/") + "/"
		prefixes = append(prefixes, p)
	}

	return &registryView{
		lg:         lg,
		registries: cfg.Registries,
		prefixes:   prefixes,
		origin:     pb.NewRegistryClient(conn),
		repos:      make(map[uuid.I]bool),
	}
}

func (v *registryView) enableRepo(id uuid.I) {
	v.mu.Lock()
	v.repos[id] = true
	v.mu.Unlock()
}

func (v *registryView) isKnownRepo(id uuid.I) bool {
	v.mu.Lock()
	_, ok := v.repos[id]
	v.mu.Unlock()
	return ok
}

func (v *registryView) watch(ctx context.Context) error {
	var wg sync.WaitGroup

	wg.Add(len(v.registries))
	watchForever := func(r string) {
		defer wg.Done()
		var tail []byte
		for {
			var err error
			tail, err = v.watchRegistry(ctx, r, tail)
			if s, ok := status.FromError(err); ok {
				if s.Code() == codes.Canceled {
					return
				}
			}

			wait := 20 * time.Second
			afterEvent := "Epoch"
			if tail != nil {
				afterEvent = fmt.Sprintf("%x", tail)
			}
			v.lg.Errorw(
				"Will retry watch registry.",
				"module", "nogfsog2nd",
				"err", err,
				"registry", r,
				"afterEvent", afterEvent,
				"retryIn", wait,
			)
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}
	for _, r := range v.registries {
		go watchForever(r)
	}

	wg.Wait()
	return ctx.Err()
}

func (v *registryView) watchRegistry(
	ctx context.Context,
	registry string, tail []byte,
) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := v.origin.Events(ctx, &pb.RegistryEventsI{
		Registry: registry,
		Watch:    true,
		After:    tail,
	})
	if err != nil {
		return tail, err
	}

	for {
		rsp, err := stream.Recv()
		if err != nil {
			return tail, err
		}
		for _, ev := range rsp.Events {
			if err := v.processEvent(ev); err != nil {
				return tail, err
			}
			tail = ev.Id
		}
	}
}

func (v *registryView) processEvent(ev *pb.RegistryEvent) error {
	switch ev.Event {
	case pb.RegistryEvent_EV_FSO_REPO_ADDED:
		inf := ev.FsoRepoInfo
		if !isBelowPrefix(v.prefixes, inf.GlobalPath) {
			v.lg.Infow(
				"Ignored repo with foreign prefix.",
				"repo", inf.GlobalPath,
			)
			return nil
		}
		repoId, err := uuid.FromBytes(inf.Id)
		if err != nil {
			return err
		}

		v.enableRepo(repoId)
		v.lg.Infow("Enabled repo.", "repoId", repoId.String())

	// Ignore unrelated.
	case pb.RegistryEvent_EV_FSO_REGISTRY_ADDED:
	case pb.RegistryEvent_EV_FSO_ROOT_ADDED:
	case pb.RegistryEvent_EV_FSO_ROOT_REMOVED:
	case pb.RegistryEvent_EV_FSO_ROOT_UPDATED:
	case pb.RegistryEvent_EV_FSO_REPO_NAMING_UPDATED:
	case pb.RegistryEvent_EV_FSO_REPO_NAMING_CONFIG_UPDATED:
	case pb.RegistryEvent_EV_FSO_REPO_ACCEPTED:
	case pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED:
	case pb.RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED:
	case pb.RegistryEvent_EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED:
	case pb.RegistryEvent_EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED:

	default:
		v.lg.Warnw(
			"Ignored unknown registry event.",
			"module", "nogfsog2nd",
			"event", ev.Event.String(),
		)
	}

	return nil
}
