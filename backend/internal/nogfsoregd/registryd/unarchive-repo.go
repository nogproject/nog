package registryd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsoregistry"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/workflows/unarchiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/wfindexes"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) BeginUnarchiveRepo(
	ctx context.Context, i *pb.BeginUnarchiveRepoI,
) (*pb.BeginUnarchiveRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoUnarchiveRepo, regName, i.Repo,
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
	if ok, reason := reg.MayUnarchiveRepo(repoId); !ok {
		return nil, status.Errorf(
			codes.FailedPrecondition, "registry: %s", reason,
		)
	}
	if repo.ErrorMessage() != "" {
		return nil, status.Errorf(
			codes.FailedPrecondition, "repo has stored error",
		)
	}

	wfVid, err := srv.unarchiveRepoWorkflows.Init(
		wfId,
		&unarchiverepowf.CmdInit{
			RegistryId:       reg.Id(),
			RegistryName:     regName,
			StartRegistryVid: startRegistryVid,
			RepoId:           repoId,
			StartRepoVid:     startRepoVid,
			RepoGlobalPath:   repo.GlobalPath(),
			RepoArchiveURL:   repo.ArchiveURL(),
			TarttTarPath:     repo.TarttTarPath(),
			AuthorName:       i.AuthorName,
			AuthorEmail:      i.AuthorEmail,
		},
	)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.BeginUnarchiveRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdBeginUnarchiveRepo{
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
	return &pb.BeginUnarchiveRepoO{
		RegistryVid:      regVid[:],
		RepoVid:          repoVid[:],
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid[:],
	}, nil
}

func (srv *Server) GetUnarchiveRepo(
	ctx context.Context, i *pb.GetUnarchiveRepoI,
) (*pb.GetUnarchiveRepoO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoUnarchiveRepo},
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
			w, err := srv.unarchiveRepoWorkflows.FindId(wfId)
			if err != nil {
				return nil, asUnarchiveRepoWorkflowGrpcError(err)
			}
			wf = w

			switch wf.StateCode() {
			case unarchiverepowf.StateUninitialized: // wait
			case unarchiverepowf.StateInitialized: // wait
			case unarchiverepowf.StateFiles: // wait
			case unarchiverepowf.StateTartt: // wait
			case unarchiverepowf.StateTarttCompleted: // wait
			case unarchiverepowf.StateFilesCompleted: // wait
			case unarchiverepowf.StateFilesFailed: // wait
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
	o := &pb.GetUnarchiveRepoO{
		WorkflowVid: wfVid[:],
		Registry:    wf.RegistryName(),
		RepoId:      repoId[:],
	}

	switch wf.StateCode() {
	// `unarchiverepowf.StateUninitialized` has been rejected at top of func.

	case unarchiverepowf.StateInitialized:
		o.StatusCode = int32(pb.StatusCode_SC_RUNNING)
		o.StatusMessage = "initializing"

	case unarchiverepowf.StateFiles:
		fallthrough
	case unarchiverepowf.StateTartt:
		fallthrough
	case unarchiverepowf.StateTarttCompleted:
		fallthrough
	case unarchiverepowf.StateTarttFailed:
		fallthrough
	case unarchiverepowf.StateFilesCompleted:
		fallthrough
	case unarchiverepowf.StateFilesFailed:
		o.StatusCode = int32(pb.StatusCode_SC_RUNNING)
		o.StatusMessage = "unarchiving files"

	case unarchiverepowf.StateFilesEnded:
		fallthrough
	case unarchiverepowf.StateGcCompleted:
		fallthrough
	case unarchiverepowf.StateCompleted:
		fallthrough
	case unarchiverepowf.StateFailed:
		fallthrough
	case unarchiverepowf.StateTerminated:
		o.StatusCode = wf.StatusCode()
		o.StatusMessage = wf.StatusMessage()

	default:
		return nil, ErrUnknownWorkflowState
	}

	return o, nil
}

func (srv *Server) BeginUnarchiveRepoFiles(
	ctx context.Context, i *pb.BeginUnarchiveRepoFilesI,
) (*pb.BeginUnarchiveRepoFilesO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnarchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnarchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	cmd := &unarchiverepowf.CmdBeginFiles{
		AclPolicy: i.AclPolicy,
	}
	vid2, err := srv.unarchiveRepoWorkflows.BeginFiles(wfId, vid, cmd)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	return &pb.BeginUnarchiveRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) BeginUnarchiveRepoTartt(
	ctx context.Context, i *pb.BeginUnarchiveRepoTarttI,
) (*pb.BeginUnarchiveRepoTarttO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnarchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnarchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	cmd := &unarchiverepowf.CmdBeginTartt{
		WorkingDir: i.WorkingDir,
	}
	vid2, err := srv.unarchiveRepoWorkflows.BeginTartt(wfId, vid, cmd)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	return &pb.BeginUnarchiveRepoTarttO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitUnarchiveRepoTartt(
	ctx context.Context, i *pb.CommitUnarchiveRepoTarttI,
) (*pb.CommitUnarchiveRepoTarttO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnarchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnarchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.unarchiveRepoWorkflows.CommitTartt(wfId, vid)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	return &pb.CommitUnarchiveRepoTarttO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) AbortUnarchiveRepoTartt(
	ctx context.Context, i *pb.AbortUnarchiveRepoTarttI,
) (*pb.AbortUnarchiveRepoTarttO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnarchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnarchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.unarchiveRepoWorkflows.AbortTartt(
		wfId, vid, i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	return &pb.AbortUnarchiveRepoTarttO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitUnarchiveRepoFiles(
	ctx context.Context, i *pb.CommitUnarchiveRepoFilesI,
) (*pb.CommitUnarchiveRepoFilesO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnarchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnarchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.unarchiveRepoWorkflows.CommitFiles(wfId, vid)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	return &pb.CommitUnarchiveRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) AbortUnarchiveRepoFiles(
	ctx context.Context, i *pb.AbortUnarchiveRepoFilesI,
) (*pb.AbortUnarchiveRepoFilesO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnarchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnarchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.unarchiveRepoWorkflows.AbortFiles(
		wfId, vid, i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	return &pb.AbortUnarchiveRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) EndUnarchiveRepoFiles(
	ctx context.Context, i *pb.EndUnarchiveRepoFilesI,
) (*pb.EndUnarchiveRepoFilesO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnarchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnarchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.unarchiveRepoWorkflows.EndFiles(wfId, vid)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	return &pb.EndUnarchiveRepoFilesO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitUnarchiveRepoGc(
	ctx context.Context, i *pb.CommitUnarchiveRepoGcI,
) (*pb.CommitUnarchiveRepoGcO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnarchiveRepo},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	vid, err := parseUnarchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.unarchiveRepoWorkflows.CommitGc(wfId, vid)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	return &pb.CommitUnarchiveRepoGcO{
		WorkflowVid: vid2[:],
	}, nil
}

func (srv *Server) CommitUnarchiveRepo(
	ctx context.Context, i *pb.CommitUnarchiveRepoI,
) (*pb.CommitUnarchiveRepoO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnarchiveRepo},
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

	wfVid, err := parseUnarchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.unarchiveRepoWorkflows.Commit(wfId, wfVid)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.CommitUnarchiveRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitUnarchiveRepo{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.unarchiveRepoWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	return &pb.CommitUnarchiveRepoO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) AbortUnarchiveRepo(
	ctx context.Context, i *pb.AbortUnarchiveRepoI,
) (*pb.AbortUnarchiveRepoO, error) {
	_, wf, err := srv.authAnyUnarchiveRepoWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecUnarchiveRepo},
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

	wfVid, err := parseUnarchiveRepoVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.unarchiveRepoWorkflows.Abort(
		wfId, wfVid,
		i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.CommitUnarchiveRepo(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitUnarchiveRepo{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.unarchiveRepoWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	return &pb.AbortUnarchiveRepoO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) RegistryBeginUnarchiveRepo(
	ctx context.Context, i *pb.RegistryBeginUnarchiveRepoI,
) (*pb.RegistryBeginUnarchiveRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecUnarchiveRepo, regName, i.Repo,
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

	cmd := &fsoregistry.CmdBeginUnarchiveRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
	}
	vid2, err := srv.registry.BeginUnarchiveRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryBeginUnarchiveRepoO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) RegistryCommitUnarchiveRepo(
	ctx context.Context, i *pb.RegistryCommitUnarchiveRepoI,
) (*pb.RegistryCommitUnarchiveRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecUnarchiveRepo, regName, i.Repo,
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

	cmd := &fsoregistry.CmdCommitUnarchiveRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
	}
	vid2, err := srv.registry.CommitUnarchiveRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryCommitUnarchiveRepoO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) RegistryAbortUnarchiveRepo(
	ctx context.Context, i *pb.RegistryAbortUnarchiveRepoI,
) (*pb.RegistryAbortUnarchiveRepoO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoExecUnarchiveRepo, regName, i.Repo,
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

	cmd := &fsoregistry.CmdAbortUnarchiveRepo{
		RepoId:     repoId,
		WorkflowId: wfId,
		Code:       i.StatusCode,
	}
	vid2, err := srv.registry.AbortUnarchiveRepo(regId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RegistryAbortUnarchiveRepoO{
		RegistryVid: vid2[:],
	}, nil
}
