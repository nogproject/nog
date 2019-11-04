package scan

import (
	"context"
	"fmt"
	"math/rand"
	slashpath "path"
	"strings"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsoschd/execute"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// See `Processor` in `../observer/observer.go`.
type Processor interface {
	ProcessRepo(ctx context.Context, repo *execute.Repo) error
}

type Config struct {
	Conn       *grpc.ClientConn
	RPCCreds   credentials.PerRPCCredentials
	Processor  Processor
	Registries []string
	Prefixes   []string
	Hosts      []string
}

type Scanner struct {
	lg         Logger
	conn       *grpc.ClientConn
	rpcCreds   grpc.CallOption
	proc       Processor
	registries []string
	prefixes   []string
	hosts      map[string]struct{}
}

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

func NewScanner(lg Logger, cfg *Config) *Scanner {
	var prefixes []string
	for _, p := range cfg.Prefixes {
		prefixes = append(prefixes, slashpath.Clean(p))
	}

	hosts := make(map[string]struct{})
	for _, h := range cfg.Hosts {
		hosts[h] = struct{}{}
	}

	return &Scanner{
		lg:         lg,
		conn:       cfg.Conn,
		proc:       cfg.Processor,
		rpcCreds:   grpc.PerRPCCredentials(cfg.RPCCreds),
		registries: cfg.Registries,
		prefixes:   prefixes,
		hosts:      hosts,
	}
}

func (s *Scanner) Scan(ctx context.Context) error {
	regs := randRotateStrings(s.registries)
	for _, r := range regs {
		if err := s.scanRegistry(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scanner) scanRegistry(ctx context.Context, registry string) error {
	s.lg.Infow("Started registry scan.", "registry", registry)

	c := pb.NewRegistryClient(s.conn)
	rsp, err := c.GetRepos(
		ctx,
		&pb.GetReposI{
			Registry: registry,
		},
		s.rpcCreds,
	)
	if err != nil {
		return err
	}

	for _, r := range randRotateRepos(rsp.Repos) {
		if err := s.scanRepo(ctx, r); err != nil {
			return err
		}
	}

	s.lg.Infow("Completed registry scan.", "registry", registry)
	return nil
}

func (s *Scanner) scanRepo(ctx context.Context, inf *pb.RepoInfo) error {
	// Silently ignore other prefix.
	if !pathIsEqualOrBelowPrefixAny(inf.GlobalPath, s.prefixes) {
		return nil
	}

	repoId, err := uuid.FromBytes(inf.Id)
	if err != nil {
		return err
	}

	c := pb.NewReposClient(s.conn)
	repo, err := c.GetRepo(ctx, &pb.GetRepoI{Repo: repoId[:]}, s.rpcCreds)
	if err != nil {
		return err
	}

	// Report host mismatch and ignore.
	host := strings.SplitN(repo.File, ":", 2)[0]
	if _, ok := s.hosts[host]; !ok {
		s.lg.Warnw(
			"Ignored prefix matched but host not.",
			"repoId", repoId.String(),
			"globalPath", repo.GlobalPath,
			"file", repo.File,
		)
		return nil
	}

	repoVid, err := ulid.ParseBytes(repo.Vid)
	if err != nil {
		return err
	}
	return s.proc.ProcessRepo(ctx, &execute.Repo{
		Id:                     repoId,
		Vid:                    repoVid,
		Registry:               repo.Registry,
		GlobalPath:             repo.GlobalPath,
		File:                   repo.File,
		Shadow:                 repo.Shadow,
		Archive:                repo.Archive,
		ArchiveRecipients:      asHexs(repo.ArchiveRecipients),
		ShadowBackup:           repo.ShadowBackup,
		ShadowBackupRecipients: asHexs(repo.ShadowBackupRecipients),
	})
}

func asHexs(ds [][]byte) []string {
	ss := make([]string, 0, len(ds))
	for _, d := range ds {
		ss = append(ss, fmt.Sprintf("%X", d))
	}
	return ss
}

// `prefix` without trailing slash.
func pathIsEqualOrBelowPrefix(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	// Equal or slash right after prefix.
	return len(path) == len(prefix) || path[len(prefix)] == '/'
}

// `prefixes` without trailing slash.
func pathIsEqualOrBelowPrefixAny(path string, prefixes []string) bool {
	for _, pfx := range prefixes {
		if pathIsEqualOrBelowPrefix(path, pfx) {
			return true
		}
	}
	return false
}

func randRotateStrings(s []string) []string {
	if len(s) == 0 {
		return s
	}
	i := rand.Intn(len(s))
	return append(s[i:], s[0:i]...)
}

func randRotateRepos(infs []*pb.RepoInfo) []*pb.RepoInfo {
	if len(infs) == 0 {
		return infs
	}
	i := rand.Intn(len(infs))
	return append(infs[i:], infs[0:i]...)
}
