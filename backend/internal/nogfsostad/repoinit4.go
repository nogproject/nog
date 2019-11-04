package nogfsostad

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad/gits"
	"github.com/nogproject/nog/backend/internal/nogfsostad/shadows"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type RepoInitializer4 struct {
	lg          Logger
	proc        *Processor
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	shadow      *shadows.Filesystem
	broadcaster *Broadcaster
	gitlab      *gits.Gitlab
	hosts       map[string]bool
}

func NewRepoInitializer4(
	lg Logger,
	proc *Processor,
	conn *grpc.ClientConn,
	sysRPCCreds credentials.PerRPCCredentials,
	hosts []string,
	shadow *shadows.Filesystem,
	broadcaster *Broadcaster,
	gitlab *gits.Gitlab,
) *RepoInitializer4 {
	hset := make(map[string]bool)
	for _, h := range hosts {
		hset[h] = true
	}
	return &RepoInitializer4{
		lg:          lg,
		proc:        proc,
		conn:        conn,
		sysRPCCreds: grpc.PerRPCCredentials(sysRPCCreds),
		shadow:      shadow,
		broadcaster: broadcaster,
		gitlab:      gitlab,
		hosts:       hset,
	}
}

func (ri *RepoInitializer4) GetRepo(
	ctx context.Context, repoId uuid.I,
) (*RepoInfo, error) {
	c := pb.NewReposClient(ri.conn)
	o, err := c.GetRepo(
		ctx,
		&pb.GetRepoI{
			Repo: repoId[:],
		},
		ri.sysRPCCreds,
	)
	if err != nil {
		return nil, err
	}

	vid, err := ulid.ParseBytes(o.Vid)
	if err != nil {
		return nil, asStrongError(err)
	}

	parseHostPath := func() (string, error) {
		toks := strings.SplitN(o.File, ":", 2)
		if len(toks) != 2 {
			err := fmt.Errorf("invalid file location `%s`", o.File)
			return "", asStrongError(err)
		}
		host := toks[0]
		if !ri.hosts[host] {
			err := fmt.Errorf("invalid host `%s`", host)
			return "", asStrongError(err)
		}
		return toks[1], nil
	}
	hostPath, err := parseHostPath()
	if err != nil {
		return nil, err
	}

	parseShadowPath := func() (string, error) {
		toks := strings.SplitN(o.Shadow, ":", 2)
		if len(toks) != 2 {
			err := fmt.Errorf(
				"invalid shadow location `%s`", o.Shadow,
			)
			return "", asStrongError(err)
		}
		host := toks[0]
		if !ri.hosts[host] {
			err := fmt.Errorf("invalid host `%s`", host)
			return "", asStrongError(err)
		}
		return toks[1], nil
	}
	shadowPath, err := parseShadowPath()
	if err != nil {
		return nil, err
	}

	var gitlabSsh string
	if o.Gitlab != "" {
		gitlabSsh = fmt.Sprintf("git@%s.git", o.Gitlab)
	}

	return &RepoInfo{
		Id:         repoId,
		Vid:        vid,
		GlobalPath: o.GlobalPath,
		HostPath:   hostPath,
		ShadowPath: shadowPath,
		GitlabSsh:  gitlabSsh,
	}, nil
}

func (ri *RepoInitializer4) EnableGitlab(
	ctx context.Context, repoId uuid.I,
) (*RepoInfo, error) {
	return ri.InitRepo(ctx, repoId)
}

