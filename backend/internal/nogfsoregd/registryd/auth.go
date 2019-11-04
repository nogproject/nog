package registryd

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nogproject/nog/backend/internal/fsoauthz"
	"github.com/nogproject/nog/backend/internal/fsoregistry"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/workflows/archiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/freezerepowf"
	"github.com/nogproject/nog/backend/internal/workflows/pingregistrywf"
	"github.com/nogproject/nog/backend/internal/workflows/splitrootwf"
	"github.com/nogproject/nog/backend/internal/workflows/unarchiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/unfreezerepowf"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const AAFsoAdminRegistry = fsoauthz.AAFsoAdminRegistry
const AAFsoAdminRoot = fsoauthz.AAFsoAdminRoot
const AAFsoAdminRepo = fsoauthz.AAFsoAdminRepo
const AAFsoDeleteRoot = fsoauthz.AAFsoDeleteRoot
const AAFsoEnableDiscoveryPath = fsoauthz.AAFsoEnableDiscoveryPath
const AAFsoInitRegistry = fsoauthz.AAFsoInitRegistry
const AAFsoInitRepo = fsoauthz.AAFsoInitRepo
const AAFsoInitRoot = fsoauthz.AAFsoInitRoot
const AAFsoReadRegistry = fsoauthz.AAFsoReadRegistry
const AAFsoReadRoot = fsoauthz.AAFsoReadRoot
const AAFsoExecDu = fsoauthz.AAFsoExecDu
const AAFsoExecPingRegistry = fsoauthz.AAFsoExecPingRegistry
const AAFsoExecSplitRoot = fsoauthz.AAFsoExecSplitRoot
const AAFsoReadRepo = fsoauthz.AAFsoReadRepo
const AAFsoFreezeRepo = fsoauthz.AAFsoFreezeRepo
const AAFsoUnfreezeRepo = fsoauthz.AAFsoUnfreezeRepo
const AAFsoExecFreezeRepo = fsoauthz.AAFsoExecFreezeRepo
const AAFsoExecUnfreezeRepo = fsoauthz.AAFsoExecUnfreezeRepo
const AAFsoArchiveRepo = fsoauthz.AAFsoArchiveRepo
const AAFsoExecArchiveRepo = fsoauthz.AAFsoExecArchiveRepo
const AAFsoUnarchiveRepo = fsoauthz.AAFsoUnarchiveRepo
const AAFsoExecUnarchiveRepo = fsoauthz.AAFsoExecUnarchiveRepo

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

func (srv *Server) authPath(
	ctx context.Context, action auth.Action, path string,
) error {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return err
	}
	return srv.authzPath(euid, action, path)
}

func (srv *Server) authzPath(
	euid auth.Identity, action auth.Action, path string,
) error {
	return srv.authorize(euid, action, auth.ActionDetails{"path": path})
}

func (srv *Server) authRegistryRepoId(
	ctx context.Context, action auth.Action, registry string, repo []byte,
) (uuid.I, error) {
	_, repoId, err := srv.authRegistryStateRepoId(
		ctx, action, registry, repo,
	)
	return repoId, err
}

func (srv *Server) authRegistryStateRepoId(
	ctx context.Context, action auth.Action, registry string, repo []byte,
) (*fsoregistry.State, uuid.I, error) {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, uuid.Nil, err
	}

	repoId, err := parseRepoId(repo)
	if err != nil {
		return nil, uuid.Nil, err
	}

	reg, err := srv.getRegistryState(registry)
	if err != nil {
		return nil, uuid.Nil, err
	}
	inf, ok := reg.RepoById(repoId)
	if !ok {
		err := status.Error(codes.NotFound, "unknown repo")
		return nil, uuid.Nil, err
	}

	if err := srv.authorize(euid, action, auth.ActionDetails{
		"path": inf.GlobalPath,
	}); err != nil {
		return nil, uuid.Nil, err
	}

	return reg, repoId, nil
}

