package events

import (
	"bytes"
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/nogproject/nog/backend/internal/eventspb"
	"github.com/nogproject/nog/backend/pkg/idid"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	mgo "gopkg.in/mgo.v2"
	bson "gopkg.in/mgo.v2/bson"
)

type PhaseCode int32

const (
	PhaseUnspecified PhaseCode = iota
	PhaseActive
	PhaseDeleting
	PhaseDeleted
)

// `PhaseUnspecified` is treated as `PhaseActive` for backward compatibility
// with old MongoDB docs that were created without phase.
func (p PhaseCode) IsActive() bool {
	return p == PhaseUnspecified || p == PhaseActive
}

func (p PhaseCode) IsInactive() bool {
	return !p.IsActive()
}

// `KeyX` is the Mongo field name for the Go field `X`.
const (
	KeyId       = "_id"
	KeyProtobuf = "pb"
	KeyHead     = "h"
	KeyEpoch    = "e"
	KeyEpochLog = "el"
	KeyTail     = "t"
	KeyPhase    = "ph"
	KeyDtime    = "dt"
	KeyTime     = "ts"
	KeySerial   = "s"
	// `KeyJournalIsUpToDate` is only used to migrate from the schema that
	// used a boolean to indicate that the serialized journal is up to date
	// without tracking the head serial.  See `Commit()`.
	KeyJournalIsUpToDate = "u"
)

type EventDoc struct {
	Id       ulid.I `bson:"_id"`
	Protobuf []byte `bson:"pb"`
}

const HeadSerialUnspecified int64 = 0

// `Head` contains the latest event.  `Epoch` contains the parent of the first
// visible event.  `Head` may equal `Epoch` to indicate an empty history.
// `Tail` contains the last event that must not be deleted.
//
// `Tail <= Epoch <= Head`.
//
// `EpochLog` contains all epochs.  When advancing `Epoch`, the epoch is also
// appended to `EpochLog`.  See `advanceOneEpoch()`.
type RefsDoc struct {
	Id       uuid.I      `bson:"_id"`
	Head     ulid.I      `bson:"h"`
	Epoch    ulid.I      `bson:"e"`
	EpochLog []EpochTime `bson:"el"`
	Tail     ulid.I      `bson:"t"`

	Serial int64 `bson:"s"`

	// `Phase` may be missing in old docs.  A missing `Phase` is handled as
	// `PhaseActive`.
	Phase PhaseCode `bson:"ph"`

	// `Dtime` is the time when the deletion started, i.e. when `Phase` was
	// set to `PhaseDeleting`.
	Dtime time.Time `bson:"dt"`
}

type EpochTime struct {
	Epoch ulid.I    `bson:"e"`
	Time  time.Time `bson:"ts"`
}

type JournalDoc struct {
	Id       idid.I `bson:"_id"`
	Serial   int64  `bson:"s"`
	Protobuf []byte `bson:"pb"`
}

type Journal struct {
	events     *mgo.Collection
	refs       *mgo.Collection
	journal    *mgo.Collection
	notifier   *notifier
	trimPolicy TrimPolicy
}

/*

`NewJournal(conn, ns)` creates a new event journal that is backed by MongoDB
collections `<ns>.events`, `<ns>.refs`, and `<ns>.journal`.

To activate event notification via Go channels, `go Serve()` with a context
that is canceled during shutdown.  Example, w/o error handling:

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	// ...

	mainJ, err := events.NewJournal(mgs, "evjournal.fsomain")
	wg.Add(1)
	go func() {
		err := mainJ.Serve(ctx)
		wg.Done()
	}()

	// ...
	// Shutdown
	cancel()

*/
func NewJournal(conn *mgo.Session, ns string) (*Journal, error) {
	events := conn.DB("").C(ns + ".events")

	// For backward compatibility, first try the historic name `heads` for
	// the refs collection.
	var refs *mgo.Collection
	for _, name := range []string{
		"heads",
		"refs",
	} {
		refs = conn.DB("").C(ns + "." + name)
		n, err := refs.Count()
		if err != nil {
			return nil, err
		}
		if n > 0 {
			break
		}
	}

	journal := conn.DB("").C(ns + ".journal")

	return &Journal{
		events:   events,
		refs:     refs,
		journal:  journal,
		notifier: newNotifier(),
	}, nil
}

func (j *Journal) SetTrimPolicy(tp TrimPolicy) {
	if j.trimPolicy != nil {
		panic("trim policy already set")
	}
	j.trimPolicy = tp
}

func (j *Journal) Serve(ctx context.Context) error {
	return j.notifier.serve(ctx)
}

