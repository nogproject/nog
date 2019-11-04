package reposd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsorepos"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (srv *Server) ReposBeginFreezeRepo(
	ctx context.Context, i *pb.ReposBeginFreezeRepoI,
) (*pb.ReposBeginFreezeRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecFreezeRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.RepoVid)
	if err != nil {
		return nil, err
	}

	wfId, err := parseWorkflowId(i.Workflow)
	if err != nil {
		return nil, err
	}

	cmd := &fsorepos.CmdBeginFreeze{
		WorkflowId: wfId,
	}
	vid2, err := srv.repos.BeginFreeze(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposBeginFreezeRepoO{
		RepoVid: vid2[:],
	}, nil
}

func (srv *Server) ReposCommitFreezeRepo(
	ctx context.Context, i *pb.ReposCommitFreezeRepoI,
) (*pb.ReposCommitFreezeRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecFreezeRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.RepoVid)
	if err != nil {
		return nil, err
	}

	wfId, err := parseWorkflowId(i.Workflow)
	if err != nil {
		return nil, err
	}

	cmd := &fsorepos.CmdCommitFreeze{
		WorkflowId: wfId,
	}
	vid2, err := srv.repos.CommitFreeze(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposCommitFreezeRepoO{
		RepoVid: vid2[:],
	}, nil
}

func (srv *Server) ReposAbortFreezeRepo(
	ctx context.Context, i *pb.ReposAbortFreezeRepoI,
) (*pb.ReposAbortFreezeRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecFreezeRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.RepoVid)
	if err != nil {
		return nil, err
	}

	wfId, err := parseWorkflowId(i.Workflow)
	if err != nil {
		return nil, err
	}

	cmd := &fsorepos.CmdAbortFreeze{
		WorkflowId:    wfId,
		StatusCode:    i.StatusCode,
		StatusMessage: i.StatusMessage,
	}
	vid2, err := srv.repos.AbortFreeze(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposAbortFreezeRepoO{
		RepoVid: vid2[:],
	}, nil
}
