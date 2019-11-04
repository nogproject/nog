package broadcast

import (
	"time"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
)

// `ConfigEventAgeDays` is the minimal age of an event before it can become the
// new epoch.
const ConfigEventAgeDays = 30
const ConfigEventAge = ConfigEventAgeDays * 24 * time.Hour

// `ConfigSinceEpochUpdate` is the time that must have passed since the epoch
// was updated before the epoch can become the new tail.
const ConfigSinceEpochUpdateDays = 30
const ConfigSinceEpochUpdate = ConfigSinceEpochUpdateDays * 24 * time.Hour

type trimPolicy struct{}

func (p *trimPolicy) NewEvent() events.EventUnmarshaler { return &Event{} }

func (p *trimPolicy) IsNewEpoch(
	epoch, first events.EventUnmarshaler,
	now time.Time,
) bool {
	cutoff := now.Add(-ConfigEventAge)
	return ulid.Time(epoch.Id()).Before(cutoff)
}

func (p *trimPolicy) IsNewTail(
	eventId ulid.I, epochTime time.Time,
	now time.Time,
) bool {
	cutoff := now.Add(-ConfigSinceEpochUpdate)
	return epochTime.Before(cutoff)
}
