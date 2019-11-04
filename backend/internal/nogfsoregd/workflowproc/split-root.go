package workflowproc

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/splitrootwf"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type splitRootWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	registry    string
	done        chan<- struct{}
	view        splitRootWorkflowView
}

type splitRootWorkflowView struct {
	workflowId    uuid.I
	vid           ulid.I
	scode         splitrootwf.StateCode
	needsAbort    bool
	root          string
	minDiskUsage  int64
	maxDiskUsage  int64
	statusCode    int32
	statusMessage string
	du            duTree
}

func (a *splitRootWorkflowActivity) ProcessRegistryWorkflowEvents(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		view := splitRootWorkflowView{
			workflowId: workflowId,
			du:         make(map[string]int64),
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
	}

	return wfstreams.WatchRegistryWorkflowEvents(
		ctx, tail, stream, a, a,
	)
}

func (view *splitRootWorkflowView) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	view.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvSplitRootStarted:
		view.scode = splitrootwf.StateInitialized
		view.root = x.GlobalRoot
		view.minDiskUsage = x.MinDiskUsage
		view.maxDiskUsage = x.MaxDiskUsage
		return nil

	case *wfevents.EvSplitRootDuAppended:
		view.scode = splitrootwf.StateDuAppending
		view.du[x.Path] = x.Usage
		return nil

	case *wfevents.EvSplitRootDuCompleted:
		if x.StatusCode == 0 {
			view.scode = splitrootwf.StateDuCompleted
		} else {
			view.scode = splitrootwf.StateDuFailed
			// If du failed, we're responsible for abort.
			view.needsAbort = true
		}
		return nil

	case *wfevents.EvSplitRootSuggestionAppended:
		view.scode = splitrootwf.StateSuggestionsAppending
		return nil

	case *wfevents.EvSplitRootAnalysisCompleted:
		if x.StatusCode == 0 {
			view.scode = splitrootwf.StateAnalysisCompleted
		} else {
			view.scode = splitrootwf.StateAnalysisFailed
			// If the analysis failed, we're responsible for abort.
			view.needsAbort = true
		}
		return nil

	case *wfevents.EvSplitRootDecisionAppended:
		view.scode = splitrootwf.StateDecisionsAppending
		return nil

	case *wfevents.EvSplitRootCompleted:
		view.statusCode = x.StatusCode
		view.statusMessage = x.StatusMessage
		if x.StatusCode == 0 {
			view.scode = splitrootwf.StateCompleted
		} else {
			view.scode = splitrootwf.StateFailed
			// We are only responsible for abort if `needsAbort`
			// has been set before.  `EvSplitRootCompleted` can
			// also result from an admin running `nogfsoctl
			// split-root abort`.
		}
		return nil

	case *wfevents.EvSplitRootCommitted:
		view.scode = splitrootwf.StateTerminated
		return nil

	default:
		return ErrUnknownEvent
	}
}

func (a *splitRootWorkflowActivity) processView(
	ctx context.Context,
	view splitRootWorkflowView,
) (bool, error) {
	switch view.scode {
	case splitrootwf.StateUninitialized:
		return a.doContinue()

	case splitrootwf.StateInitialized:
		return a.doContinue()

	case splitrootwf.StateDuAppending:
		return a.doContinue()

	case splitrootwf.StateDuFailed:
		return a.doAbortWorkflowAndQuit(
			ctx, view.workflowId, view.vid,
			1, "du failed",
		)

	case splitrootwf.StateDuCompleted:
		return a.doAnalyzeAndContinue(
			ctx, view.workflowId, view.vid,
			view.root,
			view.du,
			&analysisConfig{
				MinDiskUsage: view.minDiskUsage,
				MaxDiskUsage: view.maxDiskUsage,
			},
		)

	case splitrootwf.StateSuggestionsAppending:
		return a.doAbortAnalysisAndContinue(
			ctx, view.workflowId, view.vid,
			1, "analysis was interrupted",
		)

	case splitrootwf.StateAnalysisFailed:
		return a.doAbortWorkflowAndQuit(
			ctx, view.workflowId, view.vid,
			1, "analysis failed",
		)

	// Wait for an admin to `nogfsoctl split-root decide` and `nogfsoctl
	// split-root commit`.
	case splitrootwf.StateAnalysisCompleted:
		return a.doContinue()
	case splitrootwf.StateDecisionsAppending:
		return a.doContinue()

	case splitrootwf.StateCompleted:
		// Idempotent retry could be reasonable here, i.e.:
		//
		// ```
		// return a.doCommitWorkflowAndQuit(
		//     ctx, view.workflowId, view.vid,
		// )
		// ```
		//
		// But the admin is responsible for retrying `nogfsoctl
		// split-root commit` if it gets interrupted.
		// `doCommitWorkflowAndQuit()` here would race with the admin's
		// command and likely cause a concurrent update version
		// conflict.  The conflict would likely be resolved during the
		// first retry.  But it would be ugly, nonetheless, so we avoid
		// it and instead continue, leaving retry responsibilty to the
		// admin alone.
		return a.doContinue()

	case splitrootwf.StateFailed:
		// A similar argument as for `StateCompleted` could be made
		// here against retry.  But we must retry if another state for
		// which we are responsible triggered
		// `doAbortWorkflowAndQuit()`, because no one else would be
		// responsible.
		if view.needsAbort {
			// Idempotent retry.
			return a.doAbortWorkflowAndQuit(
				ctx, view.workflowId, view.vid,
				view.statusCode, view.statusMessage,
			)
		}
		return a.doContinue()

	case splitrootwf.StateTerminated:
		return a.doQuit()

	default:
		panic("invalid StateCode")
	}
}

