package cmdeventsrepoworkflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/internal/fsoauthz"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const AAFsoReadRepo = fsoauthz.AAFsoReadRepo

type Event struct {
	Event  string `json:"event"`
	Id     string `json:"id"`
	Parent string `json:"parent"`
	Etime  string `json:"etime"`

	Note          string `json:"note,omitempty"`
	RepoId        string `json:"repoId,omitempty"`
	RepoEventId   string `json:"repoEventId,omitempty"`
	GlobalPath    string `json:"globalPath,omitempty"`
	FileHost      string `json:"fileHost,omitempty"`
	HostPath      string `json:"hostPath,omitempty"`
	ShadowPath    string `json:"shadowPath,omitempty"`
	OldGlobalPath string `json:"oldGlobalPath,omitempty"`
	OldFileHost   string `json:"oldFileHost,omitempty"`
	OldHostPath   string `json:"oldHostPath,omitempty"`
	OldShadowPath string `json:"oldShadowPath,omitempty"`
	NewGlobalPath string `json:"newGlobalPath,omitempty"`
	NewFileHost   string `json:"newFileHost,omitempty"`
	NewHostPath   string `json:"newHostPath,omitempty"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
	Fatalw(msg string, kv ...interface{})
}

func Cmd(lg Logger, args map[string]interface{}) {
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

	c := pb.NewReposClient(conn)
	repoId := args["<repoid>"].(uuid.I)
	workflowId := args["<workflowid>"].(uuid.I)
	req := pb.RepoWorkflowEventsI{
		Repo:     repoId[:],
		Workflow: workflowId[:],
	}
	if a, ok := args["--after"].(ulid.I); ok {
		req.After = a[:]
	}
	if args["--watch"].(bool) {
		// Don't timeout during watch.
		req.Watch = true
	} else {
		ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		ctx = ctxTimeout
	}

	creds, err := connect.GetRPCCredsRepoId(
		ctx, args, AAFsoReadRepo, repoId,
	)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	stream, err := c.WorkflowEvents(ctx, &req, creds)
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
		for _, ev := range rsp.Events {
			id, err := ulid.ParseBytes(ev.Id)
			if err != nil {
				lg.Fatalw("Failed to parse Id.", "err", err)
			}
			parent, err := ulid.ParseBytes(ev.Parent)
			if err != nil {
				lg.Fatalw(
					"Failed to parse Parent.", "err", err,
				)
			}
			outev := Event{
				Event:  ev.Event.String(),
				Id:     id.String(),
				Parent: parent.String(),
				Etime:  ulid.TimeString(id),
			}

			wfev, err := wfevents.ParsePbWorkflowEvent(ev)
			if err != nil {
				lg.Fatalw(
					"Failed to parse workflow event.",
					"err", err,
				)
			}

			switch x := wfev.(type) {
			case *wfevents.EvRepoMoveStarted:
				outev.RepoId = x.RepoId.String()
				outev.RepoEventId = x.RepoEventId.String()
				outev.OldGlobalPath = x.OldGlobalPath
				outev.OldFileHost = x.OldFileHost
				outev.OldHostPath = x.OldHostPath
				outev.OldShadowPath = x.OldShadowPath
				outev.NewGlobalPath = x.NewGlobalPath
				outev.NewFileHost = x.NewFileHost
				outev.NewHostPath = x.NewHostPath

			case *wfevents.EvRepoMoveStaReleased:
				// No details.

			case *wfevents.EvRepoMoveAppAccepted:
				// No details.

			case *wfevents.EvRepoMoved:
				outev.RepoId = x.RepoId.String()
				outev.GlobalPath = x.GlobalPath
				outev.FileHost = x.FileHost
				outev.HostPath = x.HostPath
				outev.ShadowPath = x.ShadowPath

			case *wfevents.EvRepoMoveCommitted:
				// No details.

			case *wfevents.EvShadowRepoMoveStarted:
				outev.RepoId = x.RepoId.String()
				outev.RepoEventId = x.RepoEventId.String()

			case *wfevents.EvShadowRepoMoveStaDisabled:
				// No details.

			case *wfevents.EvShadowRepoMoved:
				outev.RepoId = x.RepoId.String()

			case *wfevents.EvShadowRepoMoveCommitted:
				// No details.

			default:
				outev.Note = "nogfsoctl: unknown event type"
			}

			if err := jsonOut.Encode(&outev); err != nil {
				lg.Fatalw("JSON marshal failed.", "err", err)
			}
		}
		if rsp.WillBlock {
			fmt.Fprintln(os.Stderr, "# Waiting for more events.")
		}
	}
}
