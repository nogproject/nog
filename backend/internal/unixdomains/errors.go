package unixdomains

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
	case *PreconditionError:
		return true
	case *GroupConflictError:
		return true
	case *UserConflictError:
		return true
	case *MissingUserError:
		return true
	case *MissingGroupError:
		return true
	case *CannotRemovePrimaryGroupError:
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

type PreconditionError struct {
	Reason string
}

func (err *PreconditionError) Error() string {
	return "precondition error: " + err.Reason
}

type GroupConflictError struct {
	AGroup string
	AGid   uint32
	BGroup string
	BGid   uint32
}

func (err *GroupConflictError) Error() string {
	return fmt.Sprintf(
		"conflicting groups %d(%s) and %d(%s)",
		err.AGid, err.AGroup,
		err.BGid, err.BGroup,
	)
}

type UserConflictError struct {
	AUser string
	AUid  uint32
	AGid  uint32
	BUser string
	BUid  uint32
	BGid  uint32
}

func (err *UserConflictError) Error() string {
	return fmt.Sprintf(
		"conflicting users uid=%d(%s),gid=%d and uid=%d(%s),gid=%d",
		err.AUid, err.AUser, err.AGid,
		err.BUid, err.BUser, err.BGid,
	)
}

type MissingUserError struct {
	Uid uint32
}

func (err *MissingUserError) Error() string {
	return fmt.Sprintf("missing user uid=%d", err.Uid)
}

type MissingGroupError struct {
	Gid uint32
}

func (err *MissingGroupError) Error() string {
	return fmt.Sprintf("missing group gid=%d", err.Gid)
}

type CannotRemovePrimaryGroupError struct {
	Gid uint32
}

func (err *CannotRemovePrimaryGroupError) Error() string {
	return fmt.Sprintf("cannot remove user from primary group %d", err.Gid)
}