func (j *Journal) Head(historyId uuid.I) (ulid.I, error) {
	var refs RefsDoc
	err := j.refs.Find(bson.M{
		KeyId: historyId,
	}).Select(bson.M{
		KeyHead:  1,
		KeyPhase: 1,
	}).One(&refs)
	switch {
	case err == mgo.ErrNotFound:
		return EventEpoch, nil
	case err != nil:
		return ulid.Nil, &DBError{
			Op:  OpFindHead,
			Err: err,
		}
	}

	if refs.Phase.IsInactive() {
		return EventEpoch, nil
	}

	return refs.Head, nil
}

func (j *Journal) Delete(
	historyId uuid.I, head ulid.I,
) error {
	err := j.refs.Update(bson.M{
		KeyId:   historyId,
		KeyHead: head,
		"$or": []bson.M{
			{KeyPhase: PhaseActive},
			{KeyPhase: bson.M{"$exists": false}},
		},
	}, bson.M{
		"$set": bson.M{
			KeyPhase: PhaseDeleting,
		},
		"$currentDate": bson.M{
			KeyDtime: true,
		},
	})
	switch {
	case err == mgo.ErrNotFound:
		// Double check head mismatch to detect a version conflict.
		head2, err2 := j.Head(historyId)
		switch {
		case err2 != nil:
			return err2 // `err2` is a `DBError`.
		case head2 == EventEpoch:
			return nil // ok: already deleted.
		default:
			return &VersionConflictError{
				Stored:   head2,
				Expected: head,
			}
		}
	case err != nil:
		return &DBError{
			Op:  OpUpdateHead,
			Err: err,
		}
	}

	return nil
}

func (j *Journal) Commit(
	historyId uuid.I, evs []Event,
) ([]Event, error) {
	if len(evs) == 0 {
		return evs, nil
	}

	evs, err := eventsWithId(evs)
	if err != nil {
		return nil, &InternalError{
			Op:  OpEventIds,
			Err: err,
		}
	}

	// A Mongo batch operation would be more efficient for multiple events.
	for _, ev := range evs {
		buf, err := ev.MarshalProto()
		if err != nil {
			return nil, &InternalError{
				Op:  OpEncodeEventProto,
				Err: err,
			}
		}
		if err := j.ensureStoredEvent(EventDoc{
			Id:       ev.Id(),
			Protobuf: buf,
		}); err != nil {
			return nil, err // `err` is a package error.
		}
	}

	parent := evs[0].Parent()
	head := evs[len(evs)-1].Id()
	if parent == EventEpoch {
		err := j.refs.Insert(RefsDoc{
			Id:     historyId,
			Head:   head,
			Serial: HeadSerialUnspecified,
			Phase:  PhaseActive,
		})
		if err != nil {
			return nil, &DBError{
				Op:  OpInsertHead,
				Err: err,
			}
		}
	} else {
		err := j.refs.Update(bson.M{
			KeyId:   historyId,
			KeyHead: parent,
			"$or": []bson.M{
				{KeyPhase: PhaseActive},
				{KeyPhase: bson.M{"$exists": false}},
			},
		}, bson.M{
			"$set": bson.M{
				KeyHead:   head,
				KeySerial: HeadSerialUnspecified,
				// Migrate doc from the schema that did not
				// include the phase.
				KeyPhase: PhaseActive,
			},
			// Migrate doc from schema that did not track the head
			// `Serial` but only used a boolean flag to indicate
			// whether the serialized journal is up to date.  The
			// boolean is not used anymore.
			"$unset": bson.M{
				KeyJournalIsUpToDate: "",
			},
		})
		switch {
		case err == mgo.ErrNotFound:
			// Double check head mismatch before assuming that it
			// is a version conflict.  The following cases cannot
			// be a simple version conflict:
			//
			//  - Head cannot be determined.
			//  - The history is empty, but `parent` does not equal
			//    `EventEpoch`.
			//
			head2, err2 := j.Head(historyId)
			switch {
			case err2 != nil:
				return nil, err2 // `err2` is a `DBError`.
			case head2 == EventEpoch:
				return nil, &UnknownHistoryError{
					Id: historyId,
				}
			default:
				return nil, &VersionConflictError{
					Stored:   head2,
					Expected: parent,
				}
			}
		case err != nil:
			return nil, &DBError{
				Op:  OpUpdateHead,
				Err: err,
			}
		}
	}

	j.notifier.post(historyId)

	return evs, nil
}

