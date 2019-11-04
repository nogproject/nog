package fsoregistry

import (
	"context"
	"errors"
	slashpath "path"
	"strings"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

type Preconditions struct {
	InitRepo        InitRepoAllower
	WorkflowIdCheck WorkflowIdChecker
}

type InitRepoAllower interface {
	// `IsInitRepoAllowed()` returns `deny="", err=nil` if the operation is
	// allowed to proceed.  Otherwise, it returns a `deny` reason that
	// explains why the operation cannot proceed, or an error if the check
	// could not be performed due to a network error, for example.
	//
	// If the implementation uses outgoing GRPC calls that require
	// authorization, `ctx` must have the required metadata attached with
	// `NewOutgoingContext()`.
	IsInitRepoAllowed(
		ctx context.Context,
		repo, host, hostPath string,
		subdirTracking pb.SubdirTracking,
	) (deny string, err error)
}

type WorkflowIdChecker interface {
	IsUnusedWorkflowId(uuid.I) (deny string, err error)
}

func (p *Preconditions) isInitRepoAllowed(
	ctx context.Context,
	repo, host, hostPath string,
	subdirTracking pb.SubdirTracking,
) (string, error) {
	if p.InitRepo == nil {
		return "", nil
	}
	return p.InitRepo.IsInitRepoAllowed(
		ctx,
		repo, host, hostPath,
		subdirTracking,
	)
}

func (p *Preconditions) isAllowedAsUnusedWorkflowId(
	id uuid.I,
) (string, error) {
	if p.WorkflowIdCheck == nil {
		return "", nil
	}
	return p.WorkflowIdCheck.IsUnusedWorkflowId(id)
}

func WhichSubdirTracking(
	pol *pb.FsoRepoInitPolicy, globalPath string,
) (pb.SubdirTracking, error) {
	// Before policy was added, the default had been enter-subdirs.
	if pol == nil {
		return pb.SubdirTracking_ST_ENTER_SUBDIRS, nil
	}

	if !strings.HasPrefix(globalPath, pol.GlobalRoot) {
		err := errors.New("path not equal or below root")
		return pb.SubdirTracking_ST_UNSPECIFIED, err
	}

	switch pol.Policy {
	case pb.FsoRepoInitPolicy_IPOL_SUBDIR_TRACKING_GLOBLIST:
		return whichSubdirTrackingGloblist(pol, globalPath), nil
	default:
		err := errors.New("unknown policy")
		return pb.SubdirTracking_ST_UNSPECIFIED, err
	}
}

func whichSubdirTrackingGloblist(
	pol *pb.FsoRepoInitPolicy, globalPath string,
) pb.SubdirTracking {
	// Be careful to handle repo `globalPath` equals `GlobalRoot`.  A
	// precondition is that `globalPath` is equal or below `GlobalRoot`.
	// So we can apply `TrimPrefix()` without trailing slash on
	// `GlobalRoot` to handle equal paths.
	relpath := strings.TrimPrefix(globalPath, pol.GlobalRoot)
	if relpath == "" {
		relpath = "."
	} else {
		relpath = strings.TrimLeft(relpath, "/")
	}

	for _, g := range pol.SubdirTrackingGloblist {
		matched, err := slashpath.Match(g.Pattern, relpath)
		if err != nil {
			continue // Silently ignore invalid patterns.
		}
		if matched {
			return g.SubdirTracking
		}
	}

	// The default is ignore-most, since it is safe with large trees.
	return pb.SubdirTracking_ST_IGNORE_MOST
}
