package nogechod

import (
	pb "github.com/nogproject/nog/backend/pkg/nogechopb"
	"golang.org/x/net/context"
)

type Server struct{}

func (srv *Server) Echo(
	ctx context.Context, req *pb.EchoRequest,
) (*pb.EchoResponse, error) {
	return &pb.EchoResponse{Message: req.Message}, nil
}
