// Package `registryd`: GRPC service `nogfso.Registry` to access the FSO
// registry.
package registryd

import (
	"context"
	slashpath "path"
	"time"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsomain"
	"github.com/nogproject/nog/backend/internal/fsoregistry"
	"github.com/nogproject/nog/backend/internal/fsorepos"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/internal/workflows/archiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/durootwf"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/internal/workflows/freezerepowf"
	"github.com/nogproject/nog/backend/internal/workflows/pingregistrywf"
	"github.com/nogproject/nog/backend/internal/workflows/splitrootwf"
	"github.com/nogproject/nog/backend/internal/workflows/unarchiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/unfreezerepowf"
	"github.com/nogproject/nog/backend/internal/workflows/wfindexes"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	NsFsoMain                       = "fsomain"
	NsFsoRegistry                   = "fsoreg"
	NsFsoRegistryEphemeralWorkflows = "fsoregephwfl"
)

// Canceling the server `ctx` stops streaming connections.  Use it together
// with `grpc.Server.GracefulStop()`:
//
// ```
// cancel() // non-blocking
// gsrv.GracefulStop() // blocking
// ```
//
type Server struct {
	ctx                    context.Context
	lg                     Logger
	authn                  auth.Authenticator
	authz                  auth.AnyAuthorizer
	names                  *shorteruuid.Names
	idChecker              IdChecker
	main                   *fsomain.Main
	mainId                 uuid.I
	registryJ              *events.Journal
	registry               *fsoregistry.Registry
	repos                  *fsorepos.Repos
	ephWorkflowsJ          *events.Journal
	workflowIndexes        *wfindexes.Indexes
	duRootWorkflows        *durootwf.Workflows
	pingRegistryWorkflows  *pingregistrywf.Workflows
	splitRootWorkflows     *splitrootwf.Workflows
	freezeRepoWorkflows    *freezerepowf.Workflows
	unfreezeRepoWorkflows  *unfreezerepowf.Workflows
	archiveRepoWorkflows   *archiverepowf.Workflows
	unarchiveRepoWorkflows *unarchiverepowf.Workflows
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
}

type IdChecker interface {
	IsUnusedId(uuid.I) (decision string, err error)
}

func New(
	ctx context.Context,
	lg Logger,
	authn auth.Authenticator,
	authz auth.AnyAuthorizer,
	names *shorteruuid.Names,
	idChecker IdChecker,
	main *fsomain.Main,
	mainId uuid.I,
	registryJ *events.Journal,
	registry *fsoregistry.Registry,
	repos *fsorepos.Repos,
	ephWorkflowsJ *events.Journal,
	workflowIndexes *wfindexes.Indexes,
	duRootWorkflows *durootwf.Workflows,
	pingRegistryWorkflows *pingregistrywf.Workflows,
	splitRootWorkflows *splitrootwf.Workflows,
	freezeRepoWorkflows *freezerepowf.Workflows,
	unfreezeRepoWorkflows *unfreezerepowf.Workflows,
	archiveRepoWorkflows *archiverepowf.Workflows,
	unarchiveRepoWorkflows *unarchiverepowf.Workflows,
) *Server {
	return &Server{
		ctx:                    ctx,
		lg:                     lg,
		authn:                  authn,
		authz:                  authz,
		names:                  names,
		idChecker:              idChecker,
		main:                   main,
		mainId:                 mainId,
		registryJ:              registryJ,
		registry:               registry,
		repos:                  repos,
		ephWorkflowsJ:          ephWorkflowsJ,
		workflowIndexes:        workflowIndexes,
		duRootWorkflows:        duRootWorkflows,
		pingRegistryWorkflows:  pingRegistryWorkflows,
		splitRootWorkflows:     splitRootWorkflows,
		freezeRepoWorkflows:    freezeRepoWorkflows,
		unfreezeRepoWorkflows:  unfreezeRepoWorkflows,
		archiveRepoWorkflows:   archiveRepoWorkflows,
		unarchiveRepoWorkflows: unarchiveRepoWorkflows,
	}
}

