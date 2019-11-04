package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

func cmdDu(args map[string]interface{}) {
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
	case args["begin"].(bool) && args["root"].(bool):
		cmdDuBeginRoot(args, conn)
	case args["get"].(bool) && args["root"].(bool):
		cmdDuGetRoot(args, conn)
	default:
		lg.Fatalw("Logic error: invalid `du` sub-command.")
	}
}

func cmdDuBeginRoot(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewDiskUsageClient(conn)
	registry := args["<registry>"].(string)
	globalRoot := args["<root>"].(string)
	workflowId := args["--workflow"].(uuid.I)
	i := &pb.BeginDuRootI{
		Registry:   registry,
		GlobalRoot: globalRoot,
		Workflow:   workflowId[:],
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}
	creds, err := connect.GetRPCCredsSimpleAndRepoId(
		ctx, args,
		[]connect.SimpleScope{{
			Action: AAFsoReadRegistry,
			Name:   registry,
		}, {
			Action: AAFsoReadRoot,
			Path:   globalRoot,
		}},
		nil,
	)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.BeginDuRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	mustPrintlnVidBytes("registryVid", o.RegistryVid)
	mustPrintlnVidBytes("workflowIndexVid", o.WorkflowIndexVid)
	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
}

func cmdDuGetRoot(args map[string]interface{}, conn *grpc.ClientConn) {
	optVerbose := args["--verbose"].(bool)

	ctx := context.Background()
	timeout, optWait := args["--wait"].(time.Duration)
	if !optWait {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := pb.NewDiskUsageClient(conn)
	registry := args["<registry>"].(string)
	globalRoot := args["<root>"].(string)
	workflowId := args["--workflow"].(uuid.I)
	i := &pb.GetDuRootI{
		Workflow: workflowId[:],
	}
	if optWait {
		i.JobControl = pb.JobControl_JC_WAIT
	} else {
		i.JobControl = pb.JobControl_JC_BACKGROUND
	}
	creds, err := connect.GetRPCCredsSimpleAndRepoId(
		ctx, args,
		[]connect.SimpleScope{{
			Action: AAFsoReadRegistry,
			Name:   registry,
		}, {
			Action: AAFsoReadRoot,
			Path:   globalRoot,
		}},
		nil,
	)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	stream, err := c.GetDuRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	for {
		o, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			lg.Fatalw("Stream recv failed.", "err", err)
		}
		for _, p := range o.Paths {
			fmt.Printf("%d %s\n", p.Usage, p.Path)
		}
		vid, err := ulid.ParseBytes(o.WorkflowVid)
		if err != nil {
			lg.Fatalw("Failed to parse VID.", "err", err)
		}
		if optVerbose {
			fmt.Printf("# workflowVid: %s\n", vid)
		}
	}
}
