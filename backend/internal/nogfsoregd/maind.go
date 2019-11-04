package nogfsoregd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsomain"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/maind"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func NewMainServer(
	ctx context.Context,
	authn auth.Authenticator,
	authz auth.Authorizer,
	main *fsomain.Main,
	mainId uuid.I,
	mainName string,
) *maind.Server {
	return maind.New(ctx, authn, authz, main, mainId, mainName)
}
