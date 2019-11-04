package testudod

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsoauthz"
	"github.com/nogproject/nog/backend/pkg/auth"
)

const AAFsoTestUdo = fsoauthz.AAFsoTestUdo
const AAFsoTestUdoAs = fsoauthz.AAFsoTestUdoAs

func (srv *Server) authGlobalPath(
	ctx context.Context, action auth.Action, globalPath string,
) (err error) {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return err
	}

	if err := srv.authz.Authorize(euid, action, map[string]interface{}{
		"path": globalPath,
	}); err != nil {
		return err
	}

	return nil
}

func (srv *Server) authUnixGlobalPath(
	ctx context.Context, action auth.Action, globalPath string,
) (username string, err error) {
	euid, err := srv.authnUnix.Authenticate(ctx)
	if err != nil {
		return "", err
	}
	unixLocal, ok := euid["unixLocal"].(auth.UnixIdentity)
	if !ok {
		panic("Authenticate() returned id without `unixLocal`")
	}

	if err := srv.authz.Authorize(euid, action, map[string]interface{}{
		"path": globalPath,
	}); err != nil {
		return "", err
	}

	return unixLocal.Username, nil
}
