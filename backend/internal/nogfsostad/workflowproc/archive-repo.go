package workflowproc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	"github.com/nogproject/nog/backend/internal/workflows/archiverepowf"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/pkg/timex"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

const ConfigMaxArchiveRepoRetries = 5

const ConfigArchiveRepoGCDelayDays = 7
const ConfigArchiveRepoGCDelay = ConfigArchiveRepoGCDelayDays * 24 * time.Hour

type archiveRepoWorkflowActivity struct {
	lg               Logger
	conn             *grpc.ClientConn
	sysRPCCreds      grpc.CallOption
	done             chan<- struct{}
	repoProc         RepoProcessor
	aclPropagator    AclPropagator
	privs            ArchiveRepoPrivileges
	archiveRepoSpool string
	view             archiveRepoWorkflowView
	tail             ulid.I
	nRetries         int
}

type archiveRepoWorkflowView struct {
	workflowId         uuid.I
	vid                ulid.I
	scode              archiverepowf.StateCode
	repoId             uuid.I
	startTime          time.Time
	workingDir         string
	authorName         string
	authorEmail        string
	aclPolicy          *pb.RepoAclPolicy
	filesCommittedTime time.Time
}

func (a *archiveRepoWorkflowActivity) ProcessRegistryWorkflowEvents(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		view := archiveRepoWorkflowView{
			workflowId: workflowId,
		}
		if err := wfstreams.LoadRegistryWorkflowEventsNoBlock(
			stream, &view,
		); err != nil {
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		}

		done, err := a.processView(ctx, view)
		switch {
		case err != nil:
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		case done:
			return view.vid, nil
		}

		tail = view.vid
		a.view = view
		a.tail = view.vid
	}

	return wfstreams.WatchRegistryWorkflowEvents(
		ctx, tail, stream, a, a,
	)
}

func (a *archiveRepoWorkflowActivity) WatchWorkflowEvent(
	ctx context.Context, vid ulid.I, ev wfevents.WorkflowEvent,
) (bool, error) {
	if err := a.view.LoadWorkflowEvent(vid, ev); err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *archiveRepoWorkflowActivity) WillBlock(
	ctx context.Context,
) (bool, error) {
	// Do not call a successful `processView()` again without new event.
	// This is not only an optimization but also necessary to handle
	// successful non-idempotent operations correctly.  For example,
	// `RegistryBeginArchiveRepo()` with `RegistryVid` must be called only
	// once.
	//
	// XXX Could this logic be moved to the caller of `WillBlock()`?
	if a.view.vid == a.tail {
		return a.doContinue()
	}
	done, err := a.processView(ctx, a.view)
	if err == nil {
		a.tail = a.view.vid
	}
	return done, err
}

func (view *archiveRepoWorkflowView) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	view.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvArchiveRepoStarted:
		view.scode = archiverepowf.StateInitialized
		view.repoId = x.RepoId
		view.startTime = ulid.Time(vid)
		view.authorName = x.AuthorName
		view.authorEmail = x.AuthorEmail
		return nil

	case *wfevents.EvArchiveRepoFilesStarted:
		view.scode = archiverepowf.StateFiles
		view.aclPolicy = x.AclPolicy
		return nil

	case *wfevents.EvArchiveRepoTarttCompleted:
		view.scode = archiverepowf.StateTarttCompleted
		return nil

	case *wfevents.EvArchiveRepoSwapStarted:
		view.scode = archiverepowf.StateSwapStarted
		view.workingDir = x.WorkingDir
		return nil

	case *wfevents.EvArchiveRepoFilesCompleted:
		if x.StatusCode == 0 {
			view.scode = archiverepowf.StateFilesCompleted
		} else {
			view.scode = archiverepowf.StateFilesFailed
		}
		return nil

	case *wfevents.EvArchiveRepoFilesCommitted:
		view.filesCommittedTime = ulid.Time(vid)
		view.scode = archiverepowf.StateFilesEnded
		return nil

	case *wfevents.EvArchiveRepoGcCompleted:
		view.scode = archiverepowf.StateGcCompleted
		return nil

	case *wfevents.EvArchiveRepoCompleted:
		if x.StatusCode == 0 {
			view.scode = archiverepowf.StateCompleted
		} else {
			view.scode = archiverepowf.StateFailed
		}
		return nil

	case *wfevents.EvArchiveRepoCommitted:
		view.scode = archiverepowf.StateTerminated
		return nil

	default:
		return ErrUnknownEvent
	}
}

