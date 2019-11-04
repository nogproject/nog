/*

Package `workflows/events` helps with handling lowlevel `WorkflowEvent` protos.

Use `NewPb<event>()` to create valid `WorkflowEvent` protos.  Use
`ParsePbWorkflowEvent()` to parse protos to higher level structs `Ev<event>`,
which are then typically used in a type switch, like:

    switch x := events.MustParsePbWorkflowEvent(evpb).(type) {
    case *wfevents.EvShadowRepoMoveStarted:
        ...
    }

See packages `workflows/*wf` for details about individual workflows:

 - du-root: `../durootwf/du-root.go`
 - move-repo: `../moverepowf/move-repo.go`
 - move-shadow: `../moveshadowwf/move-shadow.go`
 - ping-registry: `../pingregistrywf/ping-registry.go`

To find all locations where a certain event is processed, you need to grep for
both `EV_FSO_...` and `Ev...`, like:

    ev=EV_FSO_SHADOW_REPO_MOVE_STARTED
    evrgx="\(${ev}\|Ev$(sed <<<"${ev}" -e 's/^.*EV_FSO_//' -e 's/_//g' | tr 'A-Z' 'a-z')\)"
    git grep -i "${evrgx}"

It is often useful to specifically grep for the cases in switch statements:

    git grep -i "case.*${evrgx}:"

Struct `Event` can be used with package `internal/events` to implement event
sourcing, see specifically `internal/events.NewEngine()` and
`internal/events.Behavior.NewEvent()`.

*/
package events

import (
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
)

type Event struct {
	id     ulid.I
	parent ulid.I
	pb     pb.WorkflowEvent
}

func NewEvents(
	parent ulid.I, pbs ...pb.WorkflowEvent,
) ([]events.Event, error) {
	evs := make([]events.Event, 0, len(pbs))
	for _, pb := range pbs {
		id, err := ulid.New()
		if err != nil {
			return nil, err
		}
		e := &Event{id: id, parent: parent, pb: pb}
		e.pb.Id = e.id[:]
		e.pb.Parent = e.parent[:]
		evs = append(evs, e)
		parent = id
	}
	return evs, nil
}

func (e *Event) MarshalProto() ([]byte, error) {
	return proto.Marshal(&e.pb)
}

func (e *Event) UnmarshalProto(data []byte) error {
	var err error
	if err = proto.Unmarshal(data, &e.pb); err != nil {
		return err
	}
	if e.id, err = ulid.ParseBytes(e.pb.Id); err != nil {
		return err
	}
	if e.parent, err = ulid.ParseBytes(e.pb.Parent); err != nil {
		return err
	}
	if _, err := ParsePbWorkflowEvent(&e.pb); err != nil {
		err := errors.New("invalid event details")
		return err
	}
	return nil
}

func (e *Event) Id() ulid.I     { return e.id }
func (e *Event) Parent() ulid.I { return e.parent }

// Receiver by value.
func (e Event) WithId(id ulid.I) events.Event {
	e.id = id
	e.pb.Id = e.id[:]
	return &e
}

// Receiver by value.
func (e Event) WithParent(parent ulid.I) events.Event {
	e.parent = parent
	e.pb.Parent = e.parent[:]
	return &e
}

func (e *Event) PbWorkflowEvent() *pb.WorkflowEvent {
	return &e.pb
}

type WorkflowEvent interface {
	WorkflowEvent()
}

