package unixdomainsd

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsoauthz"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/unixdomains"
	"github.com/nogproject/nog/backend/pkg/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const AAInitUnixDomain = fsoauthz.AAInitUnixDomain
const AAReadUnixDomain = fsoauthz.AAReadUnixDomain
const AAWriteUnixDomain = fsoauthz.AAWriteUnixDomain

func (srv *Server) authorize(
	euid auth.Identity, action auth.Action, details auth.ActionDetails,
) error {
	return srv.authz.AuthorizeAny(euid,
		auth.ScopedAction{Action: action, Details: details},
	)
}

func (srv *Server) authName(
	ctx context.Context, action auth.Action, name string,
) error {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return errWithScopeName(err, action, name)
	}
	err = srv.authorize(euid, action, auth.ActionDetails{"name": name})
	return errWithScopeName(err, action, name)
}

func (srv *Server) authUnixDomainIdState(
	ctx context.Context,
	action auth.Action,
	domainIdBytes []byte,
) (*unixdomains.State, error) {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	domainId, err := parseDomainId(domainIdBytes)
	if err != nil {
		return nil, err
	}

	domain, err := srv.domains.FindId(domainId)
	if err != nil {
		return nil, asUnixDomainsError(err)
	}

	if err := srv.authorize(euid, action, auth.ActionDetails{
		"name": domain.Name(),
	}); err != nil {
		return nil, err
	}

	return domain, nil
}

func errWithScopeName(err error, action auth.Action, name string) error {
	if err == nil {
		return err
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	if st.Code() != codes.Unauthenticated {
		return err
	}

	st, err2 := st.WithDetails(&pb.AuthRequiredScope{
		Action: action.String(),
		Name:   name,
	})
	if err2 != nil {
		return err
	}

	return st.Err()
}
