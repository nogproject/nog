package statdsd

import (
	"context"
	"io"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (srv *Server) StatStatus(
	i *pb.StatStatusI, ostream pb.Stat_StatStatusServer,
) error {
	ctx := ostream.Context()
	se, err := srv.authRepoIdSession(ctx, AAFsoRefreshRepo, i.Repo)
	if err != nil {
		return err
	}

	c := pb.NewStatClient(se.conn)
	ctx2, cancel2 := context.WithCancel(copyMetadata(ctx))
	defer cancel2()
	istream, err := c.StatStatus(ctx2, i)
	if err != nil {
		return err
	}

	for {
		o, err := istream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := ostream.Send(o); err != nil {
			return err
		}
	}
}

func (srv *Server) Stat(
	ctx context.Context, i *pb.StatI,
) (*pb.StatO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoRefreshRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewStatClient(se.conn)
	return c.Stat(copyMetadata(ctx), i)
}

func (srv *Server) Sha(
	ctx context.Context, i *pb.ShaI,
) (*pb.ShaO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoRefreshRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewStatClient(se.conn)
	return c.Sha(copyMetadata(ctx), i)
}

func (srv *Server) RefreshContent(
	ctx context.Context, i *pb.RefreshContentI,
) (*pb.RefreshContentO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoRefreshRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewStatClient(se.conn)
	return c.RefreshContent(copyMetadata(ctx), i)
}

func (srv *Server) ReinitSubdirTracking(
	ctx context.Context, i *pb.ReinitSubdirTrackingI,
) (*pb.ReinitSubdirTrackingO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoInitRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewStatClient(se.conn)
	return c.ReinitSubdirTracking(copyMetadata(ctx), i)
}
