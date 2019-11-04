package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	slashpath "path"
	"sort"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

func cmdTartt(args map[string]interface{}) {
	conn, err := connect.DialX509(
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

	switch {
	case args["head"].(bool):
		cmdTarttHead(args, conn)
	case args["ls"].(bool):
		cmdTarttLs(args, conn)
	case args["config"].(bool):
		cmdTarttConfig(args, conn)
	}
}

func cmdTarttHead(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewTarttClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	i := &pb.TarttHeadI{
		Repo: repoId[:],
	}
	creds, err := getRPCCredsRepoId(ctx, args, AAFsoReadRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.TarttHead(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	repoIdO, err := uuid.FromBytes(o.Repo)
	if err != nil {
		lg.Fatalw("Invalid UUID.", "err", err)
	}
	var commitHex string
	if o.Commit == nil {
		commitHex = "0000000000000000000000000000000000000000"
	} else {
		commitHex = hex.EncodeToString(o.Commit)
	}

	jout := json.NewEncoder(os.Stdout)
	jout.SetEscapeHTML(false)
	fields := []struct {
		k string
		v interface{}
	}{
		{"repo", repoIdO.String()},
		{"commit", commitHex},
		{"author", o.Author},
		{"committer", o.Committer},
	}
	for _, f := range fields {
		fmt.Printf("%s: ", f.k)
		if err := jout.Encode(f.v); err != nil {
			lg.Fatalw("Failed to encode JSON.", "err", err)
		}
	}
}

func cmdTarttLs(args map[string]interface{}, conn *grpc.ClientConn) {
	optVerbose := args["--verbose"].(bool)
	optSha := args["--sha"].(bool)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewTarttClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	i := &pb.ListTarsI{
		Repo: repoId[:],
	}
	if commit, ok := args["<git-commit>"].([]byte); ok {
		i.Commit = commit[:]
	}
	creds, err := getRPCCredsRepoId(ctx, args, AAFsoReadRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.ListTars(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	tars := o.Tars

	if optVerbose {
		repoIdO, err := uuid.FromBytes(o.Repo)
		if err != nil {
			lg.Fatalw("Invalid UUID.", "err", err)
		}

		jout := json.NewEncoder(os.Stdout)
		jout.SetEscapeHTML(false)
		fields := []struct {
			k string
			v interface{}
		}{
			{"repo", repoIdO.String()},
			{"commit", hex.EncodeToString(o.Commit)},
			{"author", o.Author},
			{"committer", o.Committer},
		}
		for _, f := range fields {
			fmt.Printf("# %s: ", f.k)
			if err := jout.Encode(f.v); err != nil {
				lg.Fatalw("Failed to encode JSON.", "err", err)
			}
		}
	}

	sort.Slice(tars, func(i, j int) bool {
		return tars[i].Time < tars[j].Time
	})
	for _, inf := range tars {
		ty, ok := map[pb.TarInfo_TarType]string{
			pb.TarInfo_TAR_FULL:  "full",
			pb.TarInfo_TAR_PATCH: "patch",
		}[inf.TarType]
		if !ok {
			ty = "invalid"
		}
		timeString := time.Unix(inf.Time, 0).Format(time.RFC3339)

		const format = "%5s %12d %s\t%s\n"
		fmt.Printf(
			format,
			ty, len(inf.Manifest), timeString, inf.Path,
		)
		for _, m := range inf.Manifest {
			path := slashpath.Join(inf.Path, m.File)
			if optVerbose {
				fmt.Printf(
					format,
					"file", m.Size, timeString, path,
				)
			}
			if optSha {
				fmt.Printf("sha256:%x  %s\n", m.Sha256, path)
				fmt.Printf("sha512:%x  %s\n", m.Sha512, path)
			}
		}
	}
}

func cmdTarttConfig(args map[string]interface{}, conn *grpc.ClientConn) {
	optVerbose := args["--verbose"].(bool)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewTarttClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	i := &pb.GetTarttconfigI{
		Repo: repoId[:],
	}
	if commit, ok := args["<git-commit>"].([]byte); ok {
		i.Commit = commit[:]
	}
	creds, err := getRPCCredsRepoId(ctx, args, AAFsoReadRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.GetTarttconfig(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	if optVerbose {
		oRepoId, err := uuid.FromBytes(o.Repo)
		if err != nil {
			lg.Fatalw("Invalid UUID.", "err", err)
		}

		jout := json.NewEncoder(os.Stdout)
		jout.SetEscapeHTML(false)
		fields := []struct {
			k string
			v interface{}
		}{
			{"repo", oRepoId.String()},
			{"commit", hex.EncodeToString(o.Commit)},
			{"author", o.Author},
			{"committer", o.Committer},
		}
		for _, f := range fields {
			fmt.Printf("# %s: ", f.k)
			if err := jout.Encode(f.v); err != nil {
				lg.Fatalw("Failed to encode JSON.", "err", err)
			}
		}
	}
	fmt.Printf("%s", o.ConfigYaml)
}