func (srv *Server) InitRegistry(
	ctx context.Context, i *pb.InitRegistryI,
) (*pb.InitRegistryO, error) {
	regName := i.Registry
	if err := checkRegistryName(regName); err != nil {
		return nil, err
	}

	if err := srv.authName(ctx, AAFsoInitRegistry, regName); err != nil {
		return nil, err
	}

	vid, err := parseMainVid(i.MainVid)
	if err != nil {
		return nil, err
	}

	newVid, err := srv.main.InitRegistry(srv.mainId, vid, regName)
	if err != nil {
		return nil, asMainGrpcError(err)
	}

	return &pb.InitRegistryO{MainVid: newVid[:]}, nil
}

func (srv *Server) EnableEphemeralWorkflows(
	ctx context.Context, i *pb.EnableEphemeralWorkflowsI,
) (*pb.EnableEphemeralWorkflowsO, error) {
	regName := i.Registry
	id, err := srv.parseRegistryName(regName)
	if err != nil {
		return nil, err
	}
	if err := srv.authName(ctx, AAFsoInitRegistry, regName); err != nil {
		return nil, err
	}

	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	ephemeralWorkflowsId := srv.names.UUID(
		NsFsoRegistryEphemeralWorkflows, regName,
	)
	newVid, err := srv.registry.EnableEphemeralWorkflows(
		id, vid, ephemeralWorkflowsId,
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.EnableEphemeralWorkflowsO{
		Vid:                  newVid[:],
		EphemeralWorkflowsId: ephemeralWorkflowsId[:],
	}, nil
}

func (srv *Server) EnablePropagateRootAcls(
	ctx context.Context, i *pb.EnablePropagateRootAclsI,
) (*pb.EnablePropagateRootAclsO, error) {
	regName := i.Registry
	id, err := srv.parseRegistryName(regName)
	if err != nil {
		return nil, err
	}
	if err := srv.authName(ctx, AAFsoInitRegistry, regName); err != nil {
		return nil, err
	}

	vid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.registry.EnablePropagateRootAcls(id, vid)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.EnablePropagateRootAclsO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) InitRoot(
	ctx context.Context, i *pb.InitRootI,
) (*pb.InitRootO, error) {
	root := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoInitRoot, root); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	cmd := &fsoregistry.CmdInitRoot{
		GlobalRoot:      root,
		Host:            i.Host,
		HostRoot:        i.HostRoot,
		GitlabNamespace: i.GitlabNamespace,
	}
	newVid, err := srv.registry.InitRoot(id, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.InitRootO{Vid: newVid[:]}, nil
}

func (srv *Server) RemoveRoot(
	ctx context.Context, i *pb.RemoveRootI,
) (*pb.RemoveRootO, error) {
	root := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoDeleteRoot, root); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	newVid, err := srv.registry.RemoveRoot(id, vid, root)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.RemoveRootO{Vid: newVid[:]}, nil
}

func (srv *Server) UpdateRootArchiveRecipients(
	ctx context.Context, i *pb.UpdateRootArchiveRecipientsI,
) (*pb.UpdateRootArchiveRecipientsO, error) {
	rootPath := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoAdminRoot, rootPath); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}
	keys, err := parseGPGFingerprintsBytes(i.ArchiveRecipients)
	if err != nil {
		return nil, err
	}

	reg2, err := srv.registry.UpdateRootArchiveRecipients(
		id, vid, rootPath, keys,
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	vid2 := reg2.Vid()
	root2, _ := reg2.Root(rootPath)
	if root2 == nil {
		panic("inconsistent registry state")
	}
	return &pb.UpdateRootArchiveRecipientsO{
		RegistryVid:       vid2[:],
		ArchiveRecipients: root2.ArchiveRecipients.Bytes(),
	}, nil
}