func (a *archiveRepoWorkflowActivity) processView(
	ctx context.Context,
	view archiveRepoWorkflowView,
) (bool, error) {
	switch view.scode {
	case archiverepowf.StateUninitialized:
		return a.doContinue()

	case archiverepowf.StateInitialized:
		return a.doContinue()

	case archiverepowf.StateFiles:
		return a.doPollTarttThenContinue(
			ctx,
			view.workflowId, view.vid,
			view.repoId,
			view.startTime,
			view.aclPolicy,
			view.authorName, view.authorEmail,
		)

	case archiverepowf.StateTarttCompleted:
		return a.doPrepareArchiveThenContinue(
			ctx,
			view.workflowId, view.vid,
			view.repoId,
			view.startTime,
			view.aclPolicy,
			view.authorName, view.authorEmail,
		)

	case archiverepowf.StateSwapStarted:
		return a.doArchiveRepoThenContinue(
			ctx,
			view.workflowId, view.vid,
			view.repoId,
			view.workingDir,
			view.authorName, view.authorEmail,
		)

	case archiverepowf.StateFilesCompleted:
		return a.doContinue()

	case archiverepowf.StateFilesFailed:
		return a.doContinue()

	case archiverepowf.StateFilesEnded:
		return a.doGcThenQuit(
			ctx,
			view.workflowId, view.vid,
			view.repoId,
			view.workingDir,
			view.filesCommittedTime,
		)

	case archiverepowf.StateGcCompleted:
		return a.doQuit()

	case archiverepowf.StateCompleted:
		return a.doQuit()

	case archiverepowf.StateFailed:
		return a.doQuit()

	case archiverepowf.StateTerminated:
		return a.doQuit()

	default:
		panic("invalid StateCode")
	}
}

func (a *archiveRepoWorkflowActivity) doPollTarttThenContinue(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	repoId uuid.I,
	startTime time.Time,
	aclPolicy *pb.RepoAclPolicy,
	authorName, authorEmail string,
) (bool, error) {
	// Synchronize with observer6 on a per-repo basis during startup to
	// ensure that the repo is enabled.  See comment at
	// `WaitEnableRepo4()`.
	if err := a.repoProc.WaitEnableRepo4(ctx, repoId); err != nil {
		return a.doRetry(err)
	}

	inf, err := a.repoProc.TarttIsFrozenArchive(ctx, repoId)
	switch {
	case err != nil:
		return a.doRetry(err)
	case !inf.IsFrozenArchive:
		a.lg.Infow(
			"Frozen tartt archive not yet ready.",
			"repoId", repoId,
			"workflowId", workflowId,
		)
		return a.doRetrySilent()
	}

	c := pb.NewArchiveRepoClient(a.conn)
	i := &pb.CommitArchiveRepoTarttI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
		TarPath:     inf.TarPath,
	}
	o, err := c.CommitArchiveRepoTartt(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}

	// Optimization: Directly continue with
	// `doPrepareArchiveThenContinue()` to skip one event stream reading.
	vid2, err := ulid.ParseBytes(o.WorkflowVid)
	if err != nil {
		// This error should not happen.  But if it does, we can
		// continue, which will read EvArchiveRepoTarttCompleted from
		// the registry and the call doPrepareArchiveThenContinue().
		return a.doContinue()
	}
	return a.doPrepareArchiveThenContinue(
		ctx,
		workflowId, vid2,
		repoId,
		startTime,
		aclPolicy,
		authorName, authorEmail,
	)
}

