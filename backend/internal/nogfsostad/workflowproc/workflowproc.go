package workflowproc

import (
	"context"

	"github.com/nogproject/nog/backend/internal/nogfsostad"
	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/privileges"
	"github.com/nogproject/nog/backend/internal/nogfsostad/shadows"
	"github.com/nogproject/nog/backend/internal/process/grpclazy"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const ConfigMaxStreams = 20

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

type RepoProcessor interface {
	WaitEnableRepo4(ctx context.Context, id uuid.I) error
	FreezeRepo(
		ctx context.Context, repoId uuid.I, author nogfsostad.GitUser,
	) error
	UnfreezeRepo(
		ctx context.Context, repoId uuid.I, author nogfsostad.GitUser,
	) error
	TarttIsFrozenArchive(
		ctx context.Context, repoId uuid.I,
	) (*shadows.TarttIsFrozenArchiveInfo, error)
	ArchiveRepo(
		ctx context.Context,
		repoId uuid.I,
		workfingDir string,
		author nogfsostad.GitUser,
	) error
	UnarchiveRepo(
		ctx context.Context,
		repoId uuid.I,
		workfingDir string,
		author nogfsostad.GitUser,
	) error
}

type AclPropagator interface {
	PropagateAcls(ctx context.Context, src, dst string) error
}

type Privileges interface {
	privileges.UdoChattrPrivileges
}

type ArchiveRepoPrivileges interface {
	privileges.UdoChattrPrivileges
}

type UnarchiveRepoPrivileges interface {
	privileges.UdoChattrPrivileges
}

type Config struct {
	Registries         []string
	Prefixes           []string
	Conn               *grpc.ClientConn
	SysRPCCreds        credentials.PerRPCCredentials
	RepoProcessor      RepoProcessor
	Privileges         Privileges
	AclPropagator      AclPropagator
	ArchiveRepoSpool   string
	UnarchiveRepoSpool string
}

type Processor struct {
	lg         Logger
	registries []*indexActivity
	engine     *grpclazy.Engine
}

func New(lg Logger, cfg *Config) *Processor {
	var prefixes []string
	for _, p := range cfg.Prefixes {
		prefixes = append(prefixes, ensureTrailingSlash(p))
	}

	streamLimiter := semaphore.NewWeighted(ConfigMaxStreams)

	engine := grpclazy.NewEngine(
		lg,
		&grpclazy.EngineConfig{
			Conn:          cfg.Conn,
			SysRPCCreds:   cfg.SysRPCCreds,
			StreamLimiter: streamLimiter,
		},
	)

	registries := make([]*indexActivity, 0, len(cfg.Registries))
	for _, r := range cfg.Registries {
		registries = append(registries, &indexActivity{
			lg:                 lg,
			registry:           r,
			prefixes:           prefixes,
			conn:               cfg.Conn,
			sysRPCCreds:        grpc.PerRPCCredentials(cfg.SysRPCCreds),
			workflowEngine:     engine,
			repoProc:           cfg.RepoProcessor,
			privs:              cfg.Privileges,
			aclPropagator:      cfg.AclPropagator,
			archiveRepoSpool:   cfg.ArchiveRepoSpool,
			unarchiveRepoSpool: cfg.UnarchiveRepoSpool,
		})
	}

	return &Processor{
		lg:         lg,
		registries: registries,
		engine:     engine,
	}
}

func (p *Processor) Run(ctx context.Context) error {
	p.engine.SetContext(ctx)

	for _, r := range p.registries {
		err := p.engine.StartRegistryWorkflowIndexActivity(
			r.registry, r,
		)
		if err != nil {
			return err
		}
	}

	return p.engine.Run()
}

func ensureTrailingSlash(s string) string {
	if s == "" {
		return "/"
	}
	if s[len(s)-1] == '/' {
		return s
	}
	return s + "/"
}
