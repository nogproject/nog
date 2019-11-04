package testudod

import (
	"context"
	"path/filepath"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/privileges"
	"github.com/nogproject/nog/backend/pkg/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrWrongDomain = status.Error(codes.PermissionDenied, "wrong domain")
var ErrUnknownUser = status.Error(codes.PermissionDenied, "unknown user")

type Server struct {
	lg        Logger
	authn     auth.Authenticator
	domain    string
	authnUnix auth.Authenticator
	authz     auth.Authorizer
	// `Server` allows concurrent operations, relying on `Processor` to
	// serialize operations per repo if necessary.
	proc  Processor
	privs Privileges
}

type Logger interface {
}

type Processor interface {
	ResolveGlobaPathInLocalRepo(string) (repo, sub string, ok bool)
}

type Privileges interface {
	privileges.UdoStatPrivileges
}

func New(
	lg Logger,
	authn auth.Authenticator,
	domain string,
	authnUnix auth.Authenticator,
	authz auth.Authorizer,
	proc Processor,
	privs Privileges,
) *Server {
	return &Server{
		lg:        lg,
		authn:     authn,
		domain:    domain,
		authnUnix: authnUnix,
		authz:     authz,
		proc:      proc,
		privs:     privs,
	}
}

func (srv *Server) TestUdo(
	ctx context.Context, i *pb.TestUdoI,
) (*pb.TestUdoO, error) {
	// If the caller does not request a specific user, get the user from
	// the `Authenticator`, i.e. from the JWT, in order to run as the
	// requesting user.  This demonstrates operations that run on behalf of
	// the requesting user.
	//
	// If the caller requests a specific user, use it.  This demonstrates
	// sudo-like operations.
	var username string
	if i.Username == "" && i.Domain == "" {
		authUsername, err := srv.authUnixGlobalPath(
			ctx, AAFsoTestUdo, i.GlobalPath,
		)
		if err != nil {
			return nil, err
		}
		username = authUsername
	} else {
		if err := srv.authGlobalPath(
			ctx, AAFsoTestUdoAs, i.GlobalPath,
		); err != nil {
			return nil, err
		}
		if i.Domain != srv.domain {
			return nil, ErrWrongDomain
		}
		username = i.Username
	}

	repo, sub, ok := srv.proc.ResolveGlobaPathInLocalRepo(i.GlobalPath)
	if !ok {
		err := status.Error(codes.NotFound, "unknown repo")
		return nil, err
	}

	p, err := srv.privs.AcquireUdoStat(ctx, username)
	if err != nil {
		err := status.Errorf(
			codes.PermissionDenied,
			"failed to acquire privilege `UdoStat(username=%s)`: %v",
			username, err,
		)
		return nil, err
	}
	defer p.Release()

	st, err := p.Stat(ctx, filepath.Join(repo, sub))
	if err != nil {
		err := status.Errorf(
			codes.Unknown,
			"stat failed: %v", err,
		)
		return nil, err
	}

	return &pb.TestUdoO{
		ProcessUsername: username,
		Mtime:           st.Mtime,
		Mode:            st.Mode,
	}, nil
}
