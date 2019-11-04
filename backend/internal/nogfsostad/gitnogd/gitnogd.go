package gitnogd

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad/shadows"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	lg Logger
	// `Server` allows parallel operations, relying on `Processor` to
	// serialize operations per repo.
	proc  Processor
	authn auth.Authenticator
	authz auth.Authorizer
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
}

type Processor interface {
	GlobalRepoPath(repoId uuid.I) (string, bool)

	GitNogHead(ctx context.Context, repoId uuid.I) (*pb.HeadO, error)
	GitNogSummary(ctx context.Context, repoId uuid.I) (*pb.SummaryO, error)
	GitNogMeta(ctx context.Context, repoId uuid.I) (*pb.MetaO, error)

	GitNogPutPathMetadata(
		ctx context.Context, repoId uuid.I, i *pb.PutPathMetadataI,
	) (*pb.PutPathMetadataO, error)

	GitNogContent(
		ctx context.Context, repoId uuid.I, path string,
	) (*pb.ContentO, error)

	ListStatTree(
		ctx context.Context,
		repoId uuid.I,
		gitCommit []byte,
		prefix string,
		fn shadows.ListStatTreeFunc,
	) error

	ListMetaTree(
		ctx context.Context,
		repoId uuid.I,
		gitCommit []byte,
		fn shadows.ListMetaTreeFunc,
	) error
}

func New(
	lg Logger,
	authn auth.Authenticator,
	authz auth.Authorizer,
	proc Processor,
) *Server {
	return &Server{
		lg:    lg,
		proc:  proc,
		authn: authn,
		authz: authz,
	}
}

func (srv *Server) Head(ctx context.Context, i *pb.HeadI) (*pb.HeadO, error) {
	repoId, err := srv.authRepoId(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	head, err := srv.proc.GitNogHead(ctx, repoId)
	if err != nil {
		err := status.Errorf(codes.Unknown, "git failed: %s", err)
		return nil, err
	}

	o := *head
	o.Repo = i.Repo
	return &o, nil
}

func (srv *Server) Summary(
	ctx context.Context, i *pb.SummaryI,
) (*pb.SummaryO, error) {
	repoId, err := srv.authRepoId(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	summary, err := srv.proc.GitNogSummary(ctx, repoId)
	if err != nil {
		err := status.Errorf(codes.Unknown, "git failed: %s", err)
		return nil, err
	}

	o := *summary
	o.Repo = i.Repo
	return &o, nil
}

func (srv *Server) Meta(ctx context.Context, i *pb.MetaI) (*pb.MetaO, error) {
	repoId, err := srv.authRepoId(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	summary, err := srv.proc.GitNogMeta(ctx, repoId)
	if err != nil {
		err := status.Errorf(codes.Unknown, "git failed: %s", err)
		return nil, err
	}

	o := *summary
	o.Repo = i.Repo
	return &o, nil
}

func (srv *Server) PutMeta(
	ctx context.Context, i *pb.PutMetaI,
) (*pb.PutMetaO, error) {
	// `PutPathMetadata()` calls `authRepoId()`.
	o, err := srv.PutPathMetadata(ctx, &pb.PutPathMetadataI{
		Repo:            i.Repo,
		OldGitNogCommit: i.OldCommitId,
		AuthorName:      i.AuthorName,
		AuthorEmail:     i.AuthorEmail,
		CommitMessage:   i.CommitMessage,
		PathMetadata: []*pb.PathMetadata{
			&pb.PathMetadata{
				Path:         ".", // Repo root.
				MetadataJson: i.MetaJson,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &pb.PutMetaO{
		Repo:          o.Repo,
		GitNogCommit:  o.GitNogCommit,
		IsNewCommit:   o.IsNewCommit,
		GitCommits:    o.GitCommits,
		MetaAuthor:    o.MetaAuthor,
		MetaCommitter: o.MetaCommitter,
	}, nil
}

func (srv *Server) Content(
	ctx context.Context, i *pb.ContentI,
) (*pb.ContentO, error) {
	repoId, err := srv.authRepoId(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	content, err := srv.proc.GitNogContent(ctx, repoId, i.Path)
	if err != nil {
		err := status.Errorf(codes.Unknown, "git failed: %s", err)
		return nil, err
	}

	o := *content
	o.Repo = i.Repo
	return &o, nil
}
