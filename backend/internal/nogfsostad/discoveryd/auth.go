package discoveryd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsoauthz"
	"github.com/nogproject/nog/backend/pkg/auth"
)

const AAFsoFind = fsoauthz.AAFsoFind

func (srv *Server) authPath(
	ctx context.Context, action auth.Action, path string,
) error {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return err
	}
	return srv.authz.Authorize(euid, action, map[string]interface{}{
		"path": path,
	})
}
