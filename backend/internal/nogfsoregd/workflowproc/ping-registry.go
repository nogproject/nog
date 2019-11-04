package workflowproc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/pingregistrywf"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

const numPings = 3
const messagePrefix = "nogfsoregd ping "

var pingDuration = 10 * time.Second

var ErrUnknownEvent = errors.New("unknown event")

type pingRegistryWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	registry    string
	done        chan<- struct{}
	view        pingRegistryWorkflowView
}

type pingRegistryWorkflowView struct {
	workflowId uuid.I
	vid        ulid.I
	scode      pingregistrywf.StateCode
	startTime  time.Time
	pingCount  int
}

func (a *pingRegistryWorkflowActivity) ProcessRegistryWorkflowEvents(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		view := pingRegistryWorkflowView{
			workflowId: workflowId,
		}
		if err := wfstreams.LoadRegistryWorkflowEventsNoBlock(
			stream, &view,
		); err != nil {
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		}

		done, err := a.processView(ctx, view)
		switch {
		case err != nil:
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		case done:
			return view.vid, nil
		}

		tail = view.vid
		a.view = view
	}

	return wfstreams.WatchRegistryWorkflowEvents(
		ctx, tail, stream, a, a,
	)
}

func (view *pingRegistryWorkflowView) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	view.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvPingRegistryStarted:
		view.startTime = ulid.Time(vid)
		view.scode = pingregistrywf.StateInitialized
		return nil

	case *wfevents.EvServerPinged:
		view.scode = pingregistrywf.StateAppending
		// Count `nogfsoregd` pings.
		if strings.HasPrefix(x.StatusMessage, messagePrefix) {
			view.pingCount++
		}
		return nil

	case *wfevents.EvServerPingsGathered:
		view.scode = pingregistrywf.StateSummarized
		return nil

	case *wfevents.EvPingRegistryCompleted:
		view.scode = pingregistrywf.StateCompleted
		return nil

	case *wfevents.EvPingRegistryCommitted:
		view.scode = pingregistrywf.StateTerminated
		return nil

	default:
		return ErrUnknownEvent
	}
}

func (a *pingRegistryWorkflowActivity) processView(
	ctx context.Context,
	view pingRegistryWorkflowView,
) (bool, error) {
	switch view.scode {
	case pingregistrywf.StateUninitialized:
		return a.doContinue()

	case pingregistrywf.StateInitialized:
		fallthrough
	case pingregistrywf.StateAppending:
		return a.doPingQuit(
			ctx, view.workflowId, view.vid,
			view.startTime, view.pingCount,
		)

	case pingregistrywf.StateSummarized:
		return a.doQuit()

	case pingregistrywf.StateCompleted:
		return a.doQuit()

	case pingregistrywf.StateTerminated:
		return a.doQuit()

	default:
		panic("invalid StateCode")
	}
}

func (a *pingRegistryWorkflowActivity) WatchWorkflowEvent(
	ctx context.Context, vid ulid.I, ev wfevents.WorkflowEvent,
) (bool, error) {
	if err := a.view.LoadWorkflowEvent(vid, ev); err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *pingRegistryWorkflowActivity) WillBlock(
	ctx context.Context,
) (bool, error) {
	return a.processView(ctx, a.view)
}

func (a *pingRegistryWorkflowActivity) doPingQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	startTime time.Time,
	pingCount int,
) (bool, error) {
	c := pb.NewPingRegistryClient(a.conn)
	for k := pingCount + 1; k < numPings+1; k++ {
		msg := fmt.Sprintf("%s%d", messagePrefix, k)
		i := &pb.ServerPingI{
			Workflow: workflowId[:],
			// No VC to allow other servers to ping concurrently.
			WorkflowVid:   nil,
			StatusCode:    0,
			StatusMessage: msg,
		}
		o, err := c.ServerPing(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}

		v, err := ulid.ParseBytes(o.WorkflowVid)
		if err != nil {
			return a.doRetry(err)
		}
		vid = v

		a.lg.Infow(
			"Pinged registry",
			"workflowId", workflowId.String(),
			"message", msg,
		)
		time.Sleep(1 * time.Second)
	}

	remaining := time.Until(startTime.Add(pingDuration))
	if remaining > 0 {
		a.lg.Infow(
			"Sleeping remaining ping-registry duration.",
			"duration", remaining,
		)
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			return a.doRetry(ctx.Err())
		case <-timer.C:
		}
	}

	summary, err := a.pingSummary(ctx, workflowId)
	if err != nil {
		return a.doRetry(err)
	}
	i := &pb.PostServerPingSummaryI{
		Workflow:      workflowId[:],
		WorkflowVid:   summary.vid[:],
		StatusCode:    summary.statusCode,
		StatusMessage: summary.statusMessage,
	}
	_, err = c.PostServerPingSummary(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}

	return a.doQuit()
}

type pingRegistryWorkflowSummary struct {
	vid           ulid.I
	totalCount    int
	errorCount    int
	statusCode    int32
	statusMessage string
}

func (a *pingRegistryWorkflowActivity) pingSummary(
	ctx context.Context,
	workflowId uuid.I,
) (*pingRegistryWorkflowSummary, error) {
	// `cancel2()` ends the stream when returning before its EOF.
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	c := pb.NewEphemeralRegistryClient(a.conn)
	i := &pb.RegistryWorkflowEventsI{
		Registry: a.registry,
		Workflow: workflowId[:],
		Watch:    false,
	}
	stream, err := c.RegistryWorkflowEvents(ctx2, i, a.sysRPCCreds)
	if err != nil {
		return nil, err
	}

	view := pingRegistryWorkflowSummary{}
	if err := wfstreams.LoadRegistryWorkflowEventsNoBlock(
		stream,
		wfstreams.LoaderFunc(func(
			vid ulid.I, ev wfevents.WorkflowEvent,
		) error {
			view.vid = vid

			switch x := ev.(type) {
			case *wfevents.EvServerPinged:
				view.totalCount++
				if x.StatusCode != 0 {
					view.errorCount++
				}
				return nil

			default: // Ignore other events.
				return nil
			}
		}),
	); err != nil {
		return nil, err
	}

	view.statusMessage = fmt.Sprintf(
		"{ totalCount: %d, errorCount: %d }",
		view.totalCount, view.errorCount,
	)
	if view.errorCount > 0 {
		view.statusCode = 1
	}

	return &view, nil
}

func (a *pingRegistryWorkflowActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *pingRegistryWorkflowActivity) doQuit() (bool, error) {
	if a.done != nil {
		close(a.done)
	}
	return true, nil
}

func (a *pingRegistryWorkflowActivity) doRetry(err error) (bool, error) {
	return false, err
}
