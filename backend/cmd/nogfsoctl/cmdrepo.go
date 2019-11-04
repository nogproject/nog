package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/jwtauth"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/gpg"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

func cmdRepo(args map[string]interface{}) {
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
		cmdRepoEnableGitlab(args, conn)
	case args["init-tartt"].(bool):
		cmdRepoInitTartt(args, conn)
	case args["init-shadow-backup"].(bool):
		cmdRepoInitShadowBackup(args, conn)
	case args["move-shadow-backup"].(bool):
		cmdRepoMoveShadowBackup(args, conn)
	case args["enable-archive-encryption"].(bool):
		cmdRepoEnableArchiveEncryption(args, conn)
	case args["disable-archive-encryption"].(bool):
		cmdRepoDisableArchiveEncryption(args, conn)
	case args["enable-shadow-backup-encryption"].(bool):
		cmdRepoEnableShadowBackupEncryption(args, conn)
	case args["disable-shadow-backup-encryption"].(bool):
		cmdRepoDisableShadowBackupEncryption(args, conn)
	case args["begin-move-repo"].(bool):
		cmdBeginMoveRepo(args, conn)
	case args["begin-move-shadow"].(bool):
		cmdBeginMoveShadow(args, conn)
	case args["commit-move-shadow"].(bool):
		cmdCommitMoveShadow(args, conn)
	case args["begin-freeze"].(bool):
		cmdRepoBeginFreeze(args, conn)
	case args["get-freeze"].(bool):
		cmdRepoGetFreeze(args, conn)
	case args["freeze"].(bool):
		cmdRepoFreeze(args, conn)
	case args["begin-unfreeze"].(bool):
		cmdRepoBeginUnfreeze(args, conn)
	case args["get-unfreeze"].(bool):
		cmdRepoGetUnfreeze(args, conn)
	case args["unfreeze"].(bool):
		cmdRepoUnfreeze(args, conn)
	case args["begin-archive"].(bool):
		cmdRepoBeginArchive(args, conn)
	case args["get-archive"].(bool):
		cmdRepoGetArchive(args, conn)
	case args["archive"].(bool):
		cmdRepoArchive(args, conn)
	case args["begin-unarchive"].(bool):
		cmdRepoBeginUnarchive(args, conn)
	case args["get-unarchive"].(bool):
		cmdRepoGetUnarchive(args, conn)
	case args["unarchive"].(bool):
		cmdRepoUnarchive(args, conn)
	}
}