func (srv *Server) DeleteRootArchiveRecipients(
	ctx context.Context, i *pb.DeleteRootArchiveRecipientsI,
) (*pb.DeleteRootArchiveRecipientsO, error) {
	rootPath := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoAdminRoot, rootPath); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.registry.DeleteRootArchiveRecipients(
		id, vid, rootPath,
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.DeleteRootArchiveRecipientsO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) UpdateRootShadowBackupRecipients(
	ctx context.Context, i *pb.UpdateRootShadowBackupRecipientsI,
) (*pb.UpdateRootShadowBackupRecipientsO, error) {
	rootPath := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoAdminRoot, rootPath); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}
	keys, err := parseGPGFingerprintsBytes(i.ShadowBackupRecipients)
	if err != nil {
		return nil, err
	}

	reg2, err := srv.registry.UpdateRootShadowBackupRecipients(
		id, vid, rootPath, keys,
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	vid2 := reg2.Vid()
	root2, _ := reg2.Root(rootPath)
	if root2 == nil {
		panic("inconsistent registry state")
	}
	return &pb.UpdateRootShadowBackupRecipientsO{
		RegistryVid:            vid2[:],
		ShadowBackupRecipients: root2.ShadowBackupRecipients.Bytes(),
	}, nil
}

func (srv *Server) DeleteRootShadowBackupRecipients(
	ctx context.Context, i *pb.DeleteRootShadowBackupRecipientsI,
) (*pb.DeleteRootShadowBackupRecipientsO, error) {
	rootPath := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoAdminRoot, rootPath); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.registry.DeleteRootShadowBackupRecipients(
		id, vid, rootPath,
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.DeleteRootShadowBackupRecipientsO{
		RegistryVid: vid2[:],
	}, nil
}

