package registryd

import (
	"context"
	slashpath "path"
	"strings"

	"github.com/nogproject/nog/backend/internal/fsoregistry"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/workflows/splitrootwf"
	"github.com/nogproject/nog/backend/internal/workflows/wfindexes"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) GetSplitRootConfig(
	ctx context.Context, i *pb.GetSplitRootConfigI,
) (*pb.GetSplitRootConfigO, error) {
	regName := i.Registry
	rootPath := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoReadRoot, rootPath); err != nil {
		return nil, err
	}

	reg, err := srv.getRegistryState(regName)
	if err != nil {
		return nil, err
	}

	if !reg.HasRoot(rootPath) {
		return nil, ErrUnknownRoot
	}

	regVid := reg.Vid()
	o := &pb.GetSplitRootConfigO{
		RegistryVid: regVid[:],
	}
	cfg, ok := reg.SplitRootConfig(rootPath)
	if ok {
		o.Config = &pb.SplitRootConfig{
			GlobalRoot:   rootPath,
			Enabled:      true,
			MaxDepth:     cfg.MaxDepth,
			MinDiskUsage: cfg.MinDiskUsage,
			MaxDiskUsage: cfg.MaxDiskUsage,
		}
	} else {
		o.Config = &pb.SplitRootConfig{
			GlobalRoot: rootPath,
			Enabled:    false,
		}
	}
	return o, nil
}

func (srv *Server) CreateSplitRootConfig(
	ctx context.Context, i *pb.CreateSplitRootConfigI,
) (*pb.CreateSplitRootConfigO, error) {
	regName := i.Registry
	if err := srv.authName(ctx, AAFsoAdminRegistry, regName); err != nil {
		return nil, err
	}

	regId, err := srv.parseRegistryName(regName)
	if err != nil {
		return nil, err
	}
	regVid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}
	cfg := i.Config
	if cfg == nil {
		return nil, ErrMissingConfig
	}
	rootPath := slashpath.Clean(cfg.GlobalRoot)

	reg2, err := srv.registry.CreateSplitRootConfig(
		regId, regVid,
		rootPath, &fsoregistry.SplitRootConfig{
			MaxDepth:     cfg.MaxDepth,
			MinDiskUsage: cfg.MinDiskUsage,
			MaxDiskUsage: cfg.MaxDiskUsage,
		},
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}
	regVid2 := reg2.Vid()
	cfg2, ok := reg2.SplitRootConfig(rootPath)
	if !ok {
		return nil, ErrInconsistentRegistryState
	}

	return &pb.CreateSplitRootConfigO{
		RegistryVid: regVid2[:],
		Config: &pb.SplitRootConfig{
			GlobalRoot:   rootPath,
			Enabled:      true,
			MaxDepth:     cfg2.MaxDepth,
			MinDiskUsage: cfg2.MinDiskUsage,
			MaxDiskUsage: cfg2.MaxDiskUsage,
		},
	}, nil
}

func (srv *Server) UpdateSplitRootConfig(
	ctx context.Context, i *pb.UpdateSplitRootConfigI,
) (*pb.UpdateSplitRootConfigO, error) {
	regName := i.Registry
	if err := srv.authName(ctx, AAFsoAdminRegistry, regName); err != nil {
		return nil, err
	}

	regId, err := srv.parseRegistryName(regName)
	if err != nil {
		return nil, err
	}
	regVid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}
	cfg := i.Config
	if cfg == nil {
		return nil, ErrMissingConfig
	}
	rootPath := slashpath.Clean(cfg.GlobalRoot)

	reg2, err := srv.registry.UpdateSplitRootConfig(
		regId, regVid,
		rootPath, &fsoregistry.SplitRootConfig{
			MaxDepth:     cfg.MaxDepth,
			MinDiskUsage: cfg.MinDiskUsage,
			MaxDiskUsage: cfg.MaxDiskUsage,
		},
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}
	regVid2 := reg2.Vid()
	cfg2, ok := reg2.SplitRootConfig(rootPath)
	if !ok {
		return nil, ErrInconsistentRegistryState
	}

	return &pb.UpdateSplitRootConfigO{
		RegistryVid: regVid2[:],
		Config: &pb.SplitRootConfig{
			GlobalRoot:   rootPath,
			Enabled:      true,
			MaxDepth:     cfg2.MaxDepth,
			MinDiskUsage: cfg2.MinDiskUsage,
			MaxDiskUsage: cfg2.MaxDiskUsage,
		},
	}, nil
}

