package discoveryd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/nogproject/nog/backend/internal/configmap"
	registryev "github.com/nogproject/nog/backend/internal/fsoregistry/pbevents"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// `ErrForeignRoot` is used to communicate internally that the view is not
// responsible for a root.
var ErrForeignRoot = errors.New("unknown root")

type Op int

const (
	OpUnspecified Op = iota
	OpInit
	OpWatch
)

// `registryView` watches multiple registries to maintain information about the
// roots that the server is responsible for.
type registryView struct {
	lg          Logger
	registries  []string
	prefixes    []string
	hosts       map[string]bool
	origin      pb.RegistryClient
	sysRPCCreds grpc.CallOption

	// `mu` protects state that is modified from `watchRegistry()` event
	// loading and accessed in getters; at least: `initHasCompleted`,
	// `roots`, `knownRepos`.
	mu sync.Mutex

	// `initHasCompleted` is initially `false`.  It becomes `true` after
	// the initial events have been loaded.
	initHasCompleted bool

	// `roots` are the naming rules by root.  Key: `globalRoot` with
	// trailing slash.
	roots map[string]*namingConfig

	// `knownRepos` are paths without trailing slash relative to a root.
	// Key: `globalRoot` with trailing slash.
	knownRepos map[string][]string

	// `repoPaths` contains the global repo path by ID.
	repoPaths map[uuid.I]string
	// `newRepoPaths` contains the new global repo path by ID while a repo
	// is moving.
	newRepoPaths map[uuid.I]string
}

type namingConfig struct {
	globalRoot string
	hostRoot   string
	rule       string
	ruleConfig map[string]interface{}
}

func newRegistryView(
	lg Logger, conn *grpc.ClientConn, cfg *Config,
) *registryView {
	var prefixes []string
	for _, p := range cfg.Prefixes {
		p = ensureTrailingSlash(p)
		prefixes = append(prefixes, p)
	}

	hosts := make(map[string]bool)
	for _, h := range cfg.Hosts {
		hosts[h] = true
	}

	return &registryView{
		lg:           lg,
		registries:   cfg.Registries,
		prefixes:     prefixes,
		hosts:        hosts,
		origin:       pb.NewRegistryClient(conn),
		sysRPCCreds:  grpc.PerRPCCredentials(cfg.SysRPCCreds),
		roots:        make(map[string]*namingConfig),
		knownRepos:   make(map[string][]string),
		repoPaths:    make(map[uuid.I]string),
		newRepoPaths: make(map[uuid.I]string),
	}
}

func (v *registryView) addRoot(cfg namingConfig) {
	v.mu.Lock()
	v.roots[cfg.globalRoot] = &cfg
	v.knownRepos[cfg.globalRoot] = make([]string, 0)
	v.mu.Unlock()
}

func (v *registryView) removeRoot(globalRoot string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, ok := v.roots[globalRoot]; !ok {
		return false
	}

	delete(v.roots, globalRoot)
	delete(v.knownRepos, globalRoot)
	return true
}

func (v *registryView) setRootNamingRule(
	globalRoot, rule string, config map[string]interface{},
) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	r, ok := v.roots[globalRoot]
	if !ok {
		return ErrForeignRoot
	}

	dup := *r
	dup.rule = rule
	dup.ruleConfig = config
	v.roots[globalRoot] = &dup

	return nil
}

func (v *registryView) patchRootNamingConfig(
	globalRoot string, configPatch map[string]interface{},
) (err error) {
	v.mu.Lock()
	root, ok := v.roots[globalRoot]
	v.mu.Unlock()
	if !ok {
		return ErrForeignRoot
	}

	dup := *root
	dup.ruleConfig, err = configmap.Merge(root.ruleConfig, configPatch)
	if err != nil {
		return err
	}

	v.mu.Lock()
	v.roots[globalRoot] = &dup
	v.mu.Unlock()

	return nil
}

func (v *registryView) addRepo(repoId uuid.I, globalPath string) bool {
	path := ensureTrailingSlash(globalPath)

	v.mu.Lock()
	defer v.mu.Unlock()

	root, relpath, ok := findRootRelpath(v.knownRepos, path)
	if !ok {
		return false
	}
	v.repoPaths[repoId] = path
	repos := v.knownRepos[root]
	v.knownRepos[root] = append(repos, relpath)
	return true
}