// `ParsePbWorkflowEvent()` converts a protobuf struct to an `Ev*` struct.
func ParsePbWorkflowEvent(
	evpb *pb.WorkflowEvent,
) (ev WorkflowEvent, err error) {
	switch evpb.Event {
	case pb.WorkflowEvent_EV_SNAPSHOT_BEGIN:
		return fromPbSnapshotBegin(evpb)

	case pb.WorkflowEvent_EV_SNAPSHOT_END:
		return fromPbSnapshotEnd(evpb)

	case pb.WorkflowEvent_EV_WORKFLOW_INDEX_SNAPSHOT_STATE:
		return fromPbWorkflowIndexSnapshotState(evpb)

	case pb.WorkflowEvent_EV_FSO_REPO_MOVE_STARTED:
		return fromPbRepoMoveStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_REPO_MOVE_STA_RELEASED:
		return fromPbRepoMoveStaReleased(evpb)

	case pb.WorkflowEvent_EV_FSO_REPO_MOVE_APP_ACCEPTED:
		return fromPbRepoMoveAppAccepted(evpb)

	case pb.WorkflowEvent_EV_FSO_REPO_MOVE_COMMITTED:
		return fromPbRepoMoveCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_REPO_MOVED:
		return fromPbRepoMoved(evpb)

	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		return fromPbShadowRepoMoveStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVED:
		return fromPbShadowRepoMoved(evpb)

	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STA_DISABLED:
		return fromPbShadowRepoMoveStaDisabled(evpb)

	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_COMMITTED:
		return fromPbShadowRepoMoveCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_DU_ROOT_STARTED:
		return fromPbDuRootStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_DU_UPDATED:
		return fromPbDuUpdated(evpb)

	case pb.WorkflowEvent_EV_FSO_DU_ROOT_COMPLETED:
		return fromPbDuRootCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_DU_ROOT_COMMITTED:
		return fromPbDuRootCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_DU_ROOT_DELETED:
		return fromPbDuRootDeleted(evpb)

	case pb.WorkflowEvent_EV_FSO_PING_REGISTRY_STARTED:
		return fromPbPingRegistryStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_SERVER_PINGED:
		return fromPbServerPinged(evpb)

	case pb.WorkflowEvent_EV_FSO_SERVER_PINGS_GATHERED:
		return fromPbServerPingsGathered(evpb)

	case pb.WorkflowEvent_EV_FSO_PING_REGISTRY_COMPLETED:
		return fromPbPingRegistryCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_PING_REGISTRY_COMMITTED:
		return fromPbPingRegistryCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_PING_REGISTRY_DELETED:
		return fromPbPingRegistryDeleted(evpb)

	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_STARTED:
		return fromPbSplitRootStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_APPENDED:
		return fromPbSplitRootDuAppended(evpb)

	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_COMPLETED:
		return fromPbSplitRootDuCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_SUGGESTION_APPENDED:
		return fromPbSplitRootSuggestionAppended(evpb)

	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_ANALYSIS_COMPLETED:
		return fromPbSplitRootAnalysisCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DECISION_APPENDED:
		return fromPbSplitRootDecisionAppended(evpb)

	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_COMPLETED:
		return fromPbSplitRootCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_COMMITTED:
		return fromPbSplitRootCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DELETED:
		return fromPbSplitRootDeleted(evpb)

	case pb.WorkflowEvent_EV_FSO_FREEZE_REPO_STARTED_2:
		return fromPbFreezeRepoStarted2(evpb)

	case pb.WorkflowEvent_EV_FSO_FREEZE_REPO_FILES_STARTED:
		return fromPbFreezeRepoFilesStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_FREEZE_REPO_FILES_COMPLETED:
		return fromPbFreezeRepoFilesCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
		return fromPbFreezeRepoCompleted2(evpb)

	case pb.WorkflowEvent_EV_FSO_FREEZE_REPO_COMMITTED:
		return fromPbFreezeRepoCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_FREEZE_REPO_DELETED:
		return fromPbFreezeRepoDeleted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
		return fromPbUnfreezeRepoStarted2(evpb)

	case pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_STARTED:
		return fromPbUnfreezeRepoFilesStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_COMPLETED:
		return fromPbUnfreezeRepoFilesCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
		return fromPbUnfreezeRepoCompleted2(evpb)

	case pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMMITTED:
		return fromPbUnfreezeRepoCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_DELETED:
		return fromPbUnfreezeRepoDeleted(evpb)

	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_STARTED:
		return fromPbArchiveRepoStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_STARTED:
		return fromPbArchiveRepoFilesStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_TARTT_COMPLETED:
		return fromPbArchiveRepoTarttCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_SWAP_STARTED:
		return fromPbArchiveRepoSwapStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMPLETED:
		return fromPbArchiveRepoFilesCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMMITTED:
		return fromPbArchiveRepoFilesCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_GC_COMPLETED:
		return fromPbArchiveRepoGcCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
		return fromPbArchiveRepoCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMMITTED:
		return fromPbArchiveRepoCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_DELETED:
		return fromPbArchiveRepoDeleted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
		return fromPbUnarchiveRepoStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_STARTED:
		return fromPbUnarchiveRepoFilesStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_STARTED:
		return fromPbUnarchiveRepoTarttStarted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_COMPLETED:
		return fromPbUnarchiveRepoTarttCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_COMPLETED:
		return fromPbUnarchiveRepoFilesCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_COMMITTED:
		return fromPbUnarchiveRepoFilesCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_GC_COMPLETED:
		return fromPbUnarchiveRepoGcCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
		return fromPbUnarchiveRepoCompleted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMMITTED:
		return fromPbUnarchiveRepoCommitted(evpb)

	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_DELETED:
		return fromPbUnarchiveRepoDeleted(evpb)

	default:
		return nil, errors.New("unknown WorkflowEvent type")
	}
}

func MustParsePbWorkflowEvent(evpb *pb.WorkflowEvent) WorkflowEvent {
	ev, err := ParsePbWorkflowEvent(evpb)
	if err != nil {
		panic(err)
	}
	return ev
}
