package nogfsostaudod

import (
	"context"
	"os"

	pb "github.com/nogproject/nog/backend/internal/udopb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) UdoStat(
	ctx context.Context, i *pb.UdoStatI,
) (*pb.UdoStatO, error) {
	username := i.Username
	path := i.Path
	srv.lg.Infow(
		"UdoStat()",
		"module", "nogfsostaudod",
		"username", username,
		"path", path,
	)
	if username != srv.username {
		return nil, ErrUserMismatch
	}

	st, err := os.Stat(path)
	if err != nil {
		// XXX Better analyze err and distinguish GRPC codes, e.g.
		// NotFound vs. PermissionDenied.
		err := status.Errorf(
			codes.Unknown, "stat failed: %v", err,
		)
		return nil, err
	}

	return &pb.UdoStatO{
		Mtime: st.ModTime().Unix(),
		Mode:  uint32(st.Mode()),
	}, nil
}
