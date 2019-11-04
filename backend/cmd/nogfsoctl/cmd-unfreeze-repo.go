package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/parse"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

func cmdRepoBeginUnfreeze(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewUnfreezeRepoClient(conn)
	registry := args["<registry>"].(string)
	repoId := args["<repoid>"].(uuid.I)
	workflowId := args["--workflow"].(uuid.I)
	name, email, err := parse.User(args["--author"].(string))
	if err != nil {
		lg.Fatalw("Invalid author.", "err", err)
	}
	i := &pb.BeginUnfreezeRepoI{
		Registry:    registry,
		Repo:        repoId[:],
		Workflow:    workflowId[:],
		AuthorName:  name,
		AuthorEmail: email,
	}
	if args["--no-vid"].(bool) {
		i.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.RegistryVid = vid[:]
	}
	if a, ok := args["--repo-vid"].(ulid.I); ok {
		i.RepoVid = a[:]
	}

	creds, err := getRPCCredsRepoId(ctx, args, AAFsoUnfreezeRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.BeginUnfreezeRepo(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
	mustPrintlnVidBytes("repoVid", o.RepoVid)
	mustPrintlnVidBytes("workflowIndexVid", o.WorkflowIndexVid)
	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
}

func cmdRepoGetUnfreeze(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	timeout, optWait := args["--wait"].(time.Duration)
	if !optWait {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// `registry` is currently unused.  But we keep it in case we want to
	// later change auth to use the registry name.
	registry := args["<registry>"].(string)
	_ = registry

	c := pb.NewUnfreezeRepoClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	workflowId := args["<workflowid>"].(uuid.I)
	i := &pb.GetUnfreezeRepoI{
		Workflow: workflowId[:],
	}
	if optWait {
		i.JobControl = pb.JobControl_JC_WAIT
	} else {
		i.JobControl = pb.JobControl_JC_NO_WAIT
	}

	creds, err := getRPCCredsRepoId(ctx, args, AAFsoUnfreezeRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.GetUnfreezeRepo(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
	fmt.Printf("registry: %s\n", o.Registry)
	mustPrintlnUuidBytes("repo", o.RepoId)
	comment := ""
	switch o.StatusCode {
	case int32(pb.StatusCode_SC_OK):
		comment = " # ok"
	case int32(pb.StatusCode_SC_RUNNING):
		comment = " # running"
	case int32(pb.StatusCode_SC_FAILED):
		comment = " # failed"
	}
	fmt.Printf("statusCode: %d%s\n", o.StatusCode, comment)
	fmt.Printf("statusMessage: %s\n", jsonString(o.StatusMessage))
}

func cmdRepoUnfreeze(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	timeout, optWait := args["--wait"].(time.Duration)
	if !optWait {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	repoId := args["<repoid>"].(uuid.I)
	c := pb.NewUnfreezeRepoClient(conn)
	creds, err := getRPCCredsRepoId(ctx, args, AAFsoUnfreezeRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	registry := args["<registry>"].(string)
	workflowId := args["--workflow"].(uuid.I)
	name, email, err := parse.User(args["--author"].(string))
	if err != nil {
		lg.Fatalw("Invalid author.", "err", err)
	}
	beginI := &pb.BeginUnfreezeRepoI{
		Registry:    registry,
		Repo:        repoId[:],
		Workflow:    workflowId[:],
		AuthorName:  name,
		AuthorEmail: email,
	}
	if args["--no-vid"].(bool) {
		beginI.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		beginI.RegistryVid = vid[:]
	}
	if a, ok := args["--repo-vid"].(ulid.I); ok {
		beginI.RepoVid = a[:]
	}
	if _, err := c.BeginUnfreezeRepo(ctx, beginI, creds); err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	getI := &pb.GetUnfreezeRepoI{
		Workflow:   workflowId[:],
		JobControl: pb.JobControl_JC_WAIT,
	}
	o, err := c.GetUnfreezeRepo(ctx, getI, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
	fmt.Printf("registry: %s\n", o.Registry)
	mustPrintlnUuidBytes("repo", o.RepoId)
	comment := ""
	switch o.StatusCode {
	case int32(pb.StatusCode_SC_OK):
		comment = " # ok"
	case int32(pb.StatusCode_SC_RUNNING):
		comment = " # running"
	case int32(pb.StatusCode_SC_FAILED):
		comment = " # failed"
	}
	fmt.Printf("statusCode: %d%s\n", o.StatusCode, comment)
	fmt.Printf("statusMessage: %s\n", jsonString(o.StatusMessage))
}
