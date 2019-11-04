package wfindexes

import (
	"time"

	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
)

// `ConfigSinceEpochUpdate` is the time that must have passed since the epoch
// was updated before the epoch can become the new tail.
const ConfigSinceEpochUpdateDays = 30
const ConfigSinceEpochUpdate = ConfigSinceEpochUpdateDays * 24 * time.Hour

type trimPolicy struct{}

func (p *trimPolicy) NewEvent() events.EventUnmarshaler {
	return &wfevents.Event{}
}

// Every snapshot can immediately become a new epoch.
func (p *trimPolicy) IsNewEpoch(
	epoch, first events.EventUnmarshaler,
	now time.Time,
) bool {
	if first == nil {
		return false
	}
	firstPb := first.(*wfevents.Event).PbWorkflowEvent()
	return firstPb.Event == pb.WorkflowEvent_EV_SNAPSHOT_BEGIN
}

func (p *trimPolicy) IsNewTail(
	eventId ulid.I, epochTime time.Time,
	now time.Time,
) bool {
	cutoff := now.Add(-ConfigSinceEpochUpdate)
	return epochTime.Before(cutoff)
}
