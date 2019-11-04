package events

import (
	"math/rand"
	"sync"
	"time"

	"github.com/nogproject/nog/backend/pkg/errorsx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `State` is the general aggregate state as managed by the `Engine`.  Specific
// aggregates, like `fsomain`, downcast to access their state.
type State interface {
	AggregateState()
	Id() uuid.I
	Vid() ulid.I
	SetVid(ulid.I)
}

// A `Command` can be applied to change `State`.
type Command interface {
	AggregateCommand()
}

// `Behavior` is used to customize `Engine` to implement a specific aggregate.
type Behavior interface {
	NewState(id uuid.I) State
	NewEvent() Event
	NewAdvancer() Advancer
	Tell(State, Command) ([]Event, error)
}

// `Advancer` applies events to `State` to produce updated `State`.
type Advancer interface {
	Advance(State, Event) State
}

// `Engine` combines an event `Journal` with `Behavior` into a building block
// for an event sourcing aggregate.  See `fsomain` as an example.
type Engine struct {
	lock  sync.Mutex // Protects `cache`.
	cache map[uuid.I]State

	events   *Journal
	behavior Behavior
}

func NewEngine(journal *Journal, behavior Behavior) *Engine {
	return &Engine{
		cache:    make(map[uuid.I]State),
		events:   journal,
		behavior: behavior,
	}
}

func (eng *Engine) FindId(id uuid.I) (State, error) {
	eng.lock.Lock()
	s, ok := eng.cache[id]
	eng.lock.Unlock()
	if !ok {
		s = eng.behavior.NewState(id)
		s.SetVid(EventEpoch)
	}
	return eng.FindFromState(s)
}

func (eng *Engine) FindFromState(s State) (State, error) {
	var a Advancer
	it := eng.events.Find(s.Id(), s.Vid())
	ev := eng.behavior.NewEvent()
	for it.Next(ev) {
		if a == nil {
			a = eng.behavior.NewAdvancer()
		}
		s = a.Advance(s, ev)
		s.SetVid(ev.Id())
	}
	if err := it.Close(); err != nil {
		// Do not wrap `err`.  It already is a package error, likely a
		// `DBError`.  There seems to be little value in providing
		// additional context.  The unwrapped error should be clear
		// enough for the caller of `FindFromState()`.
		return nil, err
	}

	// If there are no changes, return without updating the cache.  The
	// original state either is already cached or it can be trivially
	// re-created from scratch.
	if a == nil {
		return s, nil
	}

	eng.lock.Lock()
	eng.cache[s.Id()] = s
	eng.lock.Unlock()

	return s, nil
}

// `NoVC` is a sentinel that indicates `TellIdVid()` to skip the version check.
var NoVC = ulid.One

// `RetryNoVC` is a sentinel that indicates `TellIdVid()` to skip the version
// check like `NoVC` but retry the command a few times if committing events
// fails due to a concurrent update.
var RetryNoVC = ulid.Two

func (eng *Engine) TellIdVid(
	id uuid.I, vid ulid.I, cmd Command,
) (ulid.I, error) {
	s, err := eng.TellIdVidState(id, vid, cmd)
	if err != nil {
		return ulid.Nil, err
	}
	return s.Vid(), nil
}

func (eng *Engine) TellIdVidState(
	id uuid.I, vid ulid.I, cmd Command,
) (State, error) {
	switch vid {
	case NoVC:
		return eng.tellIdVidNoVC(id, cmd)
	case RetryNoVC:
		return eng.tellIdVidRetryNoVC(id, cmd)
	default:
		return eng.tellIdVidVC(id, vid, cmd)
	}
}

func (eng *Engine) tellIdVidNoVC(
	id uuid.I, cmd Command,
) (State, error) {
	s, err := eng.FindId(id)
	if err != nil {
		return nil, err // `err` is a package error.
	}
	return eng.TellState(s, cmd)
}

var cfgRetryN = 5
var cfgRetrySleepMin = 0 * time.Millisecond
var cfgRetrySleepJitter = 5 * time.Millisecond

func (eng *Engine) tellIdVidRetryNoVC(
	id uuid.I, cmd Command,
) (State, error) {
	i := 0
	for {
		s, err := eng.FindId(id)
		if err != nil {
			return nil, err // `err` is a package error.
		}
		vid, err := eng.TellState(s, cmd)
		switch {
		case err == nil:
			return vid, err
		case i == cfgRetryN:
			return vid, &RetryNoVCError{Err: err}
		case errorsx.IsPred(err, IsVersionConflictError):
			jitter := time.Duration(
				rand.Int63n(int64(cfgRetrySleepJitter)),
			)
			time.Sleep(cfgRetrySleepMin + jitter)
			i++
			continue
		default:
			// Do not wrap `err`.  It is a package error or a
			// behavior error.
			return vid, err
		}
	}
}

func (eng *Engine) tellIdVidVC(
	id uuid.I, vid ulid.I, cmd Command,
) (State, error) {
	s, err := eng.FindId(id)
	if err != nil {
		return nil, err // `err` is a package error.
	}
	if s.Vid() != vid {
		return nil, &VersionConflictError{
			Stored:   s.Vid(),
			Expected: vid,
		}
	}
	return eng.TellState(s, cmd)
}

func (eng *Engine) TellState(s State, cmd Command) (State, error) {
	evs, err := eng.tellStateEvents(s, cmd)
	if err != nil {
		// `err` is a behavior error.  Do not wrap it, so that the
		// caller aggregate package sees its original error.
		return nil, err
	}

	if evs == nil {
		return s, nil
	}

	a := eng.behavior.NewAdvancer()
	for _, ev := range evs {
		s = a.Advance(s, ev)
		s.SetVid(ev.Id())
	}

	eng.lock.Lock()
	eng.cache[s.Id()] = s
	eng.lock.Unlock()

	return s, nil
}

func (eng *Engine) tellStateEvents(s State, cmd Command) ([]Event, error) {
	evs, err := eng.behavior.Tell(s, cmd)
	if err != nil {
		// `err` is a behavior error.  Do not wrap it, so that the
		// caller aggregate package sees its original error.
		return nil, err
	}
	if evs == nil || len(evs) == 0 {
		return nil, nil
	}

	evs, err = eng.events.Commit(s.Id(), evs)
	if err != nil {
		return nil, err // `err` is a package error.
	}

	return evs, nil
}

// `DeleteIdVid()` deletes a history.  The history is not deleted immediately,
// but only marked for deletion.  The actual deletion of the journal, refs, and
// events happens during garbage collection.
//
// For `NoVC` and `RetryNoVC`, a missing or already deleted history does not
// cause an error.
//
// For a real `vid`, the history must exist and its state must match the `vid`,
// and the `Tell()` behavior command handler must allow deletion by returning
// `nil, nil`.
func (eng *Engine) DeleteIdVid(
	id uuid.I, vid ulid.I, cmd Command,
) error {
	switch vid {
	// So far, we have no obvious use case for delete with RetryNoVC.  For
	// simplicity, use the same implementation as NoVC.
	case NoVC, RetryNoVC:
		return eng.deleteIdVidNoVC(id, cmd)
	default:
		return eng.deleteIdVidVC(id, vid, cmd)
	}
}

func (eng *Engine) deleteIdVidNoVC(
	id uuid.I, cmd Command,
) error {
	// Without version check, return early if the history is empty to avoid
	// building state.
	h, err := eng.events.Head(id)
	if err != nil {
		return err // `err` is a package error.
	}
	if h == EventEpoch { // History is empty.
		return nil
	}

	s, err := eng.FindId(id)
	if err != nil {
		return err // `err` is a package error.
	}
	return eng.deleteState(s, cmd)
}

func (eng *Engine) deleteIdVidVC(
	id uuid.I, vid ulid.I, cmd Command,
) error {
	s, err := eng.FindId(id)
	if err != nil {
		return err // `err` is a package error.
	}
	if s.Vid() != vid {
		return &VersionConflictError{
			Stored:   s.Vid(),
			Expected: vid,
		}
	}
	return eng.deleteState(s, cmd)
}

func (eng *Engine) deleteState(s State, cmd Command) error {
	evs, err := eng.behavior.Tell(s, cmd)
	if err != nil {
		// `err` is a behavior error.  Do not wrap it, so that the
		// caller aggregate package sees its original error.
		return err
	}
	if len(evs) > 0 {
		panic("invalid delete behavior")
	}

	if err := eng.events.Delete(s.Id(), s.Vid()); err != nil {
		return err // `err` is a package error.
	}

	eng.lock.Lock()
	delete(eng.cache, s.Id())
	eng.lock.Unlock()

	return nil
}
