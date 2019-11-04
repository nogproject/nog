package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	slashpath "path"
	"strings"
	"time"

	"github.com/nogproject/nog/backend/internal/fsoauthz"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/gpg"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	yaml "gopkg.in/yaml.v2"
)

func cmdGet(args map[string]interface{}) {
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
	switch {
	case args["registries"].(bool):
		cmdGetRegistries(args, conn)
	case args["roots"].(bool):
		cmdGetRoots(args, conn)
	case args["root"].(bool):
		cmdGetRoot(args, conn)
	case args["repos"].(bool):
		cmdGetRepos(args, conn)
	case args["repo"].(bool):
		cmdGetRepo(args, conn)
	}
}

// Avoid YAML flow, because it breaks long lines in an uncontrolled way.  Use
// JSON lines instead.

type RegistriesHeader struct {
	Main string `yaml:"main"`
	Vid  string `yaml:"vid"`
}

type Registry struct {
	Name      string `json:"name"`
	Confirmed bool   `json:"confirmed"`
}

func cmdGetRegistries(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	c := pb.NewMainClient(conn)
	req := pb.GetRegistriesI{}
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: fsoauthz.AAFsoReadMain,
		Name:   "main",
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	rsp, err := c.GetRegistries(ctx, &req, creds)
	if err != nil {
		logFatalRPC(lg, err)
	}

	vid, err := ulid.ParseBytes(rsp.Vid)
	if err != nil {
		lg.Fatalw("Failed to parse Vid.", "err", err)
	}
	rs := RegistriesHeader{
		Main: rsp.Main,
		Vid:  vid.String(),
	}
	buf, err := yaml.Marshal(&rs)
	if err != nil {
		lg.Fatalw("YAML marshal failed.", "err", err)
	}
	os.Stdout.Write(buf)

	if len(rsp.Registries) == 0 {
		fmt.Println("registries: []")
		return
	}

	fmt.Println("registries:")
	jout := json.NewEncoder(os.Stdout)
	jout.SetEscapeHTML(false)
	for _, pbr := range rsp.Registries {
		r := Registry{
			Name:      pbr.Name,
			Confirmed: pbr.Confirmed,
		}
		os.Stdout.Write([]byte("- "))
		if err := jout.Encode(&r); err != nil {
			lg.Fatalw("JSON marshal failed.", "err", err)
		}
	}
}

type RootsHeader struct {
	Registry string `yaml:"registry"`
	Vid      string `yaml:"vid"`
}

type Root struct {
	GlobalRoot      string `json:"globalRoot"`
	Host            string `json:"host"`
	HostRoot        string `json:"hostRoot"`
	GitlabNamespace string `json:"gitlabNamespace,omitempty"`
}

func cmdGetRoots(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	c := pb.NewRegistryClient(conn)
	req := pb.GetRootsI{
		Registry: args["<registry>"].(string),
	}
	// XXX Use 3-round-trip auth flow that determines scope from GRPC
	// status details for illustration.  It should perhaps be changed to
	// the 2-round-trip flow.
	rsp, err := c.GetRoots(ctx, &req)
	if sc, ok := authScopeFromErr(err); ok {
		creds, err2 := getRPCCredsScope(ctx, args, *sc)
		if err2 != nil {
			lg.Fatalw("Failed to get RPC creds.", "err", err2)
		}
		rsp, err = c.GetRoots(ctx, &req, creds)
	}
	if err != nil {
		logFatalRPC(lg, err)
	}

	vid, err := ulid.ParseBytes(rsp.Vid)
	if err != nil {
		lg.Fatalw("Failed to parse Vid.", "err", err)
	}
	rs := RootsHeader{
		Registry: rsp.Registry,
		Vid:      vid.String(),
	}
	buf, err := yaml.Marshal(&rs)
	if err != nil {
		lg.Fatalw("YAML marshal failed.", "err", err)
	}
	os.Stdout.Write(buf)

	if len(rsp.Roots) == 0 {
		fmt.Println("roots: []")
		return
	}

	fmt.Println("roots:")
	jout := json.NewEncoder(os.Stdout)
	jout.SetEscapeHTML(false)
	for _, pbr := range rsp.Roots {
		r := Root{
			GlobalRoot:      pbr.GlobalRoot,
			Host:            pbr.Host,
			HostRoot:        pbr.HostRoot,
			GitlabNamespace: pbr.GitlabNamespace,
		}
		os.Stdout.Write([]byte("- "))
		if err := jout.Encode(&r); err != nil {
			lg.Fatalw("JSON marshal failed.", "err", err)
		}
	}
}

func cmdGetRoot(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	registry := args["<registry>"].(string)
	globalRoot := slashpath.Clean(args["<root>"].(string))
	scopes := []interface{}{
		auth.SimpleScope{Action: AAFsoReadRoot, Path: globalRoot},
	}
	creds, err := getRPCCredsScopes(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}

	c := pb.NewRegistryClient(conn)
	i := &pb.GetRootI{
		Registry:   registry,
		GlobalRoot: globalRoot,
	}
	o, err := c.GetRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("Get failed.", "err", err)
	}

	archiveRecipients, err := gpg.ParseFingerprintsBytes(
		o.Root.ArchiveRecipients...,
	)
	if err != nil {
		lg.Fatalw(
			"Update returned malformed archive recipients.",
			"err", err,
		)
	}

	shadowBackupRecipients, err := gpg.ParseFingerprintsBytes(
		o.Root.ShadowBackupRecipients...,
	)
	if err != nil {
		lg.Fatalw(
			"Update returned malformed shadow backup recipients.",
			"err", err,
		)
	}

	fmtKV := func(k, v string) {
		fmt.Printf("%s: %s\n", k, v)
	}

	fmtKV("registry", jsonString(o.Registry))
	mustPrintlnVidBytes("registryVid", o.RegistryVid)
	fmtKV("globalRoot", jsonString(o.Root.GlobalRoot))
	fmtKV("host", jsonString(o.Root.Host))
	fmtKV("hostRoot", jsonString(o.Root.HostRoot))
	fmtKV("gitlabNamespace", jsonString(o.Root.GitlabNamespace))
	fmtKV("archiveRecipients", jsonGPGFingerprints(archiveRecipients))
	fmtKV("shadowBackupRecipients", jsonGPGFingerprints(shadowBackupRecipients))
}

