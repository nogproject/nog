package statdsd

import (
	"context"
	"io"
	slashpath "path"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (srv *Server) FindUntracked(
	i *pb.FindUntrackedI, ostream pb.Discovery_FindUntrackedServer,
) error {
	ctx := ostream.Context()
	path := slashpath.Clean(i.GlobalRoot)
	se, err := srv.authPathSession(ctx, AAFsoFind, path)
	if err != nil {
		return err
	}

	c := pb.NewDiscoveryClient(se.conn)
	ctx2, cancel2 := context.WithCancel(copyMetadata(ctx))
	defer cancel2()
	istream, err := c.FindUntracked(ctx2, i)
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
