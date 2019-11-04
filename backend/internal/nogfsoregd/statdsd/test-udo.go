package statdsd

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
)

func (srv *Server) TestUdo(
	ctx context.Context, i *pb.TestUdoI,
) (*pb.TestUdoO, error) {
	var action auth.Action = AAFsoTestUdo
	if i.Username != "" || i.Domain != "" {
		action = AAFsoTestUdoAs
	}
	se, err := srv.authPathSession(ctx, action, i.GlobalPath)
	if err != nil {
		return nil, err
	}
	c := pb.NewTestUdoClient(se.conn)
	return c.TestUdo(copyMetadata(ctx), i)
}
