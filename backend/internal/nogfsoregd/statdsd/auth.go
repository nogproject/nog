package statdsd

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/nogproject/nog/backend/internal/fsoauthz"
	"github.com/nogproject/nog/backend/internal/fsorepos"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const AAFsoFind = fsoauthz.AAFsoFind
const AAFsoReadRepo = fsoauthz.AAFsoReadRepo
const AAFsoInitRepo = fsoauthz.AAFsoInitRepo
const AAFsoRefreshRepo = fsoauthz.AAFsoRefreshRepo
const AAFsoSession = fsoauthz.AAFsoSession
const AAFsoWriteRepo = fsoauthz.AAFsoWriteRepo
const AAFsoTestUdo = fsoauthz.AAFsoTestUdo
const AAFsoTestUdoAs = fsoauthz.AAFsoTestUdoAs

// Allow `make test-t` to disable JWT auth.
var optTestingInsecureForwardToken = false

func init() {
	if os.Getenv("TESTING_INSECURE_NOGFSOREGD_FORWARD_TOKEN") != "" {
		optTestingInsecureForwardToken = true
	}
}

var ErrNoForward = errors.New("refusing to forward wildcard authorization")

func (srv *Server) authNameLocal(
	ctx context.Context, action auth.Action, name string,
) error {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return err
	}

	if err = srv.authz.Authorize(euid, action, map[string]interface{}{
		"name": name,
	}); err != nil {
		return err
	}

	return nil
}

func (srv *Server) authName(
	ctx context.Context, action auth.Action, name string,
) error {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return err
	}

	if err := srv.checkForwardOk(euid); err != nil {
		return err
	}

	if err = srv.authz.Authorize(euid, action, map[string]interface{}{
		"name": name,
	}); err != nil {
		return err
	}

	return nil
}

func (srv *Server) authPathSession(
	ctx context.Context, action auth.Action, path string,
) (*session, error) {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	if err := srv.checkForwardOk(euid); err != nil {
		return nil, err
	}

	if err := srv.authz.Authorize(euid, action, map[string]interface{}{
		"path": path,
	}); err != nil {
		return nil, err
	}

	se := srv.findSessionByPath(path)
	if se == nil {
		err := status.Errorf(
			codes.Unavailable,
			"no nogfsostad connected for `%s`", path,
		)
		return nil, err
	}

	return se, nil
}

func (srv *Server) authRepoIdSession(
	ctx context.Context, action auth.Action, repoIdBytes []byte,
) (*session, error) {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	if err := srv.checkForwardOk(euid); err != nil {
		return nil, err
	}

	repoId, err := parseRepoId(repoIdBytes)
	if err != nil {
		return nil, err
	}

	repo, err := srv.findRepoById(repoId)
	if err != nil {
		return nil, err
	}

	path := repo.GlobalPath()
	if err := srv.authz.Authorize(euid, action, map[string]interface{}{
		"path": string(path),
	}); err != nil {
		return nil, err
	}

	se := srv.findSessionByPath(path)
	if se == nil {
		err := status.Errorf(
			codes.Unavailable,
			"no nogfsostad connected for `%s`", repoId,
		)
		return nil, err
	}

	return se, nil
}

func (srv *Server) findRepoById(id uuid.I) (*fsorepos.State, error) {
	repo, err := srv.repos.FindId(id)
	if err != nil {
		err := status.Errorf(
			codes.NotFound, "unknown repo: %s", id,
		)
		return nil, err
	}
	return repo, nil
}

func parseRepoId(b []byte) (uuid.I, error) {
	id, err := uuid.FromBytes(b)
	if err != nil {
		err = status.Errorf(
			codes.InvalidArgument, "malformed repo id: %v", err,
		)
		return uuid.Nil, err
	}
	return id, nil
}

func (srv *Server) checkForwardOk(euid auth.Identity) error {
	if optTestingInsecureForwardToken {
		return nil
	}

	scopes, ok := euid["scopes"].([]auth.Scope)
	if !ok {
		srv.lg.Infow(
			"Refused to forward without scope.",
			"euid", euid,
		)
		return ErrNoForward
	}

	for _, sc := range scopes {
		if scopeContainsWildcard(sc) {
			srv.lg.Infow(
				"Refused to forward wildcard scope.",
				"euid", euid,
			)
			return ErrNoForward
		}
	}

	return nil
}

func scopeContainsWildcard(sc auth.Scope) bool {
	return anyWildcardString(sc.Actions) ||
		anyWildcardString(sc.Paths) ||
		anyWildcardString(sc.Names)
}

func anyWildcardString(vals []string) bool {
	for _, v := range vals {
		if strings.HasSuffix(v, "*") {
			return true
		}
	}
	return false
}
