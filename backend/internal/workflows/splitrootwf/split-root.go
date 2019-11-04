package splitrootwf

import (
	"fmt"

	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfev "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const ConfigMaxDuPaths = 300
const ConfigMaxSuggestions = 300

var NoVC = events.NoVC
var RetryNoVC = events.RetryNoVC

const StatusCodeExpired = int32(pb.GetSplitRootO_SC_EXPIRED)

type StateCode int

const (
	StateUninitialized StateCode = iota
	StateInitialized

	StateDuAppending
	StateDuCompleted
	StateDuFailed

	StateSuggestionsAppending
	StateAnalysisCompleted
	StateAnalysisFailed

	StateDecisionsAppending

	StateCompleted
	StateFailed

	StateTerminated
)

type State struct {
	id    uuid.I
	vid   ulid.I
	scode StateCode

	statusCode    int32
	statusMessage string

	registryId   uuid.I
	globalRoot   string
	host         string
	hostRoot     string
	maxDepth     int32
	minDiskUsage int64
	maxDiskUsage int64

	du          []PathUsage
	suggestions []Suggestion
	decisions   []Decision
	candidates  map[string]struct{}
}

type PathUsage struct {
	Path  string
	Usage int64
}

type Suggestion struct {
	Path       string
	Suggestion pb.FsoSplitRootSuggestion_Suggestion
}

type Decision struct {
	Path     string
	Decision pb.FsoSplitRootDecision_Decision
}

type CmdInit struct {
	RegistryId   uuid.I
	GlobalRoot   string
	Host         string
	HostRoot     string
	MaxDepth     int32
	MinDiskUsage int64
	MaxDiskUsage int64
}

type CmdAppendDus struct {
	Paths []PathUsage
}

type CmdCommitDu struct{}

type CmdAbortDu struct {
	Code    int32
	Message string
}

type CmdAppendSuggestions struct {
	Suggestions []Suggestion
}

type CmdCommitAnalysis struct{}

type CmdAbortAnalysis struct {
	Code    int32
	Message string
}

type CmdAppendDecision struct {
	Path     string
	Decision pb.FsoSplitRootDecision_Decision
}

type CmdCommit struct{}

type CmdAbort struct {
	Code    int32
	Message string
}

type CmdAbortExpired struct{}

type CmdEnd struct{}

type CmdDelete struct{}

func (*State) AggregateState() {}

func (*CmdInit) AggregateCommand()              {}
func (*CmdAppendDus) AggregateCommand()         {}
func (*CmdCommitDu) AggregateCommand()          {}
func (*CmdAbortDu) AggregateCommand()           {}
func (*CmdAppendSuggestions) AggregateCommand() {}
func (*CmdCommitAnalysis) AggregateCommand()    {}
func (*CmdAbortAnalysis) AggregateCommand()     {}
func (*CmdAppendDecision) AggregateCommand()    {}
func (*CmdCommit) AggregateCommand()            {}
func (*CmdAbort) AggregateCommand()             {}
func (*CmdAbortExpired) AggregateCommand()      {}
func (*CmdEnd) AggregateCommand()               {}
func (*CmdDelete) AggregateCommand()            {}

func (s *State) Id() uuid.I        { return s.id }
func (s *State) Vid() ulid.I       { return s.vid }
func (s *State) SetVid(vid ulid.I) { s.vid = vid }

type Behavior struct{}
type Event struct{ wfev.Event }

func (Behavior) NewState(id uuid.I) events.State { return &State{id: id} }
func (Behavior) NewEvent() events.Event          { return &Event{} }
func (Behavior) NewAdvancer() events.Advancer    { return &Advancer{} }

// The bools indicate which part of the state has been duplicated.
type Advancer struct {
	state      bool // The state itself.
	candidates bool
}

func (ev *Event) UnmarshalProto(data []byte) error {
	if err := ev.Event.UnmarshalProto(data); err != nil {
		return err
	}
	switch ev.Event.PbWorkflowEvent().Event {
	default:
		return &EventTypeError{}
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_STARTED:
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_APPENDED:
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_SUGGESTION_APPENDED:
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_ANALYSIS_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DECISION_APPENDED:
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_COMMITTED:
	}
	return nil
}

func (a *Advancer) Advance(s events.State, ev events.Event) events.State {
	st := s.(*State)

	if !a.state {
		dup := *st
		st = &dup
		a.state = true
	}

	detachCandidates := func() {
		if a.candidates {
			return
		}
		dup := make(map[string]struct{})
		for k, v := range st.candidates {
			dup[k] = v
		}
		st.candidates = dup
		a.candidates = true
	}

	var evpb *pb.WorkflowEvent
	switch x := ev.(type) {
	case *Event: // Event from `UnmarshalProto()`
		evpb = x.PbWorkflowEvent()
	case *wfev.Event: // Event from `Tell()`
		evpb = x.PbWorkflowEvent()
	default:
		panic("invalid event")
	}
	switch x := wfev.MustParsePbWorkflowEvent(evpb).(type) {
	case *wfev.EvSplitRootStarted:
		st.scode = StateInitialized
		st.registryId = x.RegistryId
		st.globalRoot = x.GlobalRoot
		st.host = x.Host
		st.hostRoot = x.HostRoot
		st.maxDepth = x.MaxDepth
		st.minDiskUsage = x.MinDiskUsage
		st.maxDiskUsage = x.MaxDiskUsage
		return st

	case *wfev.EvSplitRootDuAppended:
		st.scode = StateDuAppending
		st.du = append(st.du, PathUsage{
			Path:  x.Path,
			Usage: x.Usage,
		})
		return st

	case *wfev.EvSplitRootDuCompleted:
		if x.StatusCode == 0 {
			st.scode = StateDuCompleted
		} else {
			st.scode = StateDuFailed
		}
		return st

	case *wfev.EvSplitRootSuggestionAppended:
		st.scode = StateSuggestionsAppending
		st.suggestions = append(st.suggestions, Suggestion{
			Path:       x.Path,
			Suggestion: x.Suggestion,
		})
		if x.Suggestion == pb.FsoSplitRootSuggestion_S_REPO_CANDIDATE {
			detachCandidates()
			st.candidates[x.Path] = struct{}{}
		}
		return st

	case *wfev.EvSplitRootAnalysisCompleted:
		if x.StatusCode == 0 {
			st.scode = StateAnalysisCompleted
		} else {
			st.scode = StateAnalysisFailed
		}
		return st

	case *wfev.EvSplitRootDecisionAppended:
		st.scode = StateDecisionsAppending
		st.decisions = append(st.decisions, Decision{
			Path:     x.Path,
			Decision: x.Decision,
		})
		detachCandidates()
		delete(st.candidates, x.Path)
		return st

	case *wfev.EvSplitRootCompleted:
		st.statusCode = x.StatusCode
		st.statusMessage = x.StatusMessage
		if x.StatusCode == 0 {
			st.scode = StateCompleted
		} else {
			st.scode = StateFailed
		}
		return st

	case *wfev.EvSplitRootCommitted:
		st.scode = StateTerminated
		return st

	default:
		panic("invalid event")
	}
}

func (Behavior) Tell(
	s events.State, c events.Command,
) ([]events.Event, error) {
	st := s.(*State)
	switch cmd := c.(type) {
	case *CmdInit:
		return tellInit(st, cmd)
	case *CmdAppendDus:
		return tellAppendDus(st, cmd)
	case *CmdCommitDu:
		return tellCommitDu(st, cmd)
	case *CmdAbortDu:
		return tellAbortDu(st, cmd)
	case *CmdAppendSuggestions:
		return tellAppendSuggestions(st, cmd)
	case *CmdCommitAnalysis:
		return tellCommitAnalysis(st, cmd)
	case *CmdAbortAnalysis:
		return tellAbortAnalysis(st, cmd)
	case *CmdAppendDecision:
		return tellAppendDecision(st, cmd)
	case *CmdCommit:
		return tellCommit(st, cmd)
	case *CmdAbort:
		return tellAbort(st, cmd)
	case *CmdAbortExpired:
		return tellAbortExpired(st, cmd)
	case *CmdEnd:
		return tellEnd(st, cmd)
	case *CmdDelete:
		return tellDelete(st, cmd)
	default:
		return nil, &InvalidCommandError{}
	}
}

func (cmd *CmdInit) isIdempotent(st *State) bool {
	return cmd.RegistryId == st.registryId &&
		cmd.GlobalRoot == st.globalRoot &&
		cmd.Host == st.host &&
		cmd.HostRoot == st.hostRoot &&
		cmd.MaxDepth == st.maxDepth &&
		cmd.MinDiskUsage == st.minDiskUsage &&
		cmd.MaxDiskUsage == st.maxDiskUsage
}

func tellInit(st *State, cmd *CmdInit) ([]events.Event, error) {
	// The command can only be idempotent if the workflow has not advanced
	// beyond init.
	switch st.scode {
	case StateUninitialized:
		break // Init is only allowed as the first command.
	case StateInitialized:
		// Check that args are idempotent.
		if !cmd.isIdempotent(st) {
			return nil, &NotIdempotentError{}
		}
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	// XXX Maybe validate cmd fields.

	ev := &wfev.EvSplitRootStarted{
		RegistryId:   cmd.RegistryId,
		GlobalRoot:   cmd.GlobalRoot,
		Host:         cmd.Host,
		HostRoot:     cmd.HostRoot,
		MaxDepth:     cmd.MaxDepth,
		MinDiskUsage: cmd.MinDiskUsage,
		MaxDiskUsage: cmd.MaxDiskUsage,
	}
	return wrapEvents(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootStartedWorkflow(ev),
	))
}

func tellAppendDus(st *State, cmd *CmdAppendDus) ([]events.Event, error) {
	switch st.scode {
	case StateInitialized:
		// Ok to start appending, continue below.
	case StateDuAppending:
		// Ok to continue appending.
		//
		// Checking duplicate paths seems not worth it, assuming that
		// Nogfsostad uses version control when appending results.
	default:
		return nil, &StateConflictError{}
	}

	// Check the total number of paths to catch unreasonable
	// configs that would cause many events.
	if len(st.du)+len(cmd.Paths) > ConfigMaxDuPaths {
		return nil, &ResourceExhaustedError{
			Err: fmt.Sprintf(
				"more than %d du paths",
				ConfigMaxDuPaths,
			),
		}
	}

	evs := make([]pb.WorkflowEvent, 0, len(cmd.Paths))
	for _, p := range cmd.Paths {
		ev := &wfev.EvSplitRootDuAppended{
			Path:  p.Path,
			Usage: p.Usage,
		}
		evs = append(evs, wfev.NewPbSplitRootDuAppended(ev))
	}
	return wfev.NewEvents(st.Vid(), evs...)
}

func tellCommitDu(st *State, cmd *CmdCommitDu) ([]events.Event, error) {
	switch st.scode {
	case StateInitialized:
		// A successful du requires disk usage for at least one path.
		return nil, &StateConflictError{}
	case StateDuAppending:
		break // Ok to commit with disk usage for at least one path.
	case StateDuCompleted:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootDuCompletedOk(),
	)
}

func tellAbortDu(st *State, cmd *CmdAbortDu) ([]events.Event, error) {
	switch st.scode {
	case StateInitialized:
		break // Ok to fail right away.
	case StateDuAppending:
		break // Ok to fail while appending du paths.
	case StateDuFailed:
		// XXX Maybe check that cmd fields are idempotent.
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootDuCompletedError(
			cmd.Code, cmd.Message,
		),
	)
}

func tellAppendSuggestions(
	st *State, cmd *CmdAppendSuggestions,
) ([]events.Event, error) {
	switch st.scode {
	case StateDuCompleted:
		break // Ok to start appending after du completed.
	case StateSuggestionsAppending:
		// Ok to continue appending.
	default:
		return nil, &StateConflictError{}
	}

	// Check the total number of paths to catch unreasonable
	// configs that would cause many events.
	if len(st.suggestions)+len(cmd.Suggestions) > ConfigMaxSuggestions {
		return nil, &ResourceExhaustedError{
			Err: fmt.Sprintf(
				"more than %d suggestions",
				ConfigMaxSuggestions,
			),
		}
	}

	evs := make([]pb.WorkflowEvent, 0, len(cmd.Suggestions))
	for _, s := range cmd.Suggestions {
		ev := &wfev.EvSplitRootSuggestionAppended{
			Path:       s.Path,
			Suggestion: s.Suggestion,
		}
		evs = append(evs, wfev.NewPbSplitRootSuggestionAppended(ev))
	}
	return wfev.NewEvents(st.Vid(), evs...)
}

func tellCommitAnalysis(
	st *State, cmd *CmdCommitAnalysis,
) ([]events.Event, error) {
	switch st.scode {
	case StateSuggestionsAppending:
		break // Ok to commit with a suggestions for at least one path.
	case StateAnalysisCompleted:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootAnalysisCompletedOk(),
	)
}

func tellAbortAnalysis(st *State, cmd *CmdAbortAnalysis) ([]events.Event, error) {
	switch st.scode {
	case StateDuCompleted:
		break // Ok to fail right after du completed.
	case StateSuggestionsAppending:
		break // Ok to fail while appending suggestions path.
	case StateAnalysisFailed:
		// XXX Maybe check that cmd fields are idempotent.
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootAnalysisCompletedError(
			cmd.Code, cmd.Message,
		),
	)
}

func tellAppendDecision(
	st *State, cmd *CmdAppendDecision,
) ([]events.Event, error) {
	switch st.scode {
	case StateAnalysisCompleted:
		break // Ok to start appending after analysis completed.
	case StateDecisionsAppending:
		// Ok to continue appending.
	default:
		return nil, &StateConflictError{}
	}

	if !st.IsCandidate(cmd.Path) {
		return nil, &NotCandidateError{
			Path: cmd.Path,
		}
	}

	ev := &wfev.EvSplitRootDecisionAppended{
		Path:     cmd.Path,
		Decision: cmd.Decision,
	}
	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootDecisionAppended(ev),
	)
}

