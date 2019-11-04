package events

import (
	"fmt"

	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

type Op string

// `OpX` are operation codes for errors.  The `OpX` strings are chosen such
// that `${OpX} failed: ...` is valid English.
const (
	OpFindHead             Op = "finding history in heads collection"
	OpInsertHead              = "inserting into heads collection"
	OpUpdateHead              = "updating heads collection"
	OpUpdateHeadSerial        = "updating head serial"
	OpFindEvent               = "finding event in events collection"
	OpFindPreviousEvent       = "finding previous event in events collection"
	OpInsertEvent             = "inserting into events collection"
	OpInsertJournal           = "inserting journal entry"
	OpFindJournalDuplicate    = "finding duplicate journal entry"
	OpFindJournal             = "finding event in journal collection"
	OpScanJournal             = "scanning journal"
	OpFindStart               = "finding start"
	OpEventIds                = "computing event ids"
	OpEncodeEventProto        = "encoding event protobuf"
	OpDecodeJournalProto      = "decoding journal protobuf"
	OpDecodeEventProto        = "decoding event protobuf"
	OpParseEventId            = "parsing event ID"
	OpCheckEventId            = "checking event ID"
	OpParseEventParent        = "parsing event parent ID"
	OpHeadEventLookup         = "head event lookup"
	OpTailEventLookup         = "tail event lookup"
)

type UnknownHistoryError struct {
	Id uuid.I
}

func (err *UnknownHistoryError) Error() string {
	return fmt.Sprintf("unknown history %s", err.Id)
}

type VersionConflictError struct {
	Stored   ulid.I
	Expected ulid.I
}

func (err *VersionConflictError) Error() string {
	return fmt.Sprintf(
		"version conflict: stored VID %s != expected VID %s",
		err.Stored, err.Expected,
	)
}

func IsVersionConflictError(err error) bool {
	_, ok := err.(*VersionConflictError)
	return ok
}

type DBError struct {
	Op  Op
	Err error
}

func (err *DBError) Error() string {
	return "db: " + string(err.Op) + " failed: " +
		err.Err.Error()
}
func (err *DBError) Unwrap() error { return err.Err }

type InternalError struct {
	Op  Op
	Err error
}

func (err *InternalError) Error() string {
	return "internal: " + string(err.Op) + " failed: " +
		err.Err.Error()
}
func (err *InternalError) Unwrap() error { return err.Err }

type DuplicateEventMismatchError struct {
	Id ulid.I
}

func (err *DuplicateEventMismatchError) Error() string {
	return fmt.Sprintf("duplicate event %s mismatch", err.Id)
}

type DuplicateJournalEntrySerialMismatchError struct {
	HistoryId uuid.I
	EventId   ulid.I
	Stored    int64
	Expected  int64
}

func (err *DuplicateJournalEntrySerialMismatchError) Error() string {
	return fmt.Sprintf(
		"duplicate journal event %s:%s "+
			"stored serial %d != expected serial %d",
		err.HistoryId, err.EventId,
		err.Stored, err.Expected,
	)
}

type DuplicateJournalEntryProtobufMismatchError struct {
	HistoryId uuid.I
	EventId   ulid.I
}

func (err *DuplicateJournalEntryProtobufMismatchError) Error() string {
	return fmt.Sprintf(
		"duplicate journal event %s:%s protobuf mismatch",
		err.HistoryId, err.EventId,
	)
}

type CorruptedDataError struct {
	Op  Op
	Err error
}

func (err *CorruptedDataError) Error() string {
	return "data corrupted: " + string(err.Op) + " failed: " +
		err.Err.Error()
}
func (err *CorruptedDataError) Unwrap() error { return err.Err }

type EventError struct {
	Op  Op
	Err error
}

func (err *EventError) Error() string {
	return "event error: " + string(err.Op) + " failed: " +
		err.Err.Error()
}
func (err *EventError) Unwrap() error { return err.Err }

type JournalEntryParentMismatchError struct {
	EventId  ulid.I
	Actual   ulid.I
	Expected ulid.I
}

func (err *JournalEntryParentMismatchError) Error() string {
	return fmt.Sprintf(
		"journal entry %s actual parent %s != expected parent %s",
		err.EventId, err.Actual, err.Expected,
	)
}

type RetryNoVCError struct {
	Err error
}

func (err *RetryNoVCError) Error() string {
	return "last error after failed retries: " + err.Err.Error()
}
func (err *RetryNoVCError) Unwrap() error { return err.Err }

type MissingHeadEventError struct {
	Id ulid.I
}

func (err *MissingHeadEventError) Error() string {
	return fmt.Sprintf("missing head event %s", err.Id)
}

type MissingTailEventError struct {
	Id ulid.I
}

func (err *MissingTailEventError) Error() string {
	return fmt.Sprintf("missing tail event %s", err.Id)
}

type EventIdMismatchError struct {
	MongoId ulid.I
	ProtoId ulid.I
}

func (err *EventIdMismatchError) Error() string {
	return fmt.Sprintf(
		"Mongo event id %s != Protobuf event id %s",
		err.MongoId, err.ProtoId,
	)
}
