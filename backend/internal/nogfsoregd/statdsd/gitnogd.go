package statdsd

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (srv *Server) Head(ctx context.Context, i *pb.HeadI) (*pb.HeadO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewGitNogClient(se.conn)
	return c.Head(copyMetadata(ctx), i)
}

func (srv *Server) Summary(
	ctx context.Context, i *pb.SummaryI,
) (*pb.SummaryO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewGitNogClient(se.conn)
	return c.Summary(copyMetadata(ctx), i)
}

func (srv *Server) Meta(ctx context.Context, i *pb.MetaI) (*pb.MetaO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewGitNogClient(se.conn)
	return c.Meta(copyMetadata(ctx), i)
}

func (srv *Server) PutMeta(
	ctx context.Context, i *pb.PutMetaI,
) (*pb.PutMetaO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoWriteRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewGitNogClient(se.conn)
	return c.PutMeta(copyMetadata(ctx), i)
}

func (srv *Server) Content(
	ctx context.Context, i *pb.ContentI,
) (*pb.ContentO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewGitNogClient(se.conn)
	return c.Content(copyMetadata(ctx), i)
}
