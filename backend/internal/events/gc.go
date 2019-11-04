package events

import (
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

// `ConfigRecentDays` is the minimal age of an event before it may be deleted.
// The age is based on etime.
const ConfigRecentDays = 30
const ConfigRecentDuration = ConfigRecentDays * 24 * time.Hour

// `ConfigDeletingDays` is the minimal duration since refs were marked for
// deletion before the corresponding journal may be deleted.
const ConfigDeletingDays = 15
const ConfigDeletingDuration = ConfigDeletingDays * 24 * time.Hour

// `ConfigDeletedDays` is the minimal duration since refs were marked for
// deletion before the refs doc may be removed from the database.
const ConfigDeletedDays = 30
const ConfigDeletedDuration = ConfigDeletedDays * 24 * time.Hour

type eventGraph map[ulid.I]*eventNode

type eventNode struct {
	Parent *eventNode
	Color  eventColor
}

type eventColor int

const (
	colorUnspecified eventColor = iota
	colorWhite
	colorGray
	colorBlack
)

func (g eventGraph) GetDefault(id ulid.I) *eventNode {
	e, ok := g[id]
	if !ok {
		e = &eventNode{}
		g[id] = e
	}
	return e
}

func (g eventGraph) SetColor(id ulid.I, col eventColor) bool {
	e, ok := g[id]
	if !ok {
		return false
	}
	e.Color = col
	return true
}

type EventsGarbageCollector struct {
	lg      Logger
	journal *Journal
}

func NewEventsGarbageCollector(
	lg Logger, j *Journal,
) *EventsGarbageCollector {
	return &EventsGarbageCollector{
		lg:      lg,
		journal: j,
	}
}

// `Gc()` runs all garbage collectors, specifically:
//
//     GcEvents()
//     GcJournalTail()
//     GcDeletedRefs()
//     GcDeletingJournals()
//
// The order of the `GcX()` calls prefers slow over aggressive garbage
// collection.  Some garbage that could in principle be deleted right away may
// require another gc cycle to be actually deleted.
func (gc *EventsGarbageCollector) Gc(ctx context.Context) error {
	if err := gc.GcEvents(ctx); err != nil {
		return err
	}

	if err := gc.GcJournalTail(ctx); err != nil {
		return err
	}

	// Call `GcDeletedRefs()` before `GcDeletingJournals()`, so that refs
	// that `GcDeletedJournals()` changes to `PhaseDeleted` cannot be
	// deleted in this gc cycle.
	if err := gc.GcDeletedRefs(ctx); err != nil {
		return err
	}

	if err := gc.GcDeletingJournals(ctx); err != nil {
		return err
	}

	return nil
}

// `GcEvents()` deletes unreachable events.
//
// It uses mark and sweep with the following colors:
//
//  - unspecified: event is not in Mongo collection.
//  - white: event is old and unreachable.
//  - gray: event is recent or reachable via a head.
//  - black: event is a tail or has been painted.
//
// Painting starts from gray nodes and walks along parents, marking the visited
// nodes in black, until the first black node is reached, i.e. each stroke
// stops either at a tail or at a node that has been already painted (and its
// parents have been painted, too).  After painting, white events can be
// deleted.
//
// All refs, including refs in `PhaseDeleting` or `PhaseDeleted`, prevent
// events from beeing deleted.  Events will only be deleted after the refs doc
// has been deleted from the database.
func (gc *EventsGarbageCollector) GcEvents(ctx context.Context) error {
	now := time.Now()
	cutoff := now.Add(-ConfigRecentDuration)
	history, err := gc.loadColoredEventGraph(ctx, cutoff)
	if err != nil {
		return err
	}

	paintBlack(history)

	n := 0
	for _, e := range history {
		if e.Color == colorWhite {
			n++
		}
	}
	if n == 0 {
		gc.lg.Infow(
			"GC found no unreachable events.",
			"collection", gc.journal.events.FullName,
		)
		return nil
	}

	gc.lg.Infow(
		"GC found unreachable events.",
		"collection", gc.journal.events.FullName,
		"n", n,
	)
	for id, e := range history {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default: // non-blocking
		}

		if e.Color == colorWhite {
			err := gc.journal.events.Remove(bson.M{KeyId: id})
			if err != nil {
				return err
			}
			gc.lg.Infow(
				"GC removed event.",
				"collection", gc.journal.events.FullName,
				"id", id,
				"etime", ulid.Time(id),
			)
		}
	}
	gc.lg.Infow(
		"GC completed.",
		"collection", gc.journal.events.FullName,
		"n", n,
	)

	return nil
}

