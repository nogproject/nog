package nogfsoregd

import (
	"github.com/nogproject/nog/backend/internal/fsorepos"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/statdsd"
	"github.com/nogproject/nog/backend/pkg/auth"
	"google.golang.org/grpc/credentials"
)

func NewStatdsServer(
	lg Logger,
	advertiseAddr string,
	tls credentials.TransportCredentials,
	authn auth.Authenticator,
	authz auth.Authorizer,
	repos *fsorepos.Repos,
) *statdsd.Server {
	return statdsd.New(lg, advertiseAddr, tls, authn, authz, repos)
}
