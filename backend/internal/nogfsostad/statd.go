package nogfsostad

import (
	"github.com/nogproject/nog/backend/internal/nogfsostad/statd"
	"github.com/nogproject/nog/backend/pkg/auth"
	"google.golang.org/grpc/credentials"
)

func NewStatServer(
	lg Logger,
	authn auth.Authenticator,
	authz auth.Authorizer,
	proc *Processor,
	sysRPCCreds credentials.PerRPCCredentials,
) *statd.Server {
	return statd.New(lg, authn, authz, proc, sysRPCCreds)
}
