package reposd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsorepos"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (srv *Server) ReposBeginArchiveRepo(
	ctx context.Context, i *pb.ReposBeginArchiveRepoI,
) (*pb.ReposBeginArchiveRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecArchiveRepo, i.Repo)
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

	cmd := &fsorepos.CmdBeginArchive{
		WorkflowId: wfId,
	}
	vid2, err := srv.repos.BeginArchive(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposBeginArchiveRepoO{
		RepoVid: vid2[:],
	}, nil
}

func (srv *Server) ReposCommitArchiveRepo(
	ctx context.Context, i *pb.ReposCommitArchiveRepoI,
) (*pb.ReposCommitArchiveRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecArchiveRepo, i.Repo)
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

	cmd := &fsorepos.CmdCommitArchive{
		WorkflowId: wfId,
		TarPath:    i.TarPath,
	}
	vid2, err := srv.repos.CommitArchive(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposCommitArchiveRepoO{
		RepoVid: vid2[:],
	}, nil
}

func (srv *Server) ReposAbortArchiveRepo(
	ctx context.Context, i *pb.ReposAbortArchiveRepoI,
) (*pb.ReposAbortArchiveRepoO, error) {
	id, err := srv.authRepoId(ctx, AAFsoExecArchiveRepo, i.Repo)
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

	cmd := &fsorepos.CmdAbortArchive{
		WorkflowId:    wfId,
		StatusCode:    i.StatusCode,
		StatusMessage: i.StatusMessage,
	}
	vid2, err := srv.repos.AbortArchive(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ReposAbortArchiveRepoO{
		RepoVid: vid2[:],
	}, nil
}
