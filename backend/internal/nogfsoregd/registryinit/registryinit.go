// `registryinit.Processor` watches an `fsomain` event journal and tells
// `fsoregistry` to initialize registry entities.
package registryinit

import (
	"context"
	"fmt"
	"time"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsomain"
	pb "github.com/nogproject/nog/backend/internal/fsomainpb"
	"github.com/nogproject/nog/backend/internal/fsoregistry"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const NsFsoRegistry = "fsoreg"

type Processor struct {
	names    *shorteruuid.Names
	lg       Logger
	mainJ    *events.Journal
	main     *fsomain.Main
	mainId   uuid.I
	registry *fsoregistry.Registry
	tail     ulid.I
}

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

func NewProcessor(
	names *shorteruuid.Names,
	lg Logger,
	mainJ *events.Journal,
	main *fsomain.Main,
	mainId uuid.I,
	registry *fsoregistry.Registry,
) *Processor {
	return &Processor{
		names:    names,
		lg:       lg,
		mainJ:    mainJ,
		main:     main,
		mainId:   mainId,
		registry: registry,
		tail:     events.EventEpoch,
	}
}

func (p *Processor) Process(ctx context.Context) error {
	// First subscribe, then init, so that no events can get lost.
	updates := make(chan uuid.I, 100)
	p.mainJ.Subscribe(updates, p.mainId)
	defer p.mainJ.Unsubscribe(updates)

	if err := p.initRetry(ctx); err != nil {
		return err
	}

	// Trigger update() from time to time; just in case.
	forceUpdatePeriod := 1 * time.Minute
	ticker := time.NewTicker(forceUpdatePeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		case <-updates:
		}

		if err := p.retryUntilCancel(ctx, "update", func() error {
			return p.update()
		}); err != nil {
			return err
		}
	}
}

func (p *Processor) initRetry(ctx context.Context) error {
	err := p.retryUntilCancel(ctx, "init", func() error {
		return p.init(ctx)
	})
	return err
}

// `init()` is implemented in two ways to illustrate the different approaches:
//
// `initFromFind()` gets the state from the aggregate.  It determines which
// registries need initialization and initializes the watch tail to the
// aggregate VID.
//
// `initFromEvents()` loads events and constructs local state that contains
// just the information that is needed to determine which repos to initialize.
// It initializes the watch tail to the last event that it processed.
//
// Here, the strategy that directly gets the aggregate state, `initFromFind()`,
// is clearly shorter and simpler, because the aggregate state already
// contained all the necessary information.
//
// The strategy that loads events, `initFromEvents()`, might be useful in
// scenarios where the aggregate state does not yet contain the necessary
// details.  Details could be added to the aggregate state, or the events could
// be directly used to avoid modifying the aggregate state.  It might be
// desirabel to keep the aggregate state small.
func (p *Processor) init(ctx context.Context) error {
	return p.initFromFind(ctx)
	// return p.initFromEvents(ctx) // For illustration.
}

func (p *Processor) initFromFind(ctx context.Context) error {
	st, err := p.main.FindId(p.mainId)
	if err != nil {
		return err
	}

	for _, reg := range st.Registries() {
		if !reg.Confirmed {
			err := p.initRegistry(reg.Name)
			if err != nil {
				return err
			}
		}
	}

	p.tail = st.Vid()
	return nil
}

func (p *Processor) initFromEvents(ctx context.Context) error {
	st, err := p.loadStateFromEvents(ctx)
	if err != nil {
		return err
	}

	for reg, _ := range st.newRegistries {
		err := p.initRegistry(reg)
		if err != nil {
			return err
		}
	}

	p.tail = st.vid
	return nil
}

type state struct {
	vid           ulid.I
	newRegistries map[string]struct{}
}

func newState() *state {
	return &state{
		newRegistries: make(map[string]struct{}),
	}
}

func (p *Processor) loadStateFromEvents(ctx context.Context) (*state, error) {
	st := newState()
	err := p.forEachEventAfter(events.EventEpoch, func(
		vid ulid.I, mainEv *pb.Event,
	) error {
		switch mainEv.Event {
		// Not interested in name of main.
		case pb.Event_EV_FSO_MAIN_INITIALIZED:

		case pb.Event_EV_FSO_REGISTRY_ACCEPTED:
			st.AddNewRegistry(mainEv.FsoRegistryName)

		case pb.Event_EV_FSO_REGISTRY_CONFIRMED:
			st.RemoveNewRegistry(mainEv.FsoRegistryName)

		default:
			// Ignore unknown.
			p.lg.Warnw(
				"Ignored unknown main event.",
				"module", "registryinit",
				"event", mainEv.Event,
			)
		}

		st.vid = vid
		return nil
	})
	return st, err
}

func (st *state) AddNewRegistry(name string) {
	st.newRegistries[name] = struct{}{}
}

func (st *state) RemoveNewRegistry(name string) {
	delete(st.newRegistries, name)
}

func (p *Processor) update() error {
	return p.forEachEventAfter(p.tail, func(
		vid ulid.I, mainEv *pb.Event,
	) error {
		switch mainEv.Event {
		// Not interested in name of main.
		case pb.Event_EV_FSO_MAIN_INITIALIZED:

		case pb.Event_EV_FSO_REGISTRY_ACCEPTED:
			regName := mainEv.FsoRegistryName
			err := p.initRegistry(regName)
			if err != nil {
				return err
			}

		// Ignore echo of `ConfirmRegistry()`.
		case pb.Event_EV_FSO_REGISTRY_CONFIRMED:

		default:
			// Ignore unknown.
			p.lg.Warnw(
				"Ignored unknown main event.",
				"module", "registryinit",
				"event", mainEv.Event,
			)
		}

		p.tail = vid
		return nil
	})
}

func (p *Processor) initRegistry(regName string) error {
	id := p.names.UUID(NsFsoRegistry, regName)
	_, err := p.registry.Init(id, &fsoregistry.Info{
		Name: regName,
	})
	if err != nil {
		return err
	}

	_, err = p.main.ConfirmRegistry(p.mainId, fsomain.NoVC, regName)
	if err != nil {
		return err
	}

	p.lg.Infow(
		"Initialized registry.",
		"module", "registryinit",
		"registry", regName,
	)
	return nil
}

func (p *Processor) forEachEventAfter(
	after ulid.I,
	fn func(ulid.I, *pb.Event) error,
) error {
	iter := p.mainJ.Find(p.mainId, after)
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	var ev fsomain.Event
	for iter.Next(&ev) {
		if err := fn(ev.Id(), ev.PbMainEvent()); err != nil {
			return err
		}
	}
	return iterClose()
}

func (p *Processor) retryUntilCancel(
	ctx context.Context, what string, fn func() error,
) error {
	for {
		err := fn()
		if err == nil {
			return nil
		}
		wait := 20 * time.Second
		p.lg.Errorw(
			fmt.Sprintf("Will retry %s.", what),
			"module", "registryinit",
			"err", err,
			"retryIn", wait,
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}
