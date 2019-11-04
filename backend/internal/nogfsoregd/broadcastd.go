package nogfsoregd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/broadcastd"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/pkg/auth"
)

func NewBroadcastServer(
	ctx context.Context,
	lg Logger,
	authn auth.Authenticator,
	authz auth.Authorizer,
	names *shorteruuid.Names,
	broadcastJ *events.Journal,
) *broadcastd.Server {
	return broadcastd.New(ctx, lg, authn, authz, names, broadcastJ)
}
