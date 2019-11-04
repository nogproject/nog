package cmdeventsbroadcast

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/internal/fsoauthz"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const AABroadcastRead = fsoauthz.AABroadcastRead

type Event struct {
	Event  string `json:"event"`
	Id     string `json:"id"`
	Parent string `json:"parent"`
	Etime  string `json:"etime"`

	Note      string `json:"note,omitempty"`
	Entity    string `json:"entity,omitempty"`
	GitRef    string `json:"gitRef,omitempty"`
	GitCommit string `json:"gitCommit,omitempty"`
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

	c := pb.NewBroadcastClient(conn)
	req := pb.BroadcastEventsI{
		Channel: "all",
	}
	if a, ok := args["--after"].(ulid.I); ok {
		req.After = a[:]
	}
	if args["--after-now"].(bool) {
		req.AfterNow = true
	}
	if args["--watch"].(bool) {
		// Don't timeout during watch.
		req.Watch = true
	} else {
		ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		ctx = ctxTimeout
	}
	creds, err := connect.GetRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AABroadcastRead,
		Name:   req.Channel,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	stream, err := c.Events(ctx, &req, creds)
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
			switch ev.Event {
			case pb.BroadcastEvent_EV_BC_FSO_MAIN_CHANGED:
				fallthrough
			case pb.BroadcastEvent_EV_BC_FSO_REGISTRY_CHANGED:
				fallthrough
			case pb.BroadcastEvent_EV_BC_FSO_REPO_CHANGED:
				if ev.BcChange == nil {
					lg.Fatalw("Invalid event.")
				}
				entityId, err := uuid.FromBytes(
					ev.BcChange.EntityId,
				)
				if err != nil {
					lg.Fatalw(
						"Failed to parse EntityId.",
						"err", err,
					)
				}
				outev.Entity = entityId.String()

			case pb.BroadcastEvent_EV_BC_FSO_GIT_REF_UPDATED:
				if ev.BcChange == nil {
					lg.Fatalw("Invalid event.")
				}
				entityId, err := uuid.FromBytes(
					ev.BcChange.EntityId,
				)
				if err != nil {
					lg.Fatalw(
						"Failed to parse EntityId.",
						"err", err,
					)
				}

				gitRef := ev.BcChange.GitRef

				gitCommit := ev.BcChange.GitCommit
				if gitCommit == nil {
					err := errors.New("missing git commit")
					lg.Fatalw("Invalid event.", "err", err)
				}
				gitCommitHex := hex.EncodeToString(gitCommit)

				outev.Entity = entityId.String()
				outev.GitRef = gitRef
				outev.GitCommit = gitCommitHex

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
