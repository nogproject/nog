package registryd

import (
	"context"
	slashpath "path"

	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/workflows/durootwf"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/internal/workflows/wfindexes"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) BeginDuRoot(
	ctx context.Context, i *pb.BeginDuRootI,
) (*pb.BeginDuRootO, error) {
	registryName := i.Registry
	rootPath := slashpath.Clean(i.GlobalRoot)
	if err := checkRegistryName(registryName); err != nil {
		return nil, err
	}
	// XXX Maybe `checkGlobalPath()` to confirm wellformed path.
	if err := srv.authAll(
		ctx,
		authzScope{Action: AAFsoReadRegistry, Name: registryName},
		authzScope{Action: AAFsoReadRoot, Path: rootPath},
	); err != nil {
		return nil, err
	}

	registry, err := srv.getRegistryState(registryName)
	if err != nil {
		return nil, err
	}
	if i.Vid != nil {
		vid, err := ulid.ParseBytes(i.Vid)
		if err != nil {
			return nil, ErrMalformedVid
		}
		if vid != registry.Vid() {
			return nil, ErrVersionConflict
		}
	}
	root, ok := registry.Root(rootPath)
	if !ok {
		return nil, ErrUnknownRoot
	}

	wfId, err := uuid.FromBytes(i.Workflow)
	if err != nil {
		return nil, ErrMalformedWorkflowId
	}

	reason, err := srv.idChecker.IsUnusedId(wfId)
	switch {
	case err != nil:
		return nil, err
	case reason != "":
		return nil, status.Errorf(
			codes.FailedPrecondition,
			"rejected workflow ID: %s", reason,
		)
	}

	wfVid, err := srv.duRootWorkflows.Init(wfId, &durootwf.CmdInit{
		RegistryId: registry.Id(),
		GlobalRoot: root.GlobalRoot,
		Host:       root.Host,
		HostRoot:   root.HostRoot,
	})
	if err != nil {
		return nil, asWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, registryName)
	idxVid, err := srv.workflowIndexes.BeginDuRoot(
		idxId, wfindexes.NoVC, &wfindexes.CmdBeginDuRoot{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid,
			GlobalRoot:      root.GlobalRoot,
			Host:            root.Host,
			HostRoot:        root.HostRoot,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	regVid := registry.Vid()
	return &pb.BeginDuRootO{
		RegistryVid:      regVid[:],
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid[:],
	}, nil
}

func (srv *Server) AppendDuRoot(
	ctx context.Context, i *pb.AppendDuRootI,
) (*pb.AppendDuRootO, error) {
	id, err := uuid.FromBytes(i.Workflow)
	if err != nil {
		return nil, ErrMalformedWorkflowId
	}
	workflow, err := srv.duRootWorkflows.FindId(id)
	if err != nil {
		return nil, err // asWorkflowError
	}
	rootPath := workflow.GlobalRoot()
	if err := srv.authPath(ctx, AAFsoExecDu, rootPath); err != nil {
		return nil, err
	}

	vid, err := parseDuRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	for _, p := range i.Paths {
		cmd := &durootwf.CmdAppend{
			Path:  p.Path,
			Usage: p.Usage,
		}
		v, err := srv.duRootWorkflows.Append(id, vid, cmd)
		if err != nil {
			return nil, err // asXError
		}
		vid = v
	}

	return &pb.AppendDuRootO{
		WorkflowVid: vid[:],
	}, nil
}

func (srv *Server) CommitDuRoot(
	ctx context.Context, i *pb.CommitDuRootI,
) (*pb.CommitDuRootO, error) {
	wfId, err := uuid.FromBytes(i.Workflow)
	if err != nil {
		return nil, ErrMalformedWorkflowId
	}
	workflow, err := srv.duRootWorkflows.FindId(wfId)
	if err != nil {
		return nil, err // asWorkflowError
	}
	rootPath := workflow.GlobalRoot()
	if err := srv.authPath(ctx, AAFsoExecDu, rootPath); err != nil {
		return nil, err
	}

	registryId := workflow.RegistryId()
	registry, err := srv.registry.FindId(registryId)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}
	registryName := registry.Name()
	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, registryName)

	wfVid, err := parseDuRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	var wfVid2 ulid.I
	if i.StatusCode == 0 {
		wfVid2, err = srv.duRootWorkflows.Commit(wfId, wfVid)
	} else {
		wfVid2, err = srv.duRootWorkflows.Fail(
			wfId, wfVid, i.StatusCode, i.StatusMessage,
		)
	}
	if err != nil {
		return nil, err // asWorkflowError
	}

	idxVid, err := srv.workflowIndexes.CommitDuRoot(
		idxId, wfindexes.NoVC, &wfindexes.CmdCommitDuRoot{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, err // asWorkfowIndexError
	}

	wfVid3, err := srv.duRootWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, err // asWorkfowError
	}

	return &pb.CommitDuRootO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) GetDuRoot(
	i *pb.GetDuRootI, stream pb.DiskUsage_GetDuRootServer,
) error {
	// `ctx.Done()` indicates client close, see
	// <https://groups.google.com/d/msg/grpc-io/C0rAhtCUhSs/SzFDLGqiCgAJ>.
	ctx := stream.Context()

	// workflow ID -> registry ID -> registry name.
	wfId, err := uuid.FromBytes(i.Workflow)
	if err != nil {
		return ErrMalformedWorkflowId
	}
	workflow, err := srv.duRootWorkflows.FindId(wfId)
	if err != nil {
		return err // asWorkflowError
	}
	registryId := workflow.RegistryId()
	registry, err := srv.registry.FindId(registryId)
	if err != nil {
		return asRegistryGrpcError(err)
	}
	registryName := registry.Name()
	rootPath := workflow.GlobalRoot()

	if err := srv.authAll(
		ctx,
		authzScope{Action: AAFsoReadRegistry, Name: registryName},
		authzScope{Action: AAFsoReadRoot, Path: rootPath},
	); err != nil {
		return err
	}

	switch i.JobControl {
	case pb.JobControl_JC_BACKGROUND:
		return srv.getDuRootNoWait(ctx, stream, wfId)

	case pb.JobControl_JC_WAIT:
		// XXX Would be implemented similar to `Events()`.
		return status.Error(
			codes.Unimplemented, "blocking get not implemented",
		)

	default:
		return ErrMalformedJobControl
	}
}