func (j *Journal) ensureStoredEvent(want EventDoc) error {
	err := j.events.Insert(want)
	if err == nil {
		return nil
	}
	if !mgo.IsDup(err) {
		return &DBError{
			Op:  OpInsertEvent,
			Err: err,
		}
	}

	var got EventDoc
	err = j.events.Find(bson.M{KeyId: want.Id}).One(&got)
	if err != nil {
		return &DBError{
			Op:  OpFindPreviousEvent,
			Err: err,
		}
	}
	if !bytes.Equal(want.Protobuf, got.Protobuf) {
		return &DuplicateEventMismatchError{
			Id: want.Id,
		}
	}
	return nil
}

// `Subscribe()` registers the channel `ch` to receive notifications after new
// events were added to the journal for `historyId`.  Use the empty string
// `historyId=""` to receive notifications for any event.
//
// Sends are non-blocking.  Usually use a buffered channel of size 1.
func (j *Journal) Subscribe(ch chan<- uuid.I, historyId uuid.I) {
	j.notifier.subscribe(ch, historyId)
}

func (j *Journal) Unsubscribe(ch chan<- uuid.I) {
	j.notifier.unsubscribe(ch)
}

func (j *Journal) Find(historyId uuid.I, after ulid.I) *Iter {
	// Limit find by the serial of the current head as `<= headSerial`, so
	// that either all or none of the events emitted by a command are
	// returned.  If this find was not limited by the serial, it could see
	// a partially serialized journal: a concurrent commit could update the
	// head, and a concurrent find could then be appending events to the
	// serialized journal.  This find could see and return the new events,
	// which would look like a partially serialized journal.
	headSerial, epoch, err := j.ensureJournal(historyId)
	if err != nil {
		return &Iter{err: err}
	}
	// If there are no events, return empty iterator.
	if headSerial == 0 {
		return &Iter{}
	}

	min, max := idid.RangeMinMax(historyId)
	idRange := bson.M{"$gte": min, "$lte": max}

	// Use the stored epoch, which is set when the journal is trimmed.
	if after == EventEpoch {
		after = epoch
	}

	if after == EventEpoch {
		it := j.journal.Find(bson.M{
			KeyId:     idRange,
			KeySerial: bson.M{"$lte": headSerial},
		}).Sort(
			KeySerial,
		).Select(bson.M{
			KeyProtobuf: 1,
		}).Iter()
		return &Iter{mongo: it, prev: after}
	}

	var afterD JournalDoc
	err = j.journal.Find(bson.M{
		KeyId: idid.Pack(historyId, after),
	}).One(&afterD)
	if err != nil {
		return &Iter{err: &DBError{
			Op:  OpFindStart,
			Err: err,
		}}
	}

	it := j.journal.Find(bson.M{
		KeyId:     idRange,
		KeySerial: bson.M{"$gt": afterD.Serial, "$lte": headSerial},
	}).Sort(
		KeySerial,
	).Select(bson.M{
		KeyProtobuf: 1,
	}).Iter()
	return &Iter{mongo: it, prev: after}
}

func (j *Journal) ensureJournal(historyId uuid.I) (int64, ulid.I, error) {
	var refs RefsDoc
	err := j.refs.Find(bson.M{
		KeyId: historyId,
	}).Select(bson.M{
		KeyHead:   1,
		KeySerial: 1,
		KeyEpoch:  1,
		KeyTail:   1,
		KeyPhase:  1,
	}).One(&refs)
	switch {
	case err == mgo.ErrNotFound:
		// No head is acceptable.  The journal remains empty.
		return 0, ulid.Nil, nil
	case err != nil:
		return 0, ulid.Nil, &DBError{
			Op:  OpFindHead,
			Err: err,
		}
	}

	if refs.Phase.IsInactive() {
		return 0, ulid.Nil, nil
	}

	// Journal is up to date.  See `heads.Update()` below.
	if refs.Serial != HeadSerialUnspecified {
		return refs.Serial, refs.Epoch, nil
	}

	// Walk parents until existing entry.  Then insert in reverse order.
	evId := refs.Head
	var events []EventDoc
	var serial int64
	for {
		if evId == EventEpoch {
			serial = 0
			break
		}

		s, ok, err := j.findSerial(historyId, evId)
		if err != nil {
			return 0, ulid.Nil, err // `err` is a `DBError`.
		}
		if ok {
			serial = s
			break
		}

		var ev EventDoc
		err = j.events.Find(bson.M{KeyId: evId}).One(&ev)
		if err != nil {
			return 0, ulid.Nil, &DBError{
				Op:  OpFindEvent,
				Err: err,
			}
		}
		events = append(events, ev)

		if refs.Tail != ulid.Nil && evId == refs.Tail {
			serial = 0
			break
		}

		var pbev pb.Event
		if err = proto.Unmarshal(ev.Protobuf, &pbev); err != nil {
			return 0, ulid.Nil, &CorruptedDataError{
				Op:  OpDecodeEventProto,
				Err: err,
			}
		}
		if evId, err = ulid.ParseBytes(pbev.Parent); err != nil {
			return 0, ulid.Nil, &CorruptedDataError{
				Op:  OpParseEventParent,
				Err: err,
			}
		}
	}

	for i := len(events) - 1; i >= 0; i-- {
		serial++
		err := j.ensureJournalEntry(historyId, serial, events[i])
		if err != nil {
			return 0, ulid.Nil, err // `err` is a package error.
		}
	}

	// Set up-to-date flag if head is unchanged, so that next
	// `ensureJournal()` can return early.
	err = j.refs.Update(bson.M{
		KeyId:   historyId,
		KeyHead: refs.Head,
		"$or": []bson.M{
			{KeyPhase: PhaseActive},
			{KeyPhase: bson.M{"$exists": false}},
		},
	}, bson.M{
		"$set": bson.M{KeySerial: serial},
	})
	if err == mgo.ErrNotFound {
		// Ignore not found.  It may be caused by a concurrent update,
		// which will be handled during the next call to
		// `ensureJournal()`.
		return serial, refs.Epoch, nil
	}
	if err != nil {
		return 0, ulid.Nil, &DBError{
			Op:  OpUpdateHeadSerial,
			Err: err,
		}
	}

	return serial, refs.Epoch, nil
}

