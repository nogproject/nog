package main

import (
	"context"
	"os"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	yaml "gopkg.in/yaml.v2"
)

type info struct {
	Registry string `yaml:"registry"`
	Vid      string `yaml:"vid"`
	NumRoots int64  `yaml:"numRoots"`
	NumRepos int64  `yaml:"numRepos"`
}

func cmdInfo(args map[string]interface{}) {
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
	req := pb.InfoI{
		Registry: args["<registry>"].(string),
	}
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoReadRegistry,
		Name:   req.Registry,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	rsp, err := c.Info(ctx, &req, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	vid, err := ulid.ParseBytes(rsp.Vid)
	if err != nil {
		lg.Fatalw("Failed to parse Vid.", "err", err)
	}
	buf, err := yaml.Marshal(&info{
		Registry: rsp.Registry,
		Vid:      vid.String(),
		NumRoots: rsp.NumRoots,
		NumRepos: rsp.NumRepos,
	})
	if err != nil {
		lg.Fatalw("YAML marshal failed.", "err", err)
	}

	os.Stdout.Write(buf)
}