func (srv *Server) InitRepo(
	ctx context.Context, i *pb.InitRepoI,
) (*pb.InitRepoO, error) {
	path := slashpath.Clean(i.GlobalPath)
	if err := srv.authPath(ctx, AAFsoInitRepo, path); err != nil {
		return nil, err
	}

	registryId, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	cmd := &fsoregistry.CmdInitRepo{
		Context:      copyAuthorizationMetadata(ctx),
		GlobalPath:   path,
		CreatorName:  i.CreatorName,
		CreatorEmail: i.CreatorEmail,
	}
	if i.RepoId != nil {
		repoId, err := parseRepoId(i.RepoId)
		if err != nil {
			return nil, err
		}
		cmd.Id = repoId
	}
	newVid, repoId, err := srv.registry.InitRepo(registryId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.InitRepoO{
		Vid:  newVid[:],
		Repo: repoId[:],
	}, nil
}

func (srv *Server) BeginMoveRepo(
	ctx context.Context, i *pb.BeginMoveRepoI,
) (*pb.BeginMoveRepoO, error) {
	// Auth both repo and new path.
	repoId, err := srv.authRegistryRepoId(
		ctx, AAFsoAdminRepo, i.Registry, i.Repo,
	)
	if err != nil {
		return nil, err
	}
	path := slashpath.Clean(i.NewGlobalPath)
	if err := srv.authPath(ctx, AAFsoAdminRepo, path); err != nil {
		return nil, err
	}

	registryId, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	workflowId, err := parseWorkflowId(i.Workflow)
	if err != nil {
		return nil, err
	}

	cmd := &fsoregistry.CmdBeginMoveRepo{
		RepoId:                repoId,
		WorkflowId:            workflowId,
		NewGlobalPath:         path,
		IsUnchangedGlobalPath: i.IsUnchangedGlobalPath,
	}
	newVid, err := srv.registry.BeginMoveRepo(registryId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.BeginMoveRepoO{
		Vid: newVid[:],
	}, nil
}

func copyAuthorizationMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	auth, ok := md["authorization"]
	if !ok {
		return ctx
	}
	return metadata.NewOutgoingContext(
		ctx, metadata.MD{"authorization": auth},
	)
}

func (srv *Server) ReinitRepo(
	ctx context.Context, i *pb.ReinitRepoI,
) (*pb.ReinitRepoO, error) {
	repoId, err := srv.authRegistryRepoId(
		ctx, AAFsoInitRepo, i.Registry, i.Repo,
	)
	if err != nil {
		return nil, err
	}

	registryId, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	cmd := &fsoregistry.CmdReinitRepo{
		RepoId: repoId,
		Reason: i.Reason,
	}
	newVid, err := srv.registry.ReinitRepo(registryId, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.ReinitRepoO{Vid: newVid[:]}, nil
}

func (srv *Server) Info(
	ctx context.Context, req *pb.InfoI,
) (*pb.InfoO, error) {
	regName := req.Registry
	if err := srv.authName(ctx, AAFsoReadRegistry, regName); err != nil {
		return nil, err
	}

	s, err := srv.getRegistryState(regName)
	if err != nil {
		return nil, err
	}

	vid := s.Vid()
	return &pb.InfoO{
		Registry: regName,
		Vid:      vid[:],
		NumRoots: int64(s.NumRoots()),
		NumRepos: int64(s.NumRepos()),
	}, nil
}

func (srv *Server) GetRoots(
	ctx context.Context, req *pb.GetRootsI,
) (*pb.GetRootsO, error) {
	regName := req.Registry
	if err := srv.authName(ctx, AAFsoReadRegistry, regName); err != nil {
		return nil, err
	}

	s, err := srv.getRegistryState(regName)
	if err != nil {
		return nil, err
	}

	vid := s.Vid()
	rsp := &pb.GetRootsO{
		Registry: regName,
		Vid:      vid[:],
	}
	for _, r := range s.Roots() {
		rsp.Roots = append(rsp.Roots, &pb.RootInfo{
			GlobalRoot:      r.GlobalRoot,
			Host:            r.Host,
			HostRoot:        r.HostRoot,
			GitlabNamespace: r.GitlabNamespace,
		})
	}
	return rsp, nil
}

func (srv *Server) GetRoot(
	ctx context.Context, i *pb.GetRootI,
) (*pb.GetRootO, error) {
	rootPath := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoReadRoot, rootPath); err != nil {
		return nil, err
	}

	regName := i.Registry
	reg, err := srv.getRegistryState(regName)
	if err != nil {
		return nil, err
	}

	root, ok := reg.Root(rootPath)
	if !ok {
		return nil, ErrUnknownRoot
	}

	vid := reg.Vid()
	return &pb.GetRootO{
		Registry:    regName,
		RegistryVid: vid[:],
		Root: &pb.RootInfoExt{
			GlobalRoot:             root.GlobalRoot,
			Host:                   root.Host,
			HostRoot:               root.HostRoot,
			GitlabNamespace:        root.GitlabNamespace,
			ArchiveRecipients:      root.ArchiveRecipients.Bytes(),
			ShadowBackupRecipients: root.ShadowBackupRecipients.Bytes(),
		},
	}, nil
}

func (srv *Server) GetRepos(
	ctx context.Context, req *pb.GetReposI,
) (*pb.GetReposO, error) {
	regName := req.Registry
	if err := srv.authName(ctx, AAFsoReadRegistry, regName); err != nil {
		return nil, err
	}

	s, err := srv.getRegistryState(regName)
	if err != nil {
		return nil, err
	}

	var repos []*fsoregistry.RepoInfo
	prefix := req.GlobalPathPrefix
	if prefix == "" {
		repos = s.Repos()
	} else {
		repos = s.ReposPrefix(slashpath.Clean(prefix))
	}

	vid := s.Vid()
	rsp := &pb.GetReposO{
		Registry: regName,
		Vid:      vid[:],
	}
	for _, r := range repos {
		rsp.Repos = append(rsp.Repos, &pb.RepoInfo{
			Id:         r.Id[:],
			GlobalPath: r.GlobalPath,
			Confirmed:  r.Confirmed,
		})
	}
	return rsp, nil
}

func (srv *Server) Events(
	req *pb.RegistryEventsI, stream pb.Registry_EventsServer,
) error {
	regName := req.Registry
	if err := checkRegistryName(regName); err != nil {
		return err
	}

	// `ctx.Done()` indicates client close, see
	// <https://groups.google.com/d/msg/grpc-io/C0rAhtCUhSs/SzFDLGqiCgAJ>.
	ctx := stream.Context()
	if err := srv.authName(ctx, AAFsoReadRegistry, regName); err != nil {
		return err
	}

	id := srv.names.UUID(NsFsoRegistry, regName)

	after := events.EventEpoch
	if req.After != nil {
		a, err := ulid.ParseBytes(req.After)
		if err != nil {
			err := status.Errorf(
				codes.InvalidArgument, "malformed after",
			)
			return err
		}
		after = a
	}

	updated := make(chan uuid.I, 1)
	updated <- id // Trigger initial Find().

	var ticks <-chan time.Time
	if req.Watch {
		srv.registryJ.Subscribe(updated, id)
		defer srv.registryJ.Unsubscribe(updated)

		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-srv.ctx.Done():
			err := status.Errorf(codes.Unavailable, "shutdown")
			return err
		case <-updated:
		case <-ticks:
		}

		iter := srv.registryJ.Find(id, after)
		var ev fsoregistry.Event
		for iter.Next(&ev) {
			after = ev.Id() // Update tail for restart.

			evpb := ev.PbRegistryEvent()
			rsp := &pb.RegistryEventsO{
				Registry: req.Registry,
				Events:   []*pb.RegistryEvent{evpb},
			}
			if err := stream.Send(rsp); err != nil {
				_ = iter.Close()
				return err
			}
		}
		if err := iter.Close(); err != nil {
			// XXX Maybe add more detailed error case handling.
			err := status.Errorf(
				codes.Unknown, "journal error: %v", err,
			)
			return err
		}

		if !req.Watch {
			return nil
		}

		rsp := &pb.RegistryEventsO{
			Registry:  req.Registry,
			WillBlock: true,
		}
		if err := stream.Send(rsp); err != nil {
			return err
		}
	}
}

