package broadcastd

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nogproject/nog/backend/internal/broadcast"
	"github.com/nogproject/nog/backend/internal/broadcast/pbevents"
	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func (srv *Server) PostGitRefUpdated(
	ctx context.Context, i *pb.PostGitRefUpdatedI,
) (*pb.PostGitRefUpdatedO, error) {
	// Everything is delived to a single channel.  See `New()` in
	// `./broadcastd.go`.
	const channel = BroadcastAll
	if err := srv.authName(ctx, AABroadcastWrite, channel); err != nil {
		return nil, err
	}

	id, err := uuid.FromBytes(i.Repo)
	if err != nil {
		err := status.Errorf(
			codes.InvalidArgument, "malformed repo id: %v", err,
		)
		return nil, err
	}

	// XXX Maybe validate more strictly.
	if i.Ref == "" {
		err := status.Errorf(codes.InvalidArgument, "malformed ref")
		return nil, err
	}
	if i.Commit == nil {
		err := status.Errorf(codes.InvalidArgument, "malformed commit")
		return nil, err
	}

	ev := pbevents.NewGitRefUpdated(id, i.Ref, i.Commit)
	ev, err = pbevents.WithId(ev)
	if err != nil {
		err := status.Errorf(
			codes.ResourceExhausted,
			"failed to create ULID: %s", err,
		)
		return nil, err
	}

	err = srv.journalWriter.post(&ev)
	switch {
	case err == ErrPostQueueFull:
		err := status.Errorf(codes.ResourceExhausted, "%s", err)
		return nil, err
	case err != nil:
		err := status.Errorf(codes.Unknown, "%s", err)
		return nil, err
	}

	return &pb.PostGitRefUpdatedO{}, nil
}

const PostQueueLen = 200
const MaxPostBatchLen = 100

var ErrPostQueueFull = errors.New("post queue full")

type journalWriter struct {
	lg         Logger
	id         uuid.I
	broadcastJ *events.Journal
	ch         chan *pb.BroadcastEvent
}

func newJournalWriter(
	lg Logger, id uuid.I, broadcastJ *events.Journal,
) *journalWriter {
	return &journalWriter{
		lg:         lg,
		id:         id,
		broadcastJ: broadcastJ,
		ch:         make(chan *pb.BroadcastEvent, PostQueueLen),
	}
}

func (w *journalWriter) process(ctx context.Context) error {
	batch := make([]*pb.BroadcastEvent, 0, MaxPostBatchLen)
	clearBatch := func() {
		batch = make([]*pb.BroadcastEvent, 0, MaxPostBatchLen)
	}

	readNoWait := func() error {
		for {
			if len(batch) >= MaxPostBatchLen {
				return nil
			}
			select {
			default: // Don't block.
				return nil
			case <-ctx.Done():
				return ctx.Err()
			case e := <-w.ch:
				batch = append(batch, e)
			}
		}
	}

	readOneWait := func() error {
		if len(batch) >= MaxPostBatchLen {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-w.ch:
			batch = append(batch, e)
			return nil
		}
	}

	readBatch := func() error {
		for {
			if err := readNoWait(); err != nil {
				return err
			}
			if len(batch) > 0 {
				return nil
			}
			if err := readOneWait(); err != nil {
				return err
			}
		}
	}

	for {
		if err := readBatch(); err != nil {
			return err
		}

		head, err := w.broadcastJ.Head(w.id)
		if err != nil {
			wait := 10 * time.Second
			w.lg.Errorw(
				"Failed to get head.",
				"module", "broadcastd",
				"err", err,
				"retryIn", wait,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
				continue
			}
		}

		evs, err := broadcast.NewEventChain(head, batch)
		if err != nil {
			panic(err) // ulid.New() should never fail.
		}

		if _, err := w.broadcastJ.Commit(w.id, evs); err != nil {
			// XXX `Commit()` should return an error that indicates
			// concurrent update errors.  For now, assume it is a
			// concurrent update error, and quickly retry.
			millis := 10 + rand.Int63n(10)
			wait := time.Duration(millis) * time.Millisecond
			w.lg.Infow(
				"Retry committing broadcast batch.",
				"retryIn", wait,
				"err", err,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
				continue
			}
		}
		w.lg.Infow(
			"Committed broadcast batch.",
			"batchLen", len(evs),
		)

		clearBatch()
	}
}

func (w *journalWriter) post(ev *pb.BroadcastEvent) error {
	select {
	case w.ch <- ev:
		return nil
	default:
		return ErrPostQueueFull
	}
}
