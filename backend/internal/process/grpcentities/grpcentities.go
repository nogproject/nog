// Package `grpcentities` contains interfaces for running entity activities via
// gRPC.  The implementations are:
//
//  - Package `grpclazy` runs activities for available events and puts them to
//    sleep until there are new events, using
//    `LiveBroadcast.AggregateSignals()` to receive notifications about new
//    events.
//  - Package `grpceager` opens a gRPC watch stream for each activity.
//  - Package `grpccron` awakes activities at a regular interval to check
//    whether there are new events.
//
// The purpose of an activity it to process the event stream of one aggregate.
// An activity is implemented as a processing function, like
// `ProcessRegistryEvents()`, that is called with the entity identifiers, like
// `registry` or `repoId`, and a gRPC event stream.  The function reads the
// event stream and returns the ID of the last event that it successfully
// processed, called `tail`.  If it is restarted later, the gRPC event stream
// will start after the `tail`.  The `tail` is also passed as an argument to
// the processing function.  The second return value is an `error`, with the
// following meaning:
//
//  - `err == nil`: The activity completed successfully.  It will not be
//    restarted.
//  - `err == io.EOF`: The activity reached the end of the gRPC stream.  But it
//    has not yet completed.  It needs to be restarted when there are new
//    events.
//  - `err == context.Canceled`, also `grpc/codes.Canceled`: Processing
//    stopped, because the context or stream canceled.  This likely indicates
//    shutdown of activity processing.
//  - `err == SilentRetry`: Like other `err != nil`, but the retry should not
//    be logged.
//  - `err` has type `*SilentRetryAfter`: Silently retry after `err.After`.
//  - other `err != nil`: A temporary error occurred.  The activity needs to be
//    be restarted to retry processing.
//
// Activities should be prepared to handle both gRPC streams that end with
// `io.EOF` and gRPC streams that block until there are new events.  If the
// specific engine that runs the activity is known, the activity may handle
// only one of the two cases:
//
//  - `io.EOF`: packages `grpclazy` and `grpcron`;
//  - blocking: package `grcpeager`.
//
// An activity need not return as quickly as possible.  It may perform
// long-running background processing tasks.  But it must quickly return when
// the processing context cancels.
//
// The recommended implementation strategy for activities is: When called for
// the first time, load the gRPC stream until it ends or blocks without causing
// side effects, building a view of the aggregate state.  Then start processing
// based on the view.  If an error happens during initial processing, return
// `ulid.Nil` to restart the activity with loading the full stream.  After
// initial processing succeeded, switch to watching the stream and process new
// events one by one.  When watching, return the last processed event as the
// new tail to restart watching with the next event.
package grpcentities

import (
	"context"
	"errors"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

var SilentRetry = errors.New("silent retry")

func IsSilentRetry(err error) bool {
	return err == SilentRetry
}

type SilentRetryAfter struct {
	After time.Time
}

func (err *SilentRetryAfter) Error() string {
	return "retry after " + err.After.Format(time.RFC3339)
}

func IsSilentRetryAfter(err error) bool {
	_, ok := err.(*SilentRetryAfter)
	return ok
}

type RegistryActivity interface {
	ProcessRegistryEvents(
		ctx context.Context,
		registry string,
		tail ulid.I,
		stream pb.Registry_EventsClient,
	) (ulid.I, error)
}

type RegistryActivityFunc func(
	ctx context.Context,
	registry string,
	tail ulid.I,
	stream pb.Registry_EventsClient,
) (ulid.I, error)

func (f RegistryActivityFunc) ProcessRegistryEvents(
	ctx context.Context,
	registry string,
	tail ulid.I,
	stream pb.Registry_EventsClient,
) (ulid.I, error) {
	return f(ctx, registry, tail, stream)
}

type RegistryWorkflowIndexActivity interface {
	ProcessRegistryWorkflowIndexEvents(
		ctx context.Context,
		registry string,
		tail ulid.I,
		stream pb.EphemeralRegistry_RegistryWorkflowIndexEventsClient,
	) (ulid.I, error)
}

type RegistryWorkflowIndexActivityFunc func(
	ctx context.Context,
	registry string,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowIndexEventsClient,
) (ulid.I, error)

func (f RegistryWorkflowIndexActivityFunc) ProcessRegistryEvents(
	ctx context.Context,
	registry string,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowIndexEventsClient,
) (ulid.I, error) {
	return f(ctx, registry, tail, stream)
}

type RegistryWorkflowActivity interface {
	ProcessRegistryWorkflowEvents(
		ctx context.Context,
		registry string,
		workflowId uuid.I,
		tail ulid.I,
		stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
	) (ulid.I, error)
}

type RegistryWorkflowActivityFunc func(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error)

func (f RegistryWorkflowActivityFunc) ProcessRegistryWorkflowEvents(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error) {
	return f(ctx, registry, workflowId, tail, stream)
}

type RepoActivity interface {
	ProcessRepoEvents(
		ctx context.Context,
		repoId uuid.I,
		tail ulid.I,
		stream pb.Repos_EventsClient,
	) (ulid.I, error)
}

type RepoActivityFunc func(
	ctx context.Context,
	repoId uuid.I,
	tail ulid.I,
	stream pb.Repos_EventsClient,
) (ulid.I, error)

func (f RepoActivityFunc) ProcessRepoEvents(
	ctx context.Context,
	repoId uuid.I,
	tail ulid.I,
	stream pb.Repos_EventsClient,
) (ulid.I, error) {
	return f(ctx, repoId, tail, stream)
}

type RepoWorkflowActivity interface {
	ProcessRepoWorkflowEvents(
		ctx context.Context,
		repoId uuid.I,
		workflowId uuid.I,
		tail ulid.I,
		stream pb.Repos_WorkflowEventsClient,
	) (ulid.I, error)
}

type RepoWorkflowActivityFunc func(
	ctx context.Context,
	repoId uuid.I,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.Repos_WorkflowEventsClient,
) (ulid.I, error)

func (f RepoWorkflowActivityFunc) ProcessRepoWorkflowEvents(
	ctx context.Context,
	repoId uuid.I,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.Repos_WorkflowEventsClient,
) (ulid.I, error) {
	return f(ctx, repoId, workflowId, tail, stream)
}

type RegistryEngine interface {
	StartRegistryActivity(registry string, act RegistryActivity) error
}

type RegistryWorkflowIndexEngine interface {
	StartRegistryWorkflowIndexActivity(
		registry string, act RegistryWorkflowIndexActivity,
	) error
}

type RegistryWorkflowEngine interface {
	StartRegistryWorkflowActivity(
		registry string,
		workflowId uuid.I,
		act RegistryWorkflowActivity,
	) error
}

type RepoEngine interface {
	StartRepoActivity(repoId uuid.I, act RepoActivity) error
}

type RepoWorkflowEngine interface {
	StartRepoWorkflowActivity(
		repoId uuid.I, workflowId uuid.I, act RepoWorkflowActivity,
	) error
}

type Engine interface {
	RegistryEngine
	RegistryWorkflowIndexEngine
	RegistryWorkflowEngine
	RepoEngine
	RepoWorkflowEngine
}
