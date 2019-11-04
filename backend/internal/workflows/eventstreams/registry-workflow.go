//go:generate ./gogen.sh

package eventstreams

import (
	"context"
	"io"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
)

func LoadRegistryWorkflowEventsNoBlock(
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
	loader Loader,
) error {
	for {
		rsp, err := stream.Recv()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		}

		for _, pbev := range rsp.Events {
			vid, err := ulid.ParseBytes(pbev.Id)
			if err != nil {
				return err
			}
			ev, err := wfevents.ParsePbWorkflowEvent(pbev)
			if err != nil {
				return err
			}
			if err := loader.LoadWorkflowEvent(
				vid, ev,
			); err != nil {
				return err
			}
		}

		if rsp.WillBlock {
			return nil
		}
	}
}

func WatchRegistryWorkflowEvents(
	ctx context.Context,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
	watcher Watcher,
	blocker WillBlocker,
) (ulid.I, error) {
	for {
		rsp, err := stream.Recv()
		switch {
		case blocker != nil && err == io.EOF:
			done, err2 := blocker.WillBlock(ctx)
			switch {
			case err2 != nil:
				return tail, err2
			case done:
				return tail, nil
			}
			return tail, err
		case err != nil:
			return tail, err
		}

		for _, pbev := range rsp.Events {
			vid, err := ulid.ParseBytes(pbev.Id)
			if err != nil {
				return tail, err
			}
			ev, err := wfevents.ParsePbWorkflowEvent(pbev)
			if err != nil {
				return tail, err
			}

			done, err := watcher.WatchWorkflowEvent(ctx, vid, ev)
			switch {
			case err != nil:
				return tail, err
			case done:
				return tail, nil
			}

			tail = vid
		}

		if blocker != nil && rsp.WillBlock {
			done, err := blocker.WillBlock(ctx)
			switch {
			case err != nil:
				return tail, err
			case done:
				return tail, nil
			}
		}
	}
}
