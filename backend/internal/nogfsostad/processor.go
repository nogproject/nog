package nogfsostad

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/privileges"
	"github.com/nogproject/nog/backend/internal/nogfsostad/shadows"
	"github.com/nogproject/nog/backend/internal/nogfsostad/statd"
	"github.com/nogproject/nog/backend/pkg/lockmap"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

var ErrNoSudo = errors.New("no sudo privileges")

type GitUser struct {
	Name  string
	Email string
}

type GitNogWritePolicy int

const (
	GitNogWriteUnspecified GitNogWritePolicy = iota

	// `GitNogWriteAlways` always allows `PutMeta()`.  It is the preferred
	// solution.  See NOE-13.
	GitNogWriteAlways

	// `GitNogWriteUnlessGitlab` could be an alternative as discussed in
	// NOE-13.  Writes would be forbidden here and only allowed via GitLab
	// REST.  Do not use `GitNogWriteUnlessGitlab` without prior design
	// discussion.
	GitNogWriteUnlessGitlab
)

type Processor struct {
	lg          Logger
	shadow      *shadows.Filesystem
	broadcaster *Broadcaster
	privs       Privileges
	useUdo      UseUdo

	gitNogWritePolicy GitNogWritePolicy
	initLimits        *InitLimits

	mu         sync.Mutex
	repos      map[uuid.I]repoInfo
	waitEnable map[uuid.I]chan struct{}

	repoLocks lockmap.L
}

type Privileges interface {
	privileges.UdoChattrPrivileges
	privileges.UdoRenamePrivileges
}

type UseUdo struct {
	Rename bool
}

type repoInfo struct {
	globalPath string
	hostPath   string
	shadowPath string
	gitSsh     string
}

func NewProcessor(
	lg Logger,
	initLimits *InitLimits,
	shadow *shadows.Filesystem,
	broadcaster *Broadcaster,
	privs Privileges,
	useUdo UseUdo,
) *Processor {
	return &Processor{
		lg:          lg,
		shadow:      shadow,
		broadcaster: broadcaster,
		privs:       privs,
		useUdo:      useUdo,

		// `GitNogWriteAlways` is the preferred policy.  See above and
		// NOE-13.
		gitNogWritePolicy: GitNogWriteAlways,
		initLimits:        initLimits,

		repos:      make(map[uuid.I]repoInfo),
		waitEnable: make(map[uuid.I]chan struct{}),
	}
}

func (p *Processor) LockRepo4(
	ctx context.Context, repoId uuid.I,
) error {
	key := string(repoId[:])
	return p.repoLocks.Lock(ctx, key)
}

func (p *Processor) UnlockRepo4(repoId uuid.I) {
	key := string(repoId[:])
	p.repoLocks.Unlock(key)
}

