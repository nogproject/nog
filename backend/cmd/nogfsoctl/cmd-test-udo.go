package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
)

var ErrInvalidDomainUser = errors.New("invalid `<user>@<domain>`")

func cmdTestUdo(args map[string]interface{}) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

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

	i := &pb.TestUdoI{}
	action := AAFsoTestUdo
	if asUser, ok := args["--as-user"].(string); ok {
		username, domain, err := parseDomainUser(asUser)
		if err != nil {
			lg.Fatalw("Failed to parse --as-user.", "err", err)
		}
		i.Username = username
		i.Domain = domain
		action = AAFsoTestUdoAs
	}
	globalPath := args["<global-path>"].(string)
	i.GlobalPath = globalPath

	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: action,
		Path:   globalPath,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	c := pb.NewTestUdoClient(conn)
	o, err := c.TestUdo(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	jout := json.NewEncoder(os.Stdout)
	jout.SetEscapeHTML(false)
	fields := []struct {
		k string
		v interface{}
	}{
		{"processUsername", o.ProcessUsername},
		{"mtime", time.Unix(o.Mtime, 0).Format(time.RFC3339)},
		{"mode", os.FileMode(o.Mode).String()},
	}
	for _, f := range fields {
		fmt.Printf("%s: ", f.k)
		if err := jout.Encode(f.v); err != nil {
			lg.Fatalw("Failed to encode JSON.", "err", err)
		}
	}
}

func parseDomainUser(name string) (username, domain string, err error) {
	fields := strings.Split(name, "@")
	if len(fields) != 2 {
		return "", "", ErrInvalidDomainUser
	}
	return fields[0], fields[1], nil
}
