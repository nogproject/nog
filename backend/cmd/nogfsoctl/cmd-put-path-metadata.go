package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/parse"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func cmdPutPathMetadata(args map[string]interface{}) {
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

	uuI := args["<repoid>"].(uuid.I)
	name, email, err := parse.User(args["--author"].(string))
	if err != nil {
		lg.Fatalw("Invalid author.", "err", err)
	}
	i := &pb.PutPathMetadataI{
		Repo:          uuI[:],
		AuthorName:    name,
		AuthorEmail:   email,
		CommitMessage: args["--message"].(string),
	}
	if id, ok := args["--old-commit"].([]byte); ok {
		i.OldGitNogCommit = id
	}
	if id, ok := args["--old-meta-git-commit"].([]byte); ok {
		i.OldMetaGitCommit = id
	}
	for _, arg := range args["<path-metadata>"].([]string) {
		pmd := mustParsePathMetadata(arg)
		i.PathMetadata = append(i.PathMetadata, &pmd)
	}

	creds, err := getRPCCredsRepoId(ctx, args, AAFsoWriteRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.PutPathMetadata(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	uuO, err := uuid.FromBytes(o.Repo)
	if err != nil {
		lg.Fatalw("Invalid UUID.", "err", err)
	}

	fmt.Printf(`repo: "%s"`+"\n", uuO)
	fmt.Printf(`gitNogCommit: "%x"`+"\n", o.GitNogCommit)
	fmt.Printf(`metaGitCommit: "%x"`+"\n", o.GitCommits.Meta)
}

func mustParsePathMetadata(s string) pb.PathMetadata {
	fields := strings.SplitN(s, "=", 2)
	if len(fields) != 2 {
		lg.Fatalw("Invalid <path-metadata>", "arg", s)
	}
	path := fields[0]
	metaBytes := []byte(fields[1])

	var meta interface{}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		lg.Fatalw(
			"Failed to decode <path-metadata> JSON.",
			"err", err,
			"arg", s,
		)
	}

	metadataJson, err := json.Marshal(meta)
	if err != nil {
		lg.Fatalw(
			"Failed to encode <path-metadata> JSON.",
			"err", err,
			"arg", s,
		)
	}

	return pb.PathMetadata{
		Path:         path,
		MetadataJson: metadataJson,
	}
}