// `EnableRepo4()` is for `observer4/observer.go:Observer`.
func (p *Processor) EnableRepo4(
	ctx context.Context, inf *RepoInfo,
) error {
	key := string(inf.Id[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	p.mu.Lock()
	p.repos[inf.Id] = repoInfo{
		globalPath: inf.GlobalPath,
		hostPath:   inf.HostPath,
		shadowPath: inf.ShadowPath,
		gitSsh:     inf.GitlabSsh,
	}
	done := p.waitEnable[inf.Id]
	if done != nil {
		close(done)
		p.waitEnable[inf.Id] = nil
	}
	p.mu.Unlock()

	p.lg.Infow(
		"Enabled repo",
		"repoId", inf.Id.String(),
		"repo", inf.GlobalPath,
		"module", "nogfsostad",
	)
	return nil
}

// `WaitEnableRepo4()` waits until the repo has been enabled.  It is used to
// synchronize workflow processing during startup.  Specifically if a
// freeze-repo workflow is restarted, it needs to wait until the repo is
// enabled before it can call `FreezeRepo()`.  It calls `WaitEnableRepo4()` to
// ensure that.
//
// `WaitEnableRepo4()` is only useful for startup.  It does not handle
// disabling repos later.  It may return immediately for such a repo even if
// the repo is currently disabled.
func (p *Processor) WaitEnableRepo4(
	ctx context.Context, id uuid.I,
) error {
	done := p.waitEnableRepo4Chan(id)
	if done == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (p *Processor) waitEnableRepo4Chan(id uuid.I) chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If the repo is known, return immediately.
	_, ok := p.repos[id]
	if ok {
		return nil
	}

	// Otherwise check whether there is a done channel; create one if
	// necessary.  A nil channel indicates that there was a done channel
	// that has already been closed by `EnableRepo4()`.
	done, ok := p.waitEnable[id]
	if !ok {
		done = make(chan struct{})
		p.waitEnable[id] = done
	}
	return done
}

// `DisableRepo4()` is for `observer4/observer.go:Observer`.
func (p *Processor) DisableRepo4(
	ctx context.Context, repoId uuid.I,
) error {
	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	p.mu.Lock()
	inf, ok := p.repos[repoId]
	delete(p.repos, repoId)
	p.mu.Unlock()

	if !ok {
		return nil
	}
	p.lg.Infow(
		"Disabled repo",
		"repoId", repoId.String(),
		"repo", inf.globalPath,
		"module", "nogfsostad",
	)
	return nil
}

func (p *Processor) GlobalRepoPath(repoId uuid.I) (string, bool) {
	p.mu.Lock()
	inf, ok := p.repos[repoId]
	p.mu.Unlock()
	if !ok {
		return "", false
	}
	return inf.globalPath, true
}

func (p *Processor) IsKnownRepo(repoId uuid.I) bool {
	p.mu.Lock()
	_, ok := p.repos[repoId]
	p.mu.Unlock()
	return ok
}

func (p *Processor) ResolveGlobaPathInLocalRepo(
	globalPath string,
) (repo, sub string, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	l := 0
	for _, r := range p.repos {
		if !strings.HasPrefix(globalPath, r.globalPath) {
			continue
		}
		if len(r.globalPath) <= l {
			continue
		}
		l = len(r.globalPath)
		repo = r.hostPath
		sub = strings.TrimLeft(globalPath[len(r.globalPath):], "/")
	}
	if sub == "" {
		sub = "."
	}
	return repo, sub, (l > 0)
}

func (p *Processor) StatStatus(
	ctx context.Context,
	repoId uuid.I,
	fn shadows.StatStatusFunc,
) error {
	// Take the repo lock, because `git-fso status` modifies the Git config
	// to enable the stat Git filter.
	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return err
	}

	return p.shadow.StatStatus(ctx, shadowPath, fn)
}

func (p *Processor) StatRepo(
	ctx context.Context,
	repoId uuid.I,
	author statd.User,
	opts shadows.StatOptions,
) error {
	p.mu.Lock()
	inf, ok := p.repos[repoId]
	p.mu.Unlock()
	if !ok {
		err := fmt.Errorf("unknown repo `%s`", repoId)
		return err
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	p.lg.Infow("Begin stat.", "shadow", inf.shadowPath)

	const ref = "refs/heads/master-stat"
	oldHead, err := p.shadow.Ref(inf.shadowPath, ref)
	if err != nil {
		return asWeakError(err)
	}

	// XXX Should have ctx.
	err = p.shadow.Stat(inf.shadowPath, shadows.User(author), opts)
	if err != nil {
		return asStrongError(err)
	}

	// Skip push if the remote is empty.
	if inf.gitSsh != "" {
		// XXX Should have ctx.
		err = p.shadow.Push(inf.shadowPath, inf.gitSsh)
		if err != nil {
			return asWeakError(err)
		}
		p.lg.Infow("Pushed git.", "gitRemote", inf.gitSsh)
	} else {
		p.lg.Infow(
			"Skipped push to empty remote.",
			"repoId", repoId.String(),
		)
	}

	newHead, err := p.shadow.Ref(inf.shadowPath, ref)
	if err != nil {
		return asWeakError(err)
	}

	if newHead != oldHead {
		err := p.broadcaster.PostGitStatUpdated(ctx, repoId, newHead)
		if err != nil {
			p.lg.Warnw("Failed to broadcast.", "err", err)
		}
	}

	return nil
}

func (p *Processor) StatMtimeRangeOnlyAllRepos(
	ctx context.Context,
	author statd.User,
) error {
	var err error
	statOpts := shadows.StatOptions{
		MtimeRangeOnly: true,
	}
	for _, id := range p.getAllRepoIds() {
		err2 := p.StatRepo(ctx, id, author, statOpts)
		if err2 != nil {
			if err == nil {
				err = err2
			}
			if err2 == context.Canceled {
				break
			}
			p.lg.Errorw(
				"Stat mtime range failed.",
				"repoId", id.String(),
				"err", err,
			)
		} else {
			p.lg.Infow(
				"Completed stat mtime range.",
				"repoId", id.String(),
			)
		}
	}
	return err
}

func (p *Processor) ShaRepo(
	ctx context.Context, repoId uuid.I, author statd.User,
) error {
	p.mu.Lock()
	inf, ok := p.repos[repoId]
	p.mu.Unlock()
	if !ok {
		err := fmt.Errorf("unknown repo `%s`", repoId)
		return err
	}

	// Take only the sha lock during the potentially slow SHA computation.
	key := string(repoId[:])
	keySha := key + ".sha"
	if err := p.repoLocks.Lock(ctx, keySha); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(keySha)

	p.lg.Infow("Begin sha.", "shadow", inf.shadowPath)

	const ref = "refs/heads/master-sha"
	oldHead, err := p.shadow.Ref(inf.shadowPath, ref)
	if err != nil {
		return asWeakError(err)
	}

	// XXX Should have ctx.
	err = p.shadow.Sha(inf.shadowPath, shadows.User(author))
	if err != nil {
		return asStrongError(err)
	}

	// Then also take the main lock.
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	// Skip push if the remote is empty.
	if inf.gitSsh != "" {
		// XXX Should have ctx.
		err = p.shadow.Push(inf.shadowPath, inf.gitSsh)
		if err != nil {
			return asWeakError(err)
		}
		p.lg.Infow("Pushed git.", "gitRemote", inf.gitSsh)
	} else {
		p.lg.Infow(
			"Skipped push to empty remote.",
			"repoId", repoId.String(),
		)
	}

	newHead, err := p.shadow.Ref(inf.shadowPath, ref)
	if err != nil {
		return asWeakError(err)
	}

	if newHead != oldHead {
		err := p.broadcaster.PostGitShaUpdated(ctx, repoId, newHead)
		if err != nil {
			p.lg.Warnw("Failed to broadcast.", "err", err)
		}
	}

	return nil
}

func (p *Processor) RefreshContent(
	ctx context.Context, repoId uuid.I, author statd.User,
) error {
	p.mu.Lock()
	inf, ok := p.repos[repoId]
	p.mu.Unlock()
	if !ok {
		err := fmt.Errorf("unknown repo `%s`", repoId)
		return err
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	p.lg.Infow("Begin refresh content.", "shadow", inf.shadowPath)

	const ref = "refs/heads/master-content"
	oldHead, err := p.shadow.Ref(inf.shadowPath, ref)
	_ = err // oldHead may be null.

	// XXX Should have ctx.
	err = p.shadow.RefreshContent(inf.shadowPath, shadows.User(author))
	if err != nil {
		return asStrongError(err)
	}

	// Skip push if the remote is empty.
	if inf.gitSsh != "" {
		// XXX Should have ctx.
		err = p.shadow.Push(inf.shadowPath, inf.gitSsh)
		if err != nil {
			return asWeakError(err)
		}
		p.lg.Infow("Pushed git.", "gitRemote", inf.gitSsh)
	} else {
		p.lg.Infow(
			"Skipped push to empty remote.",
			"repoId", repoId.String(),
		)
	}

	newHead, err := p.shadow.Ref(inf.shadowPath, ref)
	if err != nil {
		return asWeakError(err)
	}

	if newHead != oldHead {
		err := p.broadcaster.PostGitContentUpdated(
			ctx, repoId, newHead,
		)
		if err != nil {
			p.lg.Warnw("Failed to broadcast.", "err", err)
		}
	}

	return nil
}

func (p *Processor) FreezeRepo(
	ctx context.Context, repoId uuid.I, author GitUser,
) error {
	if err := p.chattrSetImmutable(ctx, repoId); err != nil {
		return err
	}

	statOpts := shadows.StatOptions{}
	if err := p.StatRepo(
		ctx, repoId, statd.User(author), statOpts,
	); err != nil {
		return err
	}

	return nil
}

func (p *Processor) chattrSetImmutable(
	ctx context.Context, repoId uuid.I,
) error {
	if p.privs == nil {
		return ErrNoSudo
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	p.mu.Lock()
	inf, ok := p.repos[repoId]
	p.mu.Unlock()
	if !ok {
		err := fmt.Errorf("unknown repo `%s`", repoId)
		return err
	}

	sudo, err := p.privs.AcquireUdoChattr(ctx, "root")
	if err != nil {
		return err
	}
	defer sudo.Release()
	return sudo.ChattrTreeSetImmutable(ctx, inf.hostPath)
}

func (p *Processor) UnfreezeRepo(
	ctx context.Context, repoId uuid.I, author GitUser,
) error {
	if err := p.chattrUnsetImmutable(ctx, repoId); err != nil {
		return err
	}

	statOpts := shadows.StatOptions{}
	if err := p.StatRepo(
		ctx, repoId, statd.User(author), statOpts,
	); err != nil {
		return err
	}

	return nil
}

func (p *Processor) chattrUnsetImmutable(
	ctx context.Context, repoId uuid.I,
) error {
	if p.privs == nil {
		return ErrNoSudo
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	p.mu.Lock()
	inf, ok := p.repos[repoId]
	p.mu.Unlock()
	if !ok {
		err := fmt.Errorf("unknown repo `%s`", repoId)
		return err
	}

	sudo, err := p.privs.AcquireUdoChattr(ctx, "root")
	if err != nil {
		return err
	}
	defer sudo.Release()
	return sudo.ChattrTreeUnsetImmutable(ctx, inf.hostPath)
}

func (p *Processor) ReinitSubdirTracking(
	ctx context.Context,
	repoId uuid.I,
	author statd.User,
	subdirTracking pb.SubdirTracking,
) error {
	var tracking shadows.SubdirTracking
	switch subdirTracking {
	case pb.SubdirTracking_ST_ENTER_SUBDIRS:
		tracking = shadows.EnterSubdirs
	case pb.SubdirTracking_ST_BUNDLE_SUBDIRS:
		tracking = shadows.BundleSubdirs
	case pb.SubdirTracking_ST_IGNORE_SUBDIRS:
		tracking = shadows.IgnoreSubdirs
	case pb.SubdirTracking_ST_IGNORE_MOST:
		tracking = shadows.IgnoreMost
	default:
		err := errors.New("invalid subdir tracking")
		return err
	}

	p.mu.Lock()
	inf, ok := p.repos[repoId]
	p.mu.Unlock()
	if !ok {
		err := fmt.Errorf("unknown repo `%s`", repoId)
		return err
	}

	lim := p.initLimits.find(inf.globalPath)
	reason := checkInitLimit(
		subdirTracking, inf.hostPath, lim,
	)
	if reason != "" {
		err := fmt.Errorf(
			"subdir tracking limits violation: %v", reason,
		)
		return err
	}

	// Take the sha and the main lock in the same order as in Sha().
	key := string(repoId[:])
	keySha := key + ".sha"
	if err := p.repoLocks.Lock(ctx, keySha); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(keySha)
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	p.lg.Infow("Begin reinit subdir tracking.", "shadow", inf.shadowPath)

	const ref = "refs/heads/master-stat"
	oldHead, err := p.shadow.Ref(inf.shadowPath, ref)
	if err != nil {
		return asWeakError(err)
	}

	err = p.shadow.ReinitSubdirTracking(
		inf.shadowPath, shadows.User(author), tracking,
	)
	if err != nil {
		return asStrongError(err)
	}

	newHead, err := p.shadow.Ref(inf.shadowPath, ref)
	if err != nil {
		return asWeakError(err)
	}
	if newHead != oldHead {
		err := p.broadcaster.PostGitStatUpdated(ctx, repoId, newHead)
		if err != nil {
			p.lg.Warnw("Failed to broadcast.", "err", err)
		}
	}

	return nil
}

func (p *Processor) GitNogHead(
	ctx context.Context, repoId uuid.I,
) (*pb.HeadO, error) {
	// Quick check before taking the repo lock.
	_, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return nil, err
	}
	defer p.repoLocks.Unlock(key)

	// Double check when holding the repo lock.  InitRepo() may have
	// updated the repo info in the meantime.
	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}

	return p.shadow.Head(ctx, shadowPath)
}

func (p *Processor) GitNogSummary(
	ctx context.Context, repoId uuid.I,
) (*pb.SummaryO, error) {
	_, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return nil, err
	}
	defer p.repoLocks.Unlock(key)

	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}

	return p.shadow.Summary(ctx, shadowPath)
}

func (p *Processor) GitNogMeta(
	ctx context.Context, repoId uuid.I,
) (*pb.MetaO, error) {
	_, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return nil, err
	}
	defer p.repoLocks.Unlock(key)

	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}

	return p.shadow.Meta(ctx, shadowPath)
}

