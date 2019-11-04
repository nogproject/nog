package cmdeventsephreg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/jwtauth"
	"github.com/nogproject/nog/backend/internal/fsoauthz"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const AAFsoReadRegistry = fsoauthz.AAFsoReadRegistry
const AAFsoReadRepo = fsoauthz.AAFsoReadRepo
const AAFsoReadRoot = fsoauthz.AAFsoReadRoot

type Event struct {
	Event  string `json:"event"`
	Id     string `json:"id"`
	Parent string `json:"parent"`
	Etime  string `json:"etime"`

	*AclPolicy       `json:"aclPolicy,omitempty"`
	*PathDecision    `json:"pathDecision,omitempty"`
	*PathSuggestion  `json:"pathSuggestion,omitempty"`
	*PathUsage       `json:"pathUsage,omitempty"`
	*RootInfo        `json:"rootInfo,omitempty"`
	*SplitRootParams `json:"splitRootParams,omitempty"`
	*Status
	AuthorEmail      string `json:"authorEmail,omitempty"`
	AuthorName       string `json:"authorName,omitempty"`
	RegistryId       string `json:"registryId,omitempty"`
	RegistryName     string `json:"registryName,omitempty"`
	RepoArchiveURL   string `json:"repoArchiveUrl,omitempty"`
	RepoGlobalPath   string `json:"repoGlobalPath,omitempty"`
	RepoId           string `json:"repoId,omitempty"`
	StartRegistryVid string `json:"startRegistryVid,omitempty"`
	StartRepoVid     string `json:"startRepoVid,omitempty"`
	TarPath          string `json:"tarPath,omitempty"`
	WorkingDir       string `json:"workingDir,omitempty"`

	Note string `json:"note,omitempty"`
}

type AclPolicy struct {
	Policy    string `json:"policy"`
	*RootInfo `json:"rootInfo,omitempty"`
}

type Status struct {
	StatusCode    int32  `json:"statusCode"`
	StatusMessage string `json:"statusMessage"`
}

type RootInfo struct {
	GlobalRoot string `json:"globalRoot"`
	Host       string `json:"host"`
	HostRoot   string `json:"hostRoot"`
}

type SplitRootParams struct {
	MaxDepth     int32 `json:"maxDepth"`
	MinDiskUsage int64 `json:"minDiskUsage"`
	MaxDiskUsage int64 `json:"maxDiskUsage"`
}

type PathUsage struct {
	Path  string `json:"path"`
	Usage int64  `json:"usage"`
}

type PathSuggestion struct {
	Path       string `json:"path"`
	Suggestion string `json:"suggestion"`
}

type PathDecision struct {
	Path     string `json:"path"`
	Decision string `json:"decision"`
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
	Fatalw(msg string, kv ...interface{})
}

type CmdDetails struct {
	aa auth.Action
}

var (
	Du            = CmdDetails{aa: AAFsoReadRoot}
	PingRegistry  = CmdDetails{}
	SplitRoot     = CmdDetails{aa: AAFsoReadRoot}
	FreezeRepo    = CmdDetails{aa: AAFsoReadRepo}
	UnfreezeRepo  = CmdDetails{aa: AAFsoReadRepo}
	ArchiveRepo   = CmdDetails{aa: AAFsoReadRepo}
	UnarchiveRepo = CmdDetails{aa: AAFsoReadRepo}
)

func Cmd(lg Logger, args map[string]interface{}, details CmdDetails) {
	ctx := context.Background()

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

	c := pb.NewEphemeralRegistryClient(conn)
	workflowId := args["<workflowid>"].(uuid.I)
	i := &pb.RegistryWorkflowEventsI{
		Registry: args["<registry>"].(string),
		Workflow: workflowId[:],
	}
	if a, ok := args["--after"].(ulid.I); ok {
		i.After = a[:]
	}
	if args["--watch"].(bool) {
		// Don't timeout during watch.
		i.Watch = true
	} else {
		ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		ctx = ctxTimeout
	}

	scopes := []interface{}{
		auth.SimpleScope{Action: AAFsoReadRegistry, Name: i.Registry},
	}
	switch details.aa {
	case AAFsoReadRoot:
		scopes = append(scopes, auth.SimpleScope{
			Action: AAFsoReadRoot,
			Path:   args["<root>"].(string),
		})
	case AAFsoReadRepo:
		repoId := args["<repoid>"].(uuid.I)
		scopes = append(scopes, jwtauth.RepoIdScope{
			Action: AAFsoReadRepo,
			RepoId: repoId,
		})
	case "":
		break
	default:
		panic("logic error")
	}
	creds, err := connect.GetRPCCredsScopes(ctx, args, scopes)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	stream, err := c.RegistryWorkflowEvents(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	jsonOut := json.NewEncoder(os.Stdout)
	jsonOut.SetEscapeHTML(false)
	for {
		rsp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			lg.Fatalw("Stream recv failed.", "err", err)
		}
		for _, evpb := range rsp.Events {
			id, err := ulid.ParseBytes(evpb.Id)
			if err != nil {
				lg.Fatalw("Failed to parse Id.", "err", err)
			}
			parent, err := ulid.ParseBytes(evpb.Parent)
			if err != nil {
				lg.Fatalw(
					"Failed to parse Parent.", "err", err,
				)
			}
			ev, err := wfevents.ParsePbWorkflowEvent(evpb)
			if err != nil {
				lg.Fatalw("Failed to parse event.", "err", err)
			}

			outev := Event{
				Event:  evpb.Event.String(),
				Id:     id.String(),
				Parent: parent.String(),
				Etime:  ulid.TimeString(id),
			}
			recode(ev, &outev)

			if err := jsonOut.Encode(&outev); err != nil {
				lg.Fatalw("JSON marshal failed.", "err", err)
			}
		}
		if rsp.WillBlock {
			fmt.Fprintln(os.Stderr, "# Waiting for more events.")
		}
	}
}

