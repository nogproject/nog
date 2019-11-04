package reposd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsoauthz"
	"github.com/nogproject/nog/backend/internal/fsorepos"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const AAFsoAdminRepo = fsoauthz.AAFsoAdminRepo
const AAFsoConfirmRepo = fsoauthz.AAFsoConfirmRepo
const AAFsoExecArchiveRepo = fsoauthz.AAFsoExecArchiveRepo
const AAFsoExecUnarchiveRepo = fsoauthz.AAFsoExecUnarchiveRepo
const AAFsoExecFreezeRepo = fsoauthz.AAFsoExecFreezeRepo
const AAFsoExecUnfreezeRepo = fsoauthz.AAFsoExecUnfreezeRepo
const AAFsoInitRepo = fsoauthz.AAFsoInitRepo
const AAFsoInitRepoShadowBackup = fsoauthz.AAFsoInitRepoShadowBackup
const AAFsoInitRepoTartt = fsoauthz.AAFsoInitRepoTartt
const AAFsoReadRepo = fsoauthz.AAFsoReadRepo

func (srv *Server) authRepoId(
	ctx context.Context, action auth.Action, repo []byte,
) (uuid.I, error) {
	st, err := srv.authRepoIdState(ctx, action, repo)
	if err != nil {
		return uuid.Nil, err
	}
	return st.Id(), nil
}

func (srv *Server) authRepoIdState(
	ctx context.Context, action auth.Action, repo []byte,
) (*fsorepos.State, error) {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	repoId, err := parseRepoId(repo)
	if err != nil {
		return nil, err
	}

	st, err := srv.repos.FindId(repoId)
	if err != nil {
		err = status.Errorf(codes.Unknown, "repos error: %v", err)
		return nil, err
	}

	if err := srv.authz.Authorize(euid, action, map[string]interface{}{
		"path": st.GlobalPath(),
	}); err != nil {
		return nil, err
	}

	return st, nil
}

func (srv *Server) authMoveShadowWorkflow(
	ctx context.Context, action auth.Action, repo []byte, workflow []byte,
) (uuid.I, uuid.I, error) {
	repoId, err := srv.authRepoId(ctx, action, repo)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	workflowId, err := parseWorkflowId(workflow)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	st, err := srv.moveShadowWorkflows.FindId(workflowId)
	if err != nil {
		err = status.Errorf(codes.Unknown, "workflow error: %v", err)
		return uuid.Nil, uuid.Nil, err
	}

	if st.RepoId() != repoId {
		err = status.Error(
			codes.PermissionDenied, "repo does not own workflow",
		)
		return uuid.Nil, uuid.Nil, err
	}

	return repoId, workflowId, nil
}

func (srv *Server) authMoveRepoWorkflow(
	ctx context.Context, action auth.Action, repo []byte, workflow []byte,
) (uuid.I, uuid.I, error) {
	repoId, err := srv.authRepoId(ctx, action, repo)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	workflowId, err := parseWorkflowId(workflow)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	st, err := srv.moveRepoWorkflows.FindId(workflowId)
	if err != nil {
		err = status.Errorf(codes.Unknown, "workflow error: %v", err)
		return uuid.Nil, uuid.Nil, err
	}

	if st.RepoId() != repoId {
		err = status.Error(
			codes.PermissionDenied, "repo does not own workflow",
		)
		return uuid.Nil, uuid.Nil, err
	}

	return repoId, workflowId, nil
}

func parseRepoId(b []byte) (uuid.I, error) {
	id, err := uuid.FromBytes(b)
	if err != nil {
		err = status.Errorf(
			codes.InvalidArgument, "malformed repo id: %v", err,
		)
		return uuid.Nil, err
	}
	return id, nil
}

func parseWorkflowId(b []byte) (uuid.I, error) {
	id, err := uuid.FromBytes(b)
	if err != nil {
		err = status.Errorf(
			codes.InvalidArgument,
			"malformed workflow id: %v", err,
		)
		return uuid.Nil, err
	}
	return id, nil
}
