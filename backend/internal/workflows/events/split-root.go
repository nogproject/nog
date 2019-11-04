package events

import (
	"errors"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `WorkflowEvent_EV_FSO_SPLIT_ROOT_STARTED` aka `EvSplitRootStarted`.
// See split-root workflow aka splitrootwf.
type EvSplitRootStarted struct {
	RegistryId      uuid.I // only in split-root workflow.
	WorkflowId      uuid.I // only in workflow indexes.
	WorkflowEventId ulid.I // only in workflow indexes.
	GlobalRoot      string
	Host            string
	HostRoot        string
	MaxDepth        int32 // only in split-root workflow
	MinDiskUsage    int64 // only in split-root workflow
	MaxDiskUsage    int64 // only in split-root workflow
}

func (EvSplitRootStarted) WorkflowEvent() {}

func (ev *EvSplitRootStarted) validateWorkflow() error {
	if ev.RegistryId == uuid.Nil {
		return errors.New("nil RegistryId")
	}
	if ev.WorkflowId != uuid.Nil {
		return errors.New("non-nil WorkflowId")
	}
	if ev.WorkflowEventId != ulid.Nil {
		return errors.New("non-nil WorkflowEventId")
	}
	if ev.MaxDepth == 0 {
		return errors.New("zero MaxDepth")
	}
	if ev.MinDiskUsage == 0 {
		return errors.New("zero MinDiskUsage")
	}
	if ev.MaxDiskUsage == 0 {
		return errors.New("zero MaxDiskUsage")
	}
	return ev.validateCommon()
}

func (ev *EvSplitRootStarted) validateIndex() error {
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

func (ev *EvSplitRootStarted) validateCommon() error {
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

func NewPbSplitRootStartedWorkflow(ev *EvSplitRootStarted) pb.WorkflowEvent {
	if err := ev.validateWorkflow(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_STARTED,
		RegistryId: ev.RegistryId[:],
		FsoRootInfo: &pb.FsoRootInfo{
			GlobalRoot: ev.GlobalRoot,
			Host:       ev.Host,
			HostRoot:   ev.HostRoot,
		},
		FsoSplitRootParams: &pb.FsoSplitRootParams{
			MaxDepth:     ev.MaxDepth,
			MinDiskUsage: ev.MinDiskUsage,
			MaxDiskUsage: ev.MaxDiskUsage,
		},
	}
}

func NewPbSplitRootStartedIndex(ev *EvSplitRootStarted) pb.WorkflowEvent {
	if err := ev.validateIndex(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_STARTED,
		WorkflowId:      ev.WorkflowId[:],
		WorkflowEventId: ev.WorkflowEventId[:],
		FsoRootInfo: &pb.FsoRootInfo{
			GlobalRoot: ev.GlobalRoot,
			Host:       ev.Host,
			HostRoot:   ev.HostRoot,
		},
	}
}

func fromPbSplitRootStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_STARTED {
		panic("invalid event")
	}
	ev := &EvSplitRootStarted{
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

	p := evpb.FsoSplitRootParams
	if p != nil {
		ev.MaxDepth = p.MaxDepth
		ev.MinDiskUsage = p.MinDiskUsage
		ev.MaxDiskUsage = p.MaxDiskUsage
	}

	return ev, nil
}

// `WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_APPENDED` aka `EvSplitRootDuAppended`.
// See split-root workflow aka splitrootwf.
type EvSplitRootDuAppended struct {
	Path  string
	Usage int64
}

func (EvSplitRootDuAppended) WorkflowEvent() {}

func NewPbSplitRootDuAppended(ev *EvSplitRootDuAppended) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_APPENDED,
		PathDiskUsage: &pb.PathDiskUsage{
			Path:  ev.Path,
			Usage: ev.Usage,
		},
	}
}

func fromPbSplitRootDuAppended(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvSplitRootDuAppended{
		Path:  evpb.PathDiskUsage.Path,
		Usage: evpb.PathDiskUsage.Usage,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_COMPLETED` aka `EvSplitRootDuCompleted`.
// See split-root workflow aka splitrootwf.
type EvSplitRootDuCompleted struct {
	StatusCode    int32
	StatusMessage string // only if `StatusCode != 0`.
}

func (EvSplitRootDuCompleted) WorkflowEvent() {}

func NewPbSplitRootDuCompletedOk() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbSplitRootDuCompletedError(
	code int32, message string,
) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func fromPbSplitRootDuCompleted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	ev := &EvSplitRootDuCompleted{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_SPLIT_ROOT_SUGGESTION_APPENDED` aka `EvSplitRootSuggestionAppended`.
// See split-root workflow aka splitrootwf.
type EvSplitRootSuggestionAppended struct {
	Path       string
	Suggestion pb.FsoSplitRootSuggestion_Suggestion
}

func (EvSplitRootSuggestionAppended) WorkflowEvent() {}

func NewPbSplitRootSuggestionAppended(
	ev *EvSplitRootSuggestionAppended,
) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_SUGGESTION_APPENDED,
		FsoSplitRootSuggestion: &pb.FsoSplitRootSuggestion{
			Path:       ev.Path,
			Suggestion: ev.Suggestion,
		},
	}
}

func fromPbSplitRootSuggestionAppended(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	ev := &EvSplitRootSuggestionAppended{
		Path:       evpb.FsoSplitRootSuggestion.Path,
		Suggestion: evpb.FsoSplitRootSuggestion.Suggestion,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_SPLIT_ROOT_ANALYSIS_COMPLETED` aka
// `EvSplitRootAnalysisCompleted`.  See split-root workflow aka splitrootwf.
type EvSplitRootAnalysisCompleted struct {
	StatusCode    int32
	StatusMessage string // only if `StatusCode != 0`.
}

func (EvSplitRootAnalysisCompleted) WorkflowEvent() {}

func NewPbSplitRootAnalysisCompletedOk() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_ANALYSIS_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbSplitRootAnalysisCompletedError(
	code int32, message string,
) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_ANALYSIS_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func fromPbSplitRootAnalysisCompleted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	ev := &EvSplitRootAnalysisCompleted{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_SPLIT_ROOT_DECISION_APPENDED` aka `EvSplitRootDecisionAppended`.
// See split-root workflow aka splitrootwf.
type EvSplitRootDecisionAppended struct {
	Path     string
	Decision pb.FsoSplitRootDecision_Decision
}

func (EvSplitRootDecisionAppended) WorkflowEvent() {}

func NewPbSplitRootDecisionAppended(
	ev *EvSplitRootDecisionAppended,
) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DECISION_APPENDED,
		FsoSplitRootDecision: &pb.FsoSplitRootDecision{
			Path:     ev.Path,
			Decision: ev.Decision,
		},
	}
}

func fromPbSplitRootDecisionAppended(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	ev := &EvSplitRootDecisionAppended{
		Path:     evpb.FsoSplitRootDecision.Path,
		Decision: evpb.FsoSplitRootDecision.Decision,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_SPLIT_ROOT_COMPLETED` aka `EvSplitRootCompleted`.
// See split-root workflow aka splitrootwf.
type EvSplitRootCompleted struct {
	StatusCode      int32  // only in splitrootwf
	StatusMessage   string // only in splitrootwf
	WorkflowId      uuid.I // only in workflow indexes.
	WorkflowEventId ulid.I // only in workflow indexes.
}

func (EvSplitRootCompleted) WorkflowEvent() {}

func NewPbSplitRootCompletedOk() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbSplitRootCompletedError(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func NewPbSplitRootCompletedIdRef(id uuid.I, vid ulid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_COMPLETED,
		WorkflowId:      id[:],
		WorkflowEventId: vid[:],
	}
}

func fromPbSplitRootCompleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvSplitRootCompleted{
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

// `WorkflowEvent_EV_FSO_SPLIT_ROOT_COMMITTED` aka
// `EvSplitRootCommitted`.
type EvSplitRootCommitted struct{}

func (EvSplitRootCommitted) WorkflowEvent() {}

func NewPbSplitRootCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_COMMITTED,
	}
}

func fromPbSplitRootCommitted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	return &EvSplitRootCommitted{}, nil
}

// `WorkflowEvent_EV_FSO_SPLIT_ROOT_DELETED` aka `EvSplitRootDeleted`.
// See split-root workflow aka splitrootwf.
type EvSplitRootDeleted struct {
	WorkflowId uuid.I // only in workflow indexes.
}

func (EvSplitRootDeleted) WorkflowEvent() {}

func NewPbSplitRootDeleted(id uuid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DELETED,
		WorkflowId: id[:],
	}
}

func fromPbSplitRootDeleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvSplitRootDeleted{}
	id, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, err
	}
	ev.WorkflowId = id
	return ev, nil
}
