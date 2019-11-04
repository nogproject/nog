package broadcastd

import (
	"context"
	"time"

	"github.com/nogproject/nog/backend/internal/broadcast"
	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	NsBroadcast  = "broadcast"
	BroadcastAll = "all"
)

type Server struct {
	ctx           context.Context
	lg            Logger
	authn         auth.Authenticator
	authz         auth.Authorizer
	names         *shorteruuid.Names
	broadcastJ    *events.Journal
	journalWriter *journalWriter
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
	Infow(msg string, kv ...interface{})
}

func New(
	ctx context.Context,
	lg Logger,
	authn auth.Authenticator,
	authz auth.Authorizer,
	names *shorteruuid.Names,
	broadcastJ *events.Journal,
) *Server {
	return &Server{
		ctx:        ctx,
		lg:         lg,
		authn:      authn,
		authz:      authz,
		names:      names,
		broadcastJ: broadcastJ,
		journalWriter: newJournalWriter(
			lg,
			names.UUID(NsBroadcast, BroadcastAll),
			broadcastJ,
		),
	}
}

func (srv *Server) Serve() error {
	return srv.journalWriter.process(srv.ctx)
}

func (srv *Server) Events(
	req *pb.BroadcastEventsI, stream pb.Broadcast_EventsServer,
) error {
	// `ctx.Done()` indicates client close, see
	// <https://groups.google.com/d/msg/grpc-io/C0rAhtCUhSs/SzFDLGqiCgAJ>.
	ctx := stream.Context()
	if err := srv.authName(ctx, AABroadcastRead, req.Channel); err != nil {
		return err
	}

	if req.Channel != BroadcastAll {
		err := status.Errorf(
			codes.InvalidArgument, "invalid channel",
		)
		return err
	}
	id := srv.names.UUID(NsBroadcast, BroadcastAll)

	after := events.EventEpoch
	if req.After != nil {
		a, err := ulid.ParseBytes(req.After)
		if err != nil {
			err := status.Errorf(
				codes.InvalidArgument, "malformed after",
			)
			return err
		}
		after = a
	}

	if req.AfterNow {
		a, err := srv.broadcastJ.Head(id)
		if err != nil {
			err := status.Errorf(
				codes.Unknown, "%s", err,
			)
			return err
		}
		if a == events.EventEpoch {
			err := status.Errorf(
				codes.NotFound, "unknown or empty channel",
			)
			return err
		}
		after = a
	}

	updated := make(chan uuid.I, 1)
	updated <- id // Trigger initial Find().

	var ticks <-chan time.Time
	if req.Watch {
		srv.broadcastJ.Subscribe(updated, id)
		defer srv.broadcastJ.Unsubscribe(updated)

		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-srv.ctx.Done():
			err := status.Errorf(codes.Unavailable, "shutdown")
			return err
		case <-updated:
		case <-ticks:
		}

		iter := srv.broadcastJ.Find(id, after)
		var ev broadcast.Event
		for iter.Next(&ev) {
			after = ev.Id() // Update tail for restart.
			rspEv := ev.PbBroadcastEvent()
			rsp := &pb.BroadcastEventsO{
				Channel: req.Channel,
				Events:  []*pb.BroadcastEvent{rspEv},
			}
			if err := stream.Send(rsp); err != nil {
				_ = iter.Close()
				return err
			}
		}
		if err := iter.Close(); err != nil {
			// XXX Maybe add more detailed error case handling.
			err := status.Errorf(
				codes.Unknown, "journal error: %v", err,
			)
			return err
		}

		if !req.Watch {
			return nil
		}

		rsp := &pb.BroadcastEventsO{
			Channel:   req.Channel,
			WillBlock: true,
		}
		if err := stream.Send(rsp); err != nil {
			return err
		}
	}
}
