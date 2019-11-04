package registryd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsoregistry"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/workflows/unfreezerepowf"
	"github.com/nogproject/nog/backend/internal/workflows/wfindexes"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) BeginUnfreezeRepo(
	ctx context.Context, i *pb.BeginUnfreezeRepoI,
) (*pb.BeginUnfreezeRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoUnfreezeRepo, regName, i.Repo,
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
	if ok, reason := reg.MayUnfreezeRepo(repoId); !ok {
		return nil, status.Errorf(
			codes.FailedPrecondition, "registry: %s", reason,
		)
	}
	if repo.ErrorMessage() != "" {
		return nil, status.Errorf(
			codes.FailedPrecondition, "repo has stored error",
		)
	}

	wfVid, err := srv.unfreezeRepoWorkflows.Init(
		wfId,
		&unfreezerepowf.CmdInit{
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
		return nil, asUnfreezeRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.BeginUnfreezeRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdBeginUnfreezeRepo{
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
	return &pb.BeginUnfreezeRepoO{
		RegistryVid:      regVid[:],
		RepoVid:          repoVid[:],
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid[:],
	}, nil
}

func (srv *Server) BeginUnfreezeRepoFiles(
	ctx context.Context, i *pb.BeginUnfreezeRepoFilesI,
) (*pb.BeginUnfreezeRepoFilesO, error) {
	_, wf, err := srv.authAnyUnfreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnfreezeRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnfreezeRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.unfreezeRepoWorkflows.BeginFiles(wfId, vid)
	if err != nil {
		return nil, asUnfreezeRepoWorkflowGrpcError(err)
	}

	return &pb.BeginUnfreezeRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitUnfreezeRepoFiles(
	ctx context.Context, i *pb.CommitUnfreezeRepoFilesI,
) (*pb.CommitUnfreezeRepoFilesO, error) {
	_, wf, err := srv.authAnyUnfreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnfreezeRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnfreezeRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.unfreezeRepoWorkflows.CommitFiles(wfId, vid)
	if err != nil {
		return nil, asUnfreezeRepoWorkflowGrpcError(err)
	}

	return &pb.CommitUnfreezeRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) AbortUnfreezeRepoFiles(
	ctx context.Context, i *pb.AbortUnfreezeRepoFilesI,
) (*pb.AbortUnfreezeRepoFilesO, error) {
	_, wf, err := srv.authAnyUnfreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnfreezeRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnfreezeRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.unfreezeRepoWorkflows.AbortFiles(
		wfId, vid, i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asUnfreezeRepoWorkflowGrpcError(err)
	}

	return &pb.AbortUnfreezeRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitUnfreezeRepo(
	ctx context.Context, i *pb.CommitUnfreezeRepoI,
) (*pb.CommitUnfreezeRepoO, error) {
	_, wf, err := srv.authAnyUnfreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnfreezeRepo},
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

	wfVid, err := parseUnfreezeRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.unfreezeRepoWorkflows.Commit(wfId, wfVid)
	if err != nil {
		return nil, asUnfreezeRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.CommitUnfreezeRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitUnfreezeRepo{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.unfreezeRepoWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asUnfreezeRepoWorkflowGrpcError(err)
	}

	return &pb.CommitUnfreezeRepoO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) AbortUnfreezeRepo(
	ctx context.Context, i *pb.AbortUnfreezeRepoI,
) (*pb.AbortUnfreezeRepoO, error) {
	_, wf, err := srv.authAnyUnfreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnfreezeRepo},
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

	wfVid, err := parseUnfreezeRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.unfreezeRepoWorkflows.Abort(
		wfId, wfVid,
		i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asUnfreezeRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.CommitUnfreezeRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitUnfreezeRepo{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.unfreezeRepoWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asUnfreezeRepoWorkflowGrpcError(err)
	}

	return &pb.AbortUnfreezeRepoO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) GetUnfreezeRepo(
	ctx context.Context, i *pb.GetUnfreezeRepoI,
) (*pb.GetUnfreezeRepoO, error) {
	_, wf, err := srv.authAnyUnfreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoUnfreezeRepo},
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
			w, err := srv.unfreezeRepoWorkflows.FindId(wfId)
			if err != nil {
				return nil, asUnfreezeRepoWorkflowGrpcError(err)
			}
			wf = w

			switch wf.StateCode() {
			case unfreezerepowf.StateUninitialized: // wait
			case unfreezerepowf.StateInitialized: // wait
			case unfreezerepowf.StateFiles: // wait
			case unfreezerepowf.StateFilesCompleted: // wait
			case unfreezerepowf.StateFilesFailed: // wait
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
	o := &pb.GetUnfreezeRepoO{
		WorkflowVid: wfVid[:],
		Registry:    wf.RegistryName(),
		RepoId:      repoId[:],
	}

	switch wf.StateCode() {
	// `unfreezerepowf.StateUninitialized` has been rejected at top of func.

	case unfreezerepowf.StateInitialized:
		o.StatusCode = int32(pb.StatusCode_SC_RUNNING)
		o.StatusMessage = "initializing"

	case unfreezerepowf.StateFiles:
		o.StatusCode = int32(pb.StatusCode_SC_RUNNING)
		o.StatusMessage = "unfreezing files"

	case unfreezerepowf.StateFilesCompleted:
		o.StatusCode = int32(pb.StatusCode_SC_RUNNING)
		o.StatusMessage = "unfreezing files completed"

	case unfreezerepowf.StateFilesFailed:
		o.StatusCode = int32(pb.StatusCode_SC_FAILED)
		o.StatusMessage = "unfreezing files failed"

	case unfreezerepowf.StateCompleted:
		fallthrough
	case unfreezerepowf.StateFailed:
		fallthrough
	case unfreezerepowf.StateTerminated:
		o.StatusCode = wf.StatusCode()
		o.StatusMessage = wf.StatusMessage()

	default:
		return nil, ErrUnknownWorkflowState
	}

	return o, nil
}

func (srv *Server) RegistryBeginUnfreezeRepo(
	ctx context.Context, i *pb.RegistryBeginUnfreezeRepoI,
) (*pb.RegistryBeginUnfreezeRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecUnfreezeRepo, regName, i.Repo,
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

	cmd := &fsoregistry.CmdBeginUnfreezeRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
	}
	vid2, err := srv.registry.BeginUnfreezeRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryBeginUnfreezeRepoO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) RegistryCommitUnfreezeRepo(
	ctx context.Context, i *pb.RegistryCommitUnfreezeRepoI,
) (*pb.RegistryCommitUnfreezeRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecUnfreezeRepo, regName, i.Repo,
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

	cmd := &fsoregistry.CmdCommitUnfreezeRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
	}
	vid2, err := srv.registry.CommitUnfreezeRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryCommitUnfreezeRepoO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) RegistryAbortUnfreezeRepo(
	ctx context.Context, i *pb.RegistryAbortUnfreezeRepoI,
) (*pb.RegistryAbortUnfreezeRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecUnfreezeRepo, regName, i.Repo,
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

	cmd := &fsoregistry.CmdAbortUnfreezeRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
		Code:       i.StatusCode,
	}
	vid2, err := srv.registry.AbortUnfreezeRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryAbortUnfreezeRepoO{
		RegistryVid: vid2[:],
	}, nil
}
