package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func cmdStatStatus(args map[string]interface{}) {
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

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c := pb.NewStatClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	i := &pb.StatStatusI{
		Repo: repoId[:],
	}
	creds, err := connect.GetRPCCredsRepoId(
		ctx, args, AAFsoRefreshRepo, repoId,
	)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	stream, err := c.StatStatus(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	for {
		o, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			lg.Fatalw("Failed to read RPC stream.", "err", err)
		}

		for _, p := range o.Paths {
			fmt.Printf(
				"%s %s\n",
				fmtPathStatusStatus(p.Status), p.Path,
			)
		}
	}
}

func fmtPathStatusStatus(st pb.PathStatus_Status) string {
	str, ok := map[pb.PathStatus_Status]string{
		pb.PathStatus_PS_NEW:      "?",
		pb.PathStatus_PS_MODIFIED: "M",
		pb.PathStatus_PS_DELETED:  "D",
	}[st]
	if !ok {
		lg.Fatalw("Invalid PathStatus.")
	}
	return str
}