func (srv *Server) getDuRootNoWait(
	ctx context.Context,
	stream pb.DiskUsage_GetDuRootServer,
	wfId uuid.I,
) error {
	iter := srv.ephWorkflowsJ.Find(wfId, events.EventEpoch)
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	/// XXX Maybe gather events and send batch responses.

	var ev durootwf.Event
	for iter.Next(&ev) {
		vid := ev.Id()
		evx, err := wfevents.ParsePbWorkflowEvent(
			ev.PbWorkflowEvent(),
		)
		if err != nil {
			return ErrParsePb
		}

		switch x := evx.(type) {
		case *wfevents.EvDuUpdated:
			o := &pb.GetDuRootO{
				WorkflowVid: vid[:],
				Paths: []*pb.PathDiskUsage{
					{Path: x.Path, Usage: x.Usage},
				},
			}
			if err := stream.Send(o); err != nil {
				return err
			}

		case *wfevents.EvDuRootCompleted:
			if x.StatusCode != 0 {
				return status.Errorf(
					codes.Unknown, "%s", x.StatusMessage,
				)
			}
			// Send final vid in empty rsp.
			o := &pb.GetDuRootO{
				WorkflowVid: vid[:],
			}
			if err := stream.Send(o); err != nil {
				return err
			}
			return iterClose()

		default:
			// Silently ignore unknown events.
		}
	}

	return ErrPartialResult
}