type ReposHeader struct {
	Registry string `yaml:"registry"`
	Vid      string `yaml:"vid"`
}

type RepoShort struct {
	Id         string `json:"id"`
	GlobalPath string `json:"globalPath"`
	Confirmed  bool   `json:"confirmed"`
}

func cmdGetRepos(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	c := pb.NewRegistryClient(conn)
	req := pb.GetReposI{
		Registry: args["<registry>"].(string),
	}
	if arg, ok := args["--global-path-prefix"].(string); ok {
		req.GlobalPathPrefix = strings.TrimRight(arg, "/")
	}
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: fsoauthz.AAFsoReadRegistry,
		Name:   req.Registry,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	rsp, err := c.GetRepos(ctx, &req, creds)
	if err != nil {
		logFatalRPC(lg, err)
	}

	vid, err := ulid.ParseBytes(rsp.Vid)
	if err != nil {
		lg.Fatalw("Failed to parse Vid.", "err", err)
	}
	rs := ReposHeader{
		Registry: rsp.Registry,
		Vid:      vid.String(),
	}
	buf, err := yaml.Marshal(&rs)
	if err != nil {
		lg.Fatalw("YAML marshal failed.", "err", err)
	}
	os.Stdout.Write(buf)

	if len(rsp.Repos) == 0 {
		fmt.Println("repos: []")
		return
	}

	fmt.Println("repos:")
	jout := json.NewEncoder(os.Stdout)
	jout.SetEscapeHTML(false)
	for _, r := range rsp.Repos {
		uu, err := uuid.FromBytes(r.Id)
		if err != nil {
			lg.Fatalw("Invalid UUID.", "err", err)
		}
		r := RepoShort{
			Id:         uu.String(),
			GlobalPath: r.GlobalPath,
			Confirmed:  r.Confirmed,
		}
		os.Stdout.Write([]byte("- "))
		if err := jout.Encode(&r); err != nil {
			lg.Fatalw("JSON marshal failed.", "err", err)
		}
	}
}

type Repo struct {
	Repo                   string   `json:"repo"`
	Vid                    string   `json:"vid"`
	Registry               string   `json:"registry"`
	GlobalPath             string   `json:"globalPath,omitempty"`
	File                   string   `json:"file,omitempty"`
	Shadow                 string   `json:"shadow,omitempty"`
	Archive                string   `json:"archive,omitempty"`
	ArchiveRecipients      []string `json:"archiveRecipients,omitempty"`
	ShadowBackup           string   `json:"shadowBackup,omitempty"`
	ShadowBackupRecipients []string `json:"shadowBackupRecipients,omitempty"`
	StorageTier            string   `json:"storageTier"`
	Gitlab                 string   `json:"gitlab,omitempty"`
	GitlabProjectId        int64    `json:"gitlabProjectId,omitempty"`
	ErrorMessage           string   `json:"error,omitempty"`
}

func cmdGetRepo(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	c := pb.NewReposClient(conn)
	uuI := args["<repoid>"].(uuid.I)
	creds, err := getRPCCredsRepoId(ctx, args, fsoauthz.AAFsoReadRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	req := pb.GetRepoI{
		Repo: uuI[:],
	}
	rsp, err := c.GetRepo(ctx, &req, creds)
	if err != nil {
		logFatalRPC(lg, err)
	}
	uuO, err := uuid.FromBytes(rsp.Repo)
	if err != nil {
		lg.Fatalw("Failed to parse response repo id.", "err", err)
	}
	vid, err := ulid.ParseBytes(rsp.Vid)
	if err != nil {
		lg.Fatalw("Failed to parse vid.", "err", err)
	}

	// Encode as JSON.  It does not wrap long lines and, therefore, is more
	// useful on the command line than `yaml.v2`, which wraps long lines in
	// an unpredictable way.
	jout := json.NewEncoder(os.Stdout)
	jout.SetEscapeHTML(false)
	jout.SetIndent("", "  ")
	r := Repo{
		Repo:                   uuO.String(),
		Vid:                    vid.String(),
		Registry:               rsp.Registry,
		GlobalPath:             rsp.GlobalPath,
		File:                   rsp.File,
		Shadow:                 rsp.Shadow,
		Archive:                rsp.Archive,
		ArchiveRecipients:      asHexStrings(rsp.ArchiveRecipients),
		ShadowBackup:           rsp.ShadowBackup,
		ShadowBackupRecipients: asHexStrings(rsp.ShadowBackupRecipients),
		StorageTier:            rsp.StorageTier.String(),
		Gitlab:                 rsp.Gitlab,
		GitlabProjectId:        rsp.GitlabProjectId,
		ErrorMessage:           rsp.ErrorMessage,
	}
	if err := jout.Encode(&r); err != nil {
		lg.Fatalw("JSON marshal failed.", "err", err)
	}
}

func asHexStrings(ds [][]byte) []string {
	ss := make([]string, 0, len(ds))
	for _, d := range ds {
		ss = append(ss, fmt.Sprintf("%X", d))
	}
	return ss
}
