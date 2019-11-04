package fsoauthz

import (
	"github.com/nogproject/nog/backend/pkg/auth"
)

// `InsecureLogAuthz` logs authz requests, without actually checking
// permission.
type InsecureLogAuthz struct {
	lg Logger
}

func CreateInsecureLogAuthz(lg Logger) *InsecureLogAuthz {
	return &InsecureLogAuthz{
		lg: lg,
	}
}

func (a *InsecureLogAuthz) Authorize(
	euid auth.Identity, action auth.Action, opts auth.ActionDetails,
) error {
	a.lg.Infow("authz", "euid", euid, "action", action, "opts", opts)
	return nil
}
