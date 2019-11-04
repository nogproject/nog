package pingregistrywf

import (
	"fmt"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
)

func IsPackageError(err error) bool {
	switch err.(type) {
	case *UninitializedError:
		return true
	case *InvalidCommandError:
		return true
	case *StateConflictError:
		return true
	case *ArgumentNotIdempotentError:
		return true
	case *NewEventsError:
		return true
	case *JournalError:
		return true
	case *EventTypeError:
		return true
	default:
		return false
	}
}

type UninitializedError struct{}

func (err *UninitializedError) Error() string {
	return "uninitialized"
}

type InvalidCommandError struct{}

func (err *InvalidCommandError) Error() string {
	return "invalid command"
}

type StateConflictError struct{}

func (err *StateConflictError) Error() string {
	return "command conflicts with aggregate state"
}

type ArgumentNotIdempotentError struct {
	Arg string
}

func (err *ArgumentNotIdempotentError) Error() string {
	return fmt.Sprintf("command argument `%s` not idempotent", err.Arg)
}

type NewEventsError struct {
	Err error
}

func (err *NewEventsError) Error() string {
	return "new events: " + err.Err.Error()
}
func (err *NewEventsError) Unwrap() error { return err.Err }

func wrapEvents(evs []events.Event, err error) ([]events.Event, error) {
	if err == nil {
		return evs, err
	}
	return evs, &NewEventsError{Err: err}
}

type JournalError struct {
	Err error
}

func (err *JournalError) Error() string {
	return "event journal: " + err.Err.Error()
}
func (err *JournalError) Unwrap() error { return err.Err }

func wrapVid(vid ulid.I, err error) (ulid.I, error) {
	return vid, wrapJournal(err)
}

func wrapJournal(err error) error {
	if err == nil || IsPackageError(err) {
		return err
	}
	return &JournalError{Err: err}
}

type EventTypeError struct{}

func (err *EventTypeError) Error() string {
	return "invalid event type"
}
