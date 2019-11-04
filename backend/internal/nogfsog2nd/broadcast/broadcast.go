package broadcast

import (
	"context"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

var postTimeout = 2 * time.Second

type Broadcaster struct {
	rpc pb.GitBroadcasterClient
}

func NewBroadcaster(conn *grpc.ClientConn) *Broadcaster {
	return &Broadcaster{
		rpc: pb.NewGitBroadcasterClient(conn),
	}
}

func (b *Broadcaster) PostGitMetaUpdated(
	ctx context.Context, repoId uuid.I, newHead []byte,
) error {
	const ref = "refs/heads/master-meta"
	return b.postGitRefUpdated(ctx, &pb.PostGitRefUpdatedI{
		Repo:   repoId[:],
		Ref:    ref,
		Commit: newHead,
	})
}

func (b *Broadcaster) postGitRefUpdated(
	ctx context.Context, i *pb.PostGitRefUpdatedI,
) error {
	ctx2, cancel2 := context.WithTimeout(ctx, postTimeout)
	defer cancel2()
	_, err := b.rpc.PostGitRefUpdated(ctx2, i)
	return err
}
