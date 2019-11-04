package unixauth

import (
	"context"
	"errors"

	"github.com/nogproject/nog/backend/pkg/auth"
)

var ErrMissingContextAuthenticator = errors.New("missing ContextAuthenticator")
var ErrMissingDomain = errors.New("missing Domain")
var ErrMissingUnixClaim = errors.New("identity without `unix` claim")
var ErrDomainNotFound = errors.New("`unix` domain not found")

// `UserAuthn.Authenticate(ctx)` uses the `ContextAuthenticator` to get an
// `auth.Identity` from the context.  It then searches the `Domain` in the
// identity field `unix` and stores the selected entry of type
// `auth.UnixIdentity` in the identity field `unixLocal`.  `Authenticate()`
// returns an error if any of the steps fails.
type UserAuthn struct {
	ContextAuthenticator auth.Authenticator
	Domain               string
}

func (a *UserAuthn) Authenticate(ctx context.Context) (auth.Identity, error) {
	if a.ContextAuthenticator == nil {
		return nil, ErrMissingContextAuthenticator
	}
	if a.Domain == "" {
		return nil, ErrMissingDomain
	}

	id, err := a.ContextAuthenticator.Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	xids, ok := id["unix"].(auth.UnixIdentities)
	if !ok {
		return nil, ErrMissingUnixClaim
	}
	xid, ok := xids.FindDomain(a.Domain)
	if !ok {
		return nil, ErrDomainNotFound
	}

	id["unixLocal"] = xid
	return id, nil
}
