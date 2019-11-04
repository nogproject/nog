package workflowproc

import (
	"context"

	"github.com/nogproject/nog/backend/internal/process/grpclazy"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const ConfigMaxStreams = 20
const ConfigMaxConcurrentTarttRestores = 1

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

type Limiter interface {
	Acquire(ctx context.Context, n int64) error
	Release(n int64)
}

type Config struct {
	Registries  []string
	Prefixes    []string
	Hosts       []string
	CapPath     string
	Conn        *grpc.ClientConn
	SysRPCCreds credentials.PerRPCCredentials
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
	tarttLimiter := semaphore.NewWeighted(ConfigMaxConcurrentTarttRestores)

	engine := grpclazy.NewEngine(
		lg,
		&grpclazy.EngineConfig{
			Conn:          cfg.Conn,
			SysRPCCreds:   cfg.SysRPCCreds,
			StreamLimiter: streamLimiter,
		},
	)

	expectedHosts := make(map[string]struct{})
	for _, h := range cfg.Hosts {
		expectedHosts[h] = struct{}{}
	}

	registries := make([]*indexActivity, 0, len(cfg.Registries))
	for _, r := range cfg.Registries {
		registries = append(registries, &indexActivity{
			lg:             lg,
			registry:       r,
			prefixes:       prefixes,
			expectedHosts:  expectedHosts,
			capPath:        cfg.CapPath,
			conn:           cfg.Conn,
			sysRPCCreds:    grpc.PerRPCCredentials(cfg.SysRPCCreds),
			workflowEngine: engine,
			tarttLimiter:   tarttLimiter,
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
