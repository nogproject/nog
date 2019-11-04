package nogfsostaudod

import (
	"context"
	"io"
	"os/exec"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/nogproject/nog/backend/internal/udopb"
)

func (srv *Server) UdoBashPropagateAcls(
	ctx context.Context, i *pb.UdoBashPropagateAclsI,
) (*pb.UdoBashPropagateAclsO, error) {
	username := i.Username
	src := i.Source
	dst := i.Target
	srv.lg.Infow(
		"UdoBashPropagateAcls()",
		"module", "nogfsostaudod",
		"username", username,
		"source", src,
		"target", dst,
	)
	if username != srv.username {
		return nil, ErrUserMismatch
	}

	cmd := exec.CommandContext(ctx, "bash", "-s", "--", src, dst)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		_, _ = io.WriteString(stdin, propagateAclSh)
		_ = stdin.Close()
	}()
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := truncatedErrorMessage(string(out))
		srv.lg.Warnw(
			"UdoBashPropagateAcls(): failed.",
			"module", "nogfsostaudod",
			"username", username,
			"source", src,
			"target", dst,
			"err", err.Error(),
			"output", outStr,
		)
		return nil, status.Errorf(
			codes.Unknown, "bash failed: %s; output: %s",
			err, outStr,
		)
	}

	return &pb.UdoBashPropagateAclsO{}, nil
}
