package events

import (
	"errors"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `WorkflowEvent_EV_FSO_DU_ROOT_STARTED` aka `EvDuRootStarted`.
// See du-root workflow aka durootwf.
type EvDuRootStarted struct {
	RegistryId      uuid.I // only in du-root workflow.
	WorkflowId      uuid.I // only in workflow indexes.
	WorkflowEventId ulid.I // only in workflow indexes.
	GlobalRoot      string
	Host            string
	HostRoot        string
}

func (EvDuRootStarted) WorkflowEvent() {}

func (ev *EvDuRootStarted) validateWorkflow() error {
	if ev.RegistryId == uuid.Nil {
		return errors.New("nil RegistryId")
	}
	if ev.WorkflowId != uuid.Nil {
		return errors.New("non-nil WorkflowId")
	}
	if ev.WorkflowEventId != ulid.Nil {
		return errors.New("non-nil WorkflowEventId")
	}
	return ev.validateCommon()
}

func (ev *EvDuRootStarted) validateIndex() error {
	if ev.RegistryId != uuid.Nil {
		return errors.New("non-nil RegistryId")
	}
	if ev.WorkflowId == uuid.Nil {
		return errors.New("non-nil WorkflowId")
	}
	if ev.WorkflowEventId == ulid.Nil {
		return errors.New("non-nil WorkflowEventId")
	}
	return ev.validateCommon()
}

func (ev *EvDuRootStarted) validateCommon() error {
	if ev.GlobalRoot == "" {
		return errors.New("empty GlobaPath")
	}
	if ev.Host == "" {
		return errors.New("empty Host")
	}
	if ev.HostRoot == "" {
		return errors.New("empty HostRoot")
	}
	return nil
}

func NewPbDuRootStartedWorkflow(ev *EvDuRootStarted) pb.WorkflowEvent {
	if err := ev.validateWorkflow(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_DU_ROOT_STARTED,
		RegistryId: ev.RegistryId[:],
		FsoRootInfo: &pb.FsoRootInfo{
			GlobalRoot: ev.GlobalRoot,
			Host:       ev.Host,
			HostRoot:   ev.HostRoot,
		},
	}
}

func NewPbDuRootStartedIndex(ev *EvDuRootStarted) pb.WorkflowEvent {
	if err := ev.validateIndex(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_DU_ROOT_STARTED,
		WorkflowId:      ev.WorkflowId[:],
		WorkflowEventId: ev.WorkflowEventId[:],
		FsoRootInfo: &pb.FsoRootInfo{
			GlobalRoot: ev.GlobalRoot,
			Host:       ev.Host,
			HostRoot:   ev.HostRoot,
		},
	}
}

func fromPbDuRootStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_DU_ROOT_STARTED {
		panic("invalid event")
	}
	ev := &EvDuRootStarted{
		GlobalRoot: evpb.FsoRootInfo.GlobalRoot,
		Host:       evpb.FsoRootInfo.Host,
		HostRoot:   evpb.FsoRootInfo.HostRoot,
	}
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

// `WorkflowEvent_EV_FSO_DU_UPDATED` aka `EvDuUpdated`.
// See du-root workflow aka durootwf.
type EvDuUpdated struct {
	Path  string
	Usage int64
}

func (EvDuUpdated) WorkflowEvent() {}

func NewPbDuUpdated(ev *EvDuUpdated) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_DU_UPDATED,
		PathDiskUsage: &pb.PathDiskUsage{
			Path:  ev.Path,
			Usage: ev.Usage,
		},
	}
}

func fromPbDuUpdated(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvDuUpdated{
		Path:  evpb.PathDiskUsage.Path,
		Usage: evpb.PathDiskUsage.Usage,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_DU_ROOT_COMPLETED` aka `EvDuRootCompleted`.
// See du-root workflow aka durootwf.
type EvDuRootCompleted struct {
	StatusCode      int32  // only in durootwf
	StatusMessage   string // only in durootwf
	WorkflowId      uuid.I // only in workflow indexes.
	WorkflowEventId ulid.I // only in workflow indexes.
}

func (EvDuRootCompleted) WorkflowEvent() {}

func NewPbDuRootCompletedOk() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_DU_ROOT_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbDuRootCompletedError(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_DU_ROOT_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func NewPbDuRootCompletedIdRef(id uuid.I, vid ulid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_DU_ROOT_COMPLETED,
		WorkflowId:      id[:],
		WorkflowEventId: vid[:],
	}
}

func fromPbDuRootCompleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvDuRootCompleted{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
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

// `WorkflowEvent_EV_FSO_DU_ROOT_COMMITTED` aka `EvDuRootCommitted`.
// See du-root workflow aka durootwf.
type EvDuRootCommitted struct{}

func (EvDuRootCommitted) WorkflowEvent() {}

func NewPbDuRootCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_DU_ROOT_COMMITTED,
	}
}

func fromPbDuRootCommitted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	return &EvDuRootCommitted{}, nil
}

// `WorkflowEvent_EV_FSO_DU_ROOT_DELETED` aka `EvDuRootDeleted`.
// See du-root workflow aka durootwf.
type EvDuRootDeleted struct {
	WorkflowId uuid.I // only in workflow indexes.
}

func (EvDuRootDeleted) WorkflowEvent() {}

func NewPbDuRootDeleted(id uuid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_DU_ROOT_DELETED,
		WorkflowId: id[:],
	}
}

func fromPbDuRootDeleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvDuRootDeleted{}
	id, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, err
	}
	ev.WorkflowId = id
	return ev, nil
}