func (srv *Server) DeleteSplitRootConfig(
	ctx context.Context, i *pb.DeleteSplitRootConfigI,
) (*pb.DeleteSplitRootConfigO, error) {
	regName := i.Registry
	if err := srv.authName(ctx, AAFsoAdminRegistry, regName); err != nil {
		return nil, err
	}

	regId, err := srv.parseRegistryName(regName)
	if err != nil {
		return nil, err
	}
	regVid, err := parseRegistryVid(i.RegistryVid)
	if err != nil {
		return nil, err
	}

	regVid2, err := srv.registry.DeleteSplitRootConfig(
		regId, regVid, slashpath.Clean(i.GlobalRoot),
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.DeleteSplitRootConfigO{
		RegistryVid: regVid2[:],
	}, nil
}

func (srv *Server) CreateSplitRootPathFlag(
	ctx context.Context, i *pb.CreateSplitRootPathFlagI,
) (*pb.CreateSplitRootPathFlagO, error) {
	regName := i.Registry
	rootPath := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoAdminRoot, rootPath); err != nil {
		return nil, err
	}

	reg, err := srv.getRegistryState(regName)
	if err != nil {
		return nil, err
	}
	if i.RegistryVid != nil {
		regVid, err := ulid.ParseBytes(i.RegistryVid)
		if err != nil {
			return nil, ErrMalformedVid
		}
		if regVid != reg.Vid() {
			return nil, ErrVersionConflict
		}
	}
	regId := reg.Id()
	regVid := reg.Vid()

	root, ok := reg.Root(rootPath)
	if !ok {
		return nil, ErrUnknownRoot
	}
	rootPathSlash := root.GlobalRoot + "/"

	path := slashpath.Join(rootPath, i.RelativePath)
	if !strings.HasPrefix(path, rootPathSlash) {
		return nil, ErrMalformedPath
	}

	regVid2, err := srv.registry.SetPathFlags(
		regId, regVid, path, i.Flags,
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.CreateSplitRootPathFlagO{
		RegistryVid: regVid2[:],
	}, nil
}

func (srv *Server) DeleteSplitRootPathFlag(
	ctx context.Context, i *pb.DeleteSplitRootPathFlagI,
) (*pb.DeleteSplitRootPathFlagO, error) {
	regName := i.Registry
	if err := srv.authName(ctx, AAFsoAdminRegistry, regName); err != nil {
		return nil, err
	}

	reg, err := srv.getRegistryState(regName)
	if err != nil {
		return nil, err
	}
	if i.RegistryVid != nil {
		regVid, err := ulid.ParseBytes(i.RegistryVid)
		if err != nil {
			return nil, ErrMalformedVid
		}
		if regVid != reg.Vid() {
			return nil, ErrVersionConflict
		}
	}
	regId := reg.Id()
	regVid := reg.Vid()

	rootPath := slashpath.Clean(i.GlobalRoot)
	root, ok := reg.Root(rootPath)
	if !ok {
		return nil, ErrUnknownRoot
	}
	rootPathSlash := root.GlobalRoot + "/"

	path := slashpath.Join(rootPath, i.RelativePath)
	if !strings.HasPrefix(path, rootPathSlash) {
		return nil, ErrMalformedPath
	}

	regVid2, err := srv.registry.UnsetPathFlags(
		regId, regVid, path, i.Flags,
	)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return &pb.DeleteSplitRootPathFlagO{
		RegistryVid: regVid2[:],
	}, nil
}

func (srv *Server) ListSplitRootPathFlags(
	ctx context.Context, i *pb.ListSplitRootPathFlagsI,
) (*pb.ListSplitRootPathFlagsO, error) {
	regName := i.Registry
	rootPath := slashpath.Clean(i.GlobalRoot)
	if err := srv.authAny(
		ctx,
		authzScope{Action: AAFsoReadRoot, Path: rootPath},
		authzScope{Action: AAFsoExecSplitRoot, Path: rootPath},
	); err != nil {
		return nil, err
	}

	reg, err := srv.getRegistryState(regName)
	if err != nil {
		return nil, err
	}
	if i.RegistryVid != nil {
		regVid, err := ulid.ParseBytes(i.RegistryVid)
		if err != nil {
			return nil, ErrMalformedVid
		}
		if regVid != reg.Vid() {
			return nil, ErrVersionConflict
		}
	}

	regVid := reg.Vid()
	o := &pb.ListSplitRootPathFlagsO{
		RegistryVid: regVid[:],
	}

	pfMap := reg.PathFlagsPrefix(rootPath)
	if len(pfMap) > 0 {
		pfSlice := make([]*pb.FsoPathFlag, 0, len(pfMap))
		for p, f := range pfMap {
			pfSlice = append(pfSlice, &pb.FsoPathFlag{
				Path:  p,
				Flags: f,
			})
		}
		o.Paths = pfSlice
	}

	return o, nil
}

func (srv *Server) BeginSplitRoot(
	ctx context.Context, i *pb.BeginSplitRootI,
) (*pb.BeginSplitRootO, error) {
	regName := i.Registry
	rootPath := slashpath.Clean(i.GlobalRoot)
	if err := srv.authPath(ctx, AAFsoAdminRoot, rootPath); err != nil {
		return nil, err
	}

	reg, err := srv.getRegistryState(regName)
	if err != nil {
		return nil, err
	}
	if i.RegistryVid != nil {
		regVid, err := ulid.ParseBytes(i.RegistryVid)
		if err != nil {
			return nil, ErrMalformedVid
		}
		if regVid != reg.Vid() {
			return nil, ErrVersionConflict
		}
	}

	root, ok := reg.Root(rootPath)
	if !ok {
		return nil, ErrUnknownRoot
	}
	cfg, ok := reg.SplitRootConfig(rootPath)
	if !ok {
		return nil, ErrSplitRootDisabled
	}

	wfId, err := uuid.FromBytes(i.Workflow)
	if err != nil {
		return nil, ErrMalformedWorkflowId
	}
	reason, err := srv.idChecker.IsUnusedId(wfId)
	switch {
	case err != nil:
		return nil, err
	case reason != "":
		return nil, status.Errorf(
			codes.FailedPrecondition,
			"rejected workflow ID: %s", reason,
		)
	}

	// XXX We could copy the DONT_SPLIT path flags here and store them in
	// the split-root workflow init event.  But the list might be long.
	// Nogfsostad instead retrieves the flags when needed.  The flags,
	// therefore, are not protected by version control, which should be
	// good enough in practice, in particular because an admin reviews the
	// suggestions before any actions that cause permanent effects are
	// executed.
	wfVid, err := srv.splitRootWorkflows.Init(
		wfId,
		&splitrootwf.CmdInit{
			RegistryId:   reg.Id(),
			GlobalRoot:   root.GlobalRoot,
			Host:         root.Host,
			HostRoot:     root.HostRoot,
			MaxDepth:     cfg.MaxDepth,
			MinDiskUsage: cfg.MinDiskUsage,
			MaxDiskUsage: cfg.MaxDiskUsage,
		},
	)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.BeginSplitRoot(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdBeginSplitRoot{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid,
			GlobalRoot:      root.GlobalRoot,
			Host:            root.Host,
			HostRoot:        root.HostRoot,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	regVid := reg.Vid()
	return &pb.BeginSplitRootO{
		RegistryVid:      regVid[:],
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid[:],
	}, nil
}

func (srv *Server) AppendSplitRootDu(
	ctx context.Context, i *pb.AppendSplitRootDuI,
) (*pb.AppendSplitRootDuO, error) {
	_, wf, err := srv.authSplitRootWorkflowId(
		ctx, AAFsoExecDu, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	wfVid, err := parseSplitRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	dus := make([]splitrootwf.PathUsage, 0, len(i.Paths))
	for _, p := range i.Paths {
		dus = append(dus, splitrootwf.PathUsage{
			Path:  p.Path,
			Usage: p.Usage,
		})
	}
	v, err := srv.splitRootWorkflows.AppendDus(wfId, wfVid, dus)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}
	wfVid = v

	return &pb.AppendSplitRootDuO{
		WorkflowVid: wfVid[:],
	}, nil
}

func (srv *Server) CommitSplitRootDu(
	ctx context.Context, i *pb.CommitSplitRootDuI,
) (*pb.CommitSplitRootDuO, error) {
	_, wf, err := srv.authSplitRootWorkflowId(
		ctx, AAFsoExecDu, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	wfVid, err := parseSplitRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.splitRootWorkflows.CommitDu(wfId, wfVid)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}

	return &pb.CommitSplitRootDuO{
		WorkflowVid: wfVid2[:],
	}, nil
}

func (srv *Server) AbortSplitRootDu(
	ctx context.Context, i *pb.AbortSplitRootDuI,
) (*pb.AbortSplitRootDuO, error) {
	_, wf, err := srv.authSplitRootWorkflowId(
		ctx, AAFsoExecDu, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	wfVid, err := parseSplitRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.splitRootWorkflows.AbortDu(
		wfId, wfVid, i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}

	return &pb.AbortSplitRootDuO{
		WorkflowVid: wfVid2[:],
	}, nil
}

func (srv *Server) AppendSplitRootSuggestions(
	ctx context.Context, i *pb.AppendSplitRootSuggestionsI,
) (*pb.AppendSplitRootSuggestionsO, error) {
	_, wf, err := srv.authSplitRootWorkflowId(
		ctx, AAFsoExecSplitRoot, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	wfVid, err := parseSplitRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	ss := make([]splitrootwf.Suggestion, 0, len(i.Paths))
	for _, p := range i.Paths {
		ss = append(ss, splitrootwf.Suggestion{
			Path:       p.Path,
			Suggestion: p.Suggestion,
		})
	}
	v, err := srv.splitRootWorkflows.AppendSuggestions(wfId, wfVid, ss)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}
	wfVid = v

	return &pb.AppendSplitRootSuggestionsO{
		WorkflowVid: wfVid[:],
	}, nil
}

func (srv *Server) CommitSplitRootAnalysis(
	ctx context.Context, i *pb.CommitSplitRootAnalysisI,
) (*pb.CommitSplitRootAnalysisO, error) {
	_, wf, err := srv.authSplitRootWorkflowId(
		ctx, AAFsoExecSplitRoot, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	wfVid, err := parseSplitRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.splitRootWorkflows.CommitAnalysis(wfId, wfVid)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}

	return &pb.CommitSplitRootAnalysisO{
		WorkflowVid: wfVid2[:],
	}, nil
}

func (srv *Server) AbortSplitRootAnalysis(
	ctx context.Context, i *pb.AbortSplitRootAnalysisI,
) (*pb.AbortSplitRootAnalysisO, error) {
	_, wf, err := srv.authSplitRootWorkflowId(
		ctx, AAFsoExecSplitRoot, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	wfVid, err := parseSplitRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.splitRootWorkflows.AbortAnalysis(
		wfId, wfVid, i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}

	return &pb.AbortSplitRootAnalysisO{
		WorkflowVid: wfVid2[:],
	}, nil
}

func (srv *Server) AppendSplitRootDecisions(
	ctx context.Context, i *pb.AppendSplitRootDecisionsI,
) (*pb.AppendSplitRootDecisionsO, error) {
	euid, wf, err := srv.authSplitRootWorkflowId(
		ctx, AAFsoAdminRoot, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()
	rootPath := wf.GlobalRoot()

	reg, err := srv.registry.FindId(wf.RegistryId())
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	rootPathSlash := ensureTrailingSlash(rootPath)
	for _, p := range i.Paths {
		path := slashpath.Clean(p.Path)
		var relPath string
		if strings.HasPrefix(path, rootPathSlash) {
			relPath = strings.TrimPrefix(path, rootPathSlash)
		} else if path == rootPath {
			relPath = "."
		} else {
			return nil, ErrPathOutsideRoot
		}

		if !wf.IsCandidate(relPath) {
			return nil, ErrPathNotCandidate
		}

		if p.Decision == pb.AppendSplitRootDecisionsI_D_CREATE_REPO {
			if err := srv.authzPath(
				euid, AAFsoInitRepo, path,
			); err != nil {
				return nil, err
			}
		}
	}

	wfVid, err := parseSplitRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}
	if wfVid != splitrootwf.NoVC && wfVid != wf.Vid() {
		return nil, ErrVersionConflict
	}
	wfVid = wf.Vid()
	regId := reg.Id()
	regVid := reg.Vid()

	o := &pb.AppendSplitRootDecisionsO{}

	for _, p := range i.Paths {
		path := slashpath.Clean(p.Path)
		var relPath string
		if strings.HasPrefix(path, rootPathSlash) {
			relPath = strings.TrimPrefix(path, rootPathSlash)
		} else if path == rootPath {
			relPath = "."
		} else {
			panic("unreachable")
		}
		switch p.Decision {
		case pb.AppendSplitRootDecisionsI_D_CREATE_REPO:
			var repoId uuid.I
			if repo, ok := reg.RepoByPath(path); ok {
				repoId = repo.Id
			} else {
				v, r, err := srv.registry.InitRepo(
					regId, fsoregistry.RetryNoVC,
					&fsoregistry.CmdInitRepo{
						Context:      copyAuthorizationMetadata(ctx),
						GlobalPath:   path,
						CreatorName:  i.CreatorName,
						CreatorEmail: i.CreatorEmail,
					},
				)
				if err != nil {
					return nil, asRegistryGrpcError(err)
				}
				regVid = v
				repoId = r
			}

			v, err := srv.splitRootWorkflows.AppendDecision(
				wfId, wfVid,
				relPath, pb.FsoSplitRootDecision_D_CREATE_REPO,
			)
			if err != nil {
				return nil, asSplitRootWorkflowGrpcError(err)
			}
			wfVid = v

			regVid2 := regVid // unalias
			wfVid2 := wfVid   // unalias
			o.Effects = append(o.Effects, &pb.AppendSplitRootDecisionsO_Effect{
				Path:        path,
				RepoId:      repoId[:],
				RegistryVid: regVid2[:],
				WorkflowVid: wfVid2[:],
			})

		case pb.AppendSplitRootDecisionsI_D_NEVER_SPLIT:
			v, err := srv.registry.SetPathFlags(
				regId, fsoregistry.RetryNoVC,
				path, uint32(pb.FsoPathFlag_PF_DONT_SPLIT),
			)
			if err != nil {
				return nil, asRegistryGrpcError(err)
			}
			regVid = v

			v, err = srv.splitRootWorkflows.AppendDecision(
				wfId, wfVid,
				relPath, pb.FsoSplitRootDecision_D_NEVER_SPLIT,
			)
			if err != nil {
				return nil, asSplitRootWorkflowGrpcError(err)
			}
			wfVid = v

			regVid2 := regVid // unalias
			wfVid2 := wfVid   // unalias
			o.Effects = append(o.Effects, &pb.AppendSplitRootDecisionsO_Effect{
				Path:        path,
				RegistryVid: regVid2[:],
				WorkflowVid: wfVid2[:],
			})

		case pb.AppendSplitRootDecisionsI_D_IGNORE_ONCE:
			v, err := srv.splitRootWorkflows.AppendDecision(
				wfId, wfVid,
				relPath, pb.FsoSplitRootDecision_D_IGNORE_ONCE,
			)
			if err != nil {
				return nil, asSplitRootWorkflowGrpcError(err)
			}
			wfVid = v

			wfVid2 := wfVid // unalias
			o.Effects = append(o.Effects, &pb.AppendSplitRootDecisionsO_Effect{
				Path:        path,
				WorkflowVid: wfVid2[:],
			})

		default:
			return nil, ErrSplitRootDecisionUnimplemented
		}
	}

	o.WorkflowVid = wfVid[:]
	o.RegistryVid = regVid[:]
	return o, nil
}

func (srv *Server) CommitSplitRoot(
	ctx context.Context, i *pb.CommitSplitRootI,
) (*pb.CommitSplitRootO, error) {
	_, wf, err := srv.authSplitRootWorkflowId(
		ctx, AAFsoAdminRoot, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	reg, err := srv.registry.FindId(wf.RegistryId())
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}
	regName := reg.Name()

	wfVid, err := parseSplitRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.splitRootWorkflows.Commit(wfId, wfVid)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.CommitSplitRoot(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitSplitRoot{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.splitRootWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}

	return &pb.CommitSplitRootO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) AbortSplitRoot(
	ctx context.Context, i *pb.AbortSplitRootI,
) (*pb.AbortSplitRootO, error) {
	_, wf, err := srv.authAnySplitRootWorkflowId(
		ctx,
		[]auth.Action{AAFsoExecSplitRoot, AAFsoAdminRoot},
		i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()

	reg, err := srv.registry.FindId(wf.RegistryId())
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}
	regName := reg.Name()

	wfVid, err := parseSplitRootVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.splitRootWorkflows.Abort(
		wfId, wfVid,
		i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, regName)
	idxVid, err := srv.workflowIndexes.CommitSplitRoot(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitSplitRoot{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.splitRootWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asSplitRootWorkflowGrpcError(err)
	}

	return &pb.AbortSplitRootO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) GetSplitRoot(
	ctx context.Context, i *pb.GetSplitRootI,
) (*pb.GetSplitRootO, error) {
	_, wf, err := srv.authSplitRootWorkflowId(
		ctx, AAFsoAdminRoot, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := wf.Id()
	rootPath := wf.GlobalRoot()

	// If JC_WAIT, wait at least for analysis.
	if i.JobControl == pb.JobControl_JC_WAIT {
		// Subscribe first, then find to ensure that no event is lost.
		updated := make(chan uuid.I, 1)
		srv.ephWorkflowsJ.Subscribe(updated, wfId)
		defer srv.ephWorkflowsJ.Unsubscribe(updated)

	Loop:
		for {
			w, err := srv.splitRootWorkflows.FindId(wfId)
			if err != nil {
				return nil, asSplitRootWorkflowGrpcError(err)
			}
			wf = w

			switch wf.StateCode() {
			case splitrootwf.StateUninitialized: // wait
			case splitrootwf.StateInitialized: // wait
			case splitrootwf.StateDuAppending: // wait
			case splitrootwf.StateDuCompleted: // wait
			case splitrootwf.StateSuggestionsAppending: // wait
			default:
				break Loop
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-updated:
				continue Loop
			}
		}
	}

	wfVid := wf.Vid()
	o := &pb.GetSplitRootO{
		WorkflowVid: wfVid[:],
		GlobalRoot:  rootPath,
	}

	switch wf.StateCode() {
	// `splitrootwf.StateUninitialized` has been rejected at top of func.

	case splitrootwf.StateInitialized:
		fallthrough
	case splitrootwf.StateDuAppending:
		o.StatusCode = int32(pb.GetSplitRootO_SC_RUNNING)
		o.StatusMessage = "du running"

	case splitrootwf.StateDuCompleted:
		fallthrough
	case splitrootwf.StateSuggestionsAppending:
		o.StatusCode = int32(pb.GetSplitRootO_SC_RUNNING)
		o.StatusMessage = "analysis running"

	case splitrootwf.StateAnalysisCompleted:
		o.StatusCode = int32(pb.GetSplitRootO_SC_ANALYSIS_COMPLETED)
		o.StatusMessage = "analysis completed"

	case splitrootwf.StateDecisionsAppending:
		o.StatusCode = int32(pb.GetSplitRootO_SC_ANALYSIS_COMPLETED)
		o.StatusMessage = "analysis completed"

	case splitrootwf.StateDuFailed:
		o.StatusCode = int32(pb.GetSplitRootO_SC_FAILED)
		o.StatusMessage = "du failed"

	case splitrootwf.StateAnalysisFailed:
		o.StatusCode = int32(pb.GetSplitRootO_SC_FAILED)
		o.StatusMessage = "analysis failed"

	case splitrootwf.StateCompleted:
		o.StatusCode = int32(pb.GetSplitRootO_SC_COMPLETED)
		o.StatusMessage = wf.StatusMessage()

	case splitrootwf.StateFailed:
		o.StatusCode = int32(pb.GetSplitRootO_SC_FAILED)
		o.StatusMessage = wf.StatusMessage()

	case splitrootwf.StateTerminated:
		o.StatusCode = wf.StatusCode()
		o.StatusMessage = wf.StatusMessage()

	default:
		return nil, ErrUnknownWorkflowState
	}

	var du []*pb.PathDiskUsage
	for _, p := range wf.Du() {
		du = append(du, &pb.PathDiskUsage{
			Path:  p.Path,
			Usage: p.Usage,
		})
	}
	o.Du = du

	var ss []*pb.FsoSplitRootSuggestion
	for _, s := range wf.Suggestions() {
		ss = append(ss, &pb.FsoSplitRootSuggestion{
			Path:       s.Path,
			Suggestion: s.Suggestion,
		})
	}
	o.Suggestions = ss

	var ds []*pb.FsoSplitRootDecision
	for _, d := range wf.Decisions() {
		ds = append(ds, &pb.FsoSplitRootDecision{
			Path:     d.Path,
			Decision: d.Decision,
		})
	}
	o.Decisions = ds

	return o, nil
}

func ensureTrailingSlash(s string) string {
	if s == "" {
		return "/"
	}
	if s[len(s)-1] == '/' {
		return s
	}
	return s + "/"
}
