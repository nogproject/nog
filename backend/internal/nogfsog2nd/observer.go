package nogfsog2nd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/nogproject/nog/backend/internal/nogfsog2nd/gitnogd"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RegistryViewConfig struct {
	Registries []string
	Prefixes   []string
	Gitlabs    []string
}

type RegistryView struct {
	lg         Logger
	registries []string
	prefixes   []string
	gitlabs    map[string]bool

	lock  sync.Mutex
	repos map[uuid.I]*repoInfo
}

type repoInfo struct {
	id                 uuid.I
	globalPath         string
	gitlabHost         string
	gitlabPath         string
	gitlabProjectId    int64
	gitlabEnabledSince time.Time
}

func (v *RegistryView) GitlabProjectInfo(
	repoId uuid.I,
) *gitnogd.GitlabProjectInfo {
	v.lock.Lock()
	ri, ok := v.repos[repoId]
	v.lock.Unlock()
	if !ok {
		return nil
	}
	return &gitnogd.GitlabProjectInfo{
		Hostname:     ri.gitlabHost,
		ProjectId:    ri.gitlabProjectId,
		EnabledSince: ri.gitlabEnabledSince,
	}
}

func NewRegistryView(lg Logger, cfg *RegistryViewConfig) *RegistryView {
	var prefixes []string
	for _, p := range cfg.Prefixes {
		// Ensure trailing slash.
		p = strings.TrimRight(p, "/") + "/"
		prefixes = append(prefixes, p)
	}

	gitlabs := make(map[string]bool)
	for _, gl := range cfg.Gitlabs {
		gitlabs[gl] = true
	}

	return &RegistryView{
		lg:         lg,
		registries: cfg.Registries,
		prefixes:   prefixes,
		gitlabs:    gitlabs,
		repos:      make(map[uuid.I]*repoInfo),
	}
}