func (v *registryView) beginMoveRepo(
	repoId uuid.I, newGlobalPath string,
) bool {
	path := ensureTrailingSlash(newGlobalPath)

	v.mu.Lock()
	defer v.mu.Unlock()

	// Don't update `knownRepos` if path is unchanged.
	if path == v.repoPaths[repoId] {
		v.newRepoPaths[repoId] = path
		return true
	}

	root, relpath, ok := findRootRelpath(v.knownRepos, path)
	if !ok {
		return false
	}
	v.newRepoPaths[repoId] = path
	repos := v.knownRepos[root]
	v.knownRepos[root] = append(repos, relpath)
	return true
}

func (v *registryView) completeMoveRepo(repoId uuid.I) string {
	v.mu.Lock()
	defer v.mu.Unlock()

	oldPath := v.repoPaths[repoId]
	newPath := v.newRepoPaths[repoId]
	delete(v.newRepoPaths, repoId)

	// Everything is already up to date if the path is unchanged.
	if newPath == oldPath {
		return ""
	}

	// An empty new path indicates that we are no longer responsible.
	if newPath == "" {
		delete(v.repoPaths, repoId)
	} else {
		v.repoPaths[repoId] = newPath
	}

	// An empty old path indicates that we were not reponsibility.  So the
	// roots need no cleanup.
	if oldPath == "" {
		return ""
	}

	root, relpath, ok := findRootRelpath(v.knownRepos, oldPath)
	if !ok {
		return ""
	}
	// Remove old path from root.
	repos := v.knownRepos[root]
	newRepos := make([]string, 0, len(repos)-1)
	for _, r := range repos {
		if r != relpath {
			newRepos = append(newRepos, r)
		}
	}
	v.knownRepos[root] = newRepos
	return oldPath
}

func findRootRelpath(
	knownRepos map[string][]string, path string,
) (string, string, bool) {
	for root, _ := range knownRepos {
		if strings.HasPrefix(path, root) {
			relpath := strings.TrimPrefix(path, root)
			relpath = strings.TrimRight(relpath, "/")
			if relpath == "" {
				relpath = "."
			}
			return root, relpath, true
		}
	}
	return "", "", false
}

func (v *registryView) knownReposForRoot(globalRoot string) map[string]bool {
	globalRoot = ensureTrailingSlash(globalRoot)

	v.mu.Lock()
	lst := v.knownRepos[globalRoot]
	v.mu.Unlock()

	set := make(map[string]bool)
	for _, relpath := range lst {
		set[relpath] = true
	}
	return set
}

func (v *registryView) getNamingConfig(
	globalRoot string,
) (*namingConfig, error) {
	globalRoot = ensureTrailingSlash(globalRoot)

	v.mu.Lock()
	isReady := v.initHasCompleted
	c, ok := v.roots[globalRoot]
	v.mu.Unlock()

	if !isReady {
		err := status.Error(codes.Unavailable, "starting")
		return nil, err
	}

	if !ok {
		err := status.Errorf(
			codes.NotFound,
			"unknown root `%s`", globalRoot,
		)
		return nil, err
	}

	return c, nil
}

func (v *registryView) watch(ctx context.Context) error {
	// Init serially.  Then watch concurrently.
	type registryTail struct {
		name string
		tail []byte
	}
	regTails := make([]registryTail, 0, len(v.registries))
	for _, r := range v.registries {
		var tail []byte
		err := v.retryUntilCancel(ctx, "init registry", func() error {
			newTail, err := v.initRegistry(ctx, r)
			tail = newTail
			return err
		})
		if err != nil {
			return err
		}
		regTails = append(regTails, registryTail{
			name: r,
			tail: tail,
		})
	}
	v.completeInit()

	var wg sync.WaitGroup
	wg.Add(len(v.registries))
	watchForever := func(r registryTail) {
		defer wg.Done()
		tail := r.tail
		_ = v.retryUntilCancel(ctx, "watch registry", func() error {
			newTail, err := v.watchRegistry(ctx, r.name, tail)
			tail = newTail
			return err
		})
	}
	for _, r := range regTails {
		go watchForever(r)
	}

	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func (v *registryView) initRegistry(
	ctx context.Context, regName string,
) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req := &pb.RegistryEventsI{
		Registry: regName,
	}
	stream, err := v.origin.Events(ctx, req, v.sysRPCCreds)
	if err != nil {
		return nil, err
	}

	var tail []byte
	for {
		rsp, err := stream.Recv()
		if err == io.EOF {
			return tail, nil
		}
		if err != nil {
			return tail, err
		}
		for _, ev := range rsp.Events {
			if err := v.processEvent(OpInit, ev); err != nil {
				return tail, err
			}
			tail = ev.Id
		}
	}
}