func recode(ev wfevents.WorkflowEvent, o *Event) {
	switch x := ev.(type) {
	// du
	case *wfevents.EvDuRootStarted:
		o.RegistryId = x.RegistryId.String()
		o.RootInfo = &RootInfo{
			GlobalRoot: x.GlobalRoot,
			Host:       x.Host,
			HostRoot:   x.HostRoot,
		}
		return

	case *wfevents.EvDuUpdated:
		o.PathUsage = &PathUsage{
			Path:  x.Path,
			Usage: x.Usage,
		}
		return

	case *wfevents.EvDuRootCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvDuRootCommitted:
		return

	case *wfevents.EvDuRootDeleted:
		return

	// ping-registry
	case *wfevents.EvPingRegistryStarted:
		o.RegistryId = x.RegistryId.String()
		return

	case *wfevents.EvServerPinged:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvServerPingsGathered:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvPingRegistryCompleted:
		return

	case *wfevents.EvPingRegistryCommitted:
		return

	case *wfevents.EvPingRegistryDeleted:
		return

	// split-root
	case *wfevents.EvSplitRootStarted:
		o.RegistryId = x.RegistryId.String()
		o.RootInfo = &RootInfo{
			GlobalRoot: x.GlobalRoot,
			Host:       x.Host,
			HostRoot:   x.HostRoot,
		}
		o.SplitRootParams = &SplitRootParams{
			MaxDepth:     x.MaxDepth,
			MinDiskUsage: x.MinDiskUsage,
			MaxDiskUsage: x.MaxDiskUsage,
		}
		return

	case *wfevents.EvSplitRootDuAppended:
		o.PathUsage = &PathUsage{
			Path:  x.Path,
			Usage: x.Usage,
		}
		return

	case *wfevents.EvSplitRootDuCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvSplitRootSuggestionAppended:
		o.PathSuggestion = &PathSuggestion{
			Path:       x.Path,
			Suggestion: x.Suggestion.String(),
		}
		return

	case *wfevents.EvSplitRootAnalysisCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvSplitRootDecisionAppended:
		o.PathDecision = &PathDecision{
			Path:     x.Path,
			Decision: x.Decision.String(),
		}
		return

	case *wfevents.EvSplitRootCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvSplitRootCommitted:
		return

	case *wfevents.EvSplitRootDeleted:
		return

	// freeze-repo
	case *wfevents.EvFreezeRepoStarted2:
		o.RegistryId = x.RegistryId.String()
		o.RegistryName = x.RegistryName
		if x.StartRegistryVid != ulid.Nil {
			o.StartRegistryVid = x.StartRegistryVid.String()
		}
		o.RepoId = x.RepoId.String()
		if x.StartRepoVid != ulid.Nil {
			o.StartRepoVid = x.StartRepoVid.String()
		}
		o.RepoGlobalPath = x.RepoGlobalPath
		o.AuthorName = x.AuthorName
		o.AuthorEmail = x.AuthorEmail
		return

	case *wfevents.EvFreezeRepoFilesStarted:
		return

	case *wfevents.EvFreezeRepoFilesCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvFreezeRepoCompleted2:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvFreezeRepoCommitted:
		return

	case *wfevents.EvFreezeRepoDeleted:
		return

	// unfreeze-repo
	case *wfevents.EvUnfreezeRepoStarted2:
		o.RegistryId = x.RegistryId.String()
		o.RegistryName = x.RegistryName
		if x.StartRegistryVid != ulid.Nil {
			o.StartRegistryVid = x.StartRegistryVid.String()
		}
		o.RepoId = x.RepoId.String()
		if x.StartRepoVid != ulid.Nil {
			o.StartRepoVid = x.StartRepoVid.String()
		}
		o.RepoGlobalPath = x.RepoGlobalPath
		o.AuthorName = x.AuthorName
		o.AuthorEmail = x.AuthorEmail
		return

	case *wfevents.EvUnfreezeRepoFilesStarted:
		return

	case *wfevents.EvUnfreezeRepoFilesCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvUnfreezeRepoCompleted2:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvUnfreezeRepoCommitted:
		return

	case *wfevents.EvUnfreezeRepoDeleted:
		return

	// archive-repo
	case *wfevents.EvArchiveRepoStarted:
		o.RegistryId = x.RegistryId.String()
		o.RegistryName = x.RegistryName
		if x.StartRegistryVid != ulid.Nil {
			o.StartRegistryVid = x.StartRegistryVid.String()
		}
		o.RepoId = x.RepoId.String()
		if x.StartRepoVid != ulid.Nil {
			o.StartRepoVid = x.StartRepoVid.String()
		}
		o.RepoGlobalPath = x.RepoGlobalPath
		o.AuthorName = x.AuthorName
		o.AuthorEmail = x.AuthorEmail
		return

	case *wfevents.EvArchiveRepoFilesStarted:
		if pol := x.AclPolicy; pol != nil {
			o.AclPolicy = &AclPolicy{
				Policy: pol.Policy.String(),
			}
			if inf := pol.FsoRootInfo; inf != nil {
				o.AclPolicy.RootInfo = &RootInfo{
					GlobalRoot: inf.GlobalRoot,
					Host:       inf.Host,
					HostRoot:   inf.HostRoot,
				}
			}
		}
		return

	case *wfevents.EvArchiveRepoTarttCompleted:
		o.TarPath = x.TarPath
		return

	case *wfevents.EvArchiveRepoSwapStarted:
		o.WorkingDir = x.WorkingDir
		return

	case *wfevents.EvArchiveRepoFilesCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvArchiveRepoFilesCommitted:
		return

	case *wfevents.EvArchiveRepoCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvArchiveRepoGcCompleted:
		return

	case *wfevents.EvArchiveRepoCommitted:
		return

	case *wfevents.EvArchiveRepoDeleted:
		return

	// unarchive-repo
	case *wfevents.EvUnarchiveRepoStarted:
		o.RegistryId = x.RegistryId.String()
		o.RegistryName = x.RegistryName
		if x.StartRegistryVid != ulid.Nil {
			o.StartRegistryVid = x.StartRegistryVid.String()
		}
		o.RepoId = x.RepoId.String()
		if x.StartRepoVid != ulid.Nil {
			o.StartRepoVid = x.StartRepoVid.String()
		}
		o.RepoGlobalPath = x.RepoGlobalPath
		o.RepoArchiveURL = x.RepoArchiveURL
		o.AuthorName = x.AuthorName
		o.AuthorEmail = x.AuthorEmail
		return

	case *wfevents.EvUnarchiveRepoFilesStarted:
		if pol := x.AclPolicy; pol != nil {
			o.AclPolicy = &AclPolicy{
				Policy: pol.Policy.String(),
			}
			if inf := pol.FsoRootInfo; inf != nil {
				o.AclPolicy.RootInfo = &RootInfo{
					GlobalRoot: inf.GlobalRoot,
					Host:       inf.Host,
					HostRoot:   inf.HostRoot,
				}
			}
		}
		return

	case *wfevents.EvUnarchiveRepoTarttStarted:
		o.WorkingDir = x.WorkingDir
		return

	case *wfevents.EvUnarchiveRepoTarttCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvUnarchiveRepoFilesCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvUnarchiveRepoFilesCommitted:
		return

	case *wfevents.EvUnarchiveRepoCompleted:
		o.Status = &Status{
			StatusCode:    x.StatusCode,
			StatusMessage: x.StatusMessage,
		}
		return

	case *wfevents.EvUnarchiveRepoGcCompleted:
		return

	case *wfevents.EvUnarchiveRepoCommitted:
		return

	case *wfevents.EvUnarchiveRepoDeleted:
		return

	default:
		o.Note = "nogfsoctl: unknown event type"
	}
}

func mustParseWorkflowId(lg Logger, b []byte) uuid.I {
	id, err := uuid.FromBytes(b)
	if err != nil {
		lg.Fatalw("Failed to parse workflow ID.", "err", err)
	}
	return id
}

func mustParseWorkflowEventId(lg Logger, b []byte) ulid.I {
	id, err := ulid.ParseBytes(b)
	if err != nil {
		lg.Fatalw("Failed to parse workflow event ID.", "err", err)
	}
	return id
}

func asHexStrings(bs [][]byte) []string {
	if len(bs) == 0 {
		return nil
	}
	ss := make([]string, 0, len(bs))
	for _, b := range bs {
		ss = append(ss, fmt.Sprintf("%X", b))
	}
	return ss
}
