package main

import (
	"context"
	"fmt"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func cmdReinit(args map[string]interface{}) {
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
	uuI := args["<repoid>"].(uuid.I)
	i := pb.ReinitRepoI{
		Registry: args["<registry>"].(string),
		Repo:     uuI[:],
		Reason:   args["--reason"].(string),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}
	creds, err := getRPCCredsRepoId(ctx, args, AAFsoInitRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.ReinitRepo(ctx, &i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	vid, err := ulid.ParseBytes(o.Vid)
	if err != nil {
		lg.Fatalw("Malformed response.", "err", err)
	}

	fmt.Printf("registryVid: %s\n", vid)
}