func (v *RegistryView) Watch(
	ctx context.Context, conn *grpc.ClientConn,
) error {
	var wg sync.WaitGroup

	wg.Add(len(v.registries))
	watchForever := func(r string) {
		defer wg.Done()
		var tail []byte
		for {
			var err error
			tail, err = v.watchRegistry(ctx, conn, r, tail)
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

func (v *RegistryView) watchRegistry(
	ctx context.Context, conn *grpc.ClientConn,
	registry string, tail []byte,
) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c := pb.NewRegistryClient(conn)
	stream, err := c.Events(ctx, &pb.RegistryEventsI{
		Registry: registry,
		Watch:    true,
		After:    tail,
	})
	if err != nil {
		return tail, err
	}

	processEvent := func(ev *pb.RegistryEvent) error {
		switch ev.Event {
		default:
			return nil

		case pb.RegistryEvent_EV_FSO_REPO_ADDED:
			fallthrough
		case pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED:
			repoId, err := uuid.FromBytes(ev.FsoRepoInfo.Id)
			if err != nil {
				v.lg.Errorw("impossible error", "err", err)
				return nil
			}
			waitGitlab := false
			return v.addRepo(ctx, conn, repoId, waitGitlab)

		case pb.RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED:
			repoId, err := uuid.FromBytes(ev.RepoId)
			if err != nil {
				v.lg.Errorw("impossible error", "err", err)
				return nil
			}
			waitGitlab := true
			return v.addRepo(ctx, conn, repoId, waitGitlab)
		}
	}

	for {
		rsp, err := stream.Recv()
		if err != nil {
			return tail, err
		}
		for _, ev := range rsp.Events {
			err := processEvent(ev)
			if err != nil {
				return tail, err
			}
			tail = ev.Id
		}
	}
}

func (v *RegistryView) addRepo(
	ctx context.Context, conn *grpc.ClientConn, repoId uuid.I,
	waitGitlab bool,
) error {
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	// `isCancel2` returns whether `err` indicates a cancel due to
	// `cancel2()`, that is the parent context has not been canceled.
	isCancel2 := func(err error) bool {
		s, ok := status.FromError(err)
		if !ok {
			return false
		}
		if s.Code() != codes.Canceled {
			return false
		}
		select {
		case <-ctx.Done(): // Parent is also canceled.
			return false
		default:
			return true
		}
	}

	c := pb.NewReposClient(conn)
	req := pb.RepoEventsI{
		Repo:  repoId[:],
		Watch: true,
	}
	stream, err := c.Events(ctx2, &req)
	if err != nil {
		return err
	}

	var initInfo *pb.FsoRepoInitInfo
	var gitInfo *pb.FsoGitRepoInfo
	repoHasError := false
	var enabledSince time.Time

	processEvent := func(ev *pb.RepoEvent) bool {
		switch ev.Event {
		case pb.RepoEvent_EV_FSO_REPO_INIT_STARTED:
			globalPath := ev.FsoRepoInitInfo.GlobalPath
			if !belowPrefix(v.prefixes, globalPath) {
				v.lg.Infow(
					"Ignored repo with foreign prefix.",
					"repo", globalPath,
				)
				return false
			}
			initInfo = ev.FsoRepoInitInfo

		case pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED:
			dup := *initInfo
			dup.GitlabHost = ev.FsoRepoInitInfo.GitlabHost
			dup.GitlabPath = ev.FsoRepoInitInfo.GitlabPath
			initInfo = &dup

		case pb.RepoEvent_EV_FSO_GIT_REPO_CREATED:
			gitInfo = ev.FsoGitRepoInfo
			evId, err := ulid.ParseBytes(ev.Id)
			if err != nil {
				// impossible.
				return false
			}
			enabledSince = ulid.Time(evId)

		case pb.RepoEvent_EV_FSO_REPO_ERROR_SET:
			repoHasError = true
		case pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED:
			repoHasError = false

		// Ignore unrelated:
		case pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED:
		case pb.RepoEvent_EV_FSO_GIT_TO_NOG_CLONED:
		case pb.RepoEvent_EV_FSO_ARCHIVE_RECIPIENTS_UPDATED:
		case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED:

		default: // Ignore unknown.
			v.lg.Warnw(
				"Ignored unknown repo event.",
				"module", "nogfsog2nd",
				"event", ev.Event.String(),
			)
		}

		return true
	}

	// Process until we have all we need.  If the immediate stream is
	// insufficient, block a bit before failing due to missing events.
	var timeout *time.Timer
	for {
		rsp, err := stream.Recv()
		if timeout != nil {
			timeout.Stop()
			timeout = nil
		}
		if err == io.EOF {
			err := fmt.Errorf(
				"unexpected end of repo event stream `%s`",
				repoId,
			)
			return err
		}
		if isCancel2(err) {
			err := fmt.Errorf(
				"timeout waiting for repo events "+
					"required for adding `%s`", repoId,
			)
			return err
		}
		if err != nil {
			return err
		}

		for _, ev := range rsp.Events {
			if !processEvent(ev) {
				return nil
			}
		}

		if !rsp.WillBlock {
			continue
		}

		// If stream would block, stop if:
		//
		// - no Gitlab expected state and waiting not forced;
		// - Gitlab expected and info available.
		//
		if initInfo != nil && initInfo.GitlabHost == "" {
			if !waitGitlab {
				break
			}
		}
		if initInfo != nil && gitInfo != nil {
			break
		}

		if repoHasError {
			err := fmt.Errorf(
				"immediate events are incomplete "+
					"and repo has error `%s`", repoId,
			)
			return err
		}

		// Wait a bit before canceling.
		timeout = time.AfterFunc(5*time.Second, cancel2)
	}

	gitlabHost := initInfo.GitlabHost
	if gitlabHost == "" {
		v.lg.Infow(
			"Ignored repo with empty Gitlab config.",
			"repo", repoId,
		)
		return nil
	}
	if _, ok := v.gitlabs[gitlabHost]; !ok {
		v.lg.Errorw(
			"Ignored repo due to missing GitLab client.",
			"repo", repoId,
			"gitlabHost", gitlabHost,
		)
		return nil
	}

	globalPath := initInfo.GlobalPath
	ri := &repoInfo{
		id:                 repoId,
		globalPath:         globalPath,
		gitlabHost:         gitlabHost,
		gitlabPath:         initInfo.GitlabPath,
		gitlabProjectId:    gitInfo.GitlabProjectId,
		gitlabEnabledSince: enabledSince,
	}
	v.lock.Lock()
	v.repos[ri.id] = ri
	v.lock.Unlock()

	v.lg.Infow("Activated repo", "repo", repoId)

	return nil
}

func belowPrefix(prefixes []string, path string) bool {
	for _, pfx := range prefixes {
		if strings.HasPrefix(path, pfx) {
			return true
		}
	}
	return false
}
