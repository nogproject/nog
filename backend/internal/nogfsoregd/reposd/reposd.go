// Package `reposd`: GRPC service `nogfso.Repos` to access the FSO repos.
package reposd

import (
	"context"
	"time"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsorepos"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/internal/workflows/moverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/moveshadowwf"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/gpg"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	NsFsoRepo = "fsorepo"
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
	ctx                 context.Context
	lg                  Logger
	authn               auth.Authenticator
	authz               auth.Authorizer
	names               *shorteruuid.Names
	reposJ              *events.Journal
	repos               *fsorepos.Repos
	workflowsJ          *events.Journal
	moveRepoWorkflows   *moverepowf.Workflows
	moveShadowWorkflows *moveshadowwf.Workflows
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
}

func New(
	ctx context.Context,
	lg Logger,
	authn auth.Authenticator,
	authz auth.Authorizer,
	names *shorteruuid.Names,
	reposJ *events.Journal,
	repos *fsorepos.Repos,
	workflowsJ *events.Journal,
	moveRepoWorkflows *moverepowf.Workflows,
	moveShadowWorkflows *moveshadowwf.Workflows,
) *Server {
	return &Server{
		ctx:                 ctx,
		lg:                  lg,
		authn:               authn,
		authz:               authz,
		names:               names,
		reposJ:              reposJ,
		repos:               repos,
		workflowsJ:          workflowsJ,
		moveRepoWorkflows:   moveRepoWorkflows,
		moveShadowWorkflows: moveShadowWorkflows,
	}
}

func (srv *Server) GetRepo(
	ctx context.Context, req *pb.GetRepoI,
) (*pb.GetRepoO, error) {
	s, err := srv.authRepoIdState(ctx, AAFsoReadRepo, req.Repo)
	if err != nil {
		return nil, err
	}

	vid := s.Vid()
	return &pb.GetRepoO{
		Repo:                   req.Repo,
		Vid:                    vid[:],
		Registry:               s.Registry(),
		GlobalPath:             s.GlobalPath(),
		File:                   s.FileLocation(),
		Shadow:                 s.ShadowLocation(),
		Archive:                s.ArchiveURL(),
		ArchiveRecipients:      s.ArchiveRecipients().Bytes(),
		ShadowBackup:           s.ShadowBackupURL(),
		ShadowBackupRecipients: s.ShadowBackupRecipients().Bytes(),
		StorageTier:            pbStorageTier(s.StorageTier()),
		Gitlab:                 s.GitlabLocation(),
		GitlabProjectId:        s.GitlabProjectId(),
		ErrorMessage:           s.ErrorMessage(),
	}, nil
}

func pbStorageTier(s fsorepos.StorageTierCode) pb.GetRepoO_StorageTierCode {
	switch s {
	case fsorepos.StorageTierUnspecified:
		return pb.GetRepoO_ST_UNSPECIFIED
	case fsorepos.StorageOnline:
		return pb.GetRepoO_ST_ONLINE
	case fsorepos.StorageFrozen:
		return pb.GetRepoO_ST_FROZEN
	case fsorepos.StorageArchived:
		return pb.GetRepoO_ST_ARCHIVED
	case fsorepos.StorageFreezing:
		return pb.GetRepoO_ST_FREEZING
	case fsorepos.StorageFreezeFailed:
		return pb.GetRepoO_ST_FREEZE_FAILED
	case fsorepos.StorageUnfreezing:
		return pb.GetRepoO_ST_UNFREEZING
	case fsorepos.StorageUnfreezeFailed:
		return pb.GetRepoO_ST_UNFREEZE_FAILED
	case fsorepos.StorageArchiving:
		return pb.GetRepoO_ST_ARCHIVING
	case fsorepos.StorageArchiveFailed:
		return pb.GetRepoO_ST_ARCHIVE_FAILED
	case fsorepos.StorageUnarchiving:
		return pb.GetRepoO_ST_UNARCHIVING
	case fsorepos.StorageUnarchiveFailed:
		return pb.GetRepoO_ST_UNARCHIVE_FAILED
	default:
		return pb.GetRepoO_ST_UNSPECIFIED
	}
}