func (p *Processor) GitNogContent(
	ctx context.Context, repoId uuid.I, path string,
) (*pb.ContentO, error) {
	_, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return nil, err
	}
	defer p.repoLocks.Unlock(key)

	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}

	return p.shadow.Content(ctx, shadowPath, path)
}

func (p *Processor) ListStatTree(
	ctx context.Context,
	repoId uuid.I,
	gitCommit []byte,
	prefix string,
	fn shadows.ListStatTreeFunc,
) error {
	_, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return err
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return err
	}

	return p.shadow.ListStatTree(ctx, shadowPath, gitCommit, prefix, fn)
}

func (p *Processor) ListMetaTree(
	ctx context.Context,
	repoId uuid.I,
	gitCommit []byte,
	fn shadows.ListMetaTreeFunc,
) error {
	_, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return err
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return err
	}

	return p.shadow.ListMetaTree(ctx, shadowPath, gitCommit, fn)
}

func (p *Processor) GitNogPutPathMetadata(
	ctx context.Context, repoId uuid.I, i *pb.PutPathMetadataI,
) (*pb.PutPathMetadataO, error) {
	_, err := p.gitNogShadowReadWrite(repoId)
	if err != nil {
		return nil, err
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return nil, err
	}
	defer p.repoLocks.Unlock(key)

	inf, err := p.gitNogShadowReadWrite(repoId)
	if err != nil {
		return nil, err
	}
	shadowPath := inf.shadowPath
	gitSsh := inf.gitSsh

	o, err := p.shadow.PutPathMetadata(ctx, shadowPath, i)
	if err != nil {
		return nil, err
	}
	if !o.IsNewCommit {
		return o, nil
	}

	if gitSsh != "" {
		err = p.shadow.Push(shadowPath, gitSsh)
		// Log errors but do not report them to the caller.  The write
		// succeeded to the local filesystem and the commit is
		// permanent, although it will not be immediately visible via
		// GitLab.  But any operation that triggers a push later will
		// eventually write it to GitLab.  A client can compare the
		// `gitNogCommit` to detect that GitLab is not up-to-date.
		//
		// Alternative: Add details to `PutMetaO` that tell the client
		// about the partial success.
		if err != nil {
			p.lg.Errorw(
				"Failed to push.",
				"err", err,
				"gitRemote", gitSsh,
			)
		}
	}

	const ref = "refs/heads/master-meta"
	newHead, err := p.shadow.Ref(shadowPath, ref)
	if err != nil {
		p.lg.Warnw("Failed to broadcast.", "err", err)
		return o, nil
	}

	err = p.broadcaster.PostGitMetaUpdated(ctx, repoId, newHead)
	if err != nil {
		p.lg.Warnw("Failed to broadcast.", "err", err)
		return o, nil
	}

	return o, nil
}

