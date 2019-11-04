package workflowproc

import (
	"context"
	"errors"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/pingregistrywf"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// XXX The static `pingMessage` could be replaced by a dynamic string that
// contains the Nogfsostad server name, which would be passed in a command line
// argument.
const pingMessage = "nogfsostad ping"

var ErrUnknownEvent = errors.New("unknown event")

type pingRegistryWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	done        chan<- struct{}
	view        pingRegistryWorkflowView
}

type pingRegistryWorkflowView struct {
	workflowId uuid.I
	vid        ulid.I
	scode      pingregistrywf.StateCode
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

	switch ev.(type) {
	case *wfevents.EvPingRegistryStarted:
		view.scode = pingregistrywf.StateInitialized
		return nil

	case *wfevents.EvServerPinged:
		view.scode = pingregistrywf.StateAppending
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
		return a.doPingQuit(ctx, view.workflowId, view.vid)

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
) (bool, error) {
	c := pb.NewPingRegistryClient(a.conn)
	for k := 1; k < 4; k++ {
		i := &pb.ServerPingI{
			Workflow: workflowId[:],
			// No VC to allow other servers to ping concurrently.
			WorkflowVid:   nil,
			StatusCode:    0,
			StatusMessage: pingMessage,
		}
		o, err := c.ServerPing(ctx, i, a.sysRPCCreds)
		switch status.Code(err) {
		case codes.OK:
			break
		case codes.FailedPrecondition:
			// `FailedPrecondition` indicates that the workflow
			// state has advanced, which can only happen if
			// Nogfsoregd posted the ping summary.  If this
			// happens, quit immediately.  A retry would only
			// discover the advanced state and quit then.
			a.lg.Warnw(
				"Ignored failure to ping registry; "+
					"probably due to workflow timeout.",
				"err", err,
			)
			return a.doQuit()
		default:
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
			"message", pingMessage,
			"k", k,
		)
		time.Sleep(1 * time.Second)
	}

	return a.doQuit()
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
