package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/parse"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func cmdInit(args map[string]interface{}) {
	switch {
	case args["registry"].(bool):
		cmdInitRegistry(args)
	case args["root"].(bool):
		cmdInitRoot(args)
	case args["repo"].(bool):
		cmdInitRepo(args)
	}
}

func cmdInitRegistry(args map[string]interface{}) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := dialX509(
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

	c := pb.NewRegistryClient(conn)
	i := pb.InitRegistryI{
		Registry: args["<registry>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.MainVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.MainVid = vid[:]
	}
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoInitRegistry,
		Name:   i.Registry,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.InitRegistry(ctx, &i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("mainVid", o.MainVid)
}

func cmdInitRoot(args map[string]interface{}) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := dialX509(
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

	c := pb.NewRegistryClient(conn)
	i := &pb.InitRootI{
		Registry:   args["<registry>"].(string),
		Host:       args["--host"].(string),
		GlobalRoot: args["<root>"].(string),
	}

	if arg, ok := args["<host-root>"].(string); ok {
		i.HostRoot = arg
	} else {
		i.HostRoot = i.GlobalRoot
	}

	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}

	// No `--gitlab-namespace` indicates that the repo will be managed only
	// locally by nogfsostad.
	gns := args["--gitlab-namespace"]
	if gns != nil {
		i.GitlabNamespace = gns.(string)
	}

	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoInitRoot,
		Path:   i.GlobalRoot,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.InitRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("", o.Vid)
}

func cmdInitRepo(args map[string]interface{}) {
	name, email, err := parse.User(args["--author"].(string))
	if err != nil {
		lg.Fatalw("Invalid author.", "err", err)
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := dialX509(
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

	c := pb.NewRegistryClient(conn)
	i := pb.InitRepoI{
		Registry:     args["<registry>"].(string),
		GlobalPath:   args["<repo>"].(string),
		CreatorName:  name,
		CreatorEmail: email,
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}
	if a, ok := args["--uuid"].(uuid.I); ok {
		i.RepoId = a[:]
	}

	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoInitRepo,
		Path:   i.GlobalPath,
	})
	if err != nil {
		lg.Fatalw("Failed to get RPC creds.", "err", err)
	}
	o, err := c.InitRepo(ctx, &i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	vid, err := ulid.ParseBytes(o.Vid)
	if err != nil {
		lg.Fatalw("Malformed response.", "err", err)
	}
	id, err := uuid.FromBytes(o.Repo)
	if err != nil {
		lg.Fatalw("Failed to parse response.", "err", err)
	}

	fmt.Printf("registryVid: %s\nrepo: %s\n", vid, id)
}

func mustPrintlnVidBytes(field string, b []byte) {
	mustPrintlnULIDBytes(field, b)
}

func mustPrintlnULIDBytes(field string, b []byte) {
	vid, err := ulid.ParseBytes(b)
	if err != nil {
		lg.Fatalw("Malformed response.", "err", err)
	}
	if field == "" {
		fmt.Printf("%s\n", vid)
	} else {
		fmt.Printf("%s: %s\n", field, vid)
	}
}
