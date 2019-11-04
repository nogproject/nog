package broadcastd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsoauthz"
	"github.com/nogproject/nog/backend/pkg/auth"
)

const AABroadcastWrite = fsoauthz.AABroadcastWrite
const AABroadcastRead = fsoauthz.AABroadcastRead

func (srv *Server) authName(
	ctx context.Context, action auth.Action, name string,
) error {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return err
	}
	return srv.authz.Authorize(euid, action, map[string]interface{}{
		"name": name,
	})
}
