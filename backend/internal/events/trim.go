package events

import (
	"context"
	"time"

	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	mgo "gopkg.in/mgo.v2"
	bson "gopkg.in/mgo.v2/bson"
)

const ConfigTrimIntervalDays = 30
const ConfigTrimInterval = ConfigTrimIntervalDays * 24 * time.Hour

type TrimPolicy interface {
	NewEvent() EventUnmarshaler
	IsNewEpoch(epoch, first EventUnmarshaler, now time.Time) bool
	IsNewTail(eventId ulid.I, epochTime, now time.Time) bool
}

type Trimmer struct {
	lg      Logger
	journal *Journal
}

func NewTrimmer(
	lg Logger, j *Journal,
) *Trimmer {
	return &Trimmer{
		lg:      lg,
		journal: j,
	}
}

// `Trim()` first advances tails and then epochs, so that new epochs cannot
// immediately become tails, even if the `IsNewTail()` policy would allow it.
// New epochs can only become tails during future calls to `Trim()`.
func (t *Trimmer) Trim(ctx context.Context) error {
	pol := t.journal.trimPolicy
	if pol == nil {
		t.lg.Infow(
			"Skipped history trimming: no trim policy.",
			"collection", t.journal.refs.FullName,
		)
		return nil
	}

	if err := t.advanceTails(ctx, pol); err != nil {
		return err
	}

	return t.advanceEpochs(ctx, pol)
}

func (t *Trimmer) advanceTails(ctx context.Context, pol TrimPolicy) error {
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
	iter = t.journal.refs.Find(
		bson.M{},
	).Select(bson.M{
		KeyTail:     1,
		KeyEpochLog: 1,
		KeyPhase:    1,
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

		ok, err := t.advanceOneTail(
			ctx, pol, refs.Id, refs.Tail, refs.EpochLog,
		)
		if err != nil {
			return err
		}
		if ok {
			nHistories++
		}
	}

	if nHistories == 0 {
		t.lg.Infow(
			"History trimming left tails unchanged.",
			"collection", t.journal.refs.FullName,
		)
	} else {
		t.lg.Infow(
			"History trimming advanced tails.",
			"collection", t.journal.refs.FullName,
			"nHistories", nHistories,
		)
	}

	return iterClose()
}

func (t *Trimmer) advanceOneTail(
	ctx context.Context,
	pol TrimPolicy,
	id uuid.I,
	oldTail ulid.I,
	epochs []EpochTime,
) (bool, error) {
	// Use a fixed time for all `IsNewTail()` tests.  It feels potentially
	// more robust, although it should not matter here.
	now := time.Now()

	var newTail ulid.I
	foundOld := (oldTail == ulid.Nil)
	for _, tail := range epochs {
		if !foundOld {
			foundOld = (tail.Epoch == oldTail)
			continue
		}
		if pol.IsNewTail(tail.Epoch, tail.Time, now) {
			newTail = tail.Epoch
		}
	}

	if newTail == ulid.Nil {
		return false, nil
	}

	sel := bson.M{
		KeyId: id,
	}
	if oldTail != ulid.Nil {
		sel[KeyTail] = oldTail
	}
	err := t.journal.refs.Update(sel, bson.M{
		"$set": bson.M{
			KeyTail: newTail,
		},
	})
	if err != nil {
		return false, &DBError{
			Op:  OpUpdateHead,
			Err: err,
		}
	}

	t.lg.Infow(
		"History trimming advanced tail.",
		"collection", t.journal.refs.FullName,
		"historyId", id,
		"oldTail", oldTail,
		"newTail", newTail,
	)

	return true, nil
}

func (t *Trimmer) advanceEpochs(ctx context.Context, pol TrimPolicy) error {
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
	var nEvents int64
	iter = t.journal.refs.Find(
		bson.M{},
	).Select(bson.M{
		KeyEpoch:    1,
		KeyEpochLog: 1,
		KeyPhase:    1,
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

		// Avoid frequent trimming in order to limit the rate at which
		// the epoch log may grow.  The rate should allow us to keep
		// the log forever.
		//
		// A trim every 30d requires less than 0.5 kB per year:
		//
		//     366d / 30d * sizeof(EpochTime) < 13 * 32B = 416B < 0.5kB
		//
		if !isNextTrimInterval(refs.EpochLog, ConfigTrimInterval) {
			continue
		}

		n, err := t.advanceOneEpoch(ctx, pol, refs.Id, refs.Epoch)
		if err != nil {
			return err
		}
		if n > 0 {
			nHistories++
			nEvents += n
		}
	}

	if nEvents == 0 {
		t.lg.Infow(
			"History trimming left epochs unchanged.",
			"collection", t.journal.refs.FullName,
		)
	} else {
		t.lg.Infow(
			"History trimming advanced epochs.",
			"collection", t.journal.refs.FullName,
			"nHistories", nHistories,
			"nEvents", nEvents,
		)
	}

	return iterClose()
}

// `isNextTrimInterval()` returns true if the `epochLog` is empty or if the
// last `epochLog` entry is older than `trimInterval`.
func isNextTrimInterval(
	epochLog []EpochTime,
	trimInterval time.Duration,
) bool {
	if len(epochLog) == 0 {
		return true
	}
	lastEpochTime := epochLog[len(epochLog)-1].Time
	cutoff := time.Now().Add(-trimInterval)
	return lastEpochTime.Before(cutoff)
}

func (t *Trimmer) advanceOneEpoch(
	ctx context.Context,
	pol TrimPolicy,
	id uuid.I,
	oldEpoch ulid.I,
) (int64, error) {
	var iter *Iter
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	var newEpoch ulid.I
	var nEvents int64
	iter = t.journal.Find(id, EventEpoch)
	epoch := pol.NewEvent()
	first := pol.NewEvent()
	if iter.Next(epoch) {
		// Use a fixed time for all `IsNewEpoch()` tests.  It feels
		// potentially more robust, although it should not matter here.
		now := time.Now()
		var i int64 = 1
		for iter.Next(first) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default: // non-blocking
			}
			if pol.IsNewEpoch(epoch, first, now) {
				newEpoch = epoch.Id()
				nEvents = i
			}
			epoch, first = first, epoch
			i++
		}
		if pol.IsNewEpoch(epoch, nil, now) {
			newEpoch = epoch.Id()
			nEvents = i
		}
	}
	if err := iter.Close(); err != nil {
		return 0, err
	}

	if newEpoch == ulid.Nil {
		return 0, nil
	}

	if newEpoch == oldEpoch {
		panic("new epoch equals old epoch")
	}

	sel := bson.M{
		KeyId: id,
	}
	if oldEpoch != ulid.Nil {
		sel[KeyEpoch] = oldEpoch
	}
	err := t.journal.refs.Update(sel, bson.M{
		"$set": bson.M{
			KeyEpoch: newEpoch,
		},
		"$push": bson.M{
			KeyEpochLog: EpochTime{
				Epoch: newEpoch,
				Time:  time.Now(),
			},
		},
	})
	if err != nil {
		return 0, &DBError{
			Op:  OpUpdateHead,
			Err: err,
		}
	}

	t.lg.Infow(
		"History trimming advanced epoch.",
		"collection", t.journal.refs.FullName,
		"historyId", id,
		"oldEpoch", oldEpoch,
		"newEpoch", newEpoch,
		"nEvents", nEvents,
	)

	return nEvents, nil
}