func (srv *Server) RegistryWorkflowIndexEvents(
	req *pb.RegistryWorkflowIndexEventsI,
	stream pb.EphemeralRegistry_RegistryWorkflowIndexEventsServer,
) error {
	// `ctx.Done()` indicates client close, see
	// <https://groups.google.com/d/msg/grpc-io/C0rAhtCUhSs/SzFDLGqiCgAJ>.
	ctx := stream.Context()

	regName := req.Registry
	if err := checkRegistryName(regName); err != nil {
		return err
	}
	if err := srv.authName(ctx, AAFsoReadRegistry, regName); err != nil {
		return err
	}

	id := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)

	after := events.EventEpoch
	if req.After != nil {
		a, err := ulid.ParseBytes(req.After)
		if err != nil {
			err := status.Errorf(
				codes.InvalidArgument, "malformed after",
			)
			return err
		}
		after = a
	}

	updated := make(chan uuid.I, 1)
	updated <- id // Trigger initial Find().

	var ticks <-chan time.Time
	if req.Watch {
		srv.ephWorkflowsJ.Subscribe(updated, id)
		defer srv.ephWorkflowsJ.Unsubscribe(updated)

		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-srv.ctx.Done():
			err := status.Errorf(codes.Unavailable, "shutdown")
			return err
		case <-updated:
		case <-ticks:
		}

		iter := srv.ephWorkflowsJ.Find(id, after)
		var ev wfindexes.Event
		for iter.Next(&ev) {
			after = ev.Id() // Update tail for restart.

			evpb := ev.PbWorkflowEvent()
			rsp := &pb.RegistryWorkflowIndexEventsO{
				Registry: req.Registry,
				Events:   []*pb.WorkflowEvent{evpb},
			}
			if err := stream.Send(rsp); err != nil {
				_ = iter.Close()
				return err
			}
		}
		if err := iter.Close(); err != nil {
			// XXX Maybe add more detailed error case handling.
			err := status.Errorf(
				codes.Unknown, "journal error: %v", err,
			)
			return err
		}

		if !req.Watch {
			return nil
		}

		rsp := &pb.RegistryWorkflowIndexEventsO{
			Registry:  req.Registry,
			WillBlock: true,
		}
		if err := stream.Send(rsp); err != nil {
			return err
		}
	}
}

