package main

import (
	"context"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/parse"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func cmdReinitSubdirTracking(args map[string]interface{}) {
	name, email, err := parse.User(args["--author"].(string))
	if err != nil {
		lg.Fatalw("Invalid author.", "err", err)
	}

	ctx := context.Background()
	timeout, optWait := args["--wait"].(time.Duration)
	if !optWait {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var addr string
	if args["--stad"].(bool) {
		addr = args["--nogfsostad"].(string)
	} else {
		addr = args["--nogfsoregd"].(string)
	}
	conn, err := connect.DialX509(
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

	c := pb.NewStatClient(conn)
	uuI := args["<repoid>"].(uuid.I)
	i := &pb.ReinitSubdirTrackingI{
		Repo:        uuI[:],
		AuthorName:  name,
		AuthorEmail: email,
	}
	if optWait {
		i.JobControl = pb.JobControl_JC_WAIT
	} else {
		i.JobControl = pb.JobControl_JC_BACKGROUND
	}
	if args["enter-subdirs"].(bool) {
		i.SubdirTracking = pb.SubdirTracking_ST_ENTER_SUBDIRS
	} else if args["bundle-subdirs"].(bool) {
		i.SubdirTracking = pb.SubdirTracking_ST_BUNDLE_SUBDIRS
	} else if args["ignore-subdirs"].(bool) {
		i.SubdirTracking = pb.SubdirTracking_ST_IGNORE_SUBDIRS
	} else if args["ignore-most"].(bool) {
		i.SubdirTracking = pb.SubdirTracking_ST_IGNORE_MOST
	} else {
		panic("inconsistent x-subdirs")
	}
	creds, err := connect.GetRPCCredsRepoId(
		ctx, args, AAFsoInitRepo, uuI,
	)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	_, err = c.ReinitSubdirTracking(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
}
