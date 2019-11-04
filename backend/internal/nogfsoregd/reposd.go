package nogfsoregd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsorepos"
	"github.com/nogproject/nog/backend/internal/nogfsoregd/reposd"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/internal/workflows/moverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/moveshadowwf"
	"github.com/nogproject/nog/backend/pkg/auth"
)

func NewReposServer(
	ctx context.Context,
	lg Logger,
	authn auth.Authenticator,
	authz auth.Authorizer,
	names *shorteruuid.Names,
	reposJ *events.Journal,
	repos *fsorepos.Repos,
	workflowsJ *events.Journal,
	moveRepoWorkflows *moverepowf.Workflows,
	moveShadowWorkflows *moveshadowwf.Workflows,
) *reposd.Server {
	return reposd.New(
		ctx, lg,
		authn, authz,
		names,
		reposJ, repos,
		workflowsJ, moveRepoWorkflows, moveShadowWorkflows,
	)
}
