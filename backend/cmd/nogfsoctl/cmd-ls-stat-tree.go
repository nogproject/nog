package main

import (
	"context"
	"fmt"
	"io"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/gitstat"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func cmdLsStatTree(args map[string]interface{}) {
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
	i := &pb.ListStatTreeI{
		Repo:          repoId[:],
		StatGitCommit: commit[:],
	}
	if pfx, ok := args["<prefix>"].(string); ok {
		i.Prefix = pfx
	}
	creds, err := getRPCCredsRepoId(ctx, args, AAFsoReadRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	stream, err := c.ListStatTree(ctx, i, creds)
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
			printPathInfo(p)
		}
	}
}

func printPathInfo(inf *pb.PathInfo) {
	fmtPath := func(path string, m gitstat.Mode) string {
		switch {
		case m.IsDir():
			if path != "." {
				path = path + "/"
			}
		}
		return path
	}

	mode := gitstat.Mode(inf.Mode)
	ty := mode.Filetype()
	path := fmtPath(inf.Path, mode)
	size := inf.Size
	mtime := time.Unix(inf.Mtime, 0).Format(time.RFC3339)

	var details string
	switch {
	case mode.IsSymlink():
		details = fmt.Sprintf("\t -> %s", inf.Symlink)
	case mode.IsGitlink():
		details = fmt.Sprintf("\t -> %x", inf.Gitlink)
	}

	// `%13d` width overflows at 10 TB.
	fmt.Printf(
		"%s %13d %s\t%s%s\n",
		ty, size, mtime, path, details,
	)
}
