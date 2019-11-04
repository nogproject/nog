package discoveryd

import (
	"context"
	slashpath "path"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad/discoveryd/rules"
	"github.com/nogproject/nog/backend/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

type Config struct {
	Registries           []string
	Prefixes             []string
	Hosts                []string
	StdtoolsProjectsRoot string
	Authenticator        auth.Authenticator
	Authorizer           auth.Authorizer
	SysRPCCreds          credentials.PerRPCCredentials
}

type Server struct {
	lg                   Logger
	authn                auth.Authenticator
	authz                auth.Authorizer
	registryView         *registryView
	stdtoolsProjectsRoot string
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Infow(msg string, kv ...interface{})
}

// `conn` is connected to a `nogfso.Registry` service.
func New(lg Logger, conn *grpc.ClientConn, cfg *Config) *Server {
	return &Server{
		lg:                   lg,
		authn:                cfg.Authenticator,
		authz:                cfg.Authorizer,
		registryView:         newRegistryView(lg, conn, cfg),
		stdtoolsProjectsRoot: cfg.StdtoolsProjectsRoot,
	}
}

func (srv *Server) Watch(ctx context.Context) error {
	return srv.registryView.watch(ctx)
}

func (srv *Server) FindUntracked(
	i *pb.FindUntrackedI, ostream pb.Discovery_FindUntrackedServer,
) error {
	ctx := ostream.Context()
	globalRoot := slashpath.Clean(i.GlobalRoot)

	if err := srv.authPath(ctx, AAFsoFind, globalRoot); err != nil {
		return err
	}

	cfg, err := srv.registryView.getNamingConfig(globalRoot)
	if err != nil {
		return err
	}
	known := srv.registryView.knownReposForRoot(globalRoot)

	finder, err := srv.newFinder(cfg.rule, cfg.ruleConfig)
	if err != nil {
		return err
	}

	const batchSize = 64
	o := &pb.FindUntrackedO{}
	clear := func() {
		o.Candidates = make([]string, 0, batchSize)
		o.Ignored = make([]string, 0, batchSize)
	}
	clear()

	flush := func() error {
		err := ostream.Send(o)
		clear()
		return err
	}

	maybeFlush := func() error {
		if len(o.Candidates)+len(o.Ignored) < batchSize {
			return nil
		}
		return flush()
	}

	if err := finder.Find(cfg.hostRoot, known, rules.FindHandlerFuncs{
		CandidateFn: func(relpath string) error {
			o.Candidates = append(o.Candidates, relpath)
			return maybeFlush()
		},
		IgnoreFn: func(relpath string) error {
			o.Ignored = append(o.Ignored, relpath)
			return maybeFlush()
		},
	}); err != nil {
		err := status.Errorf(
			codes.Unknown,
			"failed to find files: %s", err,
		)
		return err
	}

	return flush()
}