func cmdRepoEnableGitlab(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewRegistryClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	i := &pb.EnableGitlabRepoI{
		Registry:        args["<registry>"].(string),
		Repo:            repoId[:],
		GitlabNamespace: args["<gitlab-namespace>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}

	creds, err := getRPCCredsRepoId(ctx, args, AAFsoInitRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.EnableGitlabRepo(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.Vid)
}

func cmdRepoInitTartt(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewReposClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	i := &pb.InitTarttI{
		Repo:     repoId[:],
		TarttUrl: args["<tartt-url>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}

	creds, err := getRPCCredsRepoId(ctx, args, AAFsoInitRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.InitTartt(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("repoVid", o.Vid)
}

func cmdRepoInitShadowBackup(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewReposClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	i := &pb.InitShadowBackupI{
		Repo:            repoId[:],
		ShadowBackupUrl: args["<shadow-backup-url>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}

	creds, err := getRPCCredsRepoId(ctx, args, AAFsoInitRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.InitShadowBackup(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("repoVid", o.Vid)
}

func cmdRepoMoveShadowBackup(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewReposClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	i := &pb.MoveShadowBackupI{
		Repo:               repoId[:],
		NewShadowBackupUrl: args["<shadow-backup-url>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}

	creds, err := getRPCCredsRepoId(ctx, args, AAFsoAdminRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.MoveShadowBackup(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("repoVid", o.Vid)
}

func cmdRepoEnableArchiveEncryption(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	repoId := args["<repoid>"].(uuid.I)
	scopes := []interface{}{
		jwtauth.RepoIdScope{Action: AAFsoReadRepo, RepoId: repoId},
		jwtauth.RepoIdScope{Action: AAFsoAdminRepo, RepoId: repoId},
	}
	creds, err := getRPCCredsScopes(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}

	c := pb.NewReposClient(conn)
	gI := &pb.GetRepoI{
		Repo: repoId[:],
	}
	gO, err := c.GetRepo(ctx, gI, creds)
	if err != nil {
		lg.Fatalw("Get failed.", "err", err)
	}
	vid := gO.Vid

	switch {
	case args["--no-vid"].(bool):
		break // skip check
	default:
		argVid := args["--vid"].(ulid.I)
		if !bytes.Equal(vid, argVid[:]) {
			lg.Fatalw("VID mismatch.")
		}
	}

	have, err := gpg.ParseFingerprintsBytes(gO.ArchiveRecipients...)
	if err != nil {
		lg.Fatalw(
			"Get returned malformed archive recipients",
			"err", err,
		)
	}

	want := args["<gpg-keys>"].(gpg.Fingerprints)
	if want.Equal(have) {
		fmt.Printf("# Already up to date.\n")
	} else {
		uI := &pb.UpdateArchiveRecipientsI{
			Repo:              repoId[:],
			RepoVid:           vid,
			ArchiveRecipients: want.Bytes(),
		}
		uO, err := c.UpdateArchiveRecipients(ctx, uI, creds)
		if err != nil {
			lg.Fatalw("Update failed.", "err", err)
		}
		vid = uO.RepoVid
		h, err := gpg.ParseFingerprintsBytes(uO.ArchiveRecipients...)
		if err != nil {
			lg.Fatalw(
				"Update returned malformed archive recipients",
				"err", err,
			)
		}
		have = h
		fmt.Printf("# Updated archive recipients.\n")
	}

	mustPrintlnVidBytes("repoVid", vid)
	fmt.Printf("archiveRecipients: %s\n", jsonGPGFingerprints(have))
}

func cmdRepoDisableArchiveEncryption(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	repoId := args["<repoid>"].(uuid.I)
	scopes := []interface{}{
		jwtauth.RepoIdScope{Action: AAFsoReadRepo, RepoId: repoId},
		jwtauth.RepoIdScope{Action: AAFsoAdminRepo, RepoId: repoId},
	}
	creds, err := getRPCCredsScopes(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}

	c := pb.NewReposClient(conn)
	gI := &pb.GetRepoI{
		Repo: repoId[:],
	}
	gO, err := c.GetRepo(ctx, gI, creds)
	if err != nil {
		lg.Fatalw("Get failed.", "err", err)
	}
	vid := gO.Vid

	have, err := gpg.ParseFingerprintsBytes(gO.ArchiveRecipients...)
	if err != nil {
		lg.Fatalw(
			"Get returned malformed archive recipients",
			"err", err,
		)
	}

	if len(have) == 0 {
		fmt.Printf("# Already up to date.\n")
	} else {
		uI := &pb.DeleteArchiveRecipientsI{
			Repo:    repoId[:],
			RepoVid: vid,
		}
		uO, err := c.DeleteArchiveRecipients(ctx, uI, creds)
		if err != nil {
			lg.Fatalw("Delete failed.", "err", err)
		}
		vid = uO.RepoVid
		fmt.Printf("# Deleted archive recipients.\n")
	}

	mustPrintlnVidBytes("repoVid", vid)
}

func cmdRepoEnableShadowBackupEncryption(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	repoId := args["<repoid>"].(uuid.I)
	scopes := []interface{}{
		jwtauth.RepoIdScope{Action: AAFsoReadRepo, RepoId: repoId},
		jwtauth.RepoIdScope{Action: AAFsoAdminRepo, RepoId: repoId},
	}
	creds, err := getRPCCredsScopes(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}

	c := pb.NewReposClient(conn)
	gI := &pb.GetRepoI{
		Repo: repoId[:],
	}
	gO, err := c.GetRepo(ctx, gI, creds)
	if err != nil {
		lg.Fatalw("Get failed.", "err", err)
	}
	vid := gO.Vid

	switch {
	case args["--no-vid"].(bool):
		break // skip check
	default:
		argVid := args["--vid"].(ulid.I)
		if !bytes.Equal(vid, argVid[:]) {
			lg.Fatalw("VID mismatch.")
		}
	}

	have, err := gpg.ParseFingerprintsBytes(gO.ShadowBackupRecipients...)
	if err != nil {
		lg.Fatalw(
			"Get returned malformed shadow backup recipients",
			"err", err,
		)
	}

	want := args["<gpg-keys>"].(gpg.Fingerprints)
	if want.Equal(have) {
		fmt.Printf("# Already up to date.\n")
	} else {
		uI := &pb.UpdateShadowBackupRecipientsI{
			Repo:                   repoId[:],
			RepoVid:                vid,
			ShadowBackupRecipients: want.Bytes(),
		}
		uO, err := c.UpdateShadowBackupRecipients(ctx, uI, creds)
		if err != nil {
			lg.Fatalw("Update failed.", "err", err)
		}
		vid = uO.RepoVid
		h, err := gpg.ParseFingerprintsBytes(uO.ShadowBackupRecipients...)
		if err != nil {
			lg.Fatalw(
				"Update returned malformed shadow backup recipients",
				"err", err,
			)
		}
		have = h
		fmt.Printf("# Updated shadow backup recipients.\n")
	}

	mustPrintlnVidBytes("repoVid", vid)
	fmt.Printf("shadowBackupRecipients: %s\n", jsonGPGFingerprints(have))
}

func cmdRepoDisableShadowBackupEncryption(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	repoId := args["<repoid>"].(uuid.I)
	scopes := []interface{}{
		jwtauth.RepoIdScope{Action: AAFsoReadRepo, RepoId: repoId},
		jwtauth.RepoIdScope{Action: AAFsoAdminRepo, RepoId: repoId},
	}
	creds, err := getRPCCredsScopes(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}

	c := pb.NewReposClient(conn)
	gI := &pb.GetRepoI{
		Repo: repoId[:],
	}
	gO, err := c.GetRepo(ctx, gI, creds)
	if err != nil {
		lg.Fatalw("Get failed.", "err", err)
	}
	vid := gO.Vid

	have, err := gpg.ParseFingerprintsBytes(gO.ShadowBackupRecipients...)
	if err != nil {
		lg.Fatalw(
			"Get returned malformed shadow backup recipients",
			"err", err,
		)
	}

	if len(have) == 0 {
		fmt.Printf("# Already up to date.\n")
	} else {
		uI := &pb.DeleteShadowBackupRecipientsI{
			Repo:    repoId[:],
			RepoVid: vid,
		}
		uO, err := c.DeleteShadowBackupRecipients(ctx, uI, creds)
		if err != nil {
			lg.Fatalw("Delete failed.", "err", err)
		}
		vid = uO.RepoVid
		fmt.Printf("# Deleted shadow backup recipients.\n")
	}

	mustPrintlnVidBytes("repoVid", vid)
}

func jsonGPGFingerprints(ps gpg.Fingerprints) string {
	var j strings.Builder
	enc := json.NewEncoder(&j)
	enc.SetEscapeHTML(false)

	ss := make([]string, 0, len(ps))
	for _, p := range ps {
		ss = append(ss, fmt.Sprintf("%X", p))
	}
	enc.Encode(ss)

	return strings.TrimRight(j.String(), "\n")
}

func cmdBeginMoveRepo(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewRegistryClient(conn)
	registry := args["<registry>"].(string)
	repoId := args["<repoid>"].(uuid.I)
	workflowId := args["--workflow"].(uuid.I)
	i := &pb.BeginMoveRepoI{
		Registry:              registry,
		Repo:                  repoId[:],
		Workflow:              workflowId[:],
		NewGlobalPath:         args["<new-global-path>"].(string),
		IsUnchangedGlobalPath: args["--unchanged-global-path"].(bool),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}

	creds, err := connect.GetRPCCredsSimpleAndRepoId(
		ctx, args,
		[]connect.SimpleScope{connect.SimpleScope{
			Action: AAFsoAdminRepo,
			Path:   i.NewGlobalPath,
		}},
		[]connect.RepoIdScope{connect.RepoIdScope{
			Action: AAFsoAdminRepo,
			RepoId: repoId,
		}},
	)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.BeginMoveRepo(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.Vid)
}

func cmdBeginMoveShadow(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewReposClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	workflowId := args["--workflow"].(uuid.I)
	i := &pb.BeginMoveShadowI{
		Repo:          repoId[:],
		Workflow:      workflowId[:],
		NewShadowPath: args["<new-shadow-path>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.Vid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.Vid = vid[:]
	}

	creds, err := getRPCCredsRepoId(ctx, args, AAFsoAdminRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.BeginMoveShadow(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("repoVid", o.Vid)
}

func cmdCommitMoveShadow(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewReposClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	workflowId := args["--workflow"].(uuid.I)
	i := &pb.CommitMoveShadowI{
		Repo:     repoId[:],
		Workflow: workflowId[:],
	}
	if args["--no-workflow-vid"].(bool) {
		i.WorkflowVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.WorkflowVid = vid[:]
	}

	creds, err := getRPCCredsRepoId(ctx, args, AAFsoAdminRepo, repoId)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.CommitMoveShadow(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
}
