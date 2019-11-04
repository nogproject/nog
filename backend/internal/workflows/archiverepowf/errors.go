package archiverepowf

import (
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
	case *AlreadyTerminatedError:
		return true
	case *NotIdempotentError:
		return true
	case *NewEventsError:
		return true
	case *JournalError:
		return true
	case *EventTypeError:
		return true
	case *ArgumentError:
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

type AlreadyTerminatedError struct{}

func (err *AlreadyTerminatedError) Error() string {
	return "already terminated"
}

type NotIdempotentError struct {
}

func (err *NotIdempotentError) Error() string {
	return "command not idempotent"
}

type NewEventsError struct {
	Err error
}

func (err *NewEventsError) Error() string {
	return "new events: " + err.Err.Error()
}
func (err *NewEventsError) Unwrap() error { return err.Err }

func wrapEventsNewEventsError(
	evs []events.Event, err error,
) ([]events.Event, error) {
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

func wrapVidJournalError(vid ulid.I, err error) (ulid.I, error) {
	return vid, wrapJournalError(err)
}

func wrapJournalError(err error) error {
	if err == nil || IsPackageError(err) {
		return err
	}
	return &JournalError{Err: err}
}

type EventTypeError struct{}

func (err *EventTypeError) Error() string {
	return "invalid event type"
}

type ArgumentError struct {
	Reason string
}

func (err *ArgumentError) Error() string {
	return "argument error: " + err.Reason
}
