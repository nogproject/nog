package main

import (
	"context"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/parse"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func cmdRefreshContent(args map[string]interface{}) {
	name, email, err := parse.User(args["--author"].(string))
	if err != nil {
		lg.Fatalw("Invalid author.", "err", err)
	}

	var addr string
	if args["--stad"].(bool) {
		addr = args["--nogfsostad"].(string)
	} else {
		addr = args["--nogfsoregd"].(string)
	}
	conn, err := dialX509(
		addr,
		args["--tls-cert"].(string),
		args["--tls-ca"].(string),
	)
	if err != nil {
		lg.Fatalw("Failed to dial.", "err", err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			lg.Errorw("Failed to close conn.", "err", err)
		}
	}()

	ctx := context.Background()
	timeout, optWait := args["--wait"].(time.Duration)
	if !optWait {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := pb.NewStatClient(conn)
	uuI := args["<repoid>"].(uuid.I)
	req := pb.RefreshContentI{
		Repo:        uuI[:],
		AuthorName:  name,
		AuthorEmail: email,
	}
	if optWait {
		req.JobControl = pb.JobControl_JC_WAIT
	} else {
		req.JobControl = pb.JobControl_JC_BACKGROUND
	}
	creds, err := getRPCCredsRepoId(ctx, args, AAFsoRefreshRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	_, err = c.RefreshContent(ctx, &req, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
}