func tellCommit(st *State, cmd *CmdCommit) ([]events.Event, error) {
	switch st.scode {
	case StateAnalysisCompleted:
		break // Ok to complete if analysis did not yield candidates.
	case StateDecisionsAppending:
		break // Ok to complete when all candidates have been decided.
	case StateCompleted:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	if len(st.candidates) > 0 {
		return nil, &UndecidedCandidatesError{}
	}

	return wrapEvents(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootCompletedOk(),
	))
}

// Abort must be used to end a workflow from the following states:
//
//  - StateDuFailed: The analysis cannot be performed without disk
//    usage information.
//  - StateAnalysisFailed: There is no reason to continue if the analysis
//    failed.
//  - StateAnalysisCompleted: An admin may abort when the suggestions are ready
//    without posting decisions.
//
func tellAbort(st *State, cmd *CmdAbort) ([]events.Event, error) {
	switch st.scode {
	case StateDuFailed:
		break // Ok to abort.
	case StateAnalysisFailed:
		break // Ok to abort.
	case StateAnalysisCompleted:
		break // Ok for admin to abort without posting decisions.
	case StateFailed:
		// XXX Maybe check that the cmd fields do not obviously
		// conflict with idempotency.
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootCompletedError(
			cmd.Code, cmd.Message,
		),
	)
}