func (v *registryView) completeInit() {
	v.mu.Lock()
	v.initHasCompleted = true
	nRoots := len(v.roots)
	nRepos := 0
	for _, rs := range v.knownRepos {
		nRepos += len(rs)
	}
	v.mu.Unlock()

	v.lg.Infow(
		"Initial loading of registry view completed.",
		"module", "discoveryd",
		"nRoots", nRoots,
		"nRepos", nRepos,
	)
}

func (v *registryView) watchRegistry(
	ctx context.Context,
	registry string, tail []byte,
) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req := &pb.RegistryEventsI{
		Registry: registry,
		Watch:    true,
		After:    tail,
	}
	stream, err := v.origin.Events(ctx, req, v.sysRPCCreds)
	if err != nil {
		return tail, err
	}

	v.lg.Infow(
		"Started watching registry to track discovery roots.",
		"module", "discoveryd",
		"registry", registry,
	)

	for {
		rsp, err := stream.Recv()
		if err != nil {
			return tail, err
		}
		for _, ev := range rsp.Events {
			if err := v.processEvent(OpWatch, ev); err != nil {
				return tail, err
			}
			tail = ev.Id
		}
	}
}

func (v *registryView) processEvent(op Op, ev *pb.RegistryEvent) error {
	switch ev.Event {
	case pb.RegistryEvent_EV_FSO_ROOT_ADDED:
		inf := ev.FsoRootInfo
		globalRoot := ensureTrailingSlash(inf.GlobalRoot)
		if !hasPrefixAny(globalRoot, v.prefixes) {
			v.lg.Infow(
				"Ignored root with foreign prefix.",
				"root", globalRoot,
				"module", "discoveryd",
			)
			return nil
		}
		if !v.hosts[inf.Host] {
			v.lg.Infow(
				"Ignored root with foreign host.",
				"root", globalRoot,
				"module", "discoveryd",
			)
			return nil
		}

		v.addRoot(namingConfig{
			globalRoot: globalRoot,
			hostRoot:   inf.HostRoot,
		})
		if op == OpWatch {
			v.lg.Infow(
				"Enabled root.",
				"root", globalRoot,
				"module", "discoveryd",
			)
		}
		return nil

	case pb.RegistryEvent_EV_EPHEMERAL_WORKFLOWS_ENABLED:
		return nil

	case pb.RegistryEvent_EV_FSO_ROOT_REMOVED:
		inf := ev.FsoRootInfo
		globalRoot := ensureTrailingSlash(inf.GlobalRoot)
		if v.removeRoot(globalRoot) {
			if op == OpWatch {
				v.lg.Infow(
					"Disabled root.",
					"root", globalRoot,
					"module", "discoveryd",
				)
			}
		}
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_NAMING_UPDATED:
		naming := ev.FsoRepoNaming
		globalRoot := ensureTrailingSlash(naming.GlobalRoot)
		cfg, err := configmap.ParsePb(naming.Config)
		if err != nil {
			v.lg.Errorw(
				"Ignored naming with invalid config",
				"root", globalRoot,
				"rule", naming.Rule,
				"config", naming.Config,
				"module", "discoveryd",
			)
			return nil
		}

		err = v.setRootNamingRule(globalRoot, naming.Rule, cfg)
		switch {
		case err == ErrForeignRoot:
			v.lg.Infow(
				"Ignored foreign root repo naming.",
				"root", globalRoot,
				"module", "discoveryd",
			)
			return nil
		case err != nil:
			panic("unexpected error")
		}
		if op == OpWatch {
			v.lg.Infow(
				"Set naming config.",
				"root", globalRoot,
				"rule", naming.Rule,
				"module", "discoveryd",
			)
		}
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_NAMING_CONFIG_UPDATED:
		v.processEventNamingConfigUpdated(ev)
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_ADDED:
		repoId, err := uuid.FromBytes(ev.FsoRepoInfo.Id)
		if err != nil {
			return err
		}
		globalPath := ev.FsoRepoInfo.GlobalPath
		if !v.addRepo(repoId, globalPath) {
			v.lg.Infow(
				"Ignored repo with foreign prefix.",
				"repo", globalPath,
				"module", "discoveryd",
			)
			return nil
		}
		if op == OpWatch {
			v.lg.Infow(
				"Enabled repo.",
				"repo", globalPath,
				"module", "discoveryd",
			)
		}
		return nil

	default:
		// continue with next switch.
	}

	regEv, err := registryev.FromPbValidate(*ev)
	if err != nil {
		v.lg.Errorw(
			"Ignored decode registry event error.",
			"err", err,
		)
	}
	switch x := regEv.(type) {
	// `RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED`.
	case *registryev.EvRepoMoveAccepted:
		repoId := x.RepoId
		newGlobalPath := x.NewGlobalPath
		if !v.beginMoveRepo(repoId, newGlobalPath) {
			v.lg.Infow(
				"Ignored moved repo with foreign prefix.",
				"repoId", repoId,
				"repo", newGlobalPath,
				"module", "discoveryd",
			)
			return nil
		}
		if op == OpWatch {
			v.lg.Infow(
				"Enabled moved repo.",
				"repo", newGlobalPath,
				"module", "discoveryd",
			)
		}
		return nil

	// `RegistryEvent_EV_FSO_REPO_MOVED`.
	case *registryev.EvRepoMoved:
		repoId := x.RepoId
		oldPath := v.completeMoveRepo(repoId)
		if oldPath == "" {
			return nil
		}
		if op == OpWatch {
			v.lg.Infow(
				"Disabled old path of moved repo.",
				"repoId", repoId,
				"oldPath", oldPath,
				"module", "discoveryd",
			)
		}
		return nil

	default:
		// continue with next switch.
	}

	// Ignore unrelated.
	switch ev.Event {
	// EV_FSO_ROOT_UPDATED may enable or disable GitLab, but it
	// cannot change the repo naming convention.
	case pb.RegistryEvent_EV_FSO_ROOT_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_INIT_POLICY_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_REGISTRY_ADDED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_ACCEPTED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED:
		return nil
	case pb.RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		return nil
	case pb.RegistryEvent_EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_ACL_POLICY_UPDATED:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore splitrootwf:
	switch ev.Event {
	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_ENABLED:
		return nil
	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_PARAMS_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_DISABLED:
		return nil
	case pb.RegistryEvent_EV_FSO_PATH_FLAG_SET:
		return nil
	case pb.RegistryEvent_EV_FSO_PATH_FLAG_UNSET:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore freezerepowf.
	switch ev.Event {
	case pb.RegistryEvent_EV_FSO_FREEZE_REPO_STARTED_2:
		return nil
	case pb.RegistryEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore unfreezerepowf.
	switch ev.Event {
	case pb.RegistryEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
		return nil
	case pb.RegistryEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore archiverepowf.
	switch ev.Event {
	case pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_STARTED:
		return nil
	case pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore unarchiverepowf.
	switch ev.Event {
	case pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
		return nil
	case pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
		return nil
	default:
		// continue with next switch.
	}

	v.lg.Warnw(
		"Ignored unknown registry event.",
		"module", "discoveryd",
		"event", ev.Event.String(),
	)
	return nil
}

// Maintain a list of ignore patterns, ignoring any other config.
func (v *registryView) processEventNamingConfigUpdated(ev *pb.RegistryEvent) {
	patch := ev.FsoRepoNaming
	globalRoot := ensureTrailingSlash(patch.GlobalRoot)

	cfgPatch, err := configmap.ParsePb(patch.Config)
	if err != nil {
		v.lg.Errorw(
			"Ignored naming patch with invalid config",
			"root", globalRoot,
			"rule", patch.Rule,
			"config", patch.Config,
			"module", "discoveryd",
		)
		return
	}

	err = v.patchRootNamingConfig(globalRoot, cfgPatch)
	switch {
	case err == ErrForeignRoot:
		v.lg.Infow(
			"Ignored foreign root repo naming config patch.",
			"root", globalRoot,
			"module", "discoveryd",
		)
	case err != nil:
		v.lg.Errorw(
			"Ignored failed naming config patch",
			"err", err,
			"root", globalRoot,
			"module", "discoveryd",
		)
	}
}

func (v *registryView) retryUntilCancel(
	ctx context.Context, what string, fn func() error,
) error {
	for {
		err := fn()
		if err == nil {
			return nil
		}

		// Check canceled before logging retry.
		select {
		default: // non-blocking
		case <-ctx.Done():
			return ctx.Err()
		}
		wait := 20 * time.Second
		v.lg.Errorw(
			fmt.Sprintf("Will retry %s.", what),
			"module", "discoveryd",
			"err", err,
			"retryIn", wait,
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
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

func hasPrefixAny(path string, prefixes []string) bool {
	for _, pfx := range prefixes {
		if strings.HasPrefix(path, pfx) {
			return true
		}
	}
	return false
}
