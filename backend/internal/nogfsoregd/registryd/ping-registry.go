package registryd

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/workflows/pingregistrywf"
	"github.com/nogproject/nog/backend/internal/workflows/wfindexes"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) BeginPingRegistry(
	ctx context.Context, i *pb.BeginPingRegistryI,
) (*pb.BeginPingRegistryO, error) {
	registryName := i.Registry
	if err := checkRegistryName(registryName); err != nil {
		return nil, err
	}
	if err := srv.authName(
		ctx, AAFsoAdminRegistry, registryName,
	); err != nil {
		return nil, err
	}

	registry, err := srv.getRegistryState(registryName)
	if err != nil {
		return nil, err
	}
	if i.Vid != nil {
		vid, err := ulid.ParseBytes(i.Vid)
		if err != nil {
			return nil, ErrMalformedVid
		}
		if vid != registry.Vid() {
			return nil, ErrVersionConflict
		}
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

	wfVid, err := srv.pingRegistryWorkflows.Init(
		wfId,
		&pingregistrywf.CmdInit{
			RegistryId: registry.Id(),
		},
	)
	if err != nil {
		return nil, asPingRegistryWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, registryName)
	idxVid, err := srv.workflowIndexes.BeginPingRegistry(
		idxId, wfindexes.NoVC, &wfindexes.CmdBeginPingRegistry{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	regVid := registry.Vid()
	return &pb.BeginPingRegistryO{
		RegistryVid:      regVid[:],
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid[:],
	}, nil
}

func (srv *Server) ServerPing(
	ctx context.Context, i *pb.ServerPingI,
) (*pb.ServerPingO, error) {
	_, workflow, err := srv.authPingRegistryWorkflowId(
		ctx, AAFsoExecPingRegistry, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	workflowId := workflow.Id()

	workflowVid, err := parsePingRegistryVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}
	// `ServerPing()` is likely to be called concurrently by multiple
	// servers.  Retry calls that do not request version control in order
	// to reduce the probability of reporting a version conflict to the
	// caller.
	if workflowVid == pingregistrywf.NoVC {
		workflowVid = pingregistrywf.RetryNoVC
	}

	workflowVid2, err := srv.pingRegistryWorkflows.AppendPing(
		workflowId, workflowVid, i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asPingRegistryWorkflowGrpcError(err)
	}

	return &pb.ServerPingO{
		WorkflowVid: workflowVid2[:],
	}, nil
}

func (srv *Server) PostServerPingSummary(
	ctx context.Context, i *pb.PostServerPingSummaryI,
) (*pb.PostServerPingSummaryO, error) {
	_, workflow, err := srv.authPingRegistryWorkflowId(
		ctx, AAFsoExecPingRegistry, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	workflowId := workflow.Id()

	workflowVid, err := parsePingRegistryVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	workflowVid2, err := srv.pingRegistryWorkflows.PostSummary(
		workflowId, workflowVid, i.StatusCode, i.StatusMessage,
	)
	if err != nil {
		return nil, asPingRegistryWorkflowGrpcError(err)
	}

	return &pb.PostServerPingSummaryO{
		WorkflowVid: workflowVid2[:],
	}, nil
}

func (srv *Server) CommitPingRegistry(
	ctx context.Context, i *pb.CommitPingRegistryI,
) (*pb.CommitPingRegistryO, error) {
	registryName, workflow, err := srv.authPingRegistryWorkflowId(
		ctx, AAFsoAdminRegistry, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	wfId := workflow.Id()

	wfVid, err := parsePingRegistryVid(i.WorkflowVid)
	if err != nil {
		return nil, err
	}

	wfVid2, err := srv.pingRegistryWorkflows.Commit(wfId, wfVid)
	if err != nil {
		return nil, asPingRegistryWorkflowGrpcError(err)
	}

	idxId := srv.names.UUID(NsFsoRegistryEphemeralWorkflows, registryName)
	idxVid, err := srv.workflowIndexes.CommitPingRegistry(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitPingRegistry{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid2,
		},
	)
	if err != nil {
		return nil, asWorkflowIndexGrpcError(err)
	}

	wfVid3, err := srv.pingRegistryWorkflows.End(wfId, wfVid2)
	if err != nil {
		return nil, asPingRegistryWorkflowGrpcError(err)
	}

	return &pb.CommitPingRegistryO{
		WorkflowIndexVid: idxVid[:],
		WorkflowVid:      wfVid3[:],
	}, nil
}

func (srv *Server) GetRegistryPings(
	ctx context.Context, i *pb.GetRegistryPingsI,
) (*pb.GetRegistryPingsO, error) {
	_, workflow, err := srv.authPingRegistryWorkflowId(
		ctx, AAFsoAdminRegistry, i.Workflow,
	)
	if err != nil {
		return nil, err
	}
	workflowId := workflow.Id()

	// If JC_WAIT, wait at least for summary.
	if i.JobControl == pb.JobControl_JC_WAIT {
		// Subscribe and find to ensure that no event is lost.
		updated := make(chan uuid.I, 1)
		srv.ephWorkflowsJ.Subscribe(updated, workflowId)
		defer srv.ephWorkflowsJ.Unsubscribe(updated)

	Loop:
		for {
			w, err := srv.pingRegistryWorkflows.FindId(workflowId)
			if err != nil {
				return nil, asPingRegistryWorkflowGrpcError(err)
			}
			workflow = w

			switch workflow.StateCode() {
			case pingregistrywf.StateUninitialized: // wait
			case pingregistrywf.StateInitialized: // wait
			case pingregistrywf.StateAppending: // wait
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

	workflowVid := workflow.Vid()
	o := &pb.GetRegistryPingsO{
		WorkflowVid: workflowVid[:],
	}
	for _, p := range workflow.Pings() {
		evId := p.EventId // Unalias memory.
		o.ServerPings = append(
			o.ServerPings,
			&pb.GetRegistryPingsO_Status{
				StatusCode:    p.Code,
				StatusMessage: p.Message,
				EventId:       evId[:],
			},
		)
	}
	switch workflow.StateCode() {
	case pingregistrywf.StateUninitialized:
		o.Summary = &pb.GetRegistryPingsO_Status{
			StatusCode:    int32(pb.GetRegistryPingsO_SC_ACTIVE),
			StatusMessage: "uninitialized",
		}

	case pingregistrywf.StateInitialized:
		o.Summary = &pb.GetRegistryPingsO_Status{
			StatusCode:    int32(pb.GetRegistryPingsO_SC_ACTIVE),
			StatusMessage: "initialized",
		}

	case pingregistrywf.StateAppending:
		o.Summary = &pb.GetRegistryPingsO_Status{
			StatusCode:    int32(pb.GetRegistryPingsO_SC_ACTIVE),
			StatusMessage: "appending",
		}

	case pingregistrywf.StateSummarized:
		summary := workflow.SummaryStatus()
		o.Summary = &pb.GetRegistryPingsO_Status{
			StatusCode:    int32(pb.GetRegistryPingsO_SC_SUMMARIZED),
			StatusMessage: summary.Message,
			EventId:       summary.EventId[:],
		}

	default:
		summary := workflow.SummaryStatus()
		o.Summary = &pb.GetRegistryPingsO_Status{
			StatusCode:    summary.Code,
			StatusMessage: summary.Message,
			EventId:       summary.EventId[:],
		}
	}
	return o, nil
}
