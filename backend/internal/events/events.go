/*

Package `events` implements event sourcing with MongoDB as the event store.
The main classes are `Journal` and `Engine`.

`Journal`: Event log backed by MongoDB collection.  Event notification via Go
channels.

`Engine`: Building block for event sourcing aggregates.  See packages
`fsomain`, `fsoregistry`, `fsorepos` as examples how to use `Engine`.

*/
package events

import (
	"github.com/nogproject/nog/backend/pkg/ulid"
)

type Logger interface {
	Infow(msg string, kv ...interface{})
}

type EventUnmarshaler interface {
	UnmarshalProto([]byte) error
	Id() ulid.I
	Parent() ulid.I
}

type EventMarshaler interface {
	MarshalProto() ([]byte, error)
	WithId(ulid.I) Event
	WithParent(ulid.I) Event
}

type Event interface {
	EventUnmarshaler
	EventMarshaler
}

// `EventEpoch` is the id that indicates the beginning of an event history.
var EventEpoch ulid.I

func eventsWithId(evs []Event) ([]Event, error) {
	out := make([]Event, len(evs))
	for i, ev := range evs {
		// Create parent chain.
		if i > 0 {
			ev = ev.WithParent(out[i-1].Id())
		}
		ev, err := eventWithId(ev)
		if err != nil {
			return nil, err
		}
		out[i] = ev
	}
	return out, nil
}

func eventWithId(ev Event) (Event, error) {
	if ev.Id() != ulid.Nil {
		return ev, nil
	}
	id, err := ulid.New()
	if err != nil {
		return nil, err
	}
	return ev.WithId(id), nil
}
