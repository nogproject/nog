package reposd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsorepos"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (srv *Server) ReposBeginUnarchiveRepo(
	ctx context.Context, i *pb.ReposBeginUnarchiveRepoI,
) (*pb.ReposBeginUnarchiveRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecUnarchiveRepo, i.Repo)
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

	cmd := &fsorepos.CmdBeginUnarchive{
		WorkflowId: wfId,
	}
	vid2, err := srv.repos.BeginUnarchive(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposBeginUnarchiveRepoO{
		RepoVid: vid2[:],
	}, nil
}

func (srv *Server) ReposCommitUnarchiveRepo(
	ctx context.Context, i *pb.ReposCommitUnarchiveRepoI,
) (*pb.ReposCommitUnarchiveRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecUnarchiveRepo, i.Repo)
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

	cmd := &fsorepos.CmdCommitUnarchive{
		WorkflowId: wfId,
	}
	vid2, err := srv.repos.CommitUnarchive(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposCommitUnarchiveRepoO{
		RepoVid: vid2[:],
	}, nil
}

func (srv *Server) ReposAbortUnarchiveRepo(
	ctx context.Context, i *pb.ReposAbortUnarchiveRepoI,
) (*pb.ReposAbortUnarchiveRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecUnarchiveRepo, i.Repo)
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

	cmd := &fsorepos.CmdAbortUnarchive{
		WorkflowId:    wfId,
		StatusCode:    i.StatusCode,
		StatusMessage: i.StatusMessage,
	}
	vid2, err := srv.repos.AbortUnarchive(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposAbortUnarchiveRepoO{
		RepoVid: vid2[:],
	}, nil
}