// AbortExpired can be used to abort an workflow from any initialized state.
func tellAbortExpired(
	st *State, cmd *CmdAbortExpired,
) ([]events.Event, error) {
	switch st.scode {
	case StateCompleted:
		return nil, nil // effectively idempotent
	case StateFailed:
		return nil, nil // effectively idempotent
	case StateTerminated:
		return nil, nil // effectively idempotent
	case StateUninitialized:
		return nil, &StateConflictError{}
	default:
		break // Abort from any state except the ones above.
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootCompletedError(
			StatusCodeExpired, "expired",
		),
	)
}

func tellEnd(st *State, cmd *CmdEnd) ([]events.Event, error) {
	switch st.scode {
	case StateCompleted:
		break // `End()` is allowed after `Commit()`.
	case StateFailed:
		break // `End()` is allowed after `Abort()`.
	case StateTerminated:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEvents(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootCommitted(),
	))
}

func tellDelete(st *State, cmd *CmdDelete) ([]events.Event, error) {
	switch st.scode {
	// Unitialized is the idempotent result of `Delete()`.
	case StateUninitialized:
		return nil, nil

	// `Delete()` is allowed if `End()` is missing.
	case StateCompleted:
		return nil, nil
	case StateFailed:
		return nil, nil

	// `Delete()` is allowed after `End()`.
	case StateTerminated:
		return nil, nil

	default:
		return nil, &StateConflictError{}
	}
}

