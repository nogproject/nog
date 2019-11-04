package registryd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsoregistry"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/workflows/freezerepowf"
	"github.com/nogproject/nog/backend/internal/workflows/wfindexes"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) BeginFreezeRepo(
	ctx context.Context, i *pb.BeginFreezeRepoI,
) (*pb.BeginFreezeRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoFreezeRepo, regName, i.Repo,
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
	if ok, reason := reg.MayFreezeRepo(repoId); !ok {
		return nil, status.Errorf(
			codes.FailedPrecondition, "registry: %s", reason,
		)
	}
	if repo.ErrorMessage() != "" {
		return nil, status.Errorf(
			codes.FailedPrecondition, "repo has stored error",
		)
	}

	wfVid, err := srv.freezeRepoWorkflows.Init(
		wfId,
		&freezerepowf.CmdInit{
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
		return nil, asFreezeRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.BeginFreezeRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdBeginFreezeRepo{
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
	return &pb.BeginFreezeRepoO{
		RegistryVid:      regVid[:],
		RepoVid:          repoVid[:],
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid[:],
	}, nil
}

func (srv *Server) BeginFreezeRepoFiles(
	ctx context.Context, i *pb.BeginFreezeRepoFilesI,
) (*pb.BeginFreezeRepoFilesO, error) {
	_, wf, err := srv.authAnyFreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecFreezeRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseFreezeRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.freezeRepoWorkflows.BeginFiles(wfId, vid)
	if err != nil {
		return nil, asFreezeRepoWorkflowGrpcError(err)
	}

	return &pb.BeginFreezeRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitFreezeRepoFiles(
	ctx context.Context, i *pb.CommitFreezeRepoFilesI,
) (*pb.CommitFreezeRepoFilesO, error) {
	_, wf, err := srv.authAnyFreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecFreezeRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseFreezeRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.freezeRepoWorkflows.CommitFiles(wfId, vid)
	if err != nil {
		return nil, asFreezeRepoWorkflowGrpcError(err)
	}

	return &pb.CommitFreezeRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) AbortFreezeRepoFiles(
	ctx context.Context, i *pb.AbortFreezeRepoFilesI,
) (*pb.AbortFreezeRepoFilesO, error) {
	_, wf, err := srv.authAnyFreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecFreezeRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseFreezeRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.freezeRepoWorkflows.AbortFiles(
		wfId, vid, i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asFreezeRepoWorkflowGrpcError(err)
	}

	return &pb.AbortFreezeRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitFreezeRepo(
	ctx context.Context, i *pb.CommitFreezeRepoI,
) (*pb.CommitFreezeRepoO, error) {
	_, wf, err := srv.authAnyFreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecFreezeRepo},
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

	wfVid, err := parseFreezeRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.freezeRepoWorkflows.Commit(wfId, wfVid)
	if err != nil {
		return nil, asFreezeRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.CommitFreezeRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitFreezeRepo{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.freezeRepoWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asFreezeRepoWorkflowGrpcError(err)
	}

	return &pb.CommitFreezeRepoO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) AbortFreezeRepo(
	ctx context.Context, i *pb.AbortFreezeRepoI,
) (*pb.AbortFreezeRepoO, error) {
	_, wf, err := srv.authAnyFreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecFreezeRepo},
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

	wfVid, err := parseFreezeRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.freezeRepoWorkflows.Abort(
		wfId, wfVid,
		i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asFreezeRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.CommitFreezeRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitFreezeRepo{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.freezeRepoWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asFreezeRepoWorkflowGrpcError(err)
	}

	return &pb.AbortFreezeRepoO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) GetFreezeRepo(
	ctx context.Context, i *pb.GetFreezeRepoI,
) (*pb.GetFreezeRepoO, error) {
	_, wf, err := srv.authAnyFreezeRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoFreezeRepo},
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
			w, err := srv.freezeRepoWorkflows.FindId(wfId)
			if err != nil {
				return nil, asFreezeRepoWorkflowGrpcError(err)
			}
			wf = w

			switch wf.StateCode() {
			case freezerepowf.StateUninitialized: // wait
			case freezerepowf.StateInitialized: // wait
			case freezerepowf.StateFiles: // wait
			case freezerepowf.StateFilesCompleted: // wait
			case freezerepowf.StateFilesFailed: // wait
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
	o := &pb.GetFreezeRepoO{
		WorkflowVid: wfVid[:],
		Registry:    wf.RegistryName(),
		RepoId:      repoId[:],
	}

	switch wf.StateCode() {
	// `freezerepowf.StateUninitialized` has been rejected at top of func.

	case freezerepowf.StateInitialized:
		o.StatusCode = int32(pb.StatusCode_SC_RUNNING)
		o.StatusMessage = "initializing"

	case freezerepowf.StateFiles:
		o.StatusCode = int32(pb.StatusCode_SC_RUNNING)
		o.StatusMessage = "freezing files"

	case freezerepowf.StateFilesCompleted:
		o.StatusCode = int32(pb.StatusCode_SC_RUNNING)
		o.StatusMessage = "freezing files completed"

	case freezerepowf.StateFilesFailed:
		o.StatusCode = int32(pb.StatusCode_SC_FAILED)
		o.StatusMessage = "freezing files failed"

	case freezerepowf.StateCompleted:
		fallthrough
	case freezerepowf.StateFailed:
		fallthrough
	case freezerepowf.StateTerminated:
		o.StatusCode = wf.StatusCode()
		o.StatusMessage = wf.StatusMessage()

	default:
		return nil, ErrUnknownWorkflowState
	}

	return o, nil
}

func (srv *Server) RegistryBeginFreezeRepo(
	ctx context.Context, i *pb.RegistryBeginFreezeRepoI,
) (*pb.RegistryBeginFreezeRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecFreezeRepo, regName, i.Repo,
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

	cmd := &fsoregistry.CmdBeginFreezeRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
	}
	vid2, err := srv.registry.BeginFreezeRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryBeginFreezeRepoO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) RegistryCommitFreezeRepo(
	ctx context.Context, i *pb.RegistryCommitFreezeRepoI,
) (*pb.RegistryCommitFreezeRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecFreezeRepo, regName, i.Repo,
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

	cmd := &fsoregistry.CmdCommitFreezeRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
	}
	vid2, err := srv.registry.CommitFreezeRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryCommitFreezeRepoO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) RegistryAbortFreezeRepo(
	ctx context.Context, i *pb.RegistryAbortFreezeRepoI,
) (*pb.RegistryAbortFreezeRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecFreezeRepo, regName, i.Repo,
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

	cmd := &fsoregistry.CmdAbortFreezeRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
		Code:       i.StatusCode,
	}
	vid2, err := srv.registry.AbortFreezeRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryAbortFreezeRepoO{
		RegistryVid: vid2[:],
	}, nil
}
