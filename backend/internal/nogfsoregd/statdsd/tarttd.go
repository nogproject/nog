package statdsd

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (srv *Server) TarttHead(
	ctx context.Context, i *pb.TarttHeadI,
) (*pb.TarttHeadO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewTarttClient(se.conn)
	return c.TarttHead(copyMetadata(ctx), i)
}

func (srv *Server) ListTars(
	ctx context.Context, i *pb.ListTarsI,
) (*pb.ListTarsO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewTarttClient(se.conn)
	return c.ListTars(copyMetadata(ctx), i)
}

func (srv *Server) GetTarttconfig(
	ctx context.Context, i *pb.GetTarttconfigI,
) (*pb.GetTarttconfigO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewTarttClient(se.conn)
	return c.GetTarttconfig(copyMetadata(ctx), i)
}
