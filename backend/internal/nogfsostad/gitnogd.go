package nogfsostad

import (
	"github.com/nogproject/nog/backend/internal/nogfsostad/gitnogd"
	"github.com/nogproject/nog/backend/pkg/auth"
)

func NewGitNogServer(
	lg Logger,
	authn auth.Authenticator,
	authz auth.Authorizer,
	proc *Processor,
) *gitnogd.Server {
	return gitnogd.New(lg, authn, authz, proc)
}