type Workflows struct {
	engine *events.Engine
}

func New(journal *events.Journal) *Workflows {
	return &Workflows{
		engine: events.NewEngine(journal, Behavior{}),
	}
}

func (r *Workflows) FindId(id uuid.I) (*State, error) {
	st, err := r.engine.FindId(id)
	if err != nil {
		return nil, &JournalError{Err: err}
	}
	if st.Vid() == events.EventEpoch {
		return nil, &UninitializedError{}
	}
	return st.(*State), nil
}

func (r *Workflows) Init(id uuid.I, cmd *CmdInit) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, NoVC, cmd))
}

func (r *Workflows) AppendDu(
	id uuid.I, vid ulid.I,
	path string, usage int64,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, NoVC, &CmdAppendDus{
		Paths: []PathUsage{
			{Path: path, Usage: usage},
		},
	}))
}

func (r *Workflows) AppendDus(
	id uuid.I, vid ulid.I, dus []PathUsage,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, NoVC, &CmdAppendDus{
		Paths: dus,
	}))
}

func (r *Workflows) CommitDu(
	id uuid.I, vid ulid.I,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, NoVC, &CmdCommitDu{}))
}

func (r *Workflows) AbortDu(
	id uuid.I, vid ulid.I, code int32, message string,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdAbortDu{
		Code:    code,
		Message: message,
	}))
}

