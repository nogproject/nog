package main

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func cmdClearError(args map[string]interface{}) {
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

	ctx := context.Background()
	c := pb.NewReposClient(conn)
	uuI := args["<repoid>"].(uuid.I)
	i := pb.ClearRepoErrorI{
		Repo:         uuI[:],
		ErrorMessage: args["<errmsg>"].(string),
	}
	creds, err := getRPCCredsRepoId(ctx, args, AAFsoInitRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	_, err = c.ClearRepoError(ctx, &i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
}