func (p *Processor) TarttHead(
	ctx context.Context, repoId uuid.I,
) (*pb.TarttHeadO, error) {
	// It should be safe to read master-tartt without repo lock.
	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}

	return p.shadow.TarttHead(ctx, shadowPath)
}

func (p *Processor) ListTars(
	ctx context.Context,
	repoId uuid.I,
	gitCommit []byte,
	fn shadows.ListTarsFunc,
) error {
	// It should be safe to read master-tartt without repo lock.
	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return err
	}

	return p.shadow.ListTars(ctx, shadowPath, gitCommit, fn)
}

func (p *Processor) GetTarttconfig(
	ctx context.Context,
	repoId uuid.I,
	gitCommit []byte,
) ([]byte, error) {
	// It should be safe to read master-tartt without repo lock.
	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}
	return p.shadow.GetTarttconfig(ctx, shadowPath, gitCommit)
}

func (p *Processor) TarttIsFrozenArchive(
	ctx context.Context, repoId uuid.I,
) (*shadows.TarttIsFrozenArchiveInfo, error) {
	// Take the lock to avoid any potential race, although reading master
	// branches without lock should be safe.
	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return nil, err
	}
	defer p.repoLocks.Unlock(key)

	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return nil, err
	}

	return p.shadow.TarttIsFrozenArchive(ctx, shadowPath)
}