func (r *Workflows) AppendSuggestion(
	id uuid.I, vid ulid.I,
	path string, suggestion pb.FsoSplitRootSuggestion_Suggestion,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, NoVC, &CmdAppendSuggestions{
		Suggestions: []Suggestion{
			{Path: path, Suggestion: suggestion},
		},
	}))
}

func (r *Workflows) AppendSuggestions(
	id uuid.I, vid ulid.I, suggestions []Suggestion,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, NoVC, &CmdAppendSuggestions{
		Suggestions: suggestions,
	}))
}

func (r *Workflows) CommitAnalysis(
	id uuid.I, vid ulid.I,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, NoVC, &CmdCommitAnalysis{}))
}

func (r *Workflows) AbortAnalysis(
	id uuid.I, vid ulid.I, code int32, message string,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdAbortAnalysis{
		Code:    code,
		Message: message,
	}))
}

func (r *Workflows) AppendDecision(
	id uuid.I, vid ulid.I,
	path string, decision pb.FsoSplitRootDecision_Decision,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, NoVC, &CmdAppendDecision{
		Path:     path,
		Decision: decision,
	}))
}

func (r *Workflows) Commit(
	id uuid.I, vid ulid.I,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdCommit{}))
}

func (r *Workflows) Abort(
	id uuid.I, vid ulid.I, code int32, message string,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdAbort{
		Code:    code,
		Message: message,
	}))
}

func (r *Workflows) AbortExpired(id uuid.I, vid ulid.I) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdAbortExpired{}))
}

func (r *Workflows) End(id uuid.I, vid ulid.I) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdEnd{}))
}

func (w *Workflows) Delete(id uuid.I, vid ulid.I) error {
	return wrapJournal(w.engine.DeleteIdVid(id, vid, &CmdDelete{}))
}

func (st *State) RegistryId() uuid.I {
	return st.registryId
}

func (st *State) GlobalRoot() string {
	return st.globalRoot
}

func (st *State) Du() []PathUsage {
	return st.du
}

func (st *State) Suggestions() []Suggestion {
	return st.suggestions
}

func (st *State) Decisions() []Decision {
	return st.decisions
}

func (st *State) StateCode() StateCode {
	return st.scode
}

func (st *State) IsCandidate(path string) bool {
	_, ok := st.candidates[path]
	return ok
}

func (st *State) StatusCode() int32 {
	return st.statusCode
}

func (st *State) StatusMessage() string {
	return st.statusMessage
}
