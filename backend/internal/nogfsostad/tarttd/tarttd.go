package tarttd

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
	// `Server` allows concurrent operations, relying on `Processor` to
	// serialize operations per repo if necessary.
	proc  Processor
	authn auth.Authenticator
	authz auth.Authorizer
}

type Logger interface {
}

type Processor interface {
	GlobalRepoPath(repoId uuid.I) (string, bool)
	TarttHead(ctx context.Context, repoId uuid.I) (*pb.TarttHeadO, error)
	ListTars(
		ctx context.Context,
		repoId uuid.I,
		gitCommit []byte,
		fn shadows.ListTarsFunc,
	) error
	GetTarttconfig(
		ctx context.Context,
		repoId uuid.I,
		gitCommit []byte,
	) ([]byte, error)
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

func (srv *Server) TarttHead(
	ctx context.Context, i *pb.TarttHeadI,
) (*pb.TarttHeadO, error) {
	repoId, err := srv.authRepoId(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	head, err := srv.proc.TarttHead(ctx, repoId)
	if err != nil {
		err := status.Errorf(codes.Unknown, "git failed: %s", err)
		return nil, err
	}

	o := *head
	o.Repo = i.Repo
	return &o, nil
}

func (srv *Server) ListTars(
	ctx context.Context, i *pb.ListTarsI,
) (*pb.ListTarsO, error) {
	repoId, err := srv.authRepoId(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	o := &pb.ListTarsO{
		Repo: i.Repo,
	}

	commit := i.Commit
	if commit != nil {
		if err := checkGitCommitBytes(commit); err != nil {
			return nil, err
		}
	}
	if commit == nil {
		head, err := srv.proc.TarttHead(ctx, repoId)
		if err != nil {
			err := status.Errorf(
				codes.Unknown, "git failed: %s", err,
			)
			return nil, err
		}
		o.Author = head.Author
		o.Committer = head.Committer
		commit = head.Commit
	}
	o.Commit = commit

	if err := srv.proc.ListTars(
		ctx, repoId, commit,
		func(info pb.TarInfo) error {
			o.Tars = append(o.Tars, &info)
			return nil
		},
	); err != nil {
		return nil, err
	}
	return o, nil
}

func (srv *Server) GetTarttconfig(
	ctx context.Context, i *pb.GetTarttconfigI,
) (*pb.GetTarttconfigO, error) {
	repoId, err := srv.authRepoId(ctx, AAFsoReadRepo, i.Repo)
	if err != nil {
		return nil, err
	}

	o := &pb.GetTarttconfigO{
		Repo: i.Repo,
	}

	commit := i.Commit
	if commit != nil {
		if err := checkGitCommitBytes(commit); err != nil {
			return nil, err
		}
	}
	if commit == nil {
		head, err := srv.proc.TarttHead(ctx, repoId)
		if err != nil {
			err := status.Errorf(
				codes.Unknown, "git failed: %s", err,
			)
			return nil, err
		}
		o.Author = head.Author
		o.Committer = head.Committer
		commit = head.Commit
	}
	o.Commit = commit

	yml, err := srv.proc.GetTarttconfig(ctx, repoId, commit)
	if err != nil {
		return nil, err
	}
	o.ConfigYaml = yml

	return o, nil
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
