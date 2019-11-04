package gitnogdwatchlist

import (
	"context"
	"errors"
	"io"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

var errRepoNotFound = errors.New("repo not found")

// `store.repo(id)` load events from the registry for repo `id` and returns the
// current state that is relevant for `gitnogd`.
type store struct {
	lg     Logger
	origin pb.ReposClient
}

type repo struct {
	id                 uuid.I
	vid                ulid.I
	globalPath         string
	gitlabHost         string
	gitlabProjectId    int
	gitlabEnabledSince time.Time
}

func newStore(lg Logger, conn *grpc.ClientConn) *store {
	return &store{
		lg:     lg,
		origin: pb.NewReposClient(conn),
	}
}

// XXX We could add caching here.  But it's probably fast enough without until
// we observe a relevant load in practice.
func (s *store) repo(ctx context.Context, id uuid.I) (*repo, error) {
	r := &repo{id: id}
	err := s.applyEvents(ctx, r)
	if err != nil {
		return nil, err
	}

	if r.vid == ulid.Nil {
		return nil, errRepoNotFound
	}

	return r, nil
}

func (s *store) applyEvents(ctx context.Context, r *repo) error {
	i := &pb.RepoEventsI{
		Repo: r.id[:],
	}
	if r.vid != ulid.Nil {
		i.After = r.vid[:]
	}
	stream, err := s.origin.Events(ctx, i)
	if err != nil {
		return err
	}

	for {
		rsp, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		for _, ev := range rsp.Events {
			if err := s.applyEvent(r, ev); err != nil {
				return err
			}
		}
	}
}

func (s *store) applyEvent(r *repo, ev *pb.RepoEvent) error {
	evId, err := ulid.ParseBytes(ev.Id)
	if err != nil {
		return err // impossible.
	}

	switch ev.Event {
	case pb.RepoEvent_EV_FSO_REPO_INIT_STARTED:
		inf := ev.FsoRepoInitInfo
		r.globalPath = inf.GlobalPath
		r.gitlabHost = inf.GitlabHost

	case pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED:
		r.gitlabHost = ev.FsoRepoInitInfo.GitlabHost

	case pb.RepoEvent_EV_FSO_GIT_REPO_CREATED:
		r.gitlabProjectId = int(ev.FsoGitRepoInfo.GitlabProjectId)
		r.gitlabEnabledSince = ulid.Time(evId)

	// Silently ignore unrelated:
	case pb.RepoEvent_EV_FSO_REPO_ERROR_SET:
	case pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED:
	case pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED:
	case pb.RepoEvent_EV_FSO_GIT_TO_NOG_CLONED:
	case pb.RepoEvent_EV_FSO_ARCHIVE_RECIPIENTS_UPDATED:
	case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED:

	default:
		s.lg.Warnw(
			"Ignored unknown repo event.",
			"module", "nogfsog2nd",
			"event", ev.Event.String(),
		)
	}

	r.vid = evId

	return nil
}
