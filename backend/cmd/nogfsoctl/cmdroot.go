package main

import (
	"context"
	"fmt"
	"io"
	slashpath "path"
	"strconv"
	"strings"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/internal/configmap"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/gpg"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"google.golang.org/grpc"
)

func cmdRoot(args map[string]interface{}) {
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
	case args["enable-gitlab"].(bool):
		cmdRootEnableGitlab(args, conn)
	case args["disable-gitlab"].(bool):
		cmdRootDisableGitlab(args, conn)
	case args["find-untracked"].(bool):
		cmdRootFindUntracked(args, conn)
	case args["set-repo-naming"].(bool):
		cmdRootSetRepoNaming(args, conn)
	case args["add-repo-naming-ignore"].(bool):
		cmdRootAddRepoNamingIgnore(args, conn)
	case args["enable-discovery-paths"].(bool):
		cmdEnableDiscoveryPaths(args, conn)
	case args["set-init-policy"].(bool):
		cmdRootSetInitPolicy(args, conn)
	case args["enable-archive-encryption"].(bool):
		cmdRootEnableArchiveEncryption(args, conn)
	case args["disable-archive-encryption"].(bool):
		cmdRootDisableArchiveEncryption(args, conn)
	case args["enable-shadow-backup-encryption"].(bool):
		cmdRootEnableShadowBackupEncryption(args, conn)
	case args["disable-shadow-backup-encryption"].(bool):
		cmdRootDisableShadowBackupEncryption(args, conn)
	}
}

func cmdRootEnableGitlab(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewRegistryClient(conn)
	i := &pb.EnableGitlabRootI{
		Registry:        args["<registry>"].(string),
		GlobalRoot:      slashpath.Clean(args["<root>"].(string)),
		GitlabNamespace: args["<gitlab-namespace>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}
	creds, err := connect.GetRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoInitRoot,
		Path:   i.GlobalRoot,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.EnableGitlabRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.Vid)
}

func cmdRootDisableGitlab(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewRegistryClient(conn)
	i := &pb.DisableGitlabRootI{
		Registry:   args["<registry>"].(string),
		GlobalRoot: slashpath.Clean(args["<root>"].(string)),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}
	creds, err := connect.GetRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoInitRoot,
		Path:   i.GlobalRoot,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.DisableGitlabRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.Vid)
}

func cmdRootFindUntracked(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewDiscoveryClient(conn)
	globalRoot := slashpath.Clean(args["<root>"].(string))
	i := &pb.FindUntrackedI{
		Registry:   args["<registry>"].(string),
		GlobalRoot: globalRoot,
	}
	creds, err := connect.GetRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoFind,
		Path:   i.GlobalRoot,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	stream, err := c.FindUntracked(ctx, i, creds)
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

		for _, p := range o.Candidates {
			p = slashpath.Join(globalRoot, p)
			fmt.Printf("candidate: %s\n", p)
		}
		for _, p := range o.Ignored {
			p = slashpath.Join(globalRoot, p)
			fmt.Printf("ignored: %s\n", p)
		}
	}
}

func cmdRootSetRepoNaming(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewRegistryClient(conn)
	globalRoot := slashpath.Clean(args["<root>"].(string))
	i := &pb.UpdateRepoNamingI{
		Registry: args["<registry>"].(string),
		Naming: &pb.FsoRepoNaming{
			GlobalRoot: globalRoot,
			Rule:       args["<rule>"].(string),
		},
	}
	if cJson, ok := args["<configmap>"].(string); ok {
		cm, err := configmap.NewPbFromJsonString(cJson)
		if err != nil {
			lg.Fatalw("Invalid <configmap>", "err", err)
		}
		i.Naming.Config = cm
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}
	creds, err := connect.GetRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoInitRoot,
		Path:   globalRoot,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.UpdateRepoNaming(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.Vid)
}

func cmdRootAddRepoNamingIgnore(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewRegistryClient(conn)
	globalRoot := slashpath.Clean(args["<root>"].(string))
	patterns := &pb.ConfigField_TextList{
		&pb.StringList{Vals: args["<patterns>"].([]string)},
	}
	config := &pb.ConfigMap{Fields: []*pb.ConfigField{
		&pb.ConfigField{Key: "ignore", Val: patterns},
	}}
	i := &pb.PatchRepoNamingI{
		Registry: args["<registry>"].(string),
		NamingPatch: &pb.FsoRepoNaming{
			GlobalRoot: globalRoot,
			Rule:       args["<rule>"].(string),
			Config:     config,
		},
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}
	creds, err := connect.GetRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoInitRoot,
		Path:   globalRoot,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.PatchRepoNaming(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.Vid)
}

func cmdEnableDiscoveryPaths(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewRegistryClient(conn)
	globalRoot := slashpath.Clean(args["<root>"].(string))
	i := &pb.EnableDiscoveryPathsI{
		Registry:   args["<registry>"].(string),
		GlobalRoot: globalRoot,
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}
	for _, dp := range args["<depth-paths>"].([]string) {
		toks := strings.SplitN(dp, ":", 2)
		if len(toks) != 2 {
			lg.Fatalw("Invalid depth path.", "path", dp)
		}
		depth, err := strconv.ParseInt(toks[0], 10, 32)
		if err != nil || depth < 0 {
			lg.Fatalw("Invalid depth in depth path", "path", dp)
		}
		i.DepthPaths = append(i.DepthPaths, &pb.DepthPath{
			Depth: int32(depth),
			Path:  toks[1],
		})
	}
	creds, err := connect.GetRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoEnableDiscoveryPath,
		Path:   globalRoot,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.EnableDiscoveryPaths(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.Vid)
}

