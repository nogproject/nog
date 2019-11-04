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

func cmdRegistry(args map[string]interface{}) {
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
	case args["enable-ephemeral-workflows"].(bool):
		cmdRegistryEnableEphemeralWorkflows(args, conn)
	case args["enable-propagate-root-acls"].(bool):
		cmdRegistryEnablePropagateRootAcls(args, conn)
	}
}

func cmdRegistryEnableEphemeralWorkflows(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewRegistryClient(conn)
	i := &pb.EnableEphemeralWorkflowsI{
		Registry: args["<registry>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}

	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoInitRegistry,
		Name:   i.Registry,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.EnableEphemeralWorkflows(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.Vid)
	mustPrintlnUuidBytes("ephemeralWorkflowsId", o.EphemeralWorkflowsId)
}

func cmdRegistryEnablePropagateRootAcls(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewRegistryClient(conn)
	i := &pb.EnablePropagateRootAclsI{
		Registry: args["<registry>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.RegistryVid = vid[:]
	}

	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoInitRegistry,
		Name:   i.Registry,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.EnablePropagateRootAcls(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
}

func mustPrintlnUuidBytes(field string, b []byte) {
	id, err := uuid.FromBytes(b)
	if err != nil {
		lg.Fatalw("Malformed response.", "err", err)
	}
	fmt.Printf("%s: %s\n", field, id)
}
