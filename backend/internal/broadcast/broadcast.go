package broadcast

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

type Event struct {
	id     ulid.I
	parent ulid.I
	pb     pb.BroadcastEvent
}

// `newEvents()` could be extended to build a parent chain of multiple events.
func newEvents(parent ulid.I, pb pb.BroadcastEvent) ([]events.Event, error) {
	id, err := ulid.New()
	if err != nil {
		return nil, err
	}
	e := &Event{id: id, parent: parent, pb: pb}
	e.pb.Id = e.id[:]
	e.pb.Parent = e.parent[:]
	return []events.Event{e}, nil
}

func NewEventChain(
	parent ulid.I, pbs []*pb.BroadcastEvent,
) ([]events.Event, error) {
	evs := make([]events.Event, 0, len(pbs))
	for _, pb := range pbs {
		// Keep the time of an existing ID, because it may indicate
		// when the event has been created.  But always assign new
		// entropy, because the parent chain changes.  The ID must
		// change, too, in order to avoid an duplicate event mismatch
		// when repeatedly trying to commit the event chain.
		id, err := ulid.New()
		if err != nil {
			return nil, err
		}
		if pb.Id != nil {
			idOld, err := ulid.ParseBytes(pb.Id)
			if err != nil {
				panic(err)
			}
			id.SetTime(idOld.Time())
		}
		pbDup := *pb
		pbDup.Id = id[:]
		e := &Event{id: id, parent: parent, pb: pbDup}
		e.pb.Parent = e.parent[:]
		parent = id
		evs = append(evs, e)
	}
	return evs, nil
}

func (e *Event) MarshalProto() ([]byte, error) {
	return proto.Marshal(&e.pb)
}

func (e *Event) UnmarshalProto(data []byte) error {
	var err error
	if err = proto.Unmarshal(data, &e.pb); err != nil {
		return err
	}
	if e.id, err = ulid.ParseBytes(e.pb.Id); err != nil {
		return err
	}
	if e.parent, err = ulid.ParseBytes(e.pb.Parent); err != nil {
		return err
	}

	// Verify that event contains valid details.
	c := e.pb.BcChange
	if c == nil {
		err := fmt.Errorf("missing BcChange")
		return err
	}
	if _, err := uuid.FromBytes(c.EntityId); err != nil {
		return err
	}
	// `EventId` is optional.  But if present, it must be a valid ULID.
	if c.EventId != nil {
		if _, err := ulid.ParseBytes(c.EventId); err != nil {
			return err
		}
	}

	return nil
}

func (e *Event) Id() ulid.I     { return e.id }
func (e *Event) Parent() ulid.I { return e.parent }

// Receiver by value.
func (e Event) WithId(id ulid.I) events.Event {
	e.id = id
	e.pb.Id = e.id[:]
	return &e
}

// Receiver by value.
func (e Event) WithParent(parent ulid.I) events.Event {
	e.parent = parent
	e.pb.Parent = e.parent[:]
	return &e
}

func (e *Event) PbBroadcastEvent() *pb.BroadcastEvent {
	return &e.pb
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
}

type WatchConfig struct {
	MainJ     *events.Journal
	RegistryJ *events.Journal
	ReposJ    *events.Journal
}

type Broadcaster struct {
	lg         Logger
	broadcastJ *events.Journal
	id         uuid.I
	mainJ      *events.Journal
	registryJ  *events.Journal
	reposJ     *events.Journal
}

func New(
	lg Logger, broadcastJ *events.Journal, id uuid.I, cfg WatchConfig,
) *Broadcaster {
	broadcastJ.SetTrimPolicy(&trimPolicy{})
	return &Broadcaster{
		lg:         lg,
		broadcastJ: broadcastJ,
		id:         id,
		mainJ:      cfg.MainJ,
		registryJ:  cfg.RegistryJ,
		reposJ:     cfg.ReposJ,
	}
}

func (b *Broadcaster) Process(ctx context.Context) error {
	mainU := make(chan uuid.I, 10)
	b.mainJ.Subscribe(mainU, events.WildcardTopic)
	defer b.mainJ.Unsubscribe(mainU)

	registryU := make(chan uuid.I, 100)
	b.registryJ.Subscribe(registryU, events.WildcardTopic)
	defer b.registryJ.Unsubscribe(registryU)

	reposU := make(chan uuid.I, 100)
	b.reposJ.Subscribe(reposU, events.WildcardTopic)
	defer b.reposJ.Unsubscribe(reposU)

	const maxBatchSize = 100
	updates := make(map[uuid.I]pb.BroadcastEvent_Type)

	clearUpdates := func() {
		updates = make(map[uuid.I]pb.BroadcastEvent_Type)
	}

	readUpdatesNoWait := func() error {
		// Wait a bit to batch events.
		waitC := time.After(100 * time.Millisecond)
		for {
			if len(updates) > maxBatchSize {
				return nil
			}

			select {
			case <-waitC:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			case id := <-mainU:
				updates[id] = pb.BroadcastEvent_EV_BC_FSO_MAIN_CHANGED
			case id := <-registryU:
				updates[id] = pb.BroadcastEvent_EV_BC_FSO_REGISTRY_CHANGED
			case id := <-reposU:
				updates[id] = pb.BroadcastEvent_EV_BC_FSO_REPO_CHANGED
			}
		}
	}

	readOneUpdateWait := func() error {
		if len(updates) > maxBatchSize {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case id := <-mainU:
			updates[id] = pb.BroadcastEvent_EV_BC_FSO_MAIN_CHANGED
		case id := <-registryU:
			updates[id] = pb.BroadcastEvent_EV_BC_FSO_REGISTRY_CHANGED
		case id := <-reposU:
			updates[id] = pb.BroadcastEvent_EV_BC_FSO_REPO_CHANGED
		}
		return nil
	}

	readUpdates := func() error {
		for {
			if err := readUpdatesNoWait(); err != nil {
				return err
			}
			if len(updates) > 0 {
				return nil
			}
			if err := readOneUpdateWait(); err != nil {
				return err
			}
		}
	}

	for {
		if err := readUpdates(); err != nil {
			return err
		}

		head, err := b.broadcastJ.Head(b.id)
		if err != nil {
			wait := 10 * time.Second
			b.lg.Errorw(
				"Failed to get head.",
				"module", "broadcast",
				"err", err,
				"retryIn", wait,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
				continue
			}
		}

		evs := make([]events.Event, 0, len(updates))
		for k, v := range updates {
			k := k // Avoid loop variable aliasing in k[:].
			ev, err := newEvents(head, pb.BroadcastEvent{
				Event:    v,
				BcChange: &pb.BcChange{EntityId: k[:]},
			})
			if err != nil {
				panic(err) // ulid.New() should never fail.
			}
			evs = append(evs, ev...)
			head = evs[len(evs)-1].Id()
		}

		if _, err := b.broadcastJ.Commit(b.id, evs); err != nil {
			// XXX `Commit()` should return an error that
			// explicitly indicates a concurrent update.

			// Assume concurrent update, and quickly retry.
			millis := 10 + rand.Int63n(10)
			wait := time.Duration(millis) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
				continue
			}
		}

		clearUpdates()
	}
}
