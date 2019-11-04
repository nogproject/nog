package fsoauthz

import (
	"errors"
	slashpath "path"
	"strings"

	"github.com/nogproject/nog/backend/pkg/auth"
)

var (
	ErrNoScope             = errors.New("no scope")
	ErrInsufficientDetails = errors.New(
		"isufficient details: require at least `path` or `name`",
	)
	ErrDefaultDeny          = errors.New("default deny")
	ErrMissingScopedActions = errors.New("no scoped actions")
)

// `InsecureLogScopeAuthz` determines whether an action would be permitted by
// comparing the action with the `euid` scope.  It only logs the decision
// without actually denying access.
type InsecureLogScopeAuthz struct {
	lg Logger
}

func CreateInsecureLogScopeAuthz(lg Logger) *InsecureLogScopeAuthz {
	return &InsecureLogScopeAuthz{
		lg: lg,
	}
}

func (a *InsecureLogScopeAuthz) Authorize(
	euid auth.Identity, action auth.Action, opts auth.ActionDetails,
) error {
	scopes, _ := euid["scopes"].([]auth.Scope)
	err := authorizeScopes(scopes, action, opts)
	if err != nil {
		a.lg.Infow(
			"scope authz would deny",
			"err", err,
			"euid", euid, "action", action, "opts", opts,
		)
	} else {
		a.lg.Infow(
			"scope authz would allow",
			"euid", euid, "action", action, "opts", opts,
		)
	}
	return nil
}

// `ScopeAuthz` determines whether an action is permitted by comparing the
// action with the `euid` scope.  It logs decisions with info level.
type ScopeAuthz struct {
	lg Logger
}

func CreateScopeAuthz(lg Logger) *ScopeAuthz {
	return &ScopeAuthz{
		lg: lg,
	}
}

func (a *ScopeAuthz) Authorize(
	euid auth.Identity, action auth.Action, opts auth.ActionDetails,
) error {
	scopes, _ := euid["scopes"].([]auth.Scope)
	err := authorizeScopes(scopes, action, opts)
	if err != nil {
		a.lg.Infow(
			"scope authz deny",
			"err", err,
			"euid", euid, "action", action, "opts", opts,
		)
	} else {
		a.lg.Infow(
			"scope authz allow",
			"euid", euid, "action", action, "opts", opts,
		)
	}
	return err
}

func (authz *ScopeAuthz) AuthorizeAny(
	euid auth.Identity, actions ...auth.ScopedAction,
) error {
	if len(actions) == 0 {
		panic(ErrMissingScopedActions)
	}

	scopes, _ := euid["scopes"].([]auth.Scope)
Loop:
	for _, act := range actions {
		err := authorizeScopes(scopes, act.Action, act.Details)
		switch err {
		case nil:
			authz.lg.Infow(
				"scope authz allow",
				"euid", euid,
				"action", act.Action,
				"opts", act.Details,
			)
			return nil
		case ErrDefaultDeny:
			continue Loop
		default:
			authz.lg.Infow(
				"scope authz any deny early",
				"err", err,
				"euid", euid,
				"action", act.Action,
				"opts", act.Details,
			)
			return err
		}
	}

	err := ErrDefaultDeny
	authz.lg.Infow(
		"scope authz any deny",
		"err", err,
		"euid", euid,
		"anyOfActions", actions,
	)
	return err
}

func authorizeScopes(
	scopes []auth.Scope, action auth.Action, opts auth.ActionDetails,
) error {
	if scopes == nil {
		return ErrNoScope
	}

	path, _ := opts["path"].(string)
	name, _ := opts["name"].(string)
	if path == "" && name == "" {
		return ErrInsufficientDetails
	}
	if path != "" {
		path = slashpath.Clean(path)
	}

	for _, sc := range scopes {
		if scopeMatches(sc, action, path, name) {
			return nil
		}
	}

	return ErrDefaultDeny
}

func scopeMatches(
	sc auth.Scope, action auth.Action, path, name string,
) bool {
	if !globContains(sc.Actions, string(action)) {
		return false
	}

	if path != "" && !globContains(sc.Paths, path) {
		return false
	}

	if name != "" && !globContains(sc.Names, name) {
		return false
	}

	// Double check precondition before granting access.
	if path == "" && name == "" {
		panic("require at least `path` or `name`")
	}

	return true
}

func globContains(globs []string, fixed string) bool {
	for _, g := range globs {
		if globMatches(g, fixed) {
			return true
		}
	}
	return false
}

func globMatches(g string, fixed string) bool {
	switch {
	case g == "*": // global wildcard matches anything.
		return true
	case strings.HasSuffix(g, "*"):
		prefix := g[0 : len(g)-1]
		return strings.HasPrefix(fixed, prefix)
	default:
		return fixed == g
	}
}