func (a *splitRootWorkflowActivity) WatchWorkflowEvent(
	ctx context.Context, vid ulid.I, ev wfevents.WorkflowEvent,
) (bool, error) {
	if err := a.view.LoadWorkflowEvent(vid, ev); err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *splitRootWorkflowActivity) WillBlock(
	ctx context.Context,
) (bool, error) {
	return a.processView(ctx, a.view)
}

func (a *splitRootWorkflowActivity) doAnalyzeAndContinue(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	root string,
	du duTree,
	cfg *analysisConfig,
) (bool, error) {
	known, err := a.listRepos(ctx, root)
	if err != nil {
		return a.doRetry(err)
	}

	dontSplit, err := a.listDontSplit(ctx, root)
	if err != nil {
		return a.doRetry(err)
	}

	ana := analyzer{
		cfg:       cfg,
		du:        du,
		known:     known,
		dontSplit: dontSplit,
	}
	suggestions := ana.Analyze(".", 0)

	c := pb.NewSplitRootClient(a.conn)

	{
		i := &pb.AppendSplitRootSuggestionsI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
			Paths:       suggestions,
		}
		o, err := c.AppendSplitRootSuggestions(ctx, i, a.sysRPCCreds)
		switch status.Code(err) {
		case codes.OK:
			break
		case codes.ResourceExhausted:
			// Abort if `ResourceExhausted` to avoid additional
			// resource usage.
			a.lg.Errorw(
				"Could not append split-root suggestion.",
				"err", err,
			)
			msg := fmt.Sprintf(
				"failed to append suggestion: %s", err,
			)
			return a.doAbortAnalysisAndContinue(
				ctx, workflowId, vid, 1, msg,
			)
		default:
			return a.doRetry(err)
		}
		v, err := ulid.ParseBytes(o.WorkflowVid)
		if err != nil {
			return a.doRetry(err)
		}
		vid = v
	}

	{
		i := &pb.CommitSplitRootAnalysisI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.CommitSplitRootAnalysis(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doContinue()
}

type analyzer struct {
	cfg       *analysisConfig
	du        duTree
	known     pathSet
	dontSplit pathSet
}

type analysisConfig struct {
	MinDiskUsage int64
	MaxDiskUsage int64
}

type duTree map[string]int64

type pathDiskUsage struct {
	Path  string
	Usage int64
}

func (du duTree) DiskUsage(path string) int64 {
	u, ok := du[path]
	if !ok {
		panic("invalid path")
	}
	return u
}

func (du duTree) Listdir(path string) (lst []pathDiskUsage) {
	var rgx *regexp.Regexp
	if path == "." {
		rgx = regexp.MustCompile("^[^/]+$")
	} else {
		rgx = regexp.MustCompile(
			"^" + regexp.QuoteMeta(path) + "/[^/]+$",
		)
	}

	for k, v := range du {
		if k != "." && rgx.MatchString(k) {
			lst = append(lst, pathDiskUsage{
				Path:  k,
				Usage: v,
			})
		}
	}
	return lst
}

type pathSet map[string]struct{}

func (s pathSet) Has(k string) bool {
	if s == nil {
		return false
	}
	_, ok := s[k]
	return ok
}

func (ana *analyzer) Analyze(
	path string, level int,
) (sugs []*pb.FsoSplitRootSuggestion) {
	appendSug := func(sug pb.FsoSplitRootSuggestion_Suggestion) {
		sugs = append(sugs, &pb.FsoSplitRootSuggestion{
			Path:       path,
			Suggestion: sug,
		})
	}

	if ana.dontSplit.Has(path) {
		appendSug(pb.FsoSplitRootSuggestion_S_DONT_SPLIT)
		return sugs
	}

	size := ana.du.DiskUsage(path)
	children := ana.du.Listdir(path)

	// If the path is unknown, decide.  Recurse if the dir is large enough
	// to be a candidate in order to report deep candidates even if their
	// parents are not a repo, so that deep repos can be initialized in
	// large roots before their parents, which can be used to avoid large
	// tars for the levels close to the root.  An unknown toplevel is
	// always a candidate, indepently of its size, to ensure that every
	// root will have at least one repo.  But if the toplevel is small, do
	// not recurse.
	//
	// If the repo is too small to split, decide and return.  Use
	// `SMALL_REPO` only if the repo could be split, i.e. if it has
	// children.  Note that lack of children might have different reasons:
	//
	//  - The du max depth might been reached.
	//  - All children might be smaller than `minDiskUsage`.
	//
	// For larger repos, record the repo and recurse.
	if !ana.known.Has(path) {
		if size >= ana.cfg.MinDiskUsage {
			appendSug(pb.FsoSplitRootSuggestion_S_REPO_CANDIDATE)
		} else if level == 0 {
			appendSug(pb.FsoSplitRootSuggestion_S_REPO_CANDIDATE)
			return sugs
		} else {
			appendSug(pb.FsoSplitRootSuggestion_S_SMALL_DIR)
			return sugs
		}
	} else if size <= ana.cfg.MaxDiskUsage {
		if len(children) > 0 {
			appendSug(pb.FsoSplitRootSuggestion_S_SMALL_REPO)
		} else {
			appendSug(pb.FsoSplitRootSuggestion_S_REPO)
		}
		return sugs
	} else {
		appendSug(pb.FsoSplitRootSuggestion_S_REPO)
	}

	// Recurse largest to smallest disk usage.
	sort.Slice(children, func(i, j int) bool {
		return children[i].Usage > children[j].Usage
	})
	for _, child := range children {
		sugs = append(sugs, ana.Analyze(child.Path, level+1)...)
	}

	return sugs
}

// `listRepos()` returns paths relative to the prefix.  It uses dot `.` for the
// prefix itself.
func (a *splitRootWorkflowActivity) listRepos(
	ctx context.Context,
	prefix string,
) (pathSet, error) {
	c := pb.NewRegistryClient(a.conn)
	i := &pb.GetReposI{
		Registry:         a.registry,
		GlobalPathPrefix: prefix,
	}
	o, err := c.GetRepos(ctx, i, a.sysRPCCreds)
	if err != nil {
		return nil, err
	}

	prefixSlash := prefix + "/"
	paths := make(map[string]struct{})
	for _, inf := range o.Repos {
		var rel string
		switch {
		case inf.GlobalPath == prefix:
			rel = "."
		case strings.HasPrefix(inf.GlobalPath, prefixSlash):
			rel = strings.TrimPrefix(inf.GlobalPath, prefixSlash)
		default:
			// XXX Maybe handle gracefully.
			panic("unexpected path prefix")
		}
		paths[rel] = struct{}{}
	}
	return paths, nil
}

// `listDontSplit()` returns paths relative to the prefix.  It uses dot `.` for
// the prefix itself.
func (a *splitRootWorkflowActivity) listDontSplit(
	ctx context.Context,
	root string,
) (pathSet, error) {
	c := pb.NewSplitRootClient(a.conn)
	i := &pb.ListSplitRootPathFlagsI{
		Registry:   a.registry,
		GlobalRoot: root,
	}
	o, err := c.ListSplitRootPathFlags(ctx, i, a.sysRPCCreds)
	if err != nil {
		return nil, err
	}

	rootSlash := root + "/"
	paths := make(map[string]struct{})
	for _, p := range o.Paths {
		if p.Flags&uint32(pb.FsoPathFlag_PF_DONT_SPLIT) == 0 {
			continue
		}
		var rel string
		switch {
		case p.Path == root:
			rel = "."
		case strings.HasPrefix(p.Path, rootSlash):
			rel = strings.TrimPrefix(p.Path, rootSlash)
		default:
			// XXX Maybe handle gracefully.
			panic("unexpected path prefix")
		}
		paths[rel] = struct{}{}
	}
	return paths, nil
}

func (a *splitRootWorkflowActivity) doAbortAnalysisAndContinue(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	statusCode int32,
	statusMessage string,
) (bool, error) {
	c := pb.NewSplitRootClient(a.conn)
	i := &pb.AbortSplitRootAnalysisI{
		Workflow:      workflowId[:],
		WorkflowVid:   vid[:],
		StatusCode:    statusCode,
		StatusMessage: statusMessage,
	}
	_, err := c.AbortSplitRootAnalysis(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

// See comment at `case splitrootwf.StateCompleted`.
func (a *splitRootWorkflowActivity) doCommitWorkflowAndQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
) (bool, error) {
	c := pb.NewSplitRootClient(a.conn)
	i := &pb.CommitSplitRootI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
	}
	_, err := c.CommitSplitRoot(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

func (a *splitRootWorkflowActivity) doAbortWorkflowAndQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	statusCode int32,
	statusMessage string,
) (bool, error) {
	c := pb.NewSplitRootClient(a.conn)
	i := &pb.AbortSplitRootI{
		Workflow:      workflowId[:],
		WorkflowVid:   vid[:],
		StatusCode:    statusCode,
		StatusMessage: statusMessage,
	}
	_, err := c.AbortSplitRoot(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

func (a *splitRootWorkflowActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *splitRootWorkflowActivity) doQuit() (bool, error) {
	if a.done != nil {
		close(a.done)
	}
	return true, nil
}

func (a *splitRootWorkflowActivity) doRetry(err error) (bool, error) {
	return false, err
}