func (srv *Server) authPingRegistryWorkflowId(
	ctx context.Context, action auth.Action, idBytes []byte,
) (string, *pingregistrywf.State, error) {
	workflowId, err := uuid.FromBytes(idBytes)
	if err != nil {
		return "", nil, ErrMalformedWorkflowId
	}
	workflow, err := srv.pingRegistryWorkflows.FindId(workflowId)
	if err != nil {
		return "", nil, asPingRegistryWorkflowGrpcError(err)
	}
	registryId := workflow.RegistryId()
	registry, err := srv.registry.FindId(registryId)
	if err != nil {
		return "", nil, asRegistryGrpcError(err)
	}
	registryName := registry.Name()
	if err := srv.authName(
		ctx, action, registryName,
	); err != nil {
		return "", nil, err
	}
	return registryName, workflow, nil
}

func (srv *Server) authSplitRootWorkflowId(
	ctx context.Context, action auth.Action, idBytes []byte,
) (auth.Identity, *splitrootwf.State, error) {
	return srv.authAnySplitRootWorkflowId(
		ctx, []auth.Action{action}, idBytes,
	)
}

func (srv *Server) authAnySplitRootWorkflowId(
	ctx context.Context, actions []auth.Action, idBytes []byte,
) (auth.Identity, *splitrootwf.State, error) {
	if len(actions) == 0 {
		panic("require at least one action")
	}

	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, nil, err
	}

	wfId, err := uuid.FromBytes(idBytes)
	if err != nil {
		return nil, nil, ErrMalformedWorkflowId
	}
	wf, err := srv.splitRootWorkflows.FindId(wfId)
	if err != nil {
		return nil, nil, asSplitRootWorkflowGrpcError(err)
	}

	details := auth.ActionDetails{"path": wf.GlobalRoot()}
	sas := make([]auth.ScopedAction, 0, len(actions))
	for _, a := range actions {
		sas = append(sas, auth.ScopedAction{
			Action:  a,
			Details: details,
		})
	}
	err = srv.authz.AuthorizeAny(euid, sas...)
	if err != nil {
		return nil, nil, err
	}

	return euid, wf, nil
}

func (srv *Server) authAnyFreezeRepoWorkflowId(
	ctx context.Context, actions []auth.Action, idBytes []byte,
) (auth.Identity, *freezerepowf.State, error) {
	if len(actions) == 0 {
		panic("require at least one action")
	}

	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, nil, err
	}

	wfId, err := uuid.FromBytes(idBytes)
	if err != nil {
		return nil, nil, ErrMalformedWorkflowId
	}
	wf, err := srv.freezeRepoWorkflows.FindId(wfId)
	if err != nil {
		return nil, nil, asFreezeRepoWorkflowGrpcError(err)
	}

	details := auth.ActionDetails{"path": wf.RepoGlobalPath()}
	sas := make([]auth.ScopedAction, 0, len(actions))
	for _, a := range actions {
		sas = append(sas, auth.ScopedAction{
			Action:  a,
			Details: details,
		})
	}
	err = srv.authz.AuthorizeAny(euid, sas...)
	if err != nil {
		return nil, nil, err
	}

	return euid, wf, nil
}

func (srv *Server) authAnyUnfreezeRepoWorkflowId(
	ctx context.Context, actions []auth.Action, idBytes []byte,
) (auth.Identity, *unfreezerepowf.State, error) {
	if len(actions) == 0 {
		panic("require at least one action")
	}

	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, nil, err
	}

	wfId, err := uuid.FromBytes(idBytes)
	if err != nil {
		return nil, nil, ErrMalformedWorkflowId
	}
	wf, err := srv.unfreezeRepoWorkflows.FindId(wfId)
	if err != nil {
		return nil, nil, asUnfreezeRepoWorkflowGrpcError(err)
	}

	details := auth.ActionDetails{"path": wf.RepoGlobalPath()}
	sas := make([]auth.ScopedAction, 0, len(actions))
	for _, a := range actions {
		sas = append(sas, auth.ScopedAction{
			Action:  a,
			Details: details,
		})
	}
	err = srv.authz.AuthorizeAny(euid, sas...)
	if err != nil {
		return nil, nil, err
	}

	return euid, wf, nil
}