func (ri *RepoInitializer4) InitRepo(
	ctx context.Context, repoId uuid.I,
) (*RepoInfo, error) {
	c := pb.NewReposClient(ri.conn)
	stream, err := c.Events(
		ctx,
		&pb.RepoEventsI{
			Repo: repoId[:],
		},
		ri.sysRPCCreds,
	)
	if err != nil {
		return nil, err
	}

	var initInfo *pb.FsoRepoInitInfo
	var shadowInfo *pb.FsoShadowRepoInfo
	var gitInfo *pb.FsoGitRepoInfo
	var repoError string

	handleEvent := func(ev *pb.RepoEvent) {
		switch ev.Event {
		case pb.RepoEvent_EV_FSO_REPO_INIT_STARTED:
			initInfo = ev.FsoRepoInitInfo

		case pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED:
			dup := *initInfo
			dup.GitlabHost = ev.FsoRepoInitInfo.GitlabHost
			dup.GitlabPath = ev.FsoRepoInitInfo.GitlabPath
			initInfo = &dup

		case pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED:
			shadowInfo = ev.FsoShadowRepoInfo

		case pb.RepoEvent_EV_FSO_GIT_REPO_CREATED:
			gitInfo = ev.FsoGitRepoInfo

		case pb.RepoEvent_EV_FSO_GIT_TO_NOG_CLONED:
			// Ignore unrelated.

		case pb.RepoEvent_EV_FSO_REPO_ERROR_SET:
			repoError = ev.FsoRepoErrorMessage

		case pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED:
			repoError = ""

		default:
			// Ignore unknown.
		}
	}

	var vid ulid.I
	for {
		rsp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		for _, ev := range rsp.Events {
			handleEvent(ev)
			v, err := ulid.ParseBytes(ev.Id)
			if err != nil {
				return nil, asStrongError(err)
			}
			vid = v
		}
	}

	if repoError != "" {
		err := fmt.Errorf("stored repo error: %s", repoError)
		return nil, asStoredError(err)
	}

	if initInfo == nil {
		err := fmt.Errorf("missing repo init info")
		return nil, err
	}

	globalPath := initInfo.GlobalPath
	host := initInfo.FileHost
	if !ri.hosts[host] {
		err := fmt.Errorf("invalid host `%s`", host)
		return nil, err
	}

	// Some if blocks below require a repo lock.  For simplicity, we
	// unconditionally acquire the lock here.  Locking could be pushed down
	// to the if blocks to lock only if necessary.
	if err := ri.proc.LockRepo4(ctx, repoId); err != nil {
		return nil, err
	}
	defer ri.proc.UnlockRepo4(repoId)

	hostPath := initInfo.HostPath
	var shadowPath string
	if shadowInfo == nil {
		author := shadows.User{
			Name:  initInfo.CreatorName,
			Email: initInfo.CreatorEmail,
		}

		ri.lg.Infow(
			"Begin init shadow.",
			"hostPath", hostPath,
		)
		initOpts := shadows.InitOptions{}
		switch initInfo.SubdirTracking {
		case pb.SubdirTracking_ST_UNSPECIFIED:
			fallthrough
		case pb.SubdirTracking_ST_ENTER_SUBDIRS:
			initOpts.SubdirTracking = shadows.EnterSubdirs
		case pb.SubdirTracking_ST_BUNDLE_SUBDIRS:
			initOpts.SubdirTracking = shadows.BundleSubdirs
		case pb.SubdirTracking_ST_IGNORE_SUBDIRS:
			initOpts.SubdirTracking = shadows.IgnoreSubdirs
		case pb.SubdirTracking_ST_IGNORE_MOST:
			initOpts.SubdirTracking = shadows.IgnoreMost
		default:
			ri.lg.Warnw(
				"Unknown pb `initInfo.SubdirTracking`; "+
					"using EnterSubdirs.",
				"pb", initInfo.SubdirTracking.String(),
			)
			initOpts.SubdirTracking = shadows.EnterSubdirs
		}
		si, err := ri.shadow.Init(hostPath, author, repoId, initOpts)
		if err != nil {
			return nil, asStrongError(err)
		}
		ri.lg.Infow(
			"Completed init shadow.",
			"hostPath", hostPath,
			"shadow", si.ShadowPath,
		)

		// time.Sleep(2 * time.Second) // Can be useful during testing.

		ri.lg.Infow(
			"Begin initial stat.",
			"shadow", si.ShadowPath,
		)
		err = ri.shadow.Stat(
			si.ShadowPath, author, shadows.StatOptions{},
		)
		if err != nil {
			return nil, asStrongError(err)
		}
		ri.lg.Infow(
			"Completed initial stat.",
			"shadow", si.ShadowPath,
		)

		if _, err := c.ConfirmShadow(
			ctx,
			&pb.ConfirmShadowI{
				Repo:       repoId[:],
				ShadowPath: si.ShadowPath,
			},
			ri.sysRPCCreds,
		); err != nil {
			return nil, err
		}
		ri.lg.Infow(
			"Confirmed init shadow.",
			"hostPath", hostPath,
			"shadow", si.ShadowPath,
		)

		const ref = "refs/heads/master-stat"
		head, err := ri.shadow.Ref(si.ShadowPath, ref)
		if err != nil {
			return nil, asWeakError(err)
		}
		err = ri.broadcaster.PostGitStatUpdated(ctx, repoId, head)
		if err != nil {
			ri.lg.Warnw("Failed to broadcast.", "err", err)
		}

		shadowPath = si.ShadowPath
	} else {
		shadowPath = shadowInfo.ShadowPath
	}

	gitlabHost := initInfo.GitlabHost
	gitlabPath := initInfo.GitlabPath
	var gitlabSsh string
	if gitlabHost != "" {
		gitlabSsh = fmt.Sprintf(
			"git@%s:%s.git", gitlabHost, gitlabPath,
		)
	}

	switch {
	case gitlabSsh == "": // No Gitlab config.
	case gitInfo != nil: // Already initialized.
	default: // Otherwise initialize:
		ri.lg.Infow(
			"Begin init GitLab project.",
			"gitlabPath", gitlabPath,
		)

		if ri.gitlab == nil {
			err := fmt.Errorf("GitLab disabled")
			return nil, asStrongError(err)
		}

		inf, err := ri.gitlab.Init(&gits.RepoInfo{
			GitlabHostname: gitlabHost,
			GitlabPath:     gitlabPath,
		})
		if err != nil {
			return nil, asWeakError(err)
		}
		if inf.GitlabSsh != gitlabSsh {
			err := fmt.Errorf("GitLab SSH URL mismatch")
			return nil, asStrongError(err)
		}
		ri.lg.Infow(
			"Completed init GitLab project.",
			"gitlabPath", gitlabPath,
			"gitlabId", inf.GitlabId,
		)

		// time.Sleep(2 * time.Second) // Can be useful during testing.

		ri.lg.Infow(
			"Begin initial push.",
			"gitRemote", gitlabSsh,
		)
		err = ri.shadow.Push(shadowPath, gitlabSsh)
		if err != nil {
			return nil, asWeakError(err)
		}
		ri.lg.Infow(
			"Completed initial push.",
			"gitRemote", gitlabSsh,
		)

		_, err = c.ConfirmGit(
			ctx,
			&pb.ConfirmGitI{
				Repo:            repoId[:],
				GitlabProjectId: int64(inf.GitlabId),
			},
			ri.sysRPCCreds,
		)
		if err != nil {
			return nil, err
		}
		ri.lg.Infow(
			"Confirmed init GitLab project.",
			"gitlabPath", gitlabPath,
		)
	}

	return &RepoInfo{
		Id:         repoId,
		Vid:        vid,
		GlobalPath: globalPath,
		HostPath:   hostPath,
		ShadowPath: shadowPath,
		GitlabSsh:  gitlabSsh,
	}, nil
}

func (ri *RepoInitializer4) MoveRepo(
	ctx context.Context,
	repoId uuid.I,
	oldHostPath string,
	oldShadowPath string,
	newHostPath string,
) (string, error) {
	newShadowPath := ri.shadow.ShadowPath(newHostPath, repoId)

	for {
		if isDir(newHostPath) && isDir(newShadowPath) {
			return newShadowPath, nil
		}

		newShadowParent := filepath.Dir(newShadowPath)
		newShadowParentParent := filepath.Dir(newShadowParent)
		fmt.Printf(`@admin, please move real and shadow:
    mv '%s' '%s' && git -C '%s' config fso.realdir '%s'
    mkdir -p '%s' && mv '%s' '%s'
    chmod -Rv o+rX '%s'  # should be setfacl
    chown -vR $(stat -c %%U:%%G '%s') '%s'

`,
			oldHostPath, newHostPath, oldShadowPath, newHostPath,
			newShadowParent, oldShadowPath, newShadowPath,
			newHostPath,
			newShadowParentParent, newShadowParent,
		)

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
			// Continue loop.
		}
	}
}