func (gc *EventsGarbageCollector) loadColoredEventGraph(
	ctx context.Context, cutoff time.Time,
) (eventGraph, error) {
	var history eventGraph = make(map[ulid.I]*eventNode)

	var iter *mgo.Iter
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	// Build graph, marking recent events in gray.
	iter = gc.journal.events.Find(
		bson.M{},
	).Select(bson.M{
		KeyProtobuf: 1,
	}).Iter()
	var evDoc EventDoc
	for iter.Next(&evDoc) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default: // non-blocking
		}

		evId := evDoc.Id
		parentId, err := parsePbEventParent(evId, evDoc.Protobuf)
		if err != nil {
			return nil, err
		}

		e := history.GetDefault(evId)
		p := history.GetDefault(parentId)
		e.Parent = p
		if ulid.Time(evId).Before(cutoff) {
			e.Color = colorWhite
		} else {
			e.Color = colorGray
		}
	}
	if err := iterClose(); err != nil {
		return nil, err
	}

	// Prune nodes that are not in the events collection.
	for id, e := range history {
		if e.Color == colorUnspecified {
			delete(history, id)
		} else if e.Parent.Color == colorUnspecified {
			e.Parent = nil
		}
	}

	// Mark heads in gray and tails in black.
	//
	// All refs, including refs in `PhaseDeleting` and `PhaseDeleted`, keep
	// events alive.
	iter = gc.journal.refs.Find(
		bson.M{},
	).Select(bson.M{
		KeyHead: 1,
		KeyTail: 1,
	}).Iter()
	var refs RefsDoc
	for iter.Next(&refs) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default: // non-blocking
		}

		h := refs.Head
		if !history.SetColor(h, colorGray) {
			return nil, &CorruptedDataError{
				Op:  OpHeadEventLookup,
				Err: &MissingHeadEventError{Id: h},
			}
		}

		t := refs.Tail
		if t != ulid.Nil && !history.SetColor(t, colorBlack) {
			return nil, &CorruptedDataError{
				Op:  OpTailEventLookup,
				Err: &MissingTailEventError{Id: t},
			}
		}
	}
	if err := iterClose(); err != nil {
		return nil, err
	}

	return history, nil
}

func parsePbEventParent(evId ulid.I, protobuf []byte) (ulid.I, error) {
	var ev pb.Event
	if err := proto.Unmarshal(protobuf, &ev); err != nil {
		return ulid.Nil, &CorruptedDataError{
			Op:  OpDecodeEventProto,
			Err: err,
		}
	}

	pbId, err := ulid.ParseBytes(ev.Id)
	if err != nil {
		return ulid.Nil, &CorruptedDataError{
			Op:  OpParseEventId,
			Err: err,
		}
	}
	if pbId != evId {
		return ulid.Nil, &CorruptedDataError{
			Op: OpCheckEventId,
			Err: &EventIdMismatchError{
				MongoId: evId,
				ProtoId: pbId,
			},
		}
	}

	parentId, err := ulid.ParseBytes(ev.Parent)
	if err != nil {
		return ulid.Nil, &CorruptedDataError{
			Op:  OpParseEventParent,
			Err: err,
		}
	}

	return parentId, nil
}

func paintBlack(g eventGraph) {
	for _, e := range g {
		if e.Color == colorGray {
			for e != nil && e.Color != colorBlack {
				e.Color = colorBlack
				e = e.Parent
			}
		}
	}
}

// `GcJournalTail()` deletes journal entries before the tail.
func (gc *EventsGarbageCollector) GcJournalTail(ctx context.Context) error {
	var iter *mgo.Iter
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	var nHistories int64
	var nEntries int64
	iter = gc.journal.refs.Find(
		bson.M{},
	).Select(bson.M{
		KeyTail:  1,
		KeyPhase: 1,
	}).Iter()
	var refs RefsDoc
	for iter.Next(&refs) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default: // non-blocking
		}

		// Skip inactive histories, because they are soon going to be
		// completely deleted anyway.
		if refs.Phase.IsInactive() {
			continue
		}

		n, err := gc.gcOneJournalTail(ctx, refs.Id, refs.Tail)
		if err != nil {
			return err
		}
		if n > 0 {
			nHistories++
			nEntries += n
		}
	}

	if nHistories == 0 {
		gc.lg.Infow(
			"GC found no unreachable journal entries.",
			"collection", gc.journal.journal.FullName,
		)
	} else {
		gc.lg.Infow(
			"GC removed unreachable journal entries.",
			"collection", gc.journal.journal.FullName,
			"nHistories", nHistories,
			"nEntries", nEntries,
		)
	}

	return iterClose()
}

