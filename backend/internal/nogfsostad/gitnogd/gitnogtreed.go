package gitnogd

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *Server) ListStatTree(
	i *pb.ListStatTreeI, ostream pb.GitNogTree_ListStatTreeServer,
) error {
	ctx := ostream.Context()

	repoId, err := srv.authRepoId(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return err
	}

	if err := checkGitCommitBytes(i.StatGitCommit); err != nil {
		return err
	}

	const batchSize = 64
	o := &pb.ListStatTreeO{}
	clear := func() {
		o.Paths = make([]*pb.PathInfo, 0, batchSize)
	}
	clear()

	flush := func() error {
		err := ostream.Send(o)
		clear()
		return err
	}

	maybeFlush := func() error {
		if len(o.Paths) < batchSize {
			return nil
		}
		return flush()
	}

	if err := srv.proc.ListStatTree(
		ctx, repoId, i.StatGitCommit, i.Prefix,
		func(info pb.PathInfo) error {
			o.Paths = append(o.Paths, &info)
			return maybeFlush()
		},
	); err != nil {
		return err
	}

	return flush()
}

func (srv *Server) ListMetaTree(
	i *pb.ListMetaTreeI, ostream pb.GitNogTree_ListMetaTreeServer,
) error {
	ctx := ostream.Context()

	repoId, err := srv.authRepoId(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return err
	}

	if err := checkGitCommitBytes(i.MetaGitCommit); err != nil {
		return err
	}

	const batchSize = 64
	o := &pb.ListMetaTreeO{}
	clear := func() {
		o.Paths = make([]*pb.PathMetadata, 0, batchSize)
	}
	clear()

	flush := func() error {
		err := ostream.Send(o)
		clear()
		return err
	}

	maybeFlush := func() error {
		if len(o.Paths) < batchSize {
			return nil
		}
		return flush()
	}

	if err := srv.proc.ListMetaTree(
		ctx, repoId, i.MetaGitCommit,
		func(pm pb.PathMetadata) error {
			o.Paths = append(o.Paths, &pm)
			return maybeFlush()
		},
	); err != nil {
		return err
	}

	return flush()
}

func (srv *Server) PutPathMetadata(
	ctx context.Context, i *pb.PutPathMetadataI,
) (*pb.PutPathMetadataO, error) {
	repoId, err := srv.authRepoId(ctx, AAFsoWriteRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	summary, err := srv.proc.GitNogPutPathMetadata(ctx, repoId, i)
	if err != nil {
		err := status.Errorf(codes.Unknown, "git failed: %s", err)
		return nil, err
	}

	o := *summary
	o.Repo = i.Repo
	return &o, nil
}

func checkGitCommitBytes(b []byte) error {
	if b == nil {
		err := status.Error(
			codes.InvalidArgument, "missing git commit",
		)
		return err
	}
	if len(b) != 20 {
		err := status.Error(
			codes.InvalidArgument,
			"malformed git commit: expected 20 bytes",
		)
		return err
	}
	return nil
}