func (a *archiveRepoWorkflowActivity) doPrepareArchiveThenContinue(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	repoId uuid.I,
	startTime time.Time,
	aclPolicy *pb.RepoAclPolicy,
	authorName, authorEmail string,
) (bool, error) {
	if a.archiveRepoSpool == "" {
		return a.doAbortArchiveFilesThenContinue(
			ctx, workflowId, vid,
			int32(pb.StatusCode_SC_STAD_ARCHIVE_REPO_FAILED),
			"archive-repo spool dir not configured",
		)
	}

	// The working directory contains subdirs for the placeholder and the
	// swapped realdir.  The working directory will be deleted during
	// garbage collection.
	dir := filepath.Join(
		a.archiveRepoSpool,
		fmt.Sprintf(
			"%s_r-%s_w-%s",
			startTime.Format(timex.ISO8601Basic),
			repoId, workflowId,
		),
	)
	if err := a.ensureArchiveRepoWorkingDir(
		ctx, repoId, dir, aclPolicy, authorName, authorEmail,
	); err != nil {
		// XXX Maybe inspect error to detect fatal errors and abort.
		return a.doRetry(err)
	}

	c := pb.NewArchiveRepoClient(a.conn)
	i := &pb.BeginArchiveRepoSwapI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
		WorkingDir:  dir,
	}
	_, err := c.BeginArchiveRepoSwap(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *archiveRepoWorkflowActivity) ensureArchiveRepoWorkingDir(
	ctx context.Context,
	repoId uuid.I,
	dir string,
	aclPolicy *pb.RepoAclPolicy,
	authorName, authorEmail string,
) error {
	sudo, err := a.privs.AcquireUdoChattr(ctx, "root")
	if err != nil {
		return err
	}
	defer sudo.Release()

	// Remove existing dir to handle restart.
	if exists(dir) {
		if err := sudo.ChattrTreeUnsetImmutable(ctx, dir); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}

	// Create dir and prepare placeholder.
	if err := os.Mkdir(dir, 0777); err != nil {
		return err
	}

	placeholder := filepath.Join(dir, "placeholder")
	if err := os.Mkdir(placeholder, 0777); err != nil {
		return err
	}

	inf, err := a.repoProc.TarttIsFrozenArchive(ctx, repoId)
	if err != nil {
		return err
	}

	readme := filepath.Join(placeholder, "README.md")
	fp, err := os.OpenFile(readme, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	err = placeholderReadmeMd.Execute(fp, placeholderReadmeMdData{
		StatAuthorName:     inf.StatAuthorName,
		StatAuthorEmail:    inf.StatAuthorEmail,
		StatAuthorDate:     inf.StatAuthorDate,
		Dirs:               inf.Dirs,
		Files:              inf.Files,
		FilesSize:          inf.FilesSize,
		MtimeMin:           inf.MtimeMin,
		MtimeMax:           inf.MtimeMax,
		TarTime:            inf.TarTime,
		Now:                time.Now(),
		RepoId:             repoId,
		ArchiveAuthorName:  authorName,
		ArchiveAuthorEmail: authorEmail,
	})
	err2 := fp.Close()
	switch {
	case err != nil:
		return err
	case err2 != nil:
		return err2
	}

	switch aclPolicy.Policy {
	case pb.RepoAclPolicy_P_PROPAGATE_ROOT_ACLS:
		if a.aclPropagator == nil {
			return ErrAclsDisabled
		}
		if err := a.aclPropagator.PropagateAcls(
			ctx, aclPolicy.FsoRootInfo.HostRoot, placeholder,
		); err != nil {
			return err
		}
	}

	if err := sudo.ChattrTreeSetImmutable(ctx, placeholder); err != nil {
		return err
	}

	// Fsync to ensure durability.
	return fsyncPaths([]string{
		readme,
		placeholder,
		dir,
		filepath.Dir(dir),
	})
}

type placeholderReadmeMdData struct {
	StatAuthorName     string
	StatAuthorEmail    string
	StatAuthorDate     time.Time
	Dirs               int64
	Files              int64
	FilesSize          int64
	MtimeMin           time.Time
	MtimeMax           time.Time
	TarTime            time.Time
	Now                time.Time
	RepoId             uuid.I
	ArchiveAuthorName  string
	ArchiveAuthorEmail string
}

var placeholderReadmeMd = template.Must(template.New(
	"README",
).Funcs(template.FuncMap{
	"isodate": func(v time.Time) string {
		const IsoDate = "2006-01-02"
		return v.Format(IsoDate)
	},
	"pluralize": func(word string, count int64) string {
		if count == 1 {
			return word
		}
		switch word {
		case "directory":
			return "directories"
		default:
			return word + "s"
		}
	},
}).Parse(
	`# README

This is a placeholder for a directory that has been archived and removed from
online storage.

The directory contained
{{- ""}} {{ .Files }} {{ pluralize "file" .Files  }} of total size
{{- ""}} {{ .FilesSize }} {{ pluralize "byte" .FilesSize  }} in
{{- ""}} {{ .Dirs }} {{ pluralize "directory" .Dirs  }},
last modified between {{ .MtimeMin | isodate }} and {{ .MtimeMax | isodate }}.

The directory was frozen
{{- ""}} on {{ .StatAuthorDate | isodate }} by
{{- ""}} {{ .StatAuthorName }} <{{ .StatAuthorEmail }}>.
The archive was created on {{ .TarTime | isodate }}
{{- ""}} in repository {{ .RepoId }}.
The directory was removed from online storage on {{ .Now | isodate }}
{{- ""}} by {{ .ArchiveAuthorName }} <{{ .ArchiveAuthorEmail }}>.
`))

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fsyncPaths(ps []string) error {
	for _, p := range ps {
		if err := fsyncPath(p); err != nil {
			return err
		}
	}
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

func (a *archiveRepoWorkflowActivity) doArchiveRepoThenContinue(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	repoId uuid.I,
	workingDir string,
	authorName, authorEmail string,
) (bool, error) {
	// Synchronize with observer6 on a per-repo basis during startup to
	// ensure that the repo is enabled before trying to archive it.  See
	// comment at `WaitEnableRepo4()`.
	if err := a.repoProc.WaitEnableRepo4(ctx, repoId); err != nil {
		return a.doRetry(err)
	}

	author := nogfsostad.GitUser{
		Name:  authorName,
		Email: authorEmail,
	}
	err := a.repoProc.ArchiveRepo(ctx, repoId, workingDir, author)
	if err != nil {
		// Retry a few times, because recovering from repo errors is
		// relatively expensive, and some errors might be temporary,
		// for example sudoudod might not yet be ready.
		if a.nRetries < ConfigMaxArchiveRepoRetries {
			a.nRetries++
			return a.doRetry(err)
		}
		return a.doAbortArchiveFilesThenContinue(
			ctx, workflowId, vid,
			int32(pb.StatusCode_SC_STAD_ARCHIVE_REPO_FAILED),
			truncateErrorMessage(err.Error()),
		)
	}
	a.nRetries = 0
	return a.doCommitArchiveFilesThenContinue(
		ctx, workflowId, vid,
	)
}

func (a *archiveRepoWorkflowActivity) doCommitArchiveFilesThenContinue(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
) (bool, error) {
	c := pb.NewArchiveRepoClient(a.conn)
	i := &pb.CommitArchiveRepoFilesI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
	}
	_, err := c.CommitArchiveRepoFiles(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *archiveRepoWorkflowActivity) doAbortArchiveFilesThenContinue(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	statusCode int32, statusMessage string,
) (bool, error) {
	c := pb.NewArchiveRepoClient(a.conn)
	i := &pb.AbortArchiveRepoFilesI{
		Workflow:      workflowId[:],
		WorkflowVid:   vid[:],
		StatusCode:    statusCode,
		StatusMessage: statusMessage,
	}
	_, err := c.AbortArchiveRepoFiles(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *archiveRepoWorkflowActivity) doGcThenQuit(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	repoId uuid.I,
	workingDir string,
	filesCommittedTime time.Time,
) (bool, error) {
	due := filesCommittedTime.Add(ConfigArchiveRepoGCDelay)
	if time.Now().Before(due) {
		return a.doRetrySilentAfter(due)
	}

	if exists(workingDir) {
		sudo, err := a.privs.AcquireUdoChattr(ctx, "root")
		if err != nil {
			return a.doRetry(err)
		}
		defer sudo.Release()
		if err := sudo.ChattrTreeUnsetImmutable(
			ctx, workingDir,
		); err != nil {
			return a.doRetry(err)
		}
	}
	if err := os.RemoveAll(workingDir); err != nil {
		return a.doRetry(err)
	}

	c := pb.NewArchiveRepoClient(a.conn)
	i := &pb.CommitArchiveRepoGcI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
	}
	if _, err := c.CommitArchiveRepoGc(ctx, i, a.sysRPCCreds); err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

func (a *archiveRepoWorkflowActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *archiveRepoWorkflowActivity) doQuit() (bool, error) {
	if a.done != nil {
		close(a.done)
	}
	return true, nil
}

func (a *archiveRepoWorkflowActivity) doRetry(err error) (bool, error) {
	return false, err
}

func (a *archiveRepoWorkflowActivity) doRetrySilent() (bool, error) {
	return false, grpcentities.SilentRetry
}

func (a *archiveRepoWorkflowActivity) doRetrySilentAfter(
	after time.Time,
) (bool, error) {
	return false, &grpcentities.SilentRetryAfter{After: after}
}
