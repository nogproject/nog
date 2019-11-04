package statdsd

import (
	"context"
	"io"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (srv *Server) ListStatTree(
	i *pb.ListStatTreeI, ostream pb.GitNogTree_ListStatTreeServer,
) error {
	ctx := ostream.Context()
	se, err := srv.authRepoIdSession(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return err
	}

	c := pb.NewGitNogTreeClient(se.conn)
	ctx2, cancel2 := context.WithCancel(copyMetadata(ctx))
	defer cancel2()
	istream, err := c.ListStatTree(ctx2, i)
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

func (srv *Server) ListMetaTree(
	i *pb.ListMetaTreeI, ostream pb.GitNogTree_ListMetaTreeServer,
) error {
	ctx := ostream.Context()
	se, err := srv.authRepoIdSession(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return err
	}

	c := pb.NewGitNogTreeClient(se.conn)
	ctx2, cancel2 := context.WithCancel(copyMetadata(ctx))
	defer cancel2()
	istream, err := c.ListMetaTree(ctx2, i)
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

func (srv *Server) PutPathMetadata(
	ctx context.Context, i *pb.PutPathMetadataI,
) (*pb.PutPathMetadataO, error) {
	se, err := srv.authRepoIdSession(ctx, AAFsoWriteRepo, i.Repo)
	if err != nil {
		return nil, err
	}
	c := pb.NewGitNogTreeClient(se.conn)
	return c.PutPathMetadata(copyMetadata(ctx), i)
}