func (p *Processor) ArchiveRepo(
	ctx context.Context,
	repoId uuid.I,
	workingDir string,
	author GitUser,
) error {
	if p.privs == nil {
		return ErrNoSudo
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	inf, err := p.gitNogShadowRead(repoId)
	if err != nil {
		return err
	}
	shadowPath := inf.shadowPath
	hostPath := inf.hostPath
	placeholder := filepath.Join(workingDir, "placeholder")
	trash := filepath.Join(workingDir, "trash")

	// Sudo is always used for chattr.
	sudoChattr, err := p.privs.AcquireUdoChattr(ctx, "root")
	if err != nil {
		return err
	}
	defer sudoChattr.Release()

	// Renames may be done with or without sudo.  If using sudo, Nogfsostad
	// does not need write access to the realdir, because all writes happen
	// through Nogfsostasuod.  Without sudo, renames succeed based on the
	// assumption that it is forbidden to archive repos that have the same
	// path as the root.
	rename := os.Rename
	if p.useUdo.Rename {
		sudoRename, err := p.privs.AcquireUdoRename(ctx, "root")
		if err != nil {
			return err
		}
		defer sudoRename.Release()
		rename = func(src, dst string) error {
			return sudoRename.Rename(ctx, src, dst)
		}
	}

	// If trash does not exist, the realdir must still be at its original
	// location.  Move it.
	if _, err := os.Stat(trash); os.IsNotExist(err) {
		if err := sudoChattr.ChattrUnsetImmutable(
			ctx, hostPath,
		); err != nil {
			return err
		}
		if err := rename(hostPath, trash); err != nil {
			return err
		}
		if err := fsyncPath(workingDir); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// chattr +i outside the previous if block to handle restart.
	if err := sudoChattr.ChattrSetImmutable(ctx, trash); err != nil {
		return err
	}
	// No fsync(), because we do not care whether trash is really
	// immutable.  It will be garbage collected anyway.

	// If the realdir does not exist, the placeholder must still be at its
	// original location.  Move it.
	if _, err := os.Stat(hostPath); os.IsNotExist(err) {
		if err := sudoChattr.ChattrUnsetImmutable(
			ctx, placeholder,
		); err != nil {
			return err
		}
		if err := rename(placeholder, hostPath); err != nil {
			return err
		}
		if err := fsyncPath(filepath.Dir(hostPath)); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// chattr +i outside the previous if block to handle restart.
	if err := sudoChattr.ChattrSetImmutable(ctx, hostPath); err != nil {
		return err
	}
	if err := fsyncPath(hostPath); err != nil {
		return err
	}

	return p.shadow.Archive(shadowPath, shadows.User(author))
}

func (p *Processor) UnarchiveRepo(
	ctx context.Context,
	repoId uuid.I,
	workingDir string,
	author GitUser,
) error {
	if p.privs == nil {
		return ErrNoSudo
	}

	key := string(repoId[:])
	if err := p.repoLocks.Lock(ctx, key); err != nil {
		return err
	}
	defer p.repoLocks.Unlock(key)

	inf, err := p.gitNogShadowRead(repoId)
	if err != nil {
		return err
	}
	hostPath := inf.hostPath
	restore := filepath.Join(workingDir, "restore")
	trash := filepath.Join(workingDir, "trash")

	// Sudo is always used for chattr.
	sudoChattr, err := p.privs.AcquireUdoChattr(ctx, "root")
	if err != nil {
		return err
	}
	defer sudoChattr.Release()

	// Renames may be done with or without sudo.  If using sudo, Nogfsostad
	// does not need write access to the realdir, because all writes happen
	// through Nogfsostasuod.  Without sudo, renames succeed based on the
	// assumption that it is forbidden to archive repos that have the same
	// path as the root.
	rename := os.Rename
	if p.useUdo.Rename {
		sudoRename, err := p.privs.AcquireUdoRename(ctx, "root")
		if err != nil {
			return err
		}
		defer sudoRename.Release()
		rename = func(src, dst string) error {
			return sudoRename.Rename(ctx, src, dst)
		}
	}

	// If trash does not exist, the realdir placeholder must still be at
	// its original location.  Move it.
	if _, err := os.Stat(trash); os.IsNotExist(err) {
		if err := sudoChattr.ChattrUnsetImmutable(
			ctx, hostPath,
		); err != nil {
			return err
		}
		if err := rename(hostPath, trash); err != nil {
			return err
		}
		if err := fsyncPath(workingDir); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// chattr +i outside the previous if block to handle restart.
	if err := sudoChattr.ChattrSetImmutable(ctx, trash); err != nil {
		return err
	}
	// No fsync(), because we do not care whether trash is really
	// immutable.  It will be garbage collected anyway.

	// If the realdir does not exist, the restore must still be at its
	// original location.  Move it.
	if _, err := os.Stat(hostPath); os.IsNotExist(err) {
		if err := sudoChattr.ChattrUnsetImmutable(
			ctx, restore,
		); err != nil {
			return err
		}
		if err := rename(restore, hostPath); err != nil {
			return err
		}
		if err := fsyncPath(filepath.Dir(hostPath)); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// chattr +i outside the previous if-block to handle restart.
	if err := sudoChattr.ChattrSetImmutable(ctx, hostPath); err != nil {
		return err
	}
	if err := fsyncPath(hostPath); err != nil {
		return err
	}

	// Do not run `git-fso stat`, because there is nothing to do.  The
	// realdir is immutable, and `master-stat` has already recorded its
	// state.
	//
	// Some kind of status check could be useful to double check that the
	// expected files have been restored.  But such a check would ideally
	// run before swapping the dirs.
	return nil
}

func fsyncPath(p string) error {
	fp, err := os.Open(p)
	if err != nil {
		return err
	}
	err = fp.Sync()
	_ = fp.Close()
	return err
}

func (p *Processor) GitGcAll(ctx context.Context) error {
	var err error
	for _, id := range p.getAllRepoIds() {
		err2 := p.gitGc(ctx, id)
		if err2 != nil {
			if err == nil {
				err = err2
			}
			if err2 == context.Canceled {
				break
			}
			p.lg.Errorw(
				"git gc failed.",
				"repoId", id.String(),
				"err", err,
			)
		} else {
			p.lg.Infow("Completed git gc.", "repoId", id.String())
		}
	}
	return err
}

func (p *Processor) gitGc(ctx context.Context, repoId uuid.I) error {
	shadowPath, err := p.gitNogShadowPathRead(repoId)
	if err != nil {
		return err
	}
	return p.shadow.GitGc(ctx, shadowPath)
}

func (p *Processor) gitNogShadowPathRead(repoId uuid.I) (string, error) {
	inf, err := p.gitNogShadowRead(repoId)
	if err != nil {
		return "", err
	}
	return inf.shadowPath, nil
}

func (p *Processor) gitNogShadowRead(repoId uuid.I) (*repoInfo, error) {
	p.mu.Lock()
	inf, ok := p.repos[repoId]
	p.mu.Unlock()
	if !ok {
		err := fmt.Errorf("unknown repo `%s`", repoId)
		return nil, err
	}
	return &inf, nil
}

func (p *Processor) gitNogShadowReadWrite(repoId uuid.I) (*repoInfo, error) {
	p.mu.Lock()
	inf, ok := p.repos[repoId]
	p.mu.Unlock()
	if !ok {
		err := fmt.Errorf("unknown repo `%s`", repoId)
		return nil, err
	}

	switch p.gitNogWritePolicy {
	case GitNogWriteAlways:
		// Proceed unconditionally.

	case GitNogWriteUnlessGitlab:
		if inf.gitSsh != "" {
			err := errors.New("use nogfsog2nd instead")
			return nil, err
		}

	default:
		panic("invalid gitNogWritePolicy")
	}

	return &inf, nil
}

func (p *Processor) getAllRepoIds() []uuid.I {
	p.mu.Lock()
	ids := make([]uuid.I, 0, len(p.repos))
	for id, _ := range p.repos {
		ids = append(ids, id)
	}
	p.mu.Unlock()
	return ids
}
