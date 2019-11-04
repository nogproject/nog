package events

import (
	"errors"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `WorkflowEvent_EV_FSO_PING_REGISTRY_STARTED` aka `EvPingRegistryStarted`.
type EvPingRegistryStarted struct {
	RegistryId      uuid.I // only in ping-registry workflow.
	WorkflowId      uuid.I // only in workflow indexes.
	WorkflowEventId ulid.I // only in workflow indexes.
}

func (EvPingRegistryStarted) WorkflowEvent() {}

func (ev *EvPingRegistryStarted) validateWorkflow() error {
	if ev.RegistryId == uuid.Nil {
		return errors.New("nil RegistryId")
	}
	if ev.WorkflowId != uuid.Nil {
		return errors.New("non-nil WorkflowId")
	}
	if ev.WorkflowEventId != ulid.Nil {
		return errors.New("non-nil WorkflowEventId")
	}
	return nil
}

func (ev *EvPingRegistryStarted) validateIndex() error {
	if ev.RegistryId != uuid.Nil {
		return errors.New("non-nil RegistryId")
	}
	if ev.WorkflowId == uuid.Nil {
		return errors.New("non-nil WorkflowId")
	}
	if ev.WorkflowEventId == ulid.Nil {
		return errors.New("non-nil WorkflowEventId")
	}
	return nil
}

func NewPbPingRegistryStartedWorkflow(
	ev *EvPingRegistryStarted,
) pb.WorkflowEvent {
	if err := ev.validateWorkflow(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_PING_REGISTRY_STARTED,
		RegistryId: ev.RegistryId[:],
	}
}

func NewPbPingRegistryStartedIndex(
	ev *EvPingRegistryStarted,
) pb.WorkflowEvent {
	if err := ev.validateIndex(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_PING_REGISTRY_STARTED,
		WorkflowId:      ev.WorkflowId[:],
		WorkflowEventId: ev.WorkflowEventId[:],
	}
}

func fromPbPingRegistryStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_PING_REGISTRY_STARTED {
		panic("invalid event")
	}
	ev := &EvPingRegistryStarted{}
	if evpb.RegistryId != nil {
		id, err := uuid.FromBytes(evpb.RegistryId)
		if err != nil {
			return nil, err
		}
		ev.RegistryId = id
	}
	if evpb.WorkflowId != nil {
		id, err := uuid.FromBytes(evpb.WorkflowId)
		if err != nil {
			return nil, err
		}
		ev.WorkflowId = id
	}
	if evpb.WorkflowEventId != nil {
		vid, err := ulid.ParseBytes(evpb.WorkflowEventId)
		if err != nil {
			return nil, err
		}
		ev.WorkflowEventId = vid
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_SERVER_PINGED` aka `EvServerPinged`.
type EvServerPinged struct {
	StatusCode    int32
	StatusMessage string
}

func (EvServerPinged) WorkflowEvent() {}

func NewPbServerPinged(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_SERVER_PINGED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func fromPbServerPinged(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	return &EvServerPinged{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
	}, nil
}

// `WorkflowEvent_EV_FSO_SERVER_PINGS_GATHERED` aka `EvServerPingsGathered`.
type EvServerPingsGathered struct {
	StatusCode    int32
	StatusMessage string
}

func (EvServerPingsGathered) WorkflowEvent() {}

func NewPbServerPingsGathered(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_SERVER_PINGS_GATHERED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func fromPbServerPingsGathered(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	return &EvServerPingsGathered{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
	}, nil
}

// `WorkflowEvent_EV_FSO_PING_REGISTRY_COMPLETED` aka
// `EvPingRegistryCompleted`.
type EvPingRegistryCompleted struct {
	WorkflowId      uuid.I // only in workflow indexes.
	WorkflowEventId ulid.I // only in workflow indexes.
}

func (EvPingRegistryCompleted) WorkflowEvent() {}

func NewPbPingRegistryCompleted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_PING_REGISTRY_COMPLETED,
	}
}

func NewPbPingRegistryCompletedIdRef(id uuid.I, vid ulid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_PING_REGISTRY_COMPLETED,
		WorkflowId:      id[:],
		WorkflowEventId: vid[:],
	}
}

func fromPbPingRegistryCompleted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	ev := &EvPingRegistryCompleted{}
	if evpb.WorkflowId != nil {
		id, err := uuid.FromBytes(evpb.WorkflowId)
		if err != nil {
			return nil, err
		}
		ev.WorkflowId = id
	}
	if evpb.WorkflowEventId != nil {
		vid, err := ulid.ParseBytes(evpb.WorkflowEventId)
		if err != nil {
			return nil, err
		}
		ev.WorkflowEventId = vid
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_PING_REGISTRY_COMMITTED` aka
// `EvPingRegistryCommitted`.
type EvPingRegistryCommitted struct{}

func (EvPingRegistryCommitted) WorkflowEvent() {}

func NewPbPingRegistryCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_PING_REGISTRY_COMMITTED,
	}
}

func fromPbPingRegistryCommitted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	return &EvPingRegistryCommitted{}, nil
}

// `WorkflowEvent_EV_FSO_PING_REGISTRY_DELETED` aka `EvPingRegistryDeleted`.
// See ping-registry workflow aka pingregistrywf.
type EvPingRegistryDeleted struct {
	WorkflowId uuid.I // only in workflow indexes.
}

func (EvPingRegistryDeleted) WorkflowEvent() {}

func NewPbPingRegistryDeleted(id uuid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_PING_REGISTRY_DELETED,
		WorkflowId: id[:],
	}
}

func fromPbPingRegistryDeleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvPingRegistryDeleted{}
	id, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, err
	}
	ev.WorkflowId = id
	return ev, nil
}
