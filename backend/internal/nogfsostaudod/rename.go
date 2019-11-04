package nogfsostaudod

import (
	"context"
	"os"

	pb "github.com/nogproject/nog/backend/internal/udopb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) UdoRename(
	ctx context.Context, i *pb.UdoRenameI,
) (*pb.UdoRenameO, error) {
	username := i.Username
	oldPath := i.OldPath
	newPath := i.NewPath
	srv.lg.Infow(
		"UdoRename()",
		"module", "nogfsostaudod",
		"username", username,
		"oldPath", oldPath,
		"newPath", newPath,
	)
	if username != srv.username {
		return nil, ErrUserMismatch
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		// XXX Better analyze err and distinguish gRPC codes, e.g.
		// NotFound vs. PermissionDenied.
		err2 := status.Errorf(codes.Unknown, "rename failed: %v", err)
		return nil, err2
	}

	return &pb.UdoRenameO{}, nil
}