func (srv *Server) RegistryWorkflowEvents(
	req *pb.RegistryWorkflowEventsI,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsServer,
) error {
	// `ctx.Done()` indicates client close, see
	// <https://groups.google.com/d/msg/grpc-io/C0rAhtCUhSs/SzFDLGqiCgAJ>.
	ctx := stream.Context()

	// The initial authorization checks only the registry.  The check
	// whether the registry owns the workflow is deferred until the first
	// workflow event, in order to allow watching uninitialized workflows.
	//
	// XXX An alternative strategy could try to use an immediate check and
	// use the deferred check only if the workflow is unitialized.
	registryName := req.Registry
	if err := checkRegistryName(registryName); err != nil {
		return err
	}
	if err := srv.authName(
		ctx, AAFsoReadRegistry, registryName,
	); err != nil {
		return err
	}
	registryId := srv.names.UUID(NsFsoRegistry, registryName)
	workflowId, err := parseWorkflowId(req.Workflow)
	if err != nil {
		return err
	}
	requireWorkflowIdCheck := true

	checkWorkflowId := func(evpb *pb.WorkflowEvent) error {
		if !requireWorkflowIdCheck {
			return nil
		}

		if evpb.RegistryId == nil {
			return ErrForeignRegistryWorkflow
		}
		evRegistryId, err := parseRegistryId(evpb.RegistryId)
		if err != nil {
			return err
		}
		if evRegistryId != registryId {
			return ErrForeignRegistryWorkflow
		}

		// Additional auth for known workflow types.  Default deny for
		// unknown workflow types.
		ev, err := wfevents.ParsePbWorkflowEvent(evpb)
		if err != nil {
			return ErrParsePb
		}
		switch x := ev.(type) {
		case *wfevents.EvDuRootStarted:
			//  Check read root acccess, because reading the events
			//  reveals the same information as `GetDuRoot()`.
			if err := srv.authPath(
				ctx, AAFsoReadRoot, x.GlobalRoot,
			); err != nil {
				return err
			}

		case *wfevents.EvPingRegistryStarted:
			// No additional check for ping-registry workflow.
			// AAFsoReadRegistry is sufficient.
			break

		case *wfevents.EvSplitRootStarted:
			//  Check AAFsoReadRoot, because the events reveal
			//  root-specific information.
			//
			//  Do not check AAFsoAdminRegistry, because the events
			//  do not contain sensitive information.
			if err := srv.authPath(
				ctx, AAFsoReadRoot, x.GlobalRoot,
			); err != nil {
				return err
			}

		case *wfevents.EvFreezeRepoStarted2:
			if err := srv.authPath(
				ctx, AAFsoReadRepo, x.RepoGlobalPath,
			); err != nil {
				return err
			}

		case *wfevents.EvUnfreezeRepoStarted2:
			if err := srv.authPath(
				ctx, AAFsoReadRepo, x.RepoGlobalPath,
			); err != nil {
				return err
			}

		case *wfevents.EvArchiveRepoStarted:
			if err := srv.authPath(
				ctx, AAFsoReadRepo, x.RepoGlobalPath,
			); err != nil {
				return err
			}

		case *wfevents.EvUnarchiveRepoStarted:
			if err := srv.authPath(
				ctx, AAFsoReadRepo, x.RepoGlobalPath,
			); err != nil {
				return err
			}

		default:
			return ErrDenyUnknownWorkflowType
		}

		requireWorkflowIdCheck = false
		return nil
	}

	// `after` is the tail event that has been read from the journal.
	// Reading initially starts with the first event, which is required for
	// the deferred auth check.
	after := events.EventEpoch

	// `sendAfter` determines whether events are sent to the client: events
	// are sent if `sendAfter == events.Epoch`.  `sendAfter` is initialized
	// with `req.After` and reset to `EventEpoch` when the `sendAfter`
	// event is found.  From then on, all further events are sent.
	sendAfter := events.EventEpoch
	if req.After != nil {
		a, err := ulid.ParseBytes(req.After)
		if err != nil {
			return ErrMalformedAfter
		}
		sendAfter = a
	}

	updated := make(chan uuid.I, 1)
	updated <- workflowId // Trigger initial Find().

	var ticks <-chan time.Time
	if req.Watch {
		srv.ephWorkflowsJ.Subscribe(updated, workflowId)
		defer srv.ephWorkflowsJ.Unsubscribe(updated)

		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-srv.ctx.Done():
			return ErrShutdown
		case <-updated:
		case <-ticks:
		}

		iter := srv.ephWorkflowsJ.Find(workflowId, after)
		var ev wfevents.Event
		for iter.Next(&ev) {
			after = ev.Id() // Update tail for restart.
			evpb := ev.PbWorkflowEvent()

			if err := checkWorkflowId(evpb); err != nil {
				_ = iter.Close()
				return err
			}

			if sendAfter != events.EventEpoch {
				if ev.Id() == sendAfter {
					// Send further events.
					sendAfter = events.EventEpoch
				}
				continue
			}

			rsp := &pb.RegistryWorkflowEventsO{
				Registry: req.Registry,
				Workflow: req.Workflow,
				Events:   []*pb.WorkflowEvent{evpb},
			}
			if err := stream.Send(rsp); err != nil {
				_ = iter.Close()
				return err
			}
		}
		if err := iter.Close(); err != nil {
			// XXX Maybe add more detailed error case handling.
			err := status.Errorf(
				codes.Unknown,
				"workflow journal error: %v", err,
			)
			return err
		}

		if !req.Watch {
			return nil
		}

		rsp := &pb.RegistryWorkflowEventsO{
			Registry:  req.Registry,
			Workflow:  req.Workflow,
			WillBlock: true,
		}
		if err := stream.Send(rsp); err != nil {
			return err
		}
	}
}

