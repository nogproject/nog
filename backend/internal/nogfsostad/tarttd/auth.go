package tarttd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsoauthz"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const AAFsoReadRepo = fsoauthz.AAFsoReadRepo

func (srv *Server) authRepoId(
	ctx context.Context, action auth.Action, repo []byte,
) (uuid.I, error) {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	repoId, err := parseRepoId(repo)
	if err != nil {
		return uuid.Nil, err
	}

	if err := srv.authorize(euid, action, repoId); err != nil {
		return uuid.Nil, err
	}

	return repoId, nil
}

func parseRepoId(idBytes []byte) (uuid.I, error) {
	id, err := uuid.FromBytes(idBytes)
	if err != nil {
		err = status.Errorf(
			codes.InvalidArgument, "invalid uuid: %v", err,
		)
		return uuid.Nil, err
	}
	return id, nil
}

func (srv *Server) authorize(
	euid auth.Identity,
	action auth.Action,
	repoId uuid.I,
) error {
	path, ok := srv.proc.GlobalRepoPath(repoId)
	if !ok {
		return status.Error(codes.NotFound, "unknown repo")
	}
	return srv.authz.Authorize(euid, action, map[string]interface{}{
		"path": string(path),
	})
}
