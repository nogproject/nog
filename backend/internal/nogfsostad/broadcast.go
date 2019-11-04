package nogfsostad

import (
	"context"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad/shadows"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var postTimeout = 2 * time.Second

type Broadcaster struct {
	rpc         pb.GitBroadcasterClient
	sysRPCCreds grpc.CallOption
}

func NewBroadcaster(
	lg Logger,
	conn *grpc.ClientConn,
	sysRPCCreds credentials.PerRPCCredentials,
) *Broadcaster {
	return &Broadcaster{
		rpc:         pb.NewGitBroadcasterClient(conn),
		sysRPCCreds: grpc.PerRPCCredentials(sysRPCCreds),
	}
}

func (b *Broadcaster) PostGitStatUpdated(
	ctx context.Context, repoId uuid.I, newHead shadows.Oid,
) error {
	const ref = "refs/heads/master-stat"
	return b.postGitRefUpdated(ctx, &pb.PostGitRefUpdatedI{
		Repo:   repoId[:],
		Ref:    ref,
		Commit: newHead[:],
	})
}

func (b *Broadcaster) PostGitShaUpdated(
	ctx context.Context, repoId uuid.I, newHead shadows.Oid,
) error {
	const ref = "refs/heads/master-sha"
	return b.postGitRefUpdated(ctx, &pb.PostGitRefUpdatedI{
		Repo:   repoId[:],
		Ref:    ref,
		Commit: newHead[:],
	})
}

func (b *Broadcaster) PostGitContentUpdated(
	ctx context.Context, repoId uuid.I, newHead shadows.Oid,
) error {
	const ref = "refs/heads/master-content"
	return b.postGitRefUpdated(ctx, &pb.PostGitRefUpdatedI{
		Repo:   repoId[:],
		Ref:    ref,
		Commit: newHead[:],
	})
}

func (b *Broadcaster) PostGitMetaUpdated(
	ctx context.Context, repoId uuid.I, newHead shadows.Oid,
) error {
	const ref = "refs/heads/master-meta"
	return b.postGitRefUpdated(ctx, &pb.PostGitRefUpdatedI{
		Repo:   repoId[:],
		Ref:    ref,
		Commit: newHead[:],
	})
}

func (b *Broadcaster) postGitRefUpdated(
	ctx context.Context, i *pb.PostGitRefUpdatedI,
) error {
	ctx2, cancel2 := context.WithTimeout(ctx, postTimeout)
	defer cancel2()
	_, err := b.rpc.PostGitRefUpdated(ctx2, i, b.sysRPCCreds)
	return err
}
