package workflowproc

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/splitrootwf"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type splitRootWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	done        chan<- struct{}
	view        splitRootWorkflowView
}

type splitRootWorkflowView struct {
	workflowId   uuid.I
	vid          ulid.I
	scode        splitrootwf.StateCode
	root         string
	maxDepth     int32
	minDiskUsage int64
}

func (a *splitRootWorkflowActivity) ProcessRegistryWorkflowEvents(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		view := splitRootWorkflowView{
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

func (view *splitRootWorkflowView) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	view.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvSplitRootStarted:
		view.scode = splitrootwf.StateInitialized
		view.root = x.HostRoot
		view.maxDepth = x.MaxDepth
		view.minDiskUsage = x.MinDiskUsage
		return nil

	case *wfevents.EvSplitRootDuAppended:
		view.scode = splitrootwf.StateDuAppending
		return nil

	// Transition directly to `StateTerminated` when du has completed.
	// Other intermediate states are irrelevant.
	case *wfevents.EvSplitRootDuCompleted:
		view.scode = splitrootwf.StateTerminated
		return nil

	// Handle termination before du completed.  It is unlikely, but it can
	// in principle happen when GC expires the workflow.
	case *wfevents.EvSplitRootCompleted:
		view.scode = splitrootwf.StateTerminated
		return nil

	default: // Silently ignore irrelevant events.
		return nil
	}
}

func (a *splitRootWorkflowActivity) processView(
	ctx context.Context,
	view splitRootWorkflowView,
) (bool, error) {
	switch view.scode {
	case splitrootwf.StateUninitialized:
		return a.doContinue()

	case splitrootwf.StateInitialized:
		return a.doRunDuQuit(
			ctx, view.workflowId, view.vid,
			view.root, view.maxDepth, view.minDiskUsage,
		)

	case splitrootwf.StateDuAppending:
		return a.doAbortDuAndQuit(
			ctx, view.workflowId, view.vid,
			"du was interrupted",
		)

	case splitrootwf.StateTerminated:
		return a.doQuit()

	default:
		panic("invalid StateCode")
	}
}

func (a *splitRootWorkflowActivity) WatchWorkflowEvent(
	ctx context.Context, vid ulid.I, ev wfevents.WorkflowEvent,
) (bool, error) {
	if err := a.view.LoadWorkflowEvent(vid, ev); err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *splitRootWorkflowActivity) WillBlock(
	ctx context.Context,
) (bool, error) {
	return a.processView(ctx, a.view)
}

func (a *splitRootWorkflowActivity) doRunDuQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	root string,
	maxDepth int32,
	minDiskUsage int64,
) (bool, error) {
	doFailDu := func(err error) (bool, error) {
		var msg string
		switch x := err.(type) {
		case *exec.ExitError:
			msg = fmt.Sprintf(
				"du failed: %s; stderr: %s",
				err, string(x.Stderr),
			)
		default:
			msg = fmt.Sprintf("du failed: %s", err)
		}
		return a.doAbortDuAndQuit(ctx, workflowId, vid, msg)
	}

	duCmd := du0Command(
		ctx,
		root,
		fmt.Sprintf("--max-depth=%d", maxDepth),
		fmt.Sprintf("--threshold=%d", minDiskUsage),
	)
	duStdout, err := duCmd.StdoutPipe()
	if err != nil {
		return doFailDu(err)
	}

	if err := duCmd.Start(); err != nil {
		return doFailDu(err)
	}
	// If du is still running on return, kill and wait to avoid zombies.
	defer func() {
		if duCmd == nil {
			return
		}
		_ = duCmd.Process.Kill()
		_ = duCmd.Wait()
	}()

	lines := bufio.NewScanner(duStdout)
	lines.Split(Scan0s)
	c := pb.NewSplitRootClient(a.conn)
	nLines := 0
	for lines.Scan() {
		usage, path, err := parseDuLine(lines.Text())
		if err != nil {
			return doFailDu(err)
		}

		i := &pb.AppendSplitRootDuI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
			Paths: []*pb.PathDiskUsage{{
				Path:  path,
				Usage: usage,
			}},
		}
		o, err := c.AppendSplitRootDu(ctx, i, a.sysRPCCreds)
		switch status.Code(err) {
		case codes.OK:
			break
		case codes.ResourceExhausted:
			// Abort if `ResourceExhausted` to avoid additional
			// resource usage.
			a.lg.Errorw(
				"Could not append split-root du.",
				"err", err,
			)
			return doFailDu(err)
		default:
			return a.doRetry(err)
		}
		v, err := ulid.ParseBytes(o.WorkflowVid)
		if err != nil {
			return a.doRetry(err)
		}
		vid = v
		nLines++
	}

	if err := duCmd.Wait(); err != nil {
		return doFailDu(err)
	}
	duCmd = nil // Disable defer kill.
	if err := lines.Err(); err != nil {
		return doFailDu(err)
	}

	// If du reported something, return.  Otherwise the total must be less
	// than the threshold.  If so, run du without threshold.
	if nLines > 0 {
		return a.doCommitQuit(ctx, workflowId, vid)
	}

	duSumOut, err := du0Command(ctx, root, "--summarize").Output()
	if err != nil {
		return doFailDu(err)
	}
	duSumOut = bytes.TrimRight(duSumOut, "\x00")
	usage, path, err := parseDuLine(string(duSumOut))
	if err != nil {
		return doFailDu(err)
	}

	i := &pb.AppendSplitRootDuI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
		Paths: []*pb.PathDiskUsage{{
			Path:  path,
			Usage: usage,
		}},
	}
	o, err := c.AppendSplitRootDu(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	v, err := ulid.ParseBytes(o.WorkflowVid)
	if err != nil {
		return a.doRetry(err)
	}
	vid = v

	return a.doCommitQuit(ctx, workflowId, vid)
}

func (a *splitRootWorkflowActivity) doCommitQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
) (bool, error) {
	c := pb.NewSplitRootClient(a.conn)
	i := &pb.CommitSplitRootDuI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
	}
	_, err := c.CommitSplitRootDu(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

func (a *splitRootWorkflowActivity) doAbortDuAndQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	message string,
) (bool, error) {
	c := pb.NewSplitRootClient(a.conn)
	i := &pb.AbortSplitRootDuI{
		Workflow:      workflowId[:],
		WorkflowVid:   vid[:],
		StatusCode:    1,
		StatusMessage: message,
	}
	_, err := c.AbortSplitRootDu(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

func (a *splitRootWorkflowActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *splitRootWorkflowActivity) doQuit() (bool, error) {
	if a.done != nil {
		close(a.done)
	}
	return true, nil
}

func (a *splitRootWorkflowActivity) doRetry(err error) (bool, error) {
	return false, err
}