func cmdRootSetInitPolicy(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	globalRoot := slashpath.Clean(args["<root>"].(string))

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	i := &pb.UpdateRepoInitPolicyI{
		Registry: args["<registry>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}
	policy := &pb.FsoRepoInitPolicy{
		GlobalRoot: globalRoot,
		Policy:     pb.FsoRepoInitPolicy_IPOL_SUBDIR_TRACKING_GLOBLIST,
	}
	globArgs := args["<subdir-tracking-globs>"].([][2]string)
	globs := make(
		[]*pb.FsoRepoInitPolicy_SubdirTrackingGlob, 0, len(globArgs),
	)
	for _, glob := range globArgs {
		var st pb.SubdirTracking
		switch glob[0] {
		case "enter-subdirs":
			st = pb.SubdirTracking_ST_ENTER_SUBDIRS
		case "bundle-subdirs":
			st = pb.SubdirTracking_ST_BUNDLE_SUBDIRS
		case "ignore-subdirs":
			st = pb.SubdirTracking_ST_IGNORE_SUBDIRS
		case "ignore-most":
			st = pb.SubdirTracking_ST_IGNORE_MOST
		default:
			panic("invalid <subdir-tracking-globs>")
		}
		globs = append(globs, &pb.FsoRepoInitPolicy_SubdirTrackingGlob{
			Pattern:        glob[1],
			SubdirTracking: st,
		})
	}
	policy.SubdirTrackingGloblist = globs
	i.Policy = policy

	creds, err := connect.GetRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoInitRoot,
		Path:   globalRoot,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	c := pb.NewRegistryClient(conn)
	o, err := c.UpdateRepoInitPolicy(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.Vid)
}

func cmdRootEnableArchiveEncryption(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	globalRoot := slashpath.Clean(args["<root>"].(string))
	scopes := []interface{}{
		auth.SimpleScope{Action: AAFsoAdminRoot, Path: globalRoot},
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
	registry := args["<registry>"].(string)
	want := args["<gpg-keys>"].(gpg.Fingerprints)
	i := &pb.UpdateRootArchiveRecipientsI{
		Registry:          registry,
		GlobalRoot:        globalRoot,
		ArchiveRecipients: want.Bytes(),
	}
	if args["--no-vid"].(bool) {
		i.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.RegistryVid = vid[:]
	}
	o, err := c.UpdateRootArchiveRecipients(ctx, i, creds)
	if err != nil {
		lg.Fatalw("Update failed.", "err", err)
	}
	have, err := gpg.ParseFingerprintsBytes(o.ArchiveRecipients...)
	if err != nil {
		lg.Fatalw(
			"Update returned malformed archive recipients.",
			"err", err,
		)
	}

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
	fmt.Printf("archiveRecipients: %s\n", jsonGPGFingerprints(have))
}

func cmdRootDisableArchiveEncryption(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	globalRoot := slashpath.Clean(args["<root>"].(string))
	scopes := []interface{}{
		auth.SimpleScope{Action: AAFsoAdminRoot, Path: globalRoot},
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
	registry := args["<registry>"].(string)
	i := &pb.DeleteRootArchiveRecipientsI{
		Registry:   registry,
		GlobalRoot: globalRoot,
	}
	if args["--no-vid"].(bool) {
		i.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.RegistryVid = vid[:]
	}
	o, err := c.DeleteRootArchiveRecipients(ctx, i, creds)
	if err != nil {
		lg.Fatalw("Delete failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
}

func cmdRootEnableShadowBackupEncryption(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	globalRoot := slashpath.Clean(args["<root>"].(string))
	scopes := []interface{}{
		auth.SimpleScope{Action: AAFsoAdminRoot, Path: globalRoot},
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
	registry := args["<registry>"].(string)
	want := args["<gpg-keys>"].(gpg.Fingerprints)
	i := &pb.UpdateRootShadowBackupRecipientsI{
		Registry:               registry,
		GlobalRoot:             globalRoot,
		ShadowBackupRecipients: want.Bytes(),
	}
	if args["--no-vid"].(bool) {
		i.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.RegistryVid = vid[:]
	}
	o, err := c.UpdateRootShadowBackupRecipients(ctx, i, creds)
	if err != nil {
		lg.Fatalw("Update failed.", "err", err)
	}
	have, err := gpg.ParseFingerprintsBytes(o.ShadowBackupRecipients...)
	if err != nil {
		lg.Fatalw(
			"Update returned malformed archive recipients.",
			"err", err,
		)
	}

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
	fmt.Printf("shadowBackupRecipients: %s\n", jsonGPGFingerprints(have))
}

func cmdRootDisableShadowBackupEncryption(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	globalRoot := slashpath.Clean(args["<root>"].(string))
	scopes := []interface{}{
		auth.SimpleScope{Action: AAFsoAdminRoot, Path: globalRoot},
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
	registry := args["<registry>"].(string)
	i := &pb.DeleteRootShadowBackupRecipientsI{
		Registry:   registry,
		GlobalRoot: globalRoot,
	}
	if args["--no-vid"].(bool) {
		i.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.RegistryVid = vid[:]
	}
	o, err := c.DeleteRootShadowBackupRecipients(ctx, i, creds)
	if err != nil {
		lg.Fatalw("Delete failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
}