func (srv *Server) ConfirmShadow(
	ctx context.Context, i *pb.ConfirmShadowI,
) (*pb.ConfirmShadowO, error) {
	id, err := srv.authRepoId(ctx, AAFsoConfirmRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.Vid)
	if err != nil {
		return nil, err
	}
	newVid, err := srv.repos.ConfirmShadow(id, vid, i.ShadowPath)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ConfirmShadowO{Vid: newVid[:]}, nil
}

func (srv *Server) PostMoveRepoStaReleased(
	ctx context.Context, i *pb.PostMoveRepoStaReleasedI,
) (*pb.PostMoveRepoStaReleasedO, error) {
	// Same permissions as `ConfirmShadow()`.
	repoId, workflowId, err := srv.authMoveRepoWorkflow(
		ctx, AAFsoConfirmRepo, i.Repo, i.Workflow,
	)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	// Execute the command on the workflow.
	_ = repoId
	newVid, err := srv.moveRepoWorkflows.PostStadReleased(
		workflowId, vid,
	)
	if err != nil {
		return nil, err
	}

	return &pb.PostMoveRepoStaReleasedO{WorkflowVid: newVid[:]}, nil
}

func (srv *Server) PostMoveRepoAppAccepted(
	ctx context.Context, i *pb.PostMoveRepoAppAcceptedI,
) (*pb.PostMoveRepoAppAcceptedO, error) {
	repoId, workflowId, err := srv.authMoveRepoWorkflow(
		ctx, AAFsoConfirmRepo, i.Repo, i.Workflow,
	)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	// Execute the command on the workflow.
	_ = repoId
	newVid, err := srv.moveRepoWorkflows.PostAppAccepted(
		workflowId, vid,
	)
	if err != nil {
		return nil, err
	}

	return &pb.PostMoveRepoAppAcceptedO{WorkflowVid: newVid[:]}, nil
}

func (srv *Server) CommitMoveRepo(
	ctx context.Context, i *pb.CommitMoveRepoI,
) (*pb.CommitMoveRepoO, error) {
	// Same permissions as `ConfirmShadow()`.
	repoId, workflowId, err := srv.authMoveRepoWorkflow(
		ctx, AAFsoConfirmRepo, i.Repo, i.Workflow,
	)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	// Execute the command on the workflow.
	_ = repoId
	newVid, err := srv.moveRepoWorkflows.Commit(
		workflowId, vid, i.NewShadowPath,
	)
	if err != nil {
		return nil, err
	}

	return &pb.CommitMoveRepoO{WorkflowVid: newVid[:]}, nil
}

func (srv *Server) BeginMoveShadow(
	ctx context.Context, i *pb.BeginMoveShadowI,
) (*pb.BeginMoveShadowO, error) {
	id, err := srv.authRepoId(ctx, AAFsoAdminRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.Vid)
	if err != nil {
		return nil, err
	}

	workflowId, err := parseWorkflowId(i.Workflow)
	if err != nil {
		return nil, err
	}

	newVid, err := srv.repos.BeginMoveShadow(
		id, vid, workflowId, i.NewShadowPath,
	)
	if err != nil {
		return nil, err
	}

	return &pb.BeginMoveShadowO{Vid: newVid[:]}, nil
}

func (srv *Server) CommitMoveShadow(
	ctx context.Context, i *pb.CommitMoveShadowI,
) (*pb.CommitMoveShadowO, error) {
	repoId, workflowId, err := srv.authMoveShadowWorkflow(
		ctx, AAFsoAdminRepo, i.Repo, i.Workflow,
	)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	// Execute the command on the workflow.  Nogfsoregd replicate will
	// execute the corresponding command on the repo.
	_ = repoId
	newVid, err := srv.moveShadowWorkflows.Commit(workflowId, vid)
	if err != nil {
		return nil, err
	}

	return &pb.CommitMoveShadowO{WorkflowVid: newVid[:]}, nil
}