func (srv *Server) EnableGitlabRoot(
	ctx context.Context, i *pb.EnableGitlabRootI,
) (*pb.EnableGitlabRootO, error) {
	root := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoInitRoot, root); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	cmd := &fsoregistry.CmdEnableGitlab{
		GlobalRoot:      root,
		GitlabNamespace: i.GitlabNamespace,
	}
	newVid, err := srv.registry.EnableGitlab(id, vid, cmd)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.EnableGitlabRootO{Vid: newVid[:]}, nil
}

func (srv *Server) DisableGitlabRoot(
	ctx context.Context, i *pb.DisableGitlabRootI,
) (*pb.DisableGitlabRootO, error) {
	root := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoInitRoot, root); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	newVid, err := srv.registry.DisableGitlab(id, vid, root)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.DisableGitlabRootO{Vid: newVid[:]}, nil
}

func (srv *Server) UpdateRepoNaming(
	ctx context.Context, i *pb.UpdateRepoNamingI,
) (*pb.UpdateRepoNamingO, error) {
	naming := i.Naming
	if naming == nil {
		err := status.Error(codes.InvalidArgument, "missing naming")
		return nil, err
	}
	root := slashpath.Clean(naming.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoInitRoot, root); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	newVid, err := srv.registry.SetRepoNaming(id, vid, naming)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.UpdateRepoNamingO{Vid: newVid[:]}, nil
}

