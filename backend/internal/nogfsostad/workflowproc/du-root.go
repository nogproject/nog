package workflowproc

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/workflows/durootwf"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

type duRootWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	done        chan<- struct{}
	view        duRootWorkflowView
}

type duRootWorkflowView struct {
	workflowId uuid.I
	vid        ulid.I
	scode      durootwf.StateCode
	root       string
	paths      map[string]struct{}
}

func (v *duRootWorkflowView) addPath(p string) {
	if v.paths == nil {
		v.paths = make(map[string]struct{})
	}
	v.paths[p] = struct{}{}
}

func (a *duRootWorkflowActivity) ProcessRegistryWorkflowEvents(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		view := duRootWorkflowView{
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

func (view *duRootWorkflowView) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	view.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvDuRootStarted:
		view.scode = durootwf.StateInitialized
		view.root = x.HostRoot
		return nil

	case *wfevents.EvDuUpdated:
		view.scode = durootwf.StateAppending
		view.addPath(x.Path)
		return nil

	case *wfevents.EvDuRootCompleted:
		view.scode = durootwf.StateCompleted
		return nil

	case *wfevents.EvDuRootCommitted:
		view.scode = durootwf.StateTerminated
		return nil

	default: // Silently ignore unknown events.
		return nil
	}
}

func (a *duRootWorkflowActivity) processView(
	ctx context.Context,
	view duRootWorkflowView,
) (bool, error) {
	switch view.scode {
	case durootwf.StateUninitialized:
		return a.doContinue()

	case durootwf.StateInitialized:
		return a.doRunDuQuit(
			ctx, view.workflowId, view.vid, view.root,
		)

	case durootwf.StateAppending:
		return a.doReRunDuQuit(
			ctx, view.workflowId, view.vid, view.root, view.paths,
		)

	case durootwf.StateCompleted:
		return a.doReCommitQuit(ctx, view.workflowId)

	case durootwf.StateTerminated:
		return a.doQuit()

	default:
		panic("invalid StateCode")
	}
}

func (a *duRootWorkflowActivity) WatchWorkflowEvent(
	ctx context.Context, vid ulid.I, ev wfevents.WorkflowEvent,
) (bool, error) {
	if err := a.view.LoadWorkflowEvent(vid, ev); err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *duRootWorkflowActivity) WillBlock(
	ctx context.Context,
) (bool, error) {
	return a.processView(ctx, a.view)
}

func (a *duRootWorkflowActivity) doRunDuQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	root string,
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
		return a.doFailWorkflowQuit(ctx, workflowId, vid, msg)
	}

	duCmd := du0Command(ctx, root, "--threshold=10G", "--max-depth=3")
	duStdout, err := duCmd.StdoutPipe()
	if err != nil {
		return doFailDu(err)
	}

	if err := duCmd.Start(); err != nil {
		return doFailDu(err)
	}
	// If du is still running on return, kill it and wait to avoid zombies.
	defer func() {
		if duCmd == nil {
			return
		}
		_ = duCmd.Process.Kill()
		_ = duCmd.Wait()
	}()

	lines := bufio.NewScanner(duStdout)
	lines.Split(Scan0s)
	c := pb.NewDiskUsageClient(a.conn)
	nLines := 0
	for lines.Scan() {
		usage, path, err := parseDuLine(lines.Text())
		if err != nil {
			return doFailDu(err)
		}

		i := &pb.AppendDuRootI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
			Paths: []*pb.PathDiskUsage{{
				Path:  path,
				Usage: usage,
			}},
		}
		o, err := c.AppendDuRoot(ctx, i, a.sysRPCCreds)
		if err != nil {
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

	// If du reported at least a total, return.  Otherwise the total must
	// be less than the threshold: run du summary without threshold.
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

	i := &pb.AppendDuRootI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
		Paths: []*pb.PathDiskUsage{{
			Path:  path,
			Usage: usage,
		}},
	}
	o, err := c.AppendDuRoot(ctx, i, a.sysRPCCreds)
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

func (a *duRootWorkflowActivity) doReRunDuQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	root string,
	paths map[string]struct{},
) (bool, error) {
	// Do not re-run.  Fail workflow instead.
	return a.doFailWorkflowQuit(ctx, workflowId, vid, "du was interrupted")
}

func (a *duRootWorkflowActivity) doCommitQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
) (bool, error) {
	c := pb.NewDiskUsageClient(a.conn)
	i := &pb.CommitDuRootI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
	}
	_, err := c.CommitDuRoot(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

func (a *duRootWorkflowActivity) doFailWorkflowQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	message string,
) (bool, error) {
	c := pb.NewDiskUsageClient(a.conn)
	i := &pb.CommitDuRootI{
		Workflow:      workflowId[:],
		WorkflowVid:   vid[:],
		StatusCode:    1,
		StatusMessage: message,
	}
	_, err := c.CommitDuRoot(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

// `doReCommitQuit()` tries to terminate a workflow that has already been
// committed.  It avoids errors and retries, because the workflow effect has
// already been achieved, specifically:
//
//  - It calls gRPC `CommitDuRoot()` without version control.
//  - It ignores errors.
//
// If `doReCommitQuit()` ignored an error, it might be retried after a server
// restart, depending on how much progress it made before the error happened.
func (a *duRootWorkflowActivity) doReCommitQuit(
	ctx context.Context,
	workflowId uuid.I,
) (bool, error) {
	c := pb.NewDiskUsageClient(a.conn)
	i := &pb.CommitDuRootI{
		Workflow: workflowId[:],
	}
	_, err := c.CommitDuRoot(ctx, i, a.sysRPCCreds)
	if err != nil {
		a.lg.Errorw(
			"Ignored error to re-commit du-root workflow.",
			"err", err,
		)
	}
	return a.doQuit()
}

func (a *duRootWorkflowActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *duRootWorkflowActivity) doQuit() (bool, error) {
	if a.done != nil {
		close(a.done)
	}
	return true, nil
}

func (a *duRootWorkflowActivity) doRetry(err error) (bool, error) {
	return false, err
}
