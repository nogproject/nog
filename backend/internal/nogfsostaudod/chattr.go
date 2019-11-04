package nogfsostaudod

import (
	"context"
	"os/exec"

	pb "github.com/nogproject/nog/backend/internal/udopb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) UdoChattrSetImmutable(
	ctx context.Context, i *pb.UdoChattrSetImmutableI,
) (*pb.UdoChattrSetImmutableO, error) {
	username := i.Username
	path := i.Path
	srv.lg.Infow(
		"UdoChattrSetImmutable()",
		"module", "nogfsostaudod",
		"username", username,
		"path", path,
	)
	if username != srv.username {
		return nil, ErrUserMismatch
	}

	cmd := exec.CommandContext(ctx, "chattr", "+i", "--", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := truncatedErrorMessage(string(out))
		srv.lg.Warnw(
			"UdoChattrSetImmutable(): chattr failed.",
			"module", "nogfsostaudod",
			"username", username,
			"path", path,
			"err", err.Error(),
			"output", outStr,
		)
		return nil, status.Errorf(
			codes.Unknown, "chattr failed: %s; output: %s",
			err, outStr,
		)
	}

	return &pb.UdoChattrSetImmutableO{}, nil
}

func (srv *Server) UdoChattrUnsetImmutable(
	ctx context.Context, i *pb.UdoChattrUnsetImmutableI,
) (*pb.UdoChattrUnsetImmutableO, error) {
	username := i.Username
	path := i.Path
	srv.lg.Infow(
		"UdoChattrUnsetImmutable()",
		"module", "nogfsostaudod",
		"username", username,
		"path", path,
	)
	if username != srv.username {
		return nil, ErrUserMismatch
	}

	cmd := exec.CommandContext(ctx, "chattr", "-i", "--", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := truncatedErrorMessage(string(out))
		srv.lg.Warnw(
			"UdoChattrUnsetImmutable(): chattr failed.",
			"module", "nogfsostaudod",
			"username", username,
			"path", path,
			"err", err.Error(),
			"output", outStr,
		)
		return nil, status.Errorf(
			codes.Unknown, "chattr failed: %s; output: %s",
			err, outStr,
		)
	}

	return &pb.UdoChattrUnsetImmutableO{}, nil
}

func (srv *Server) UdoChattrTreeSetImmutable(
	ctx context.Context, i *pb.UdoChattrTreeSetImmutableI,
) (*pb.UdoChattrTreeSetImmutableO, error) {
	username := i.Username
	path := i.Path
	srv.lg.Infow(
		"UdoChattrTreeSetImmutable()",
		"module", "nogfsostaudod",
		"username", username,
		"path", path,
	)
	if username != srv.username {
		return nil, ErrUserMismatch
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", `
set -o errexit -o nounset -o pipefail -o noglob
echo "path: $1"
cd "$1"
find . \( -type f -o -type d \) -print0 \
| xargs -0 chattr +i --
`, "--", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := truncatedErrorMessage(string(out))
		srv.lg.Warnw(
			"UdoChattrTreeSetImmutable(): chattr failed.",
			"module", "nogfsostaudod",
			"username", username,
			"path", path,
			"err", err.Error(),
			"output", outStr,
		)
		return nil, status.Errorf(
			codes.Unknown, "bash failed: %s; output: %s",
			err, outStr,
		)
	}

	return &pb.UdoChattrTreeSetImmutableO{}, nil
}

func (srv *Server) UdoChattrTreeUnsetImmutable(
	ctx context.Context, i *pb.UdoChattrTreeUnsetImmutableI,
) (*pb.UdoChattrTreeUnsetImmutableO, error) {
	username := i.Username
	path := i.Path
	srv.lg.Infow(
		"UdoChattrTreeUnsetImmutable()",
		"module", "nogfsostaudod",
		"username", username,
		"path", path,
	)
	if username != srv.username {
		return nil, ErrUserMismatch
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", `
set -o errexit -o nounset -o pipefail -o noglob
echo "path: $1"
cd "$1"
find . \( -type f -o -type d \) -print0 \
| xargs -0 chattr -i --
`, "--", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := truncatedErrorMessage(string(out))
		srv.lg.Warnw(
			"UdoChattrTreeUnsetImmutable(): chattr failed.",
			"module", "nogfsostaudod",
			"username", username,
			"path", path,
			"err", err.Error(),
			"output", outStr,
		)
		return nil, status.Errorf(
			codes.Unknown, "bash failed: %s; output: %s",
			err, outStr,
		)
	}

	return &pb.UdoChattrTreeUnsetImmutableO{}, nil
}