func (gc *EventsGarbageCollector) gcOneJournalTail(
	ctx context.Context,
	historyId uuid.I,
	tailId ulid.I,
) (int64, error) {
	if tailId == ulid.Nil {
		return 0, nil
	}

	serial, ok, err := gc.journal.findSerial(historyId, tailId)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}

	min, max := idid.RangeMinMax(historyId)
	inf, err := gc.journal.journal.RemoveAll(bson.M{
		KeyId:     bson.M{"$gte": min, "$lte": max},
		KeySerial: bson.M{"$lt": serial},
	})
	if err != nil {
		return 0, err
	}
	return int64(inf.Removed), nil
}

// `GcDeletedRefs()` deletes refs from the database that are in `PhaseDeleted`
// and have initially been marked for deletion more than
// `ConfigDeletedDuration` ago.
func (gc *EventsGarbageCollector) GcDeletedRefs(
	ctx context.Context,
) error {
	now := time.Now()
	cutoff := now.Add(-ConfigDeletedDuration)

	inf, err := gc.journal.refs.RemoveAll(bson.M{
		KeyPhase: PhaseDeleted,
		KeyDtime: bson.M{"$lt": cutoff},
	})
	if err != nil {
		return err
	}
	nHistories := inf.Removed

	if nHistories == 0 {
		gc.lg.Infow(
			"GC found no refs of deleted histories.",
			"collection", gc.journal.refs.FullName,
		)
	} else {
		gc.lg.Infow(
			"GC removed refs of deleted histories.",
			"collection", gc.journal.refs.FullName,
			"nHistories", nHistories,
		)
	}

	return nil
}

// `GcDeletedJournals()` removes journals of refs that are in `PhaseDeleting`
// and have been initially marked for deletion more than
// `ConfigDeletingDuration` ago.  The refs are then changed to `PhaseDeleted`,
// so that they are deleted by a future `GcDeletedRefs()`.
func (gc *EventsGarbageCollector) GcDeletingJournals(
	ctx context.Context,
) error {
	now := time.Now()
	cutoff := now.Add(-ConfigDeletingDuration)

	var iter *mgo.Iter
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	var nHistories int64
	var nEntries int64
	iter = gc.journal.refs.Find(bson.M{
		KeyPhase: PhaseDeleting,
	}).Select(bson.M{
		KeyPhase: 1,
		KeyDtime: 1,
	}).Iter()
	var refs RefsDoc
	for iter.Next(&refs) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default: // non-blocking
		}

		// Double-check phase.
		if refs.Phase != PhaseDeleting {
			panic("unexpected phase")
		}
		// Skip refs that have only recently changed to
		// `PhaseDeleting`.
		if !refs.Dtime.Before(cutoff) {
			continue
		}

		n, err := gc.gcOneDeletingJournal(ctx, refs.Id)
		if err != nil {
			return err
		}
		if n > 0 {
			nHistories++
			nEntries += n
		}
	}

	if nHistories == 0 {
		gc.lg.Infow(
			"GC found no journal entries of deleted histories.",
			"collection", gc.journal.journal.FullName,
		)
	} else {
		gc.lg.Infow(
			"GC removed journal entries of deleted histories.",
			"collection", gc.journal.journal.FullName,
			"nHistories", nHistories,
			"nEntries", nEntries,
		)
	}

	return iterClose()
}

func (gc *EventsGarbageCollector) gcOneDeletingJournal(
	ctx context.Context,
	historyId uuid.I,
) (int64, error) {
	// Mark the journal as invalid by setting
	// `KeySerial=HeadSerialUnspecified` before actually deleting it.  It
	// might be helpful if we want to resurrect the history through direct
	// manipulation of the database.  We could reset the refs to
	// `PhaseActive`, and the journal would be correctly rebuild.
	//
	// The argument above assumes that `RemoveAll()` either completely
	// fails, deleting no doc, or complete succeeds, deleting all the
	// selected docs.  Because we do not intent to actually resurrect
	// histories, we do not care whether this assumption always holds in
	// practice.
	if err := gc.journal.refs.Update(bson.M{
		KeyId:    historyId,
		KeyPhase: PhaseDeleting,
	}, bson.M{
		"$set": bson.M{
			KeySerial: HeadSerialUnspecified,
		},
	}); err != nil {
		return 0, err
	}

	min, max := idid.RangeMinMax(historyId)
	inf, err := gc.journal.journal.RemoveAll(bson.M{
		KeyId: bson.M{"$gte": min, "$lte": max},
	})
	if err != nil {
		return 0, err
	}
	nEntries := int64(inf.Removed)

	gc.lg.Infow(
		"GC removed journal entries of deleted history.",
		"collection", gc.journal.journal.FullName,
		"historyId", historyId,
		"nEntries", nEntries,
	)

	if err := gc.journal.refs.Update(bson.M{
		KeyId:    historyId,
		KeyPhase: PhaseDeleting,
	}, bson.M{
		"$set": bson.M{
			KeyPhase: PhaseDeleted,
		},
	}); err != nil {
		return 0, err
	}

	return nEntries, nil
}