func (j *Journal) ensureJournalEntry(
	historyId uuid.I, serial int64, ev EventDoc,
) error {
	want := JournalDoc{
		Id:       idid.Pack(historyId, ev.Id),
		Serial:   serial,
		Protobuf: ev.Protobuf,
	}
	err := j.journal.Insert(want)
	if err == nil {
		return nil
	}
	if !mgo.IsDup(err) {
		return &DBError{
			Op:  OpInsertJournal,
			Err: err,
		}
	}

	var got JournalDoc
	err = j.journal.Find(bson.M{KeyId: want.Id}).One(&got)
	if err != nil {
		return &DBError{
			Op:  OpFindJournalDuplicate,
			Err: err,
		}
	}
	if got.Serial != want.Serial {
		return &DuplicateJournalEntrySerialMismatchError{
			HistoryId: historyId,
			EventId:   ev.Id,
			Stored:    got.Serial,
			Expected:  want.Serial,
		}
	}
	if !bytes.Equal(got.Protobuf, want.Protobuf) {
		return &DuplicateJournalEntryProtobufMismatchError{
			HistoryId: historyId,
			EventId:   ev.Id,
		}
	}
	return nil
}

func (j *Journal) findSerial(
	historyId uuid.I, eventId ulid.I,
) (int64, bool, error) {
	id := idid.Pack(historyId, eventId)
	var ent JournalDoc
	err := j.journal.Find(bson.M{KeyId: id}).One(&ent)
	switch {
	case err == mgo.ErrNotFound:
		return 0, false, nil
	case err != nil:
		return 0, false, &DBError{
			Op:  OpFindJournal,
			Err: err,
		}
	}
	return ent.Serial, true, nil
}

// Valid `Iter` states:
//
//  - `mongo == nil && err == nil`: empty iterator.
//  - `mongo == nil && err != nil`: error before Mongo query.
//  - `mongo != nil`: active Mongo query.
//
type Iter struct {
	mongo *mgo.Iter
	prev  ulid.I
	err   error
}

func (it *Iter) Close() error {
	if it.mongo == nil {
		return it.err
	}
	errmg := it.mongo.Close()
	if it.err != nil {
		return it.err
	}
	if errmg != nil {
		return &DBError{
			Op:  OpScanJournal,
			Err: errmg,
		}
	}
	return nil
}

func (it *Iter) Next(ev EventUnmarshaler) bool {
	if it.err != nil {
		return false
	}
	if it.mongo == nil {
		return false
	}

	var d JournalDoc
	if !it.mongo.Next(&d) {
		return false
	}
	err := ev.UnmarshalProto(d.Protobuf)
	if err != nil {
		// Do not use `CorruptedDataError` here, because an unmarshal
		// error here does not imply corrupted data.  The
		// `EventUnmarshaler` may, for example, decode the protobuf and
		// then reject the event type.  This may happen if a UUID is
		// used with the wrong aggregate type, e.g. a ping-registry
		// workflow id is used as if it was a split-root workflow id.
		it.err = &EventError{
			Op:  OpDecodeJournalProto,
			Err: err,
		}
		return false
	}

	if ev.Parent() != it.prev {
		it.err = &JournalEntryParentMismatchError{
			EventId:  ev.Id(),
			Actual:   ev.Parent(),
			Expected: it.prev,
		}
		return false
	}
	it.prev = ev.Id()

	return true
}