func (srv *Server) PatchRepoNaming(
	ctx context.Context, i *pb.PatchRepoNamingI,
) (*pb.PatchRepoNamingO, error) {
	patch := i.NamingPatch
	if patch == nil {
		err := status.Error(
			codes.InvalidArgument, "missing naming patch",
		)
		return nil, err
	}
	root := slashpath.Clean(patch.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoInitRoot, root); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	newVid, err := srv.registry.PatchRepoNaming(id, vid, patch)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.PatchRepoNamingO{Vid: newVid[:]}, nil
}

func (srv *Server) EnableDiscoveryPaths(
	ctx context.Context, i *pb.EnableDiscoveryPathsI,
) (*pb.EnableDiscoveryPathsO, error) {
	globalRoot := slashpath.Clean(i.GlobalRoot)
	if len(i.DepthPaths) == 0 {
		err := status.Error(
			codes.InvalidArgument, "missing paths",
		)
		return nil, err
	}

	depthPaths := make([]fsoregistry.DepthPath, 0, len(i.DepthPaths))
	for _, dp := range i.DepthPaths {
		if dp.Depth < 0 || dp.Depth > 10 {
			err := status.Error(
				codes.InvalidArgument, "depth out of range",
			)
			return nil, err
		}
		depthPaths = append(depthPaths, fsoregistry.DepthPath{
			Depth: int(dp.Depth),
			Path:  dp.Path,
		})
	}

	if err := srv.authPath(
		ctx, AAFsoEnableDiscoveryPath, globalRoot,
	); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	newVid, err := srv.registry.EnableDiscoveryPaths(
		id, vid, globalRoot, depthPaths,
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.EnableDiscoveryPathsO{Vid: newVid[:]}, nil
}

func (srv *Server) UpdateRepoInitPolicy(
	ctx context.Context, i *pb.UpdateRepoInitPolicyI,
) (*pb.UpdateRepoInitPolicyO, error) {
	policy := i.Policy
	if policy == nil {
		err := status.Error(codes.InvalidArgument, "missing policy")
		return nil, err
	}
	root := slashpath.Clean(policy.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoInitRoot, root); err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	newVid, err := srv.registry.SetRepoInitPolicy(id, vid, policy)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.UpdateRepoInitPolicyO{Vid: newVid[:]}, nil
}

func (srv *Server) EnableGitlabRepo(
	ctx context.Context, i *pb.EnableGitlabRepoI,
) (*pb.EnableGitlabRepoO, error) {
	repoId, err := srv.authRegistryRepoId(
		ctx, AAFsoInitRepo, i.Registry, i.Repo,
	)
	if err != nil {
		return nil, err
	}

	id, err := srv.parseRegistryName(i.Registry)
	if err != nil {
		return nil, err
	}
	vid, err := parseRegistryVid(i.Vid)
	if err != nil {
		return nil, err
	}
	newVid, err := srv.registry.EnableGitlabRepo(
		id, vid, repoId, i.GitlabNamespace,
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.EnableGitlabRepoO{Vid: newVid[:]}, nil
}

func (srv *Server) GetRepoAclPolicy(
	ctx context.Context, i *pb.GetRepoAclPolicyI,
) (*pb.GetRepoAclPolicyO, error) {
	regName := i.Registry
	reg, repoId, err := srv.authRegistryStateRepoId(
		ctx, AAFsoReadRepo, regName, i.Repo,
	)
	if err != nil {
		return nil, err
	}

	policy, _ := reg.RepoAclPolicy(repoId)
	vid := reg.Vid()
	return &pb.GetRepoAclPolicyO{
		Registry:    regName,
		RegistryVid: vid[:],
		Policy:      policy,
	}, nil
}
