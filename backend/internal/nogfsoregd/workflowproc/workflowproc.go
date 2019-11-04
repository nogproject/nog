package workflowproc

import (
	"context"
	"strings"

	"github.com/nogproject/nog/backend/internal/process/grpclazy"
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

type Config struct {
	Registries  []string
	Conn        *grpc.ClientConn
	SysRPCCreds credentials.PerRPCCredentials
}

type Processor struct {
	lg         Logger
	registries []*indexActivity
	engine     *grpclazy.Engine
}

func New(lg Logger, cfg *Config) *Processor {
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
			lg:             lg,
			registry:       r,
			conn:           cfg.Conn,
			sysRPCCreds:    grpc.PerRPCCredentials(cfg.SysRPCCreds),
			workflowEngine: engine,
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

func errorContainsAny(err error, substrs []string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, substr := range substrs {
		if strings.Contains(msg, substr) {
			return true
		}
	}
	return false
}
