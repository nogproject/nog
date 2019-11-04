package eventstreams

import (
	"context"

	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
)

type Loader interface {
	LoadWorkflowEvent(vid ulid.I, ev wfevents.WorkflowEvent) error
}

type LoaderFunc func(vid ulid.I, ev wfevents.WorkflowEvent) error

func (f LoaderFunc) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	return f(vid, ev)
}

type Watcher interface {
	WatchWorkflowEvent(
		ctx context.Context,
		vid ulid.I,
		ev wfevents.WorkflowEvent,
	) (done bool, err error)
}

type WatcherFunc func(
	ctx context.Context,
	vid ulid.I,
	ev wfevents.WorkflowEvent,
) (done bool, err error)

func (f WatcherFunc) WatchWorkflowEvent(
	ctx context.Context,
	vid ulid.I,
	ev wfevents.WorkflowEvent,
) (done bool, err error) {
	return f(ctx, vid, ev)
}

type WillBlocker interface {
	WillBlock(ctx context.Context) (done bool, err error)
}

type WillBlockerFunc func(ctx context.Context) (done bool, err error)

func (f WillBlockerFunc) WillBlock(
	ctx context.Context,
) (done bool, err error) {
	return f(ctx)
}
