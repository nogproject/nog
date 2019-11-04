package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func cmdLsMetaTree(args map[string]interface{}) {
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
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c := pb.NewGitNogTreeClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	commit := args["<git-commit>"].([]byte)
	i := &pb.ListMetaTreeI{
		Repo:          repoId[:],
		MetaGitCommit: commit[:],
	}
	creds, err := getRPCCredsRepoId(ctx, args, AAFsoReadRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	stream, err := c.ListMetaTree(ctx, i, creds)
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
			printPathMetadata(p)
		}
	}
}

func printPathMetadata(pmd *pb.PathMetadata) {
	m := strings.TrimSpace(string(pmd.MetadataJson))
	fmt.Printf("%s=%s\n", pmd.Path, m)
}
