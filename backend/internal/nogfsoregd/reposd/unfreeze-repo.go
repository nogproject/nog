package reposd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsorepos"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (srv *Server) ReposBeginUnfreezeRepo(
	ctx context.Context, i *pb.ReposBeginUnfreezeRepoI,
) (*pb.ReposBeginUnfreezeRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecUnfreezeRepo, i.Repo)
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

	cmd := &fsorepos.CmdBeginUnfreeze{
		WorkflowId: wfId,
	}
	vid2, err := srv.repos.BeginUnfreeze(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposBeginUnfreezeRepoO{
		RepoVid: vid2[:],
	}, nil
}

func (srv *Server) ReposCommitUnfreezeRepo(
	ctx context.Context, i *pb.ReposCommitUnfreezeRepoI,
) (*pb.ReposCommitUnfreezeRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecUnfreezeRepo, i.Repo)
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

	cmd := &fsorepos.CmdCommitUnfreeze{
		WorkflowId: wfId,
	}
	vid2, err := srv.repos.CommitUnfreeze(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposCommitUnfreezeRepoO{
		RepoVid: vid2[:],
	}, nil
}

func (srv *Server) ReposAbortUnfreezeRepo(
	ctx context.Context, i *pb.ReposAbortUnfreezeRepoI,
) (*pb.ReposAbortUnfreezeRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecUnfreezeRepo, i.Repo)
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

	cmd := &fsorepos.CmdAbortUnfreeze{
		WorkflowId:    wfId,
		StatusCode:    i.StatusCode,
		StatusMessage: i.StatusMessage,
	}
	vid2, err := srv.repos.AbortUnfreeze(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposAbortUnfreezeRepoO{
		RepoVid: vid2[:],
	}, nil
}