func (srv *Server) PostMoveShadowStaDisabled(
	ctx context.Context, i *pb.PostMoveShadowStaDisabledI,
) (*pb.PostMoveShadowStaDisabledO, error) {
	// Same permissions required as for `ConfirmShadow()`.
	repoId, workflowId, err := srv.authMoveShadowWorkflow(
		ctx, AAFsoConfirmRepo, i.Repo, i.Workflow,
	)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	// Execute the command on the workflow.
	_ = repoId
	newVid, err := srv.moveShadowWorkflows.PostStadDisabled(
		workflowId, vid,
	)
	if err != nil {
		return nil, err
	}

	return &pb.PostMoveShadowStaDisabledO{WorkflowVid: newVid[:]}, nil
}

func (srv *Server) InitTartt(
	ctx context.Context, i *pb.InitTarttI,
) (*pb.InitTarttO, error) {
	id, err := srv.authRepoId(ctx, AAFsoInitRepoTartt, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.Vid)
	if err != nil {
		return nil, err
	}

	newVid, err := srv.repos.InitTartt(id, vid, i.TarttUrl)
	if err != nil {
		return nil, err
	}

	return &pb.InitTarttO{Vid: newVid[:]}, nil
}

func (srv *Server) UpdateArchiveRecipients(
	ctx context.Context, i *pb.UpdateArchiveRecipientsI,
) (*pb.UpdateArchiveRecipientsO, error) {
	id, err := srv.authRepoId(ctx, AAFsoAdminRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.RepoVid)
	if err != nil {
		return nil, err
	}

	keys, err := parseGPGFingerprintsBytes(i.ArchiveRecipients)
	if err != nil {
		return nil, err
	}

	repo2, err := srv.repos.UpdateArchiveRecipients(id, vid, keys)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	vid2 := repo2.Vid()
	return &pb.UpdateArchiveRecipientsO{
		RepoVid:           vid2[:],
		ArchiveRecipients: repo2.ArchiveRecipients().Bytes(),
	}, nil
}

