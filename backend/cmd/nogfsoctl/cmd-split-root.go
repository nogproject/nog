package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	slashpath "path"
	"strings"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/parse"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

func cmdSplitRoot(args map[string]interface{}) {
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
	case args["enable-root"].(bool):
		cmdSplitRootEnableRoot(args, conn)
	case args["disable-root"].(bool):
		cmdSplitRootDisableRoot(args, conn)
	case args["dont-split"].(bool):
		cmdSplitRootDontSplit(args, conn)
	case args["allow-split"].(bool):
		cmdSplitRootAllowSplit(args, conn)
	case args["config"].(bool):
		cmdSplitRootConfig(args, conn)
	case args["begin"].(bool):
		cmdSplitRootBegin(args, conn)
	case args["get"].(bool):
		cmdSplitRootGet(args, conn)
	case args["decide"].(bool):
		cmdSplitRootDecide(args, conn)
	case args["commit"].(bool):
		cmdSplitRootCommit(args, conn)
	case args["abort"].(bool):
		cmdSplitRootAbort(args, conn)
	default:
		lg.Fatalw("Logic error: invalid `enable-root` sub-command.")
	}
}

func cmdSplitRootEnableRoot(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	registry := args["<registry>"].(string)
	root := args["<root>"].(string)
	scopes := []auth.SimpleScope{
		{Action: AAFsoReadRoot, Path: root},
		{Action: AAFsoAdminRegistry, Name: registry},
	}
	creds, err := getRPCCredsSimple(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}

	c := pb.NewSplitRootClient(conn)
	getI := &pb.GetSplitRootConfigI{
		Registry:   registry,
		GlobalRoot: root,
	}
	getO, err := c.GetSplitRootConfig(ctx, getI, creds)
	if err != nil {
		lg.Fatalw("Get failed.", "err", err)
	}
	vid := getO.RegistryVid
	cfg := getO.Config

	if argVid, ok := args["--vid"].(ulid.I); ok {
		if !bytes.Equal(vid, argVid[:]) {
			lg.Fatalw("Version mismatch.")
		}
	}

	changed := false
	if maxDepth, ok := args["--max-depth"].(int32); ok {
		if cfg.MaxDepth != maxDepth {
			cfg.MaxDepth = maxDepth
			changed = true
		}
	}
	if minDu, ok := args["--min-du"].(int64); ok {
		if cfg.MinDiskUsage != minDu {
			cfg.MinDiskUsage = minDu
			changed = true
		}
	}
	if maxDu, ok := args["--max-du"].(int64); ok {
		if cfg.MaxDiskUsage != maxDu {
			cfg.MaxDiskUsage = maxDu
			changed = true
		}
	}

	switch {
	case !cfg.Enabled:
		createI := &pb.CreateSplitRootConfigI{
			Registry:    registry,
			RegistryVid: vid,
			Config:      cfg,
		}
		createO, err := c.CreateSplitRootConfig(ctx, createI, creds)
		if err != nil {
			lg.Fatalw("Create failed.", "err", err)
		}
		vid = createO.RegistryVid
		cfg = createO.Config
		fmt.Printf("# Created config.\n")

	case changed:
		updateI := &pb.UpdateSplitRootConfigI{
			Registry:    registry,
			RegistryVid: vid,
			Config:      cfg,
		}
		updateO, err := c.UpdateSplitRootConfig(ctx, updateI, creds)
		if err != nil {
			lg.Fatalw("Update failed.", "err", err)
		}
		vid = updateO.RegistryVid
		cfg = updateO.Config
		fmt.Printf("# Updated config.\n")

	default:
		fmt.Printf("# Already up to date.\n")
	}

	mustPrintlnVidBytes("registryVid", vid)
	fmt.Printf("globalRoot: %s\n", cfg.GlobalRoot)
	fmt.Printf("maxDepth: %d\n", cfg.MaxDepth)
	fmt.Printf("minDiskUsage: %d\n", cfg.MinDiskUsage)
	fmt.Printf("maxDiskUsage: %d\n", cfg.MaxDiskUsage)
}

