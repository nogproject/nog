package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

func cmdPingRegistry(args map[string]interface{}) {
	conn, err := connect.DialX509(
		args["--nogfsoregd"].(string),
		args["--tls-cert"].(string),
		args["--tls-ca"].(string),
	)
	if err != nil {
		lg.Fatalw("Failed to dial nogfsoregd.", "err", err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			lg.Errorw("Failed to close conn.", "err", err)
		}
	}()

	switch {
	case args["begin"].(bool):
		cmdPingRegistryBegin(args, conn)
	case args["commit"].(bool):
		cmdPingRegistryCommit(args, conn)
	case args["get"].(bool):
		cmdPingRegistryGet(args, conn)
	default:
		lg.Fatalw("Logic error: invalid `ping-registry` sub-command.")
	}
}

func cmdPingRegistryBegin(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewPingRegistryClient(conn)
	registry := args["<registry>"].(string)
	workflowId := args["--workflow"].(uuid.I)
	i := &pb.BeginPingRegistryI{
		Registry: registry,
		Workflow: workflowId[:],
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoAdminRegistry,
		Name:   registry,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.BeginPingRegistry(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	mustPrintlnVidBytes("registryVid", o.RegistryVid)
	mustPrintlnVidBytes("workflowIndexVid", o.WorkflowIndexVid)
	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
}

func cmdPingRegistryCommit(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewPingRegistryClient(conn)
	registry := args["<registry>"].(string)
	workflowId := args["--workflow"].(uuid.I)
	i := &pb.CommitPingRegistryI{
		Workflow: workflowId[:],
	}
	if args["--no-vid"].(bool) {
		i.WorkflowVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.WorkflowVid = vid[:]
	}
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoAdminRegistry,
		Name:   registry,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.CommitPingRegistry(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	mustPrintlnVidBytes("workflowIndexVid", o.WorkflowIndexVid)
	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
}

func cmdPingRegistryGet(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	timeout, optWait := args["--wait"].(time.Duration)
	if !optWait {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := pb.NewPingRegistryClient(conn)
	registry := args["<registry>"].(string)
	workflowId := args["--workflow"].(uuid.I)
	i := &pb.GetRegistryPingsI{
		Workflow: workflowId[:],
	}
	if optWait {
		i.JobControl = pb.JobControl_JC_WAIT
	} else {
		i.JobControl = pb.JobControl_JC_NO_WAIT
	}
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoAdminRegistry,
		Name:   registry,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	o, err := c.GetRegistryPings(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)

	if len(o.ServerPings) > 0 {
		fmt.Println("pings:")
	} else {
		fmt.Println("pings: []")
	}
	for _, p := range o.ServerPings {
		evId, err := ulid.ParseBytes(p.EventId)
		if err != nil {
			lg.Fatalw(
				"Failed to parse ping event ID.",
				"err", err,
			)
		}
		fmt.Printf(
			" - { etime: %s, code: %d, message: \"%s\" }\n",
			ulid.TimeString(evId),
			p.StatusCode, p.StatusMessage,
		)
	}

	if o.Summary.EventId != nil {
		evId, err := ulid.ParseBytes(o.Summary.EventId)
		if err != nil {
			lg.Fatalw(
				"Failed to parse ping event ID.",
				"err", err,
			)
		}
		fmt.Printf("statusEtime: %s\n", ulid.TimeString(evId))
	}
	fmt.Printf("statusCode: %d\n", o.Summary.StatusCode)
	comment := ""
	switch o.Summary.StatusCode {
	case int32(pb.GetRegistryPingsO_SC_SUMMARIZED):
		comment = " # uncommitted"
	}
	fmt.Printf(
		"statusMessage: %s%s\n",
		o.Summary.StatusMessage, comment,
	)
}
