package registryd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsoregistry"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/workflows/archiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/wfindexes"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) BeginArchiveRepo(
	ctx context.Context, i *pb.BeginArchiveRepoI,
) (*pb.BeginArchiveRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoArchiveRepo, regName, i.Repo,
	)
	if err != nil {
		return nil, err
	}
	startRegistryVid := ulid.Nil
	if vidBytes := i.RegistryVid; vidBytes != nil {
		vid, err := ulid.ParseBytes(vidBytes)
		if err != nil {
			return nil, ErrMalformedVid
		}
		if vid != reg.Vid() {
			return nil, ErrVersionConflict
		}
		startRegistryVid = vid
	}

	repo, err := srv.repos.FindId(repoId)
	if err != nil {
		err = status.Errorf(codes.Unknown, "repos error: %v", err)
		return nil, err
	}
	startRepoVid := ulid.Nil
	if vidBytes := i.RepoVid; vidBytes != nil {
		vid, err := ulid.ParseBytes(vidBytes)
		if err != nil {
			return nil, ErrMalformedVid
		}
		if vid != repo.Vid() {
			return nil, ErrVersionConflict
		}
		startRepoVid = vid
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

	// Check some preconditions to avoid initializing a workflow that would
	// very likely fail.  The checks here are an optimization.  The real
	// checks happen when the workflow executes registry and repo commands.
	// Even if the preconditions are satisfied now, the workflow may later
	// fail due to concurrent state changes.
	if ok, reason := reg.MayArchiveRepo(repoId); !ok {
		return nil, status.Errorf(
			codes.FailedPrecondition, "registry: %s", reason,
		)
	}
	if ok, reason := repo.MayArchive(); !ok {
		return nil, status.Errorf(
			codes.FailedPrecondition, "repo: %s", reason,
		)
	}

	wfVid, err := srv.archiveRepoWorkflows.Init(
		wfId,
		&archiverepowf.CmdInit{
			RegistryId:       reg.Id(),
			RegistryName:     regName,
			StartRegistryVid: startRegistryVid,
			RepoId:           repoId,
			StartRepoVid:     startRepoVid,
			RepoGlobalPath:   repo.GlobalPath(),
			AuthorName:       i.AuthorName,
			AuthorEmail:      i.AuthorEmail,
		},
	)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.BeginArchiveRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdBeginArchiveRepo{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid,
			GlobalPath:      repo.GlobalPath(),
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	regVid := reg.Vid()
	repoVid := repo.Vid()
	return &pb.BeginArchiveRepoO{
		RegistryVid:      regVid[:],
		RepoVid:          repoVid[:],
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid[:],
	}, nil
}

func (srv *Server) GetArchiveRepo(
	ctx context.Context, i *pb.GetArchiveRepoI,
) (*pb.GetArchiveRepoO, error) {
	_, wf, err := srv.authAnyArchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoArchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	// If JC_WAIT, wait at least for analysis.
	if i.JobControl == pb.JobControl_JC_WAIT {
		// Subscribe first, then find to ensure that no event is lost.
		updated := make(chan uuid.I, 1)
		srv.ephWorkflowsJ.Subscribe(updated, wfId)
		defer srv.ephWorkflowsJ.Unsubscribe(updated)

	Loop:
		for {
			w, err := srv.archiveRepoWorkflows.FindId(wfId)
			if err != nil {
				return nil, asArchiveRepoWorkflowGrpcError(err)
			}
			wf = w

			switch wf.StateCode() {
			case archiverepowf.StateUninitialized: // wait
			case archiverepowf.StateInitialized: // wait
			case archiverepowf.StateFiles: // wait
			case archiverepowf.StateTarttCompleted: // wait
			case archiverepowf.StateSwapStarted: // wait
			case archiverepowf.StateFilesCompleted: // wait
			case archiverepowf.StateFilesFailed: // wait
			default:
				break Loop
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-updated:
				continue Loop
			}
		}
	}

	wfVid := wf.Vid()
	repoId := wf.RepoId()
	o := &pb.GetArchiveRepoO{
		WorkflowVid: wfVid[:],
		Registry:    wf.RegistryName(),
		RepoId:      repoId[:],
	}

	switch wf.StateCode() {
	// `archiverepowf.StateUninitialized` has been rejected at top of func.

	case archiverepowf.StateInitialized:
		o.StatusCode = int32(pb.StatusCode_SC_RUNNING)
		o.StatusMessage = "initializing"

	case archiverepowf.StateFiles:
		fallthrough
	case archiverepowf.StateTarttCompleted:
		fallthrough
	case archiverepowf.StateSwapStarted:
		fallthrough
	case archiverepowf.StateFilesCompleted:
		fallthrough
	case archiverepowf.StateFilesFailed:
		o.StatusCode = int32(pb.StatusCode_SC_RUNNING)
		o.StatusMessage = "archiving files"

	case archiverepowf.StateFilesEnded:
		fallthrough
	case archiverepowf.StateGcCompleted:
		fallthrough
	case archiverepowf.StateCompleted:
		fallthrough
	case archiverepowf.StateFailed:
		fallthrough
	case archiverepowf.StateTerminated:
		o.StatusCode = wf.StatusCode()
		o.StatusMessage = wf.StatusMessage()

	default:
		return nil, ErrUnknownWorkflowState
	}

	return o, nil
}

func (srv *Server) BeginArchiveRepoFiles(
	ctx context.Context, i *pb.BeginArchiveRepoFilesI,
) (*pb.BeginArchiveRepoFilesO, error) {
	_, wf, err := srv.authAnyArchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecArchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseArchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	cmd := &archiverepowf.CmdBeginFiles{
		AclPolicy: i.AclPolicy,
	}
	vid2, err := srv.archiveRepoWorkflows.BeginFiles(wfId, vid, cmd)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	return &pb.BeginArchiveRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitArchiveRepoTartt(
	ctx context.Context, i *pb.CommitArchiveRepoTarttI,
) (*pb.CommitArchiveRepoTarttO, error) {
	_, wf, err := srv.authAnyArchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecArchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseArchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	cmd := &archiverepowf.CmdCommitTartt{
		TarPath: i.TarPath,
	}
	vid2, err := srv.archiveRepoWorkflows.CommitTartt(wfId, vid, cmd)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	return &pb.CommitArchiveRepoTarttO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) BeginArchiveRepoSwap(
	ctx context.Context, i *pb.BeginArchiveRepoSwapI,
) (*pb.BeginArchiveRepoSwapO, error) {
	_, wf, err := srv.authAnyArchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecArchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseArchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	cmd := &archiverepowf.CmdBeginSwap{
		WorkingDir: i.WorkingDir,
	}
	vid2, err := srv.archiveRepoWorkflows.BeginSwap(wfId, vid, cmd)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	return &pb.BeginArchiveRepoSwapO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitArchiveRepoFiles(
	ctx context.Context, i *pb.CommitArchiveRepoFilesI,
) (*pb.CommitArchiveRepoFilesO, error) {
	_, wf, err := srv.authAnyArchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecArchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseArchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.archiveRepoWorkflows.CommitFiles(wfId, vid)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	return &pb.CommitArchiveRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) AbortArchiveRepoFiles(
	ctx context.Context, i *pb.AbortArchiveRepoFilesI,
) (*pb.AbortArchiveRepoFilesO, error) {
	_, wf, err := srv.authAnyArchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecArchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseArchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.archiveRepoWorkflows.AbortFiles(
		wfId, vid, i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	return &pb.AbortArchiveRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) EndArchiveRepoFiles(
	ctx context.Context, i *pb.EndArchiveRepoFilesI,
) (*pb.EndArchiveRepoFilesO, error) {
	_, wf, err := srv.authAnyArchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecArchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseArchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.archiveRepoWorkflows.EndFiles(wfId, vid)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	return &pb.EndArchiveRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitArchiveRepoGc(
	ctx context.Context, i *pb.CommitArchiveRepoGcI,
) (*pb.CommitArchiveRepoGcO, error) {
	_, wf, err := srv.authAnyArchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecArchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseArchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.archiveRepoWorkflows.CommitGc(wfId, vid)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	return &pb.CommitArchiveRepoGcO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitArchiveRepo(
	ctx context.Context, i *pb.CommitArchiveRepoI,
) (*pb.CommitArchiveRepoO, error) {
	_, wf, err := srv.authAnyArchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecArchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	reg, err := srv.registry.FindId(wf.RegistryId())
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}
	regName := reg.Name()

	wfVid, err := parseArchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.archiveRepoWorkflows.Commit(wfId, wfVid)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.CommitArchiveRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitArchiveRepo{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.archiveRepoWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	return &pb.CommitArchiveRepoO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) AbortArchiveRepo(
	ctx context.Context, i *pb.AbortArchiveRepoI,
) (*pb.AbortArchiveRepoO, error) {
	_, wf, err := srv.authAnyArchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecArchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	reg, err := srv.registry.FindId(wf.RegistryId())
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}
	regName := reg.Name()

	wfVid, err := parseArchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.archiveRepoWorkflows.Abort(
		wfId, wfVid,
		i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.CommitArchiveRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitArchiveRepo{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.archiveRepoWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asArchiveRepoWorkflowGrpcError(err)
	}

	return &pb.AbortArchiveRepoO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) RegistryBeginArchiveRepo(
	ctx context.Context, i *pb.RegistryBeginArchiveRepoI,
) (*pb.RegistryBeginArchiveRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecArchiveRepo, regName, i.Repo,
	)
	if err != nil {
		return nil, err
	}
	regId := reg.Id()

	vid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}

	wfId, err := parseWorkflowId(i.Workflow)
	if err != nil {
		return nil, err
	}

	cmd := &fsoregistry.CmdBeginArchiveRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
	}
	vid2, err := srv.registry.BeginArchiveRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryBeginArchiveRepoO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) RegistryCommitArchiveRepo(
	ctx context.Context, i *pb.RegistryCommitArchiveRepoI,
) (*pb.RegistryCommitArchiveRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecArchiveRepo, regName, i.Repo,
	)
	if err != nil {
		return nil, err
	}
	regId := reg.Id()

	vid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}

	wfId, err := parseWorkflowId(i.Workflow)
	if err != nil {
		return nil, err
	}

	cmd := &fsoregistry.CmdCommitArchiveRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
	}
	vid2, err := srv.registry.CommitArchiveRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryCommitArchiveRepoO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) RegistryAbortArchiveRepo(
	ctx context.Context, i *pb.RegistryAbortArchiveRepoI,
) (*pb.RegistryAbortArchiveRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecArchiveRepo, regName, i.Repo,
	)
	if err != nil {
		return nil, err
	}
	regId := reg.Id()

	vid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}

	wfId, err := parseWorkflowId(i.Workflow)
	if err != nil {
		return nil, err
	}

	cmd := &fsoregistry.CmdAbortArchiveRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
		Code:       i.StatusCode,
	}
	vid2, err := srv.registry.AbortArchiveRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryAbortArchiveRepoO{
		RegistryVid: vid2[:],
	}, nil
}