// nogfsoctl [options] split-root disable-root <registry> (--vid=<vid>|--no-vid) <root>
func cmdSplitRootDisableRoot(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	registry := args["<registry>"].(string)
	root := args["<root>"].(string)
	scopes := []auth.SimpleScope{
		{Action: AAFsoAdminRegistry, Name: registry},
	}
	creds, err := getRPCCredsSimple(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}

	c := pb.NewSplitRootClient(conn)
	i := &pb.DeleteSplitRootConfigI{
		Registry:   registry,
		GlobalRoot: root,
	}
	if args["--no-vid"].(bool) {
		i.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.RegistryVid = vid[:]
	}
	o, err := c.DeleteSplitRootConfig(ctx, i, creds)
	if err != nil {
		lg.Fatalw("Delete failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
}

func cmdSplitRootDontSplit(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewSplitRootClient(conn)
	registry := args["<registry>"].(string)
	root := args["<root>"].(string)

	path := args["<path>"].(string)
	if slashpath.IsAbs(path) {
		rootSlash := root + "/"
		if path == root {
			path = "."
		} else if !strings.HasPrefix(path, rootSlash) {
			lg.Fatalw("<path> not below <root>.")
		} else {
			path = strings.TrimPrefix(path, rootSlash)
			if path == "" {
				path = "."
			}
		}
	}

	i := &pb.CreateSplitRootPathFlagI{
		Registry:     registry,
		GlobalRoot:   root,
		RelativePath: path,
		Flags:        uint32(pb.FsoPathFlag_PF_DONT_SPLIT),
	}
	if args["--no-vid"].(bool) {
		i.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.RegistryVid = vid[:]
	}
	scopes := []auth.SimpleScope{
		{Action: AAFsoAdminRoot, Path: root},
	}
	creds, err := getRPCCredsSimple(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}
	o, err := c.CreateSplitRootPathFlag(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
}

func cmdSplitRootAllowSplit(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewSplitRootClient(conn)
	registry := args["<registry>"].(string)
	root := args["<root>"].(string)

	path := args["<path>"].(string)
	if slashpath.IsAbs(path) {
		rootSlash := root + "/"
		if path == root {
			path = "."
		} else if !strings.HasPrefix(path, rootSlash) {
			lg.Fatalw("<path> not below <root>.")
		} else {
			path = strings.TrimPrefix(path, rootSlash)
			if path == "" {
				path = "."
			}
		}
	}

	i := &pb.DeleteSplitRootPathFlagI{
		Registry:     registry,
		GlobalRoot:   root,
		RelativePath: path,
		Flags:        uint32(pb.FsoPathFlag_PF_DONT_SPLIT),
	}
	if args["--no-vid"].(bool) {
		i.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.RegistryVid = vid[:]
	}
	scopes := []auth.SimpleScope{
		{Action: AAFsoAdminRegistry, Name: registry},
	}
	creds, err := getRPCCredsSimple(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}
	o, err := c.DeleteSplitRootPathFlag(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
}

func cmdSplitRootConfig(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	registry := args["<registry>"].(string)
	root := args["<root>"].(string)
	c := pb.NewSplitRootClient(conn)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	scopes := []auth.SimpleScope{
		{Action: AAFsoReadRoot, Path: root},
	}
	creds, err := getRPCCredsSimple(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}

	getI := &pb.GetSplitRootConfigI{
		Registry:   registry,
		GlobalRoot: root,
	}
	getO, err := c.GetSplitRootConfig(ctx, getI, creds)
	if err != nil {
		lg.Fatalw("Get failed.", "err", err)
	}
	regVid := getO.RegistryVid
	cfg := getO.Config

	lsI := &pb.ListSplitRootPathFlagsI{
		Registry:    registry,
		RegistryVid: regVid[:],
		GlobalRoot:  root,
	}
	lsO, err := c.ListSplitRootPathFlags(ctx, lsI, creds)
	if err != nil {
		lg.Fatalw("Get failed.", "err", err)
	}
	dontSplit := make([]string, 0, len(lsO.Paths))
	for _, p := range lsO.Paths {
		if p.Flags&uint32(pb.FsoPathFlag_PF_DONT_SPLIT) != 0 {
			dontSplit = append(dontSplit, p.Path)
		}
	}

	mustPrintlnVidBytes("registryVid", regVid)
	fmt.Printf("globalRoot: %s\n", cfg.GlobalRoot)
	fmt.Printf("maxDepth: %d\n", cfg.MaxDepth)
	fmt.Printf("minDiskUsage: %d\n", cfg.MinDiskUsage)
	fmt.Printf("maxDiskUsage: %d\n", cfg.MaxDiskUsage)
	if len(dontSplit) == 0 {
		fmt.Println("dontSplit: []")
	} else {
		fmt.Println("dontSplit:")
		for _, p := range dontSplit {
			fmt.Printf(" - %s\n", jsonString(p))
		}
	}
}

func cmdSplitRootBegin(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewSplitRootClient(conn)
	registry := args["<registry>"].(string)
	root := args["<root>"].(string)
	workflowId := args["--workflow"].(uuid.I)
	i := &pb.BeginSplitRootI{
		Registry:   registry,
		GlobalRoot: root,
		Workflow:   workflowId[:],
	}
	if args["--no-vid"].(bool) {
		i.RegistryVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.RegistryVid = vid[:]
	}
	scopes := []auth.SimpleScope{
		{Action: AAFsoAdminRoot, Path: root},
	}
	creds, err := getRPCCredsSimple(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}
	o, err := c.BeginSplitRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
	mustPrintlnVidBytes("workflowIndexVid", o.WorkflowIndexVid)
	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
}

func cmdSplitRootGet(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	timeout, optWait := args["--wait"].(time.Duration)
	if !optWait {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// `registry` is currently unused.  But we keep it in case we want to
	// later change auth to be based on the registry name.
	registry := args["<registry>"].(string)
	_ = registry

	c := pb.NewSplitRootClient(conn)
	root := args["<root>"].(string)
	workflowId := args["<workflowid>"].(uuid.I)
	i := &pb.GetSplitRootI{
		Workflow: workflowId[:],
	}
	if optWait {
		i.JobControl = pb.JobControl_JC_WAIT
	} else {
		i.JobControl = pb.JobControl_JC_NO_WAIT
	}
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAFsoAdminRoot,
		Path:   root,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	o, err := c.GetSplitRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	root = o.GlobalRoot

	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
	comment := ""
	switch o.StatusCode {
	case int32(pb.GetSplitRootO_SC_OK):
		comment = " # ok"
	case int32(pb.GetSplitRootO_SC_RUNNING):
		comment = " # running"
	case int32(pb.GetSplitRootO_SC_ANALYSIS_COMPLETED):
		comment = " # analysis completed"
	case int32(pb.GetSplitRootO_SC_FAILED):
		comment = " # failed"
	case int32(pb.GetSplitRootO_SC_COMPLETED):
		comment = " # completed"
	case int32(pb.GetSplitRootO_SC_EXPIRED):
		comment = " # expired"
	}
	fmt.Printf("statusCode: %d%s\n", o.StatusCode, comment)
	fmt.Printf("statusMessage: %s\n", jsonString(o.StatusMessage))

	du := make(map[string]int64)
	for _, p := range o.Du {
		du[p.Path] = p.Usage
	}

	// Compute undecided path at client.  Report them in suggestion order.
	undecided := make(map[string]struct{})

	if len(o.Suggestions) == 0 {
		fmt.Println("suggestions: []")
	} else {
		fmt.Println("suggestions:")
		for _, s := range o.Suggestions {
			abs := slashpath.Join(root, s.Path)
			fmt.Printf(
				` - { suggestion: %16s, du: %16d, path: %s }`+"\n",
				s.Suggestion, du[s.Path], jsonString(abs),
			)
			if s.Suggestion == pb.FsoSplitRootSuggestion_S_REPO_CANDIDATE {
				undecided[s.Path] = struct{}{}
			}
		}
	}

	if len(o.Decisions) == 0 {
		fmt.Println("decisions: []")
	} else {
		fmt.Println("decisions:")
		for _, d := range o.Decisions {
			abs := slashpath.Join(root, d.Path)
			fmt.Printf(
				` - { decision: %18s, path: %s }`+"\n",
				d.Decision, jsonString(abs),
			)
			delete(undecided, d.Path)
		}
	}

	if len(undecided) == 0 {
		fmt.Println("undecided: []")
	} else {
		fmt.Println("undecided:")
		for _, s := range o.Suggestions {
			if _, ok := undecided[s.Path]; ok {
				abs := slashpath.Join(root, s.Path)
				fmt.Printf(
					" - %s\n",
					jsonString(abs),
				)
			}
		}
	}
}

func jsonString(s string) string {
	var j strings.Builder
	enc := json.NewEncoder(&j)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	return strings.TrimRight(j.String(), "\n")
}

func cmdSplitRootDecide(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// `registry` is currently unused.  But we keep it in case we want to
	// later change auth to be based on the registry name.
	registry := args["<registry>"].(string)
	_ = registry

	c := pb.NewSplitRootClient(conn)
	root := args["<root>"].(string)
	workflowId := args["<workflowid>"].(uuid.I)
	i := &pb.AppendSplitRootDecisionsI{
		Workflow: workflowId[:],
	}
	if args["--no-vid"].(bool) {
		i.WorkflowVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.WorkflowVid = vid[:]
	}

	// First apply the decisions that reduce the candidate set without
	// initializing new repos, because their effects are easier to revert.
	for _, p := range args["--ignore-once"].([]string) {
		i.Paths = append(i.Paths, &pb.AppendSplitRootDecisionsI_PathDecision{
			Path:     p,
			Decision: pb.AppendSplitRootDecisionsI_D_IGNORE_ONCE,
		})
	}
	for _, p := range args["--never-split"].([]string) {
		i.Paths = append(i.Paths, &pb.AppendSplitRootDecisionsI_PathDecision{
			Path:     p,
			Decision: pb.AppendSplitRootDecisionsI_D_NEVER_SPLIT,
		})
	}

	scopes := []auth.SimpleScope{
		{Action: AAFsoAdminRoot, Path: root},
	}

	// Then apply the decision that create new repos.
	needAuthor := false
	for _, p := range args["--init-repo"].([]string) {
		i.Paths = append(i.Paths, &pb.AppendSplitRootDecisionsI_PathDecision{
			Path:     p,
			Decision: pb.AppendSplitRootDecisionsI_D_CREATE_REPO,
		})
		needAuthor = true
		scopes = append(scopes, auth.SimpleScope{
			Action: AAFsoInitRepo,
			Path:   p,
		})
	}
	if needAuthor {
		a, ok := args["--author"].(string)
		if !ok {
			lg.Fatalw("Missing --author.")
		}
		name, email, err := parse.User(a)
		if err != nil {
			lg.Fatalw(
				"Failed to parse --author",
				"err", err,
			)
		}
		i.CreatorName = name
		i.CreatorEmail = email
	}

	creds, err := getRPCCredsSimple(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}
	o, err := c.AppendSplitRootDecisions(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	jsonOut := json.NewEncoder(os.Stdout)
	jsonOut.SetEscapeHTML(false)

	mustPrintlnVidBytes("registryVid", o.RegistryVid)
	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)

	if len(o.Effects) == 0 {
		fmt.Println("effects: []")
	} else {
		fmt.Println("effects:")
		for _, e := range o.Effects {
			var eff struct {
				Path        string `json:"path"`
				RepoId      string `json:"repoId,omitempty"`
				RegistryVid string `json:"registryVid,omitempty"`
				WorkflowVid string `json:"workflowVid,omitempty"`
			}
			eff.Path = e.Path
			if e.RepoId != nil {
				id, err := uuid.FromBytes(e.RepoId)
				if err != nil {
					lg.Fatalw(
						"Malformed response.",
						"err", err,
					)
				}
				eff.RepoId = id.String()
			}
			if e.RegistryVid != nil {
				vid, err := ulid.ParseBytes(e.RegistryVid)
				if err != nil {
					lg.Fatalw(
						"Malformed response.",
						"err", err,
					)
				}
				eff.RegistryVid = vid.String()
			}
			if e.WorkflowVid != nil {
				vid, err := ulid.ParseBytes(e.WorkflowVid)
				if err != nil {
					lg.Fatalw(
						"Malformed response.",
						"err", err,
					)
				}
				eff.WorkflowVid = vid.String()
			}
			fmt.Printf(" - ")
			if err := jsonOut.Encode(&eff); err != nil {
				lg.Fatalw("JSON marshal failed.", "err", err)
			}
		}
	}
}

func cmdSplitRootCommit(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// `registry` is currently unused.  But we keep it in case we want to
	// later change auth to be based on the registry name.
	registry := args["<registry>"].(string)
	_ = registry

	c := pb.NewSplitRootClient(conn)
	root := args["<root>"].(string)
	workflowId := args["<workflowid>"].(uuid.I)
	i := &pb.CommitSplitRootI{
		Workflow: workflowId[:],
	}
	if args["--no-vid"].(bool) {
		i.WorkflowVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.WorkflowVid = vid[:]
	}

	scopes := []auth.SimpleScope{
		{Action: AAFsoAdminRoot, Path: root},
	}
	creds, err := getRPCCredsSimple(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}
	o, err := c.CommitSplitRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
	mustPrintlnVidBytes("workflowIndexVid", o.WorkflowIndexVid)
}

func cmdSplitRootAbort(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// `registry` is currently unused.  But we keep it in case we want to
	// later change auth to be based on the registry name.
	registry := args["<registry>"].(string)
	_ = registry

	c := pb.NewSplitRootClient(conn)
	root := args["<root>"].(string)
	workflowId := args["<workflowid>"].(uuid.I)
	i := &pb.AbortSplitRootI{
		Workflow:      workflowId[:],
		StatusCode:    1,
		StatusMessage: "nogfsoctl abort",
	}
	if args["--no-vid"].(bool) {
		i.WorkflowVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.WorkflowVid = vid[:]
	}

	scopes := []auth.SimpleScope{
		{Action: AAFsoAdminRoot, Path: root},
	}
	creds, err := getRPCCredsSimple(ctx, args, scopes)
	if err != nil {
		lg.Fatalw(
			"Failed to get token.",
			"scopes", scopes,
			"err", err,
		)
	}
	o, err := c.AbortSplitRoot(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnVidBytes("workflowVid", o.WorkflowVid)
	mustPrintlnVidBytes("workflowIndexVid", o.WorkflowIndexVid)
}
