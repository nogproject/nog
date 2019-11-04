package main

import (
	"context"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
)

func cmdRemoveRoot(args map[string]interface{}) {
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
	i := &pb.RemoveRootI{
		Registry:   args["<registry>"].(string),
		GlobalRoot: args["<root>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}

	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoDeleteRoot,
		Path:   i.GlobalRoot,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.RemoveRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("", o.Vid)
}