func (srv *Server) authAnyArchiveRepoWorkflowId(
	ctx context.Context, actions []auth.Action, idBytes []byte,
) (auth.Identity, *archiverepowf.State, error) {
	if len(actions) == 0 {
		panic("require at least one action")
	}

	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, nil, err
	}

	wfId, err := uuid.FromBytes(idBytes)
	if err != nil {
		return nil, nil, ErrMalformedWorkflowId
	}
	wf, err := srv.archiveRepoWorkflows.FindId(wfId)
	if err != nil {
		return nil, nil, asArchiveRepoWorkflowGrpcError(err)
	}

	details := auth.ActionDetails{"path": wf.RepoGlobalPath()}
	sas := make([]auth.ScopedAction, 0, len(actions))
	for _, a := range actions {
		sas = append(sas, auth.ScopedAction{
			Action:  a,
			Details: details,
		})
	}
	err = srv.authz.AuthorizeAny(euid, sas...)
	if err != nil {
		return nil, nil, err
	}

	return euid, wf, nil
}

func (srv *Server) authAnyUnarchiveRepoWorkflowId(
	ctx context.Context, actions []auth.Action, idBytes []byte,
) (auth.Identity, *unarchiverepowf.State, error) {
	if len(actions) == 0 {
		panic("require at least one action")
	}

	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, nil, err
	}

	wfId, err := uuid.FromBytes(idBytes)
	if err != nil {
		return nil, nil, ErrMalformedWorkflowId
	}
	wf, err := srv.unarchiveRepoWorkflows.FindId(wfId)
	if err != nil {
		return nil, nil, asUnarchiveRepoWorkflowGrpcError(err)
	}

	details := auth.ActionDetails{"path": wf.RepoGlobalPath()}
	sas := make([]auth.ScopedAction, 0, len(actions))
	for _, a := range actions {
		sas = append(sas, auth.ScopedAction{
			Action:  a,
			Details: details,
		})
	}
	err = srv.authz.AuthorizeAny(euid, sas...)
	if err != nil {
		return nil, nil, err
	}

	return euid, wf, nil
}

type authzScope struct {
	Action auth.Action
	Name   string
	Path   string
}

func (srv *Server) authAll(
	ctx context.Context, scopes ...authzScope,
) error {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return err
	}
	if len(scopes) == 0 {
		panic("authAll() requires at least one scope")
	}
	for _, sc := range scopes {
		var k string
		var v string
		switch {
		case sc.Name != "":
			k = "name"
			v = sc.Name
		case sc.Path != "":
			k = "path"
			v = sc.Path
		default:
			panic("invalid authAll() scope")
		}
		if err := srv.authorize(
			euid,
			sc.Action,
			auth.ActionDetails{k: v},
		); err != nil {
			return err
		}
	}
	return nil
}

func (srv *Server) authAny(
	ctx context.Context, scopes ...authzScope,
) error {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return err
	}
	if len(scopes) == 0 {
		panic("authAny() requires at least one scope")
	}

	sas := make([]auth.ScopedAction, 0, len(scopes))
	for _, sc := range scopes {
		var k string
		var v string
		switch {
		case sc.Name != "":
			k = "name"
			v = sc.Name
		case sc.Path != "":
			k = "path"
			v = sc.Path
		default:
			panic("invalid authAny() scope")
		}
		sas = append(sas, auth.ScopedAction{
			Action:  sc.Action,
			Details: auth.ActionDetails{k: v},
		})
	}
	return srv.authz.AuthorizeAny(euid, sas...)
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
