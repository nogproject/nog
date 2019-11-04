package cmdeventsuxdom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/internal/fsoauthz"
	uxev "github.com/nogproject/nog/backend/internal/unixdomains/events"
	pb "github.com/nogproject/nog/backend/internal/unixdomainspb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const AAReadUnixDomain = fsoauthz.AAReadUnixDomain

type Event struct {
	Event  string `json:"event"`
	Id     string `json:"id"`
	Parent string `json:"parent"`
	Etime  string `json:"etime"`

	DomainName string `json:"domainName,omitempty"`
	DomainId   string `json:"domainId,omitempty"`
	Group      string `json:"group,omitempty"`
	Gid        uint32 `json:"gid,omitempty"`
	User       string `json:"user,omitempty"`
	Uid        uint32 `json:"uid,omitempty"`

	Note string `json:"note,omitempty"`
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

	c := pb.NewUnixDomainsClient(conn)
	domainName := args["<domain>"].(string)
	creds, err := connect.GetRPCCredsScopes(ctx, args, []interface{}{
		auth.SimpleScope{Action: AAReadUnixDomain, Name: domainName},
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	domainId := resolveUnixDomain(lg, ctx, c, domainName, creds)
	i := &pb.UnixDomainEventsI{
		DomainId: domainId[:],
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
	stream, err := c.UnixDomainEvents(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	jout := json.NewEncoder(os.Stdout)
	jout.SetEscapeHTML(false)
	for {
		o, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			lg.Fatalw("Stream recv failed.", "err", err)
		}
		for _, evpb := range o.Events {
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
			ev, err := uxev.ParsePbUnixDomainEvent(evpb)
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

			if err := jout.Encode(&outev); err != nil {
				lg.Fatalw("JSON marshal failed.", "err", err)
			}
		}
		if o.WillBlock {
			fmt.Fprintln(os.Stderr, "# Waiting for more events.")
		}
	}
}

func recode(ev uxev.UnixDomainEvent, o *Event) {
	switch x := ev.(type) {
	case *uxev.EvDomainCreated:
		o.DomainName = x.Name
		return

	case *uxev.EvGroupCreated:
		o.Group = x.Group
		o.Gid = x.Gid
		return

	case *uxev.EvUserCreated:
		o.User = x.User
		o.Uid = x.Uid
		o.Gid = x.Gid
		return

	case *uxev.EvGroupUserAdded:
		o.Gid = x.Gid
		o.Uid = x.Uid
		return

	case *uxev.EvGroupUserRemoved:
		o.Gid = x.Gid
		o.Uid = x.Uid
		return

	case *uxev.EvUserDeleted:
		o.Uid = x.Uid
		return

	case *uxev.EvGroupDeleted:
		o.Gid = x.Gid
		return

	default:
		o.Note = "nogfsoctl: unknown event type"
		return
	}
}

func resolveUnixDomain(
	lg Logger,
	ctx context.Context,
	c pb.UnixDomainsClient,
	domainName string,
	creds grpc.CallOption,
) uuid.I {
	i := &pb.GetUnixDomainI{
		DomainName: domainName,
	}
	o, err := c.GetUnixDomain(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	id, err := uuid.FromBytes(o.DomainId)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	return id
}
