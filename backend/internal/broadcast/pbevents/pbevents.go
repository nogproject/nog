package pbevents

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func WithId(ev pb.BroadcastEvent) (pb.BroadcastEvent, error) {
	id, err := ulid.New()
	if err != nil {
		return ev, err
	}
	ev.Id = id[:]
	return ev, nil
}

func NewGitRefUpdated(
	repo uuid.I, ref string, commit []byte,
) pb.BroadcastEvent {
	return pb.BroadcastEvent{
		Event: pb.BroadcastEvent_EV_BC_FSO_GIT_REF_UPDATED,
		BcChange: &pb.BcChange{
			EntityId:  repo[:],
			GitRef:    ref,
			GitCommit: commit,
		},
	}
}