func (srv *Server) DeleteArchiveRecipients(
	ctx context.Context, i *pb.DeleteArchiveRecipientsI,
) (*pb.DeleteArchiveRecipientsO, error) {
	id, err := srv.authRepoId(ctx, AAFsoAdminRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.RepoVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.repos.DeleteArchiveRecipients(id, vid)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.DeleteArchiveRecipientsO{
		RepoVid: vid2[:],
	}, nil
}

func (srv *Server) InitShadowBackup(
	ctx context.Context, i *pb.InitShadowBackupI,
) (*pb.InitShadowBackupO, error) {
	id, err := srv.authRepoId(ctx, AAFsoInitRepoShadowBackup, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.Vid)
	if err != nil {
		return nil, err
	}

	newVid, err := srv.repos.InitShadowBackup(id, vid, i.ShadowBackupUrl)
	if err != nil {
		return nil, err
	}

	return &pb.InitShadowBackupO{Vid: newVid[:]}, nil
}

func (srv *Server) MoveShadowBackup(
	ctx context.Context, i *pb.MoveShadowBackupI,
) (*pb.MoveShadowBackupO, error) {
	id, err := srv.authRepoId(ctx, AAFsoAdminRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.Vid)
	if err != nil {
		return nil, err
	}

	newVid, err := srv.repos.MoveShadowBackup(
		id, vid, i.NewShadowBackupUrl,
	)
	if err != nil {
		return nil, err
	}

	return &pb.MoveShadowBackupO{Vid: newVid[:]}, nil
}

func (srv *Server) UpdateShadowBackupRecipients(
	ctx context.Context, i *pb.UpdateShadowBackupRecipientsI,
) (*pb.UpdateShadowBackupRecipientsO, error) {
	id, err := srv.authRepoId(ctx, AAFsoAdminRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.RepoVid)
	if err != nil {
		return nil, err
	}

	keys, err := parseGPGFingerprintsBytes(i.ShadowBackupRecipients)
	if err != nil {
		return nil, err
	}

	repo2, err := srv.repos.UpdateShadowBackupRecipients(id, vid, keys)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	vid2 := repo2.Vid()
	return &pb.UpdateShadowBackupRecipientsO{
		RepoVid:                vid2[:],
		ShadowBackupRecipients: repo2.ShadowBackupRecipients().Bytes(),
	}, nil
}

func (srv *Server) DeleteShadowBackupRecipients(
	ctx context.Context, i *pb.DeleteShadowBackupRecipientsI,
) (*pb.DeleteShadowBackupRecipientsO, error) {
	id, err := srv.authRepoId(ctx, AAFsoAdminRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.RepoVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.repos.DeleteShadowBackupRecipients(id, vid)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.DeleteShadowBackupRecipientsO{
		RepoVid: vid2[:],
	}, nil
}

func (srv *Server) ConfirmGit(
	ctx context.Context, i *pb.ConfirmGitI,
) (*pb.ConfirmGitO, error) {
	id, err := srv.authRepoId(ctx, AAFsoConfirmRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.Vid)
	if err != nil {
		return nil, err
	}
	newVid, err := srv.repos.ConfirmGit(id, vid, i.GitlabProjectId)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ConfirmGitO{Vid: newVid[:]}, nil
}

// Accept a larger limit than nogfsostad sends.
const LimitErrorMessageMaxLength = 200

func (srv *Server) SetRepoError(
	ctx context.Context, i *pb.SetRepoErrorI,
) (*pb.SetRepoErrorO, error) {
	// `AAFsoConfirmRepo` to allow `nogfsoregd` to store error if init
	// shadow or init Git fails.
	id, err := srv.authRepoId(ctx, AAFsoConfirmRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	if len(i.ErrorMessage) > LimitErrorMessageMaxLength {
		err = status.Errorf(
			codes.InvalidArgument, "error message too long",
		)
		return nil, err
	}

	vid, err := parseVid(i.Vid)
	if err != nil {
		return nil, err
	}
	cmd := &fsorepos.CmdSetRepoError{
		ErrorMessage: i.ErrorMessage,
	}
	newVid, err := srv.repos.SetRepoError(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.SetRepoErrorO{Vid: newVid[:]}, nil
}

func (srv *Server) ClearRepoError(
	ctx context.Context, i *pb.ClearRepoErrorI,
) (*pb.ClearRepoErrorO, error) {
	id, err := srv.authRepoId(ctx, AAFsoInitRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	vid, err := parseVid(i.Vid)
	if err != nil {
		return nil, err
	}
	cmd := &fsorepos.CmdClearRepoError{
		ErrorMessage: i.ErrorMessage,
	}
	newVid, err := srv.repos.ClearRepoError(id, vid, cmd)
	if err != nil {
		return nil, asReposGrpcError(err)
	}

	return &pb.ClearRepoErrorO{Vid: newVid[:]}, nil
}

func (srv *Server) Events(
	req *pb.RepoEventsI, stream pb.Repos_EventsServer,
) error {
	// `ctx.Done()` indicates client close, see
	// <https://groups.google.com/d/msg/grpc-io/C0rAhtCUhSs/SzFDLGqiCgAJ>.
	ctx := stream.Context()
	id, err := srv.authRepoId(ctx, AAFsoReadRepo, req.Repo)
	if err != nil {
		return err
	}

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
		srv.reposJ.Subscribe(updated, id)
		defer srv.reposJ.Unsubscribe(updated)

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

		iter := srv.reposJ.Find(id, after)
		var ev fsorepos.Event
		for iter.Next(&ev) {
			after = ev.Id() // Update tail for restart.
			evpb := ev.PbRepoEvent()
			rsp := &pb.RepoEventsO{
				Repo:   req.Repo,
				Events: []*pb.RepoEvent{evpb},
			}
			if err := stream.Send(rsp); err != nil {
				_ = iter.Close()
				return err
			}
		}
		if err := iter.Close(); err != nil {
			// XXX Maybe add more detailed error case handling.
			err := status.Errorf(
				codes.Unknown, "repo journal error: %v", err,
			)
			return err
		}

		if !req.Watch {
			return nil
		}

		rsp := &pb.RepoEventsO{
			Repo:      req.Repo,
			WillBlock: true,
		}
		if err := stream.Send(rsp); err != nil {
			return err
		}
	}
}

func (srv *Server) WorkflowEvents(
	req *pb.RepoWorkflowEventsI, stream pb.Repos_WorkflowEventsServer,
) error {
	// `ctx.Done()` indicates client close, see
	// <https://groups.google.com/d/msg/grpc-io/C0rAhtCUhSs/SzFDLGqiCgAJ>.
	ctx := stream.Context()

	// The initial authorization checks only the repo.  The check whether
	// the repo owns the workflow is deferred until the first workflow
	// event in order to allow watching uninitialized workflows.
	repoId, err := srv.authRepoId(ctx, AAFsoReadRepo, req.Repo)
	if err != nil {
		return err
	}
	workflowId, err := parseWorkflowId(req.Workflow)
	if err != nil {
		return err
	}
	requireWorkflowIdCheck := true

	checkWorkflowId := func(evpb *pb.WorkflowEvent) error {
		if !requireWorkflowIdCheck {
			return nil
		}

		errDeny := status.Error(
			codes.PermissionDenied, "repo does not own workflow",
		)
		if evpb.RepoId == nil {
			return errDeny
		}
		evRepoId, err := parseRepoId(evpb.RepoId)
		if err != nil {
			return err
		}
		if evRepoId != repoId {
			return errDeny
		}

		requireWorkflowIdCheck = false
		return nil
	}

	// `after` is the tail event that has been read from the journal.
	// Reading initially starts with the first event, which is required for
	// the deferred auth check.
	after := events.EventEpoch

	// `sendAfter` determines whether events are sent to the client.  It is
	// initialized with `req.After` and set to `EventEpoch` when the after
	// event has been seen.  Further events are then sent.
	sendAfter := events.EventEpoch
	if req.After != nil {
		a, err := ulid.ParseBytes(req.After)
		if err != nil {
			err := status.Errorf(
				codes.InvalidArgument, "malformed after",
			)
			return err
		}
		sendAfter = a
	}

	updated := make(chan uuid.I, 1)
	updated <- workflowId // Trigger initial Find().

	var ticks <-chan time.Time
	if req.Watch {
		srv.workflowsJ.Subscribe(updated, workflowId)
		defer srv.workflowsJ.Unsubscribe(updated)

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

		iter := srv.workflowsJ.Find(workflowId, after)
		var ev wfevents.Event
		for iter.Next(&ev) {
			after = ev.Id() // Update tail for restart.
			evpb := ev.PbWorkflowEvent()

			if err := checkWorkflowId(evpb); err != nil {
				return err
			}

			if sendAfter != events.EventEpoch {
				if ev.Id() == sendAfter {
					// Further events are sent.
					sendAfter = events.EventEpoch
				}
				continue
			}

			rsp := &pb.RepoWorkflowEventsO{
				Repo:     req.Repo,
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

		rsp := &pb.RepoWorkflowEventsO{
			Repo:      req.Repo,
			Workflow:  req.Workflow,
			WillBlock: true,
		}
		if err := stream.Send(rsp); err != nil {
			return err
		}
	}
}

func parseVid(b []byte) (ulid.I, error) {
	if b == nil {
		return fsorepos.NoVC, nil
	}

	vid, err := ulid.ParseBytes(b)
	if err != nil {
		err := status.Errorf(
			codes.InvalidArgument, "malformed vid: %s", err,
		)
		return ulid.Nil, err
	}

	return vid, nil
}

func asReposGrpcError(err error) error {
	if err == nil {
		return nil
	}
	return status.Errorf(codes.Unknown, "repos error: %v", err)
}

func parseGPGFingerprintsBytes(bs [][]byte) (gpg.Fingerprints, error) {
	ps, err := gpg.ParseFingerprintsBytes(bs...)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", err)
	}
	return ps, nil
}
