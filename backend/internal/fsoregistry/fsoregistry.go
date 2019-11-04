// vim: sw=8

/*

Package `fsoregistry` implements an event-sourced aggregate that contains FSO
registries.  See package `fsomain` for an oveview.

Registries manage lists of roots and lists of repos.  Repo details are in
separate entities, implemented by package `fsorepos`.

*/
package fsoregistry

import (
	"context"
	"fmt"
	slashpath "path"
	"sort"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/nogproject/nog/backend/internal/configmap"
	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsoregistry/pbevents"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/gpg"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// XXX Maybe factor out into a common package, e.g. `storagetier`, that is used
// by packages `fsoregistry` and `fsorepos`.
type StorageTierCode int

const (
	StorageTierUnspecified StorageTierCode = iota
	StorageOnline
	StorageFrozen
	StorageArchived
	StorageFreezing
	StorageFreezeFailed
	StorageUnfreezing
	StorageUnfreezeFailed
	StorageArchiving
	StorageArchiveFailed
	StorageUnarchiving
	StorageUnarchiveFailed
)

var NoVC = events.NoVC
var RetryNoVC = events.RetryNoVC

type State struct {
	id                   uuid.I
	ephemeralWorkflowsId uuid.I
	vid                  ulid.I
	info                 *Info
	roots                map[string]*rootState
	reposByName          map[string]*RepoInfo
	reposById            map[uuid.I]*RepoInfo
	pathFlags            map[string]uint32
	repoAclPolicy        pb.RepoAclPolicy_Policy
}

type rootState struct {
	info RootInfo

	// `repoNaming` is the original unpatched pb.  It is used with
	// `proto.Equal()`, but only if `repoNamingIsPatched==false`.
	// `repoNamingConfig` is the parsed config with all patches applied.
	repoNaming          *pb.FsoRepoNaming
	repoNamingIsPatched bool
	repoNamingConfig    map[string]interface{}

	repoInitPolicy *pb.FsoRepoInitPolicy

	splitRootConfig *SplitRootConfig
}

type Event struct {
	id     ulid.I
	parent ulid.I
	pb     pb.RegistryEvent
}

type Info struct {
	Name string
}

type RootInfo struct {
	GlobalRoot             string
	Host                   string
	HostRoot               string
	GitlabNamespace        string
	ArchiveRecipients      gpg.Fingerprints
	ShadowBackupRecipients gpg.Fingerprints
}

type SplitRootConfig struct {
	MaxDepth     int32
	MinDiskUsage int64
	MaxDiskUsage int64
}

type CmdInitRegistry struct {
	Name string
}

type CmdEnableEphemeralWorkflows struct {
	EphemeralWorkflowsId uuid.I
}

type CmdEnablePropagateRootAcls struct{}

type CmdInitRoot struct {
	GlobalRoot      string
	Host            string
	HostRoot        string
	GitlabNamespace string
}

type CmdRemoveRoot struct {
	GlobalRoot string
}

func (*CmdRemoveRoot) AggregateCommand() {}

type CmdEnableGitlab struct {
	GlobalRoot      string
	GitlabNamespace string
}

type CmdDisableGitlab struct {
	GlobalRoot string
}

type CmdEnableGitlabRepo struct {
	RepoId          uuid.I
	GitlabNamespace string
}

func (*CmdEnableGitlabRepo) AggregateCommand() {}

type CmdSetRepoNaming struct {
	Naming *pb.FsoRepoNaming
}

func (*CmdSetRepoNaming) AggregateCommand() {}

type CmdPatchRepoNaming struct {
	NamingPatch *pb.FsoRepoNaming
}

func (*CmdPatchRepoNaming) AggregateCommand() {}

func (*CmdEnableDiscoveryPaths) AggregateCommand() {}

type DepthPath struct {
	Depth int
	Path  string
}

type CmdEnableDiscoveryPaths struct {
	GlobalRoot string
	DepthPaths []DepthPath
}

type CmdSetRepoInitPolicy struct {
	Policy *pb.FsoRepoInitPolicy
}

func (*CmdSetRepoInitPolicy) AggregateCommand() {}

type CmdCreateSplitRootConfig struct {
	GlobalRoot string
	Config     *SplitRootConfig
}

func (*CmdCreateSplitRootConfig) AggregateCommand() {}

type CmdUpdateSplitRootConfig struct {
	GlobalRoot string
	Config     *SplitRootConfig
}

func (*CmdUpdateSplitRootConfig) AggregateCommand() {}

type CmdDeleteSplitRootConfig struct {
	GlobalRoot string
}

func (*CmdDeleteSplitRootConfig) AggregateCommand() {}

type CmdUpdateRootArchiveRecipients struct {
	GlobalRoot string
	Keys       gpg.Fingerprints
}

func (*CmdUpdateRootArchiveRecipients) AggregateCommand() {}

type CmdDeleteRootArchiveRecipients struct {
	GlobalRoot string
}

func (*CmdDeleteRootArchiveRecipients) AggregateCommand() {}

type CmdUpdateRootShadowBackupRecipients struct {
	GlobalRoot string
	Keys       gpg.Fingerprints
}

func (*CmdUpdateRootShadowBackupRecipients) AggregateCommand() {}

type CmdDeleteRootShadowBackupRecipients struct {
	GlobalRoot string
}

func (*CmdDeleteRootShadowBackupRecipients) AggregateCommand() {}

// `Creator*` is stored in the event, but not in the aggregate state, because
// it is only of temporary interest.
type RepoInfo struct {
	Id              uuid.I
	GlobalPath      string
	GitlabNamespace string
	Confirmed       bool
	ReinitReason    string
	lastRepoEventId ulid.I

	// move-repo workflow states:
	//
	//  - `moveRepoWorkflow == nil`: workflow never started
	//  - `moveRepoWorkflow != nil && newGlobalPath != ""`: workflow
	//    active.
	//  - `moveRepoWorkflow != nil && newGlobalPath == ""`: workflow
	//    has ended, currently no active workflow.
	moveRepoWorkflow uuid.I
	newGlobalPath    string

	StorageTier StorageTierCode

	// `storageWorkflowId` contains the ID of the last active workflow.  It
	// is used in idempotency checks.
	storageWorkflowId uuid.I
}

func (inf *RepoInfo) hasActiveMoveRepo() bool {
	return inf.moveRepoWorkflow != uuid.Nil && inf.newGlobalPath != ""
}

type CmdInitRepo struct {
	// `Context` is used when calling `IsInitRepoAllowed()`.  If
	// `IsInitRepoAllowed()` uses outgoing GRPC calls that require
	// authorization, `Context` must have the required metadata attached
	// with `NewOutgoingContext()`.
	Context      context.Context
	Id           uuid.I
	GlobalPath   string
	CreatorName  string
	CreatorEmail string
}

func (c *CmdInitRepo) ensureId() (*CmdInitRepo, error) {
	if c.Id != uuid.Nil {
		return c, nil
	}
	dup := *c
	if id, err := uuid.NewRandom(); err != nil {
		return nil, err
	} else {
		dup.Id = id
	}
	return &dup, nil
}

type CmdReinitRepo struct {
	RepoId uuid.I
	Reason string
}

type CmdConfirmRepo struct {
	RepoId      uuid.I
	RepoEventId ulid.I
}

type CmdBeginMoveRepo struct {
	RepoId                uuid.I
	WorkflowId            uuid.I
	NewGlobalPath         string
	IsUnchangedGlobalPath bool
}

type CmdCommitMoveRepo struct {
	RepoId      uuid.I
	WorkflowId  uuid.I
	RepoEventId ulid.I
	GlobalPath  string
}

type CmdPostShadowRepoMoveStarted struct {
	RepoId      uuid.I
	RepoEventId ulid.I
	WorkflowId  uuid.I
}

type CmdSetPathFlags struct {
	Path  string
	Flags uint32
}

type CmdUnsetPathFlags struct {
	Path  string
	Flags uint32
}

type CmdBeginFreezeRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

type CmdCommitFreezeRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

type CmdAbortFreezeRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
	Code       int32
}

type CmdBeginUnfreezeRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

type CmdCommitUnfreezeRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

type CmdAbortUnfreezeRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
	Code       int32
}

type CmdBeginArchiveRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

type CmdCommitArchiveRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

type CmdAbortArchiveRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
	Code       int32
}

type CmdBeginUnarchiveRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

type CmdCommitUnarchiveRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

type CmdAbortUnarchiveRepo struct {
	RepoId     uuid.I
	WorkflowId uuid.I
	Code       int32
}

func (*State) AggregateState()                          {}
func (*CmdInitRegistry) AggregateCommand()              {}
func (*CmdEnableEphemeralWorkflows) AggregateCommand()  {}
func (*CmdEnablePropagateRootAcls) AggregateCommand()   {}
func (*CmdInitRoot) AggregateCommand()                  {}
func (*CmdEnableGitlab) AggregateCommand()              {}
func (*CmdDisableGitlab) AggregateCommand()             {}
func (*CmdInitRepo) AggregateCommand()                  {}
func (*CmdReinitRepo) AggregateCommand()                {}
func (*CmdConfirmRepo) AggregateCommand()               {}
func (*CmdBeginMoveRepo) AggregateCommand()             {}
func (*CmdCommitMoveRepo) AggregateCommand()            {}
func (*CmdPostShadowRepoMoveStarted) AggregateCommand() {}
func (*CmdSetPathFlags) AggregateCommand()              {}
func (*CmdUnsetPathFlags) AggregateCommand()            {}
func (*CmdBeginFreezeRepo) AggregateCommand()           {}
func (*CmdCommitFreezeRepo) AggregateCommand()          {}
func (*CmdAbortFreezeRepo) AggregateCommand()           {}
func (*CmdBeginUnfreezeRepo) AggregateCommand()         {}
func (*CmdCommitUnfreezeRepo) AggregateCommand()        {}
func (*CmdAbortUnfreezeRepo) AggregateCommand()         {}
func (*CmdBeginArchiveRepo) AggregateCommand()          {}
func (*CmdCommitArchiveRepo) AggregateCommand()         {}
func (*CmdAbortArchiveRepo) AggregateCommand()          {}
func (*CmdBeginUnarchiveRepo) AggregateCommand()        {}
func (*CmdCommitUnarchiveRepo) AggregateCommand()       {}
func (*CmdAbortUnarchiveRepo) AggregateCommand()        {}

func (s *State) Id() uuid.I        { return s.id }
func (s *State) Vid() ulid.I       { return s.vid }
func (s *State) SetVid(vid ulid.I) { s.vid = vid }

func newEvents(
	parent ulid.I, pbs ...pb.RegistryEvent,
) ([]events.Event, error) {
	evs := make([]events.Event, 0, len(pbs))
	for _, pb := range pbs {
		id, err := ulid.New()
		if err != nil {
			return nil, err
		}
		e := &Event{id: id, parent: parent, pb: pb}
		e.pb.Id = e.id[:]
		e.pb.Parent = e.parent[:]
		evs = append(evs, e)
		parent = id
	}
	return evs, nil
}

func (e *Event) MarshalProto() ([]byte, error) {
	return proto.Marshal(&e.pb)
}

func (e *Event) UnmarshalProto(data []byte) error {
	var err error
	if err = proto.Unmarshal(data, &e.pb); err != nil {
		return err
	}
	if e.id, err = ulid.ParseBytes(e.pb.Id); err != nil {
		return err
	}
	if e.parent, err = ulid.ParseBytes(e.pb.Parent); err != nil {
		return err
	}
	if _, err := pbevents.FromPbValidate(e.pb); err != nil {
		return &EventDetailsError{Err: err}
	}

	// XXX Legacy check.  It should be moved to `FromPbValidate()`.
	if e.pb.FsoRepoInfo != nil {
		if _, err := uuid.FromBytes(e.pb.FsoRepoInfo.Id); err != nil {
			return err
		}
	}

	return nil
}

func (e *Event) Id() ulid.I     { return e.id }
func (e *Event) Parent() ulid.I { return e.parent }

// Receiver by value.
func (e Event) WithId(id ulid.I) events.Event {
	e.id = id
	e.pb.Id = e.id[:]
	return &e
}

// Receiver by value.
func (e Event) WithParent(parent ulid.I) events.Event {
	e.parent = parent
	e.pb.Parent = e.parent[:]
	return &e
}

func (e *Event) PbRegistryEvent() *pb.RegistryEvent {
	return &e.pb
}

type Behavior struct {
	pre Preconditions
}

func (*Behavior) NewState(id uuid.I) events.State { return &State{id: id} }
func (*Behavior) NewEvent() events.Event          { return &Event{} }
func (*Behavior) NewAdvancer() events.Advancer    { return &Advancer{} }

// The `Advancer` maintains flags to indicate which sub-state has been touched.
// Each sub-state is cloned only once by each advancer in order to avoid
// unnecessary intermediate copies when processing a sequence of events.
// Loading from an event history is fast.
//
// But adding repos one-by-one still takes O(n^2) time, because the repos map
// is duplicated during each insert.  O(n^2) seems acceptable up to 10k repos.
// It may even be acceptable for a larger number, assuming that adding repos is
// rare and related operations are expensive, like creating the repo clones at
// various places.
//
// If we wanted to optimize, we could switch to a map implementation that uses
// structural sharing, like <https://github.com/mediocregopher/seq>.  Or we
// could use an ad hoc approach that stores new repos into a smaller map or
// list first and rebuilds the full map only every k inserts.
type Advancer struct {
	main      bool
	roots     bool
	repos     bool
	pathFlags bool
}

func (a *Advancer) Advance(s events.State, ev events.Event) events.State {
	evpb := ev.(*Event).pb
	st := s.(*State)

	if !a.main {
		dup := *st
		st = &dup
		a.main = true
	}

	detachRepos := func() {
		if a.repos {
			return
		}
		st.reposByName = dupReposByName(st.reposByName)
		st.reposById = dupReposById(st.reposById)
		a.repos = true
	}

	detachPathFlags := func() {
		if a.pathFlags {
			return
		}
		dup := make(map[string]uint32)
		for k, v := range st.pathFlags {
			dup[k] = v
		}
		st.pathFlags = dup
		a.pathFlags = true
	}

	switch x := pbevents.FromPbMust(evpb).(type) {
	case *pbevents.EvRegistryAdded:
		st.repoAclPolicy = pb.RepoAclPolicy_P_NO_ACLS
		info := Info{
			Name: x.FsoRegistryInfo.Name,
		}
		st.info = &info
		st.roots = make(map[string]*rootState)
		a.roots = true
		st.reposByName = make(map[string]*RepoInfo)
		st.reposById = make(map[uuid.I]*RepoInfo)
		a.repos = true

	case *pbevents.EvEphemeralWorkflowsEnabled:
		st.ephemeralWorkflowsId = x.EphemeralWorkflowsId
		return st

	case *pbevents.EvRepoAclPolicyUpdated:
		st.repoAclPolicy = x.Policy
		return st

	case *pbevents.EvRootAdded:
		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}

		evinf := x.FsoRootInfo
		st.roots[evinf.GlobalRoot] = &rootState{info: RootInfo{
			GlobalRoot:      evinf.GlobalRoot,
			Host:            evinf.Host,
			HostRoot:        evinf.HostRoot,
			GitlabNamespace: evinf.GitlabNamespace,
		}}

	case *pbevents.EvRootRemoved:
		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}
		delete(st.roots, x.GlobalRoot)

	case *pbevents.EvRootUpdated:
		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}

		evinf := x.FsoRootInfo
		globalRoot := evinf.GlobalRoot
		rootSt := st.roots[globalRoot]
		if rootSt == nil {
			// There must have been an `EvRootAdded` for this root.
			panic("inconsistent state")
		}
		dup := *rootSt
		dup.info.Host = evinf.Host
		dup.info.HostRoot = evinf.HostRoot
		dup.info.GitlabNamespace = evinf.GitlabNamespace
		st.roots[globalRoot] = &dup

	case *pbevents.EvRepoNamingUpdated:
		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}

		globalRoot := x.FsoRepoNaming.GlobalRoot
		rootSt := st.roots[globalRoot]
		if rootSt == nil {
			// There must have been an `EvRootAdded` for this root.
			panic("inconsistent state")
		}
		cfgMap, err := configmap.ParsePb(x.FsoRepoNaming.Config)
		if err != nil {
			panic(err)
		}
		dup := *rootSt
		dup.repoNaming = &x.FsoRepoNaming
		dup.repoNamingIsPatched = false
		dup.repoNamingConfig = cfgMap
		st.roots[globalRoot] = &dup

	// Set `repoNamingIsPatched` to indicate that the original naming has
	// been modified, which will disable the idempotency check in
	// `tellSetRepoNaming()`: `CmdSetRepoNaming` can reset the naming.
	case *pbevents.EvRepoNamingConfigUpdated:
		globalRoot := x.GlobalRoot
		rootSt := st.roots[globalRoot]
		if rootSt == nil {
			// There must have been an `EvRootAdded` for this root.
			panic("inconsistent state")
		}
		patch, err := configmap.ParsePb(&x.ConfigPatch)
		if err != nil {
			panic(err)
		}
		cfgMap, err := configmap.Merge(rootSt.repoNamingConfig, patch)
		if err != nil {
			panic(err)
		}

		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}
		dup := *rootSt
		dup.repoNamingIsPatched = true
		dup.repoNamingConfig = cfgMap
		st.roots[globalRoot] = &dup

	case *pbevents.EvRepoInitPolicyUpdated:
		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}

		globalRoot := x.FsoRepoInitPolicy.GlobalRoot
		rootSt := st.roots[globalRoot]
		if rootSt == nil {
			// There must have been an `EvRootAdded` for this root.
			panic("inconsistent state")
		}
		dup := *rootSt
		dup.repoInitPolicy = &x.FsoRepoInitPolicy
		st.roots[globalRoot] = &dup

	case *pbevents.EvRootArchiveRecipientsUpdated:
		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}
		globalRoot := x.GlobalRoot
		rootSt := st.roots[globalRoot]
		if rootSt == nil {
			// There must have been an `EvRootAdded` for this root.
			panic("inconsistent state")
		}
		keys, err := gpg.ParseFingerprintsBytes(x.Keys...)
		if err != nil {
			panic(err)
		}
		dup := *rootSt
		dup.info.ArchiveRecipients = keys
		st.roots[globalRoot] = &dup

	case *pbevents.EvRootShadowBackupRecipientsUpdated:
		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}
		globalRoot := x.GlobalRoot
		rootSt := st.roots[globalRoot]
		if rootSt == nil {
			// There must have been an `EvRootAdded` for this root.
			panic("inconsistent state")
		}
		keys, err := gpg.ParseFingerprintsBytes(x.Keys...)
		if err != nil {
			panic(err)
		}
		dup := *rootSt
		dup.info.ShadowBackupRecipients = keys
		st.roots[globalRoot] = &dup

	case *pbevents.EvRepoAccepted:
		if !a.repos {
			st.reposByName = dupReposByName(st.reposByName)
			st.reposById = dupReposById(st.reposById)
			a.repos = true
		}

		uu, err := uuid.FromBytes(x.FsoRepoInfo.Id)
		if err != nil {
			panic("invalid UUID")
		}

		globalPath := x.FsoRepoInfo.GlobalPath
		root := findRootInfoForRepo(st.roots, globalPath)
		if root == nil {
			panic("unknown root")
		}

		info := RepoInfo{
			Id:              uu,
			GlobalPath:      x.FsoRepoInfo.GlobalPath,
			GitlabNamespace: root.GitlabNamespace,
		}
		st.reposByName[info.GlobalPath] = &info
		st.reposById[info.Id] = &info

	case *pbevents.EvRepoAdded:
		if !a.repos {
			st.reposByName = dupReposByName(st.reposByName)
			st.reposById = dupReposById(st.reposById)
			a.repos = true
		}
		uu, err := uuid.FromBytes(x.FsoRepoInfo.Id)
		if err != nil {
			panic("invalid UUID")
		}
		info := *st.reposById[uu] // copy
		info.Confirmed = true
		info.StorageTier = StorageOnline
		st.reposByName[info.GlobalPath] = &info
		st.reposById[info.Id] = &info

	case *pbevents.EvRepoMoveAccepted:
		if !a.repos {
			st.reposByName = dupReposByName(st.reposByName)
			st.reposById = dupReposById(st.reposById)
			a.repos = true
		}

		info := *st.reposById[x.RepoId] // copy
		info.moveRepoWorkflow = x.WorkflowId
		info.newGlobalPath = x.NewGlobalPath
		st.reposByName[info.GlobalPath] = &info
		st.reposById[info.Id] = &info

	case *pbevents.EvRepoMoved:
		if !a.repos {
			st.reposByName = dupReposByName(st.reposByName)
			st.reposById = dupReposById(st.reposById)
			a.repos = true
		}

		inf := *st.reposById[x.RepoId] // copy
		if x.WorkflowId != inf.moveRepoWorkflow {
			panic("workflow ID mismatch")
		}
		if x.GlobalPath != inf.newGlobalPath {
			panic("new global path mismatch")
		}
		delete(st.reposByName, inf.GlobalPath)
		inf.GlobalPath = x.GlobalPath
		inf.newGlobalPath = ""
		st.reposByName[inf.GlobalPath] = &inf
		st.reposById[inf.Id] = &inf

	case *pbevents.EvRepoReinitAccepted:
		if !a.repos {
			st.reposByName = dupReposByName(st.reposByName)
			st.reposById = dupReposById(st.reposById)
			a.repos = true
		}
		uu, err := uuid.FromBytes(x.RepoId)
		if err != nil {
			panic("invalid UUID")
		}
		info := *st.reposById[uu] // copy
		info.ReinitReason = x.Reason
		st.reposByName[info.GlobalPath] = &info
		st.reposById[info.Id] = &info

	case *pbevents.EvShadowRepoMoveStarted:
		if !a.repos {
			st.reposByName = dupReposByName(st.reposByName)
			st.reposById = dupReposById(st.reposById)
			a.repos = true
		}
		inf := *st.reposById[x.RepoId] // copy
		inf.lastRepoEventId = x.RepoEventId
		st.reposByName[inf.GlobalPath] = &inf
		st.reposById[inf.Id] = &inf

	case *pbevents.EvRepoEnableGitlabAccepted:
		if !a.repos {
			st.reposByName = dupReposByName(st.reposByName)
			st.reposById = dupReposById(st.reposById)
			a.repos = true
		}
		info := *st.reposById[x.RepoId] // copy
		info.GitlabNamespace = x.GitlabNamespace
		st.reposByName[info.GlobalPath] = &info
		st.reposById[info.Id] = &info

	case *pbevents.EvSplitRootEnabled:
		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}

		globalRoot := x.GlobalRoot
		rootSt := st.roots[globalRoot]
		if rootSt == nil {
			// There must have been an `EvRootAdded` for this root.
			panic("inconsistent state")
		}
		dup := *rootSt
		dup.splitRootConfig = &SplitRootConfig{}
		st.roots[globalRoot] = &dup

	case *pbevents.EvSplitRootParamsUpdated:
		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}

		globalRoot := x.GlobalRoot
		rootSt := st.roots[globalRoot]
		if rootSt == nil {
			// There must have been an `EvRootAdded` for this root.
			panic("inconsistent state")
		}
		dup := *rootSt
		dup.splitRootConfig = &SplitRootConfig{
			MaxDepth:     x.MaxDepth,
			MinDiskUsage: x.MinDiskUsage,
			MaxDiskUsage: x.MaxDiskUsage,
		}
		st.roots[globalRoot] = &dup

	case *pbevents.EvSplitRootDisabled:
		if !a.roots {
			st.roots = dupRoots(st.roots)
			a.roots = true
		}

		globalRoot := x.GlobalRoot
		rootSt := st.roots[globalRoot]
		if rootSt == nil {
			// There must have been an `EvRootAdded` for this root.
			panic("inconsistent state")
		}
		dup := *rootSt
		dup.splitRootConfig = nil
		st.roots[globalRoot] = &dup

	case *pbevents.EvPathFlagSet:
		detachPathFlags()
		f := st.pathFlags[x.Path]
		st.pathFlags[x.Path] = f | x.Flags

	case *pbevents.EvPathFlagUnset:
		detachPathFlags()
		f := st.pathFlags[x.Path] &^ x.Flags
		if f == 0 {
			delete(st.pathFlags, x.Path)
		} else {
			st.pathFlags[x.Path] = f
		}

	case *pbevents.EvFreezeRepoStarted2:
		detachRepos()
		repo := *st.reposById[x.RepoId] // copy
		repo.StorageTier = StorageFreezing
		repo.storageWorkflowId = x.WorkflowId
		st.reposByName[repo.GlobalPath] = &repo
		st.reposById[repo.Id] = &repo

	case *pbevents.EvFreezeRepoCompleted2:
		detachRepos()
		repo := *st.reposById[x.RepoId] // copy
		if x.StatusCode == 0 {
			repo.StorageTier = StorageFrozen
		} else {
			repo.StorageTier = StorageFreezeFailed
		}
		st.reposByName[repo.GlobalPath] = &repo
		st.reposById[repo.Id] = &repo

	case *pbevents.EvUnfreezeRepoStarted2:
		detachRepos()
		repo := *st.reposById[x.RepoId] // copy
		repo.StorageTier = StorageUnfreezing
		repo.storageWorkflowId = x.WorkflowId
		st.reposByName[repo.GlobalPath] = &repo
		st.reposById[repo.Id] = &repo

	case *pbevents.EvUnfreezeRepoCompleted2:
		detachRepos()
		repo := *st.reposById[x.RepoId] // copy
		if x.StatusCode == 0 {
			repo.StorageTier = StorageOnline
		} else {
			repo.StorageTier = StorageUnfreezeFailed
		}
		st.reposByName[repo.GlobalPath] = &repo
		st.reposById[repo.Id] = &repo

	case *pbevents.EvArchiveRepoStarted:
		detachRepos()
		repo := *st.reposById[x.RepoId] // copy
		repo.StorageTier = StorageArchiving
		repo.storageWorkflowId = x.WorkflowId
		st.reposByName[repo.GlobalPath] = &repo
		st.reposById[repo.Id] = &repo

	case *pbevents.EvArchiveRepoCompleted:
		detachRepos()
		repo := *st.reposById[x.RepoId] // copy
		if x.StatusCode == 0 {
			repo.StorageTier = StorageArchived
		} else {
			repo.StorageTier = StorageArchiveFailed
		}
		st.reposByName[repo.GlobalPath] = &repo
		st.reposById[repo.Id] = &repo

	case *pbevents.EvUnarchiveRepoStarted:
		detachRepos()
		repo := *st.reposById[x.RepoId] // copy
		repo.StorageTier = StorageUnarchiving
		repo.storageWorkflowId = x.WorkflowId
		st.reposByName[repo.GlobalPath] = &repo
		st.reposById[repo.Id] = &repo

	case *pbevents.EvUnarchiveRepoCompleted:
		detachRepos()
		repo := *st.reposById[x.RepoId] // copy
		if x.StatusCode == 0 {
			repo.StorageTier = StorageFrozen
		} else {
			repo.StorageTier = StorageUnarchiveFailed
		}
		st.reposByName[repo.GlobalPath] = &repo
		st.reposById[repo.Id] = &repo

	default:
		panic("invalid event")
	}

	return st
}

func dupRoots(src map[string]*rootState) map[string]*rootState {
	dup := make(map[string]*rootState)
	for k, v := range src {
		dup[k] = v
	}
	return dup
}

func dupReposByName(src map[string]*RepoInfo) map[string]*RepoInfo {
	dup := make(map[string]*RepoInfo)
	for k, v := range src {
		dup[k] = v
	}
	return dup
}

func dupReposById(src map[uuid.I]*RepoInfo) map[uuid.I]*RepoInfo {
	dup := make(map[uuid.I]*RepoInfo)
	for k, v := range src {
		dup[k] = v
	}
	return dup
}

func (b *Behavior) Tell(
	s events.State, c events.Command,
) ([]events.Event, error) {
	state := s.(*State)
	switch cmd := c.(type) {
	case *CmdInitRegistry:
		return tellInitRegistry(state, cmd)
	case *CmdEnableEphemeralWorkflows:
		return tellEnableEphemeralWorkflows(state, cmd)
	case *CmdEnablePropagateRootAcls:
		return tellEnablePropagateRootAcls(state, cmd)
	case *CmdInitRoot:
		return tellInitRoot(state, cmd)
	case *CmdRemoveRoot:
		return tellRemoveRoot(state, cmd)
	case *CmdEnableGitlab:
		return tellEnableGitlab(state, cmd)
	case *CmdDisableGitlab:
		return tellDisableGitlab(state, cmd)
	case *CmdSetRepoNaming:
		return tellSetRepoNaming(state, cmd)
	case *CmdPatchRepoNaming:
		return tellPatchRepoNaming(state, cmd)
	case *CmdEnableDiscoveryPaths:
		return tellEnableDiscoveryPaths(state, cmd)
	case *CmdSetRepoInitPolicy:
		return tellSetRepoInitPolicy(state, cmd)
	case *CmdUpdateRootArchiveRecipients:
		return tellUpdateRootArchiveRecipients(state, cmd)
	case *CmdDeleteRootArchiveRecipients:
		return tellDeleteRootArchiveRecipients(state, cmd)
	case *CmdUpdateRootShadowBackupRecipients:
		return tellUpdateRootShadowBackupRecipients(state, cmd)
	case *CmdDeleteRootShadowBackupRecipients:
		return tellDeleteRootShadowBackupRecipients(state, cmd)
	case *CmdInitRepo:
		return b.tellInitRepo(state, cmd)
	case *CmdReinitRepo:
		return tellReinitRepo(state, cmd)
	case *CmdEnableGitlabRepo:
		return tellEnableGitlabRepo(state, cmd)
	case *CmdConfirmRepo:
		return tellConfirmRepo(state, cmd)
	case *CmdBeginMoveRepo:
		return b.tellBeginMoveRepo(state, cmd)
	case *CmdCommitMoveRepo:
		return b.tellCommitMoveRepo(state, cmd)
	case *CmdPostShadowRepoMoveStarted:
		return tellPostShadowRepoMoveStarted(state, cmd)
	case *CmdCreateSplitRootConfig:
		return tellCreateSplitRootConfig(state, cmd)
	case *CmdUpdateSplitRootConfig:
		return tellUpdateSplitRootConfig(state, cmd)
	case *CmdDeleteSplitRootConfig:
		return tellDeleteSplitRootConfig(state, cmd)
	case *CmdSetPathFlags:
		return tellSetPathFlags(state, cmd)
	case *CmdUnsetPathFlags:
		return tellUnsetPathFlags(state, cmd)
	case *CmdBeginFreezeRepo:
		return tellBeginFreezeRepo(state, cmd)
	case *CmdCommitFreezeRepo:
		return tellCommitFreezeRepo(state, cmd)
	case *CmdAbortFreezeRepo:
		return tellAbortFreezeRepo(state, cmd)
	case *CmdBeginUnfreezeRepo:
		return tellBeginUnfreezeRepo(state, cmd)
	case *CmdCommitUnfreezeRepo:
		return tellCommitUnfreezeRepo(state, cmd)
	case *CmdAbortUnfreezeRepo:
		return tellAbortUnfreezeRepo(state, cmd)
	case *CmdBeginArchiveRepo:
		return tellBeginArchiveRepo(state, cmd)
	case *CmdCommitArchiveRepo:
		return tellCommitArchiveRepo(state, cmd)
	case *CmdAbortArchiveRepo:
		return tellAbortArchiveRepo(state, cmd)
	case *CmdBeginUnarchiveRepo:
		return tellBeginUnarchiveRepo(state, cmd)
	case *CmdCommitUnarchiveRepo:
		return tellCommitUnarchiveRepo(state, cmd)
	case *CmdAbortUnarchiveRepo:
		return tellAbortUnarchiveRepo(state, cmd)
	default:
		return nil, ErrCommandUnknown
	}
}

func tellInitRegistry(
	state *State, cmd *CmdInitRegistry,
) ([]events.Event, error) {
	// If already initialized, only check that the command is idempotent.
	if state.info != nil {
		if cmd.Name != state.info.Name {
			return nil, ErrConflictInit
		}
		return nil, nil
	}

	// XXX more validation?

	return newEvents(state.Vid(), pbevents.NewRegistryAdded(cmd.Name))
}

func tellEnableEphemeralWorkflows(
	state *State, cmd *CmdEnableEphemeralWorkflows,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	if cmd.EphemeralWorkflowsId == uuid.Nil {
		return nil, ErrMalformedEphemeralWorkflowsId
	}

	// Already initialized.
	if state.ephemeralWorkflowsId != uuid.Nil {
		if cmd.EphemeralWorkflowsId != state.ephemeralWorkflowsId {
			return nil, ErrConflictEphemeralWorkflowsId
		}
		return nil, nil // Already initialized.
	}

	return newEvents(state.Vid(), pbevents.NewEphemeralWorkflowsEnabled(
		cmd.EphemeralWorkflowsId,
	))
}

func tellEnablePropagateRootAcls(
	state *State, cmd *CmdEnablePropagateRootAcls,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	if state.repoAclPolicy == pb.RepoAclPolicy_P_PROPAGATE_ROOT_ACLS {
		return nil, nil // idempotent
	}

	return newEvents(
		state.Vid(),
		pbevents.NewRepoAclPolicyUpdated(
			pb.RepoAclPolicy_P_PROPAGATE_ROOT_ACLS,
		),
	)
}

func tellInitRoot(state *State, cmd *CmdInitRoot) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	// XXX Validate cmd.

	groot := strings.TrimRight(cmd.GlobalRoot, "/")
	hroot := strings.TrimRight(cmd.HostRoot, "/")

	if _, ok := state.roots[groot]; ok {
		// Already initialized.
		return nil, nil
	}

	gns := cmd.GitlabNamespace
	gns = strings.TrimRight(gns, "/")
	if gns == "" {
		// Empty `GitlabNamespace` indicates:
		//
		// - nogfsostad does not push.
		// - meta is managed by nogfsostad.
		//
	} else if !strings.Contains(gns, "/") {
		// XXX Should be validated more strictly, probably regex.
		return nil, ErrGitlabNamespaceMissingSlash
	}

	// XXX Check invariants, e.g. no nesting.

	return newEvents(state.Vid(), pbevents.NewRootAdded(
		&pb.FsoRootInfo{
			GlobalRoot:      groot,
			Host:            cmd.Host,
			HostRoot:        hroot,
			GitlabNamespace: gns,
		},
	))
}

func tellRemoveRoot(state *State, cmd *CmdRemoveRoot) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	root := strings.TrimRight(cmd.GlobalRoot, "/")
	_, ok := state.roots[root]
	if !ok {
		return nil, ErrUnknownRoot
	}

	return newEvents(state.Vid(), pbevents.NewRootRemoved(root))
}

func tellEnableGitlab(
	state *State, cmd *CmdEnableGitlab,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	gns := cmd.GitlabNamespace
	gns = strings.TrimRight(gns, "/")
	if !strings.Contains(gns, "/") {
		// XXX Should be validated more strictly, probably regex.
		return nil, ErrGitlabNamespaceMissingSlash
	}

	root := strings.TrimRight(cmd.GlobalRoot, "/")
	old, ok := state.roots[root]
	if !ok {
		return nil, ErrUnknownRoot
	}

	if old.info.GitlabNamespace == gns {
		// Already up-to-date.
		return nil, nil
	}

	return newEvents(state.Vid(), pbevents.NewRootUpdated(
		&pb.FsoRootInfo{
			GlobalRoot:      old.info.GlobalRoot,
			Host:            old.info.Host,
			HostRoot:        old.info.HostRoot,
			GitlabNamespace: gns,
		},
	))
}

func tellDisableGitlab(
	state *State, cmd *CmdDisableGitlab,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	root := strings.TrimRight(cmd.GlobalRoot, "/")
	old, ok := state.roots[root]
	if !ok {
		return nil, ErrUnknownRoot
	}

	if old.info.GitlabNamespace == "" {
		// Already disabled
		return nil, nil
	}

	return newEvents(state.Vid(), pbevents.NewRootUpdated(
		&pb.FsoRootInfo{
			GlobalRoot:      old.info.GlobalRoot,
			Host:            old.info.Host,
			HostRoot:        old.info.HostRoot,
			GitlabNamespace: "",
		},
	))
}

func tellSetRepoNaming(
	state *State, cmd *CmdSetRepoNaming,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	naming := cmd.Naming
	if err := pbevents.ValidateRepoNaming(naming); err != nil {
		return nil, err
	}

	root := strings.TrimRight(naming.GlobalRoot, "/")
	rootSt, ok := state.roots[root]
	if !ok {
		return nil, ErrUnknownRoot
	}

	if !rootSt.repoNamingIsPatched &&
		proto.Equal(rootSt.repoNaming, naming) {
		// Already up-to-date.
		return nil, nil
	}

	return newEvents(state.Vid(), pbevents.NewRepoNamingUpdated(naming))
}

func tellPatchRepoNaming(
	state *State, cmd *CmdPatchRepoNaming,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	patch := cmd.NamingPatch
	if err := pbevents.ValidateRepoNamingPatch(patch); err != nil {
		return nil, err
	}

	root := strings.TrimRight(patch.GlobalRoot, "/")
	rootSt, ok := state.roots[root]
	if !ok {
		return nil, ErrUnknownRoot
	}

	if patch.Rule != rootSt.repoNaming.Rule {
		return nil, ErrNamingRuleMismatch
	}

	return newEvents(
		state.Vid(), pbevents.NewRepoNamingConfigUpdated(patch),
	)
}

func tellEnableDiscoveryPaths(
	state *State, cmd *CmdEnableDiscoveryPaths,
) ([]events.Event, error) {
	globalRoot := cmd.GlobalRoot
	depthPaths := cmd.DepthPaths
	for _, dp := range depthPaths {
		if !pathIsBelowPrefixStrict(dp.Path, globalRoot) {
			return nil, ErrPathNotStrictlyBelow
		}
		if dp.Depth < 0 || dp.Depth > 2 {
			return nil, ErrPathDepthOutOfRange
		}
	}

	root, ok := state.roots[globalRoot]
	if !ok {
		return nil, ErrUnknownRoot
	}

	// Require an init policy to avoid enabling paths that could result in
	// unexpectedly large repos.  The init policy typically has a default
	// rule `ignore-most` or `bundle-subdirs`.
	if root.repoInitPolicy == nil {
		return nil, ErrRootWithoutInitPolicy
	}

	// Require repo naming `PathPatterns`, because it is the only rule that
	// currently handles a set of explicitly enabled paths.
	naming := root.repoNaming
	if naming == nil {
		return nil, ErrRootWithoutRepoNaming
	}
	if naming.Rule != "PathPatterns" {
		return nil, &EnablePathRuleError{Rule: naming.Rule}
	}

	// `enabledPaths` is a list of `<depth> <path>`.
	cfgMap := root.repoNamingConfig
	haveSet := make(map[string]bool)
	if depthPaths, ok := cfgMap["enabledPaths"].([]string); ok {
		for _, dp := range depthPaths {
			haveSet[dp] = true
		}
	}

	addSet := make([]string, 0, len(depthPaths))
	for _, dp := range depthPaths {
		relpath := strings.TrimPrefix(dp.Path, globalRoot)
		relpath = strings.Trim(relpath, "/")
		dpString := fmt.Sprintf("%d %s", dp.Depth, relpath)
		if !haveSet[dpString] {
			addSet = append(addSet, dpString)
			haveSet[dpString] = true
		}
	}

	if len(addSet) == 0 {
		return nil, nil // Already up-to-date.
	}

	patch := &pb.FsoRepoNaming{
		GlobalRoot: globalRoot,
		Rule:       "PathPatterns",
		Config: &pb.ConfigMap{Fields: []*pb.ConfigField{
			&pb.ConfigField{
				Key: "enabledPaths",
				Val: &pb.ConfigField_TextList{
					&pb.StringList{Vals: addSet},
				},
			},
		}},
	}
	// It must be valid by construction.  Double check anyway before
	// storing the event.
	if err := pbevents.ValidateRepoNamingPatch(patch); err != nil {
		return nil, &InternalError{What: "invalid patch", Err: err}
	}

	return newEvents(
		state.Vid(), pbevents.NewRepoNamingConfigUpdated(patch),
	)
}

func tellSetRepoInitPolicy(
	state *State, cmd *CmdSetRepoInitPolicy,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	policy := cmd.Policy
	if err := pbevents.ValidateRepoInitPolicy(policy); err != nil {
		return nil, err
	}

	root := strings.TrimRight(policy.GlobalRoot, "/")
	rootSt, ok := state.roots[root]
	if !ok {
		return nil, ErrUnknownRoot
	}

	if proto.Equal(rootSt.repoInitPolicy, policy) {
		// Already up-to-date.
		return nil, nil
	}

	return newEvents(
		state.Vid(), pbevents.NewRepoInitPolicyUpdated(policy),
	)
}

func (cmd *CmdUpdateRootArchiveRecipients) checkTell() error {
	if len(cmd.Keys) == 0 {
		return ErrNoGPGKeys
	}
	if cmd.Keys.HasDuplicate() {
		return ErrDuplicateGPGKeys
	}
	return nil
}

func tellUpdateRootArchiveRecipients(
	st *State, cmd *CmdUpdateRootArchiveRecipients,
) ([]events.Event, error) {
	if err := cmd.checkTell(); err != nil {
		return nil, err
	}

	if st.info == nil {
		return nil, ErrNotInitialized
	}

	rootPath := strings.TrimRight(cmd.GlobalRoot, "/")
	rootSt, ok := st.roots[rootPath]
	if !ok {
		return nil, ErrUnknownRoot
	}

	if rootSt.info.ArchiveRecipients.Equal(cmd.Keys) {
		return nil, nil // idempotent
	}

	return newEvents(st.Vid(),
		pbevents.NewRootArchiveRecipientsUpdated(
			rootPath, cmd.Keys.Bytes(),
		),
	)
}

func tellDeleteRootArchiveRecipients(
	st *State, cmd *CmdDeleteRootArchiveRecipients,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrNotInitialized
	}

	rootPath := strings.TrimRight(cmd.GlobalRoot, "/")
	rootSt, ok := st.roots[rootPath]
	if !ok {
		return nil, ErrUnknownRoot
	}

	if len(rootSt.info.ArchiveRecipients) == 0 {
		return nil, nil // idempotent
	}

	return newEvents(st.Vid(),
		pbevents.NewRootArchiveRecipientsUpdated(
			rootPath, nil,
		),
	)
}

func (cmd *CmdUpdateRootShadowBackupRecipients) checkTell() error {
	if len(cmd.Keys) == 0 {
		return ErrNoGPGKeys
	}
	if cmd.Keys.HasDuplicate() {
		return ErrDuplicateGPGKeys
	}
	return nil
}

func tellUpdateRootShadowBackupRecipients(
	st *State, cmd *CmdUpdateRootShadowBackupRecipients,
) ([]events.Event, error) {
	if err := cmd.checkTell(); err != nil {
		return nil, err
	}

	if st.info == nil {
		return nil, ErrNotInitialized
	}

	rootPath := strings.TrimRight(cmd.GlobalRoot, "/")
	rootSt, ok := st.roots[rootPath]
	if !ok {
		return nil, ErrUnknownRoot
	}

	if rootSt.info.ShadowBackupRecipients.Equal(cmd.Keys) {
		return nil, nil // idempotent
	}

	return newEvents(st.Vid(),
		pbevents.NewRootShadowBackupRecipientsUpdated(
			rootPath, cmd.Keys.Bytes(),
		),
	)
}

func tellDeleteRootShadowBackupRecipients(
	st *State, cmd *CmdDeleteRootShadowBackupRecipients,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrNotInitialized
	}

	rootPath := strings.TrimRight(cmd.GlobalRoot, "/")
	rootSt, ok := st.roots[rootPath]
	if !ok {
		return nil, ErrUnknownRoot
	}

	if len(rootSt.info.ShadowBackupRecipients) == 0 {
		return nil, nil // idempotent
	}

	return newEvents(st.Vid(),
		pbevents.NewRootShadowBackupRecipientsUpdated(
			rootPath, nil,
		),
	)
}

func (b *Behavior) tellInitRepo(
	state *State, cmd *CmdInitRepo,
) ([]events.Event, error) {
	if cmd.CreatorName == "" {
		return nil, ErrCreatorNameMissing
	}
	if cmd.CreatorEmail == "" {
		return nil, ErrCreatorEmailMissing
	}

	if state.info == nil {
		return nil, ErrUninitialized
	}

	gpath := strings.TrimRight(cmd.GlobalPath, "/")
	if _, ok := state.reposByName[gpath]; ok {
		return nil, ErrConflictRepoInit
	}

	root := findRootStateForRepo(state.roots, gpath)
	if root == nil {
		return nil, ErrUnknownRoot
	}

	tracking, err := WhichSubdirTracking(root.repoInitPolicy, gpath)
	if err != nil {
		return nil, err
	}

	parent := findParentRepo(state.reposByName, gpath)
	if parent != nil && parent.StorageTier != StorageOnline {
		return nil, ErrParentRepoImmutable
	}

	reason, err := b.pre.isInitRepoAllowed(
		cmd.Context,
		gpath, root.info.Host, root.info.hostPath(gpath),
		tracking,
	)
	if err != nil {
		return nil, err
	}
	if reason != "" {
		return nil, &InitRepoDenyError{Reason: reason}
	}

	// XXX Check invariants, e.g. no nesting.

	return newEvents(state.Vid(), pbevents.NewRepoAccepted(
		&pb.FsoRepoInfo{
			Id:           cmd.Id[:],
			GlobalPath:   gpath,
			CreatorName:  cmd.CreatorName,
			CreatorEmail: cmd.CreatorEmail,
		},
	))
}

func tellReinitRepo(state *State, cmd *CmdReinitRepo) ([]events.Event, error) {
	if cmd.Reason == "" {
		return nil, ErrReasonEmpty
	}

	if state.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := state.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	if repo.ReinitReason == cmd.Reason {
		return nil, ErrReasonAlreadyApplied
	}

	return newEvents(state.Vid(), pbevents.NewRepoReinitAccepted(
		repo.Id[:],
		repo.GlobalPath,
		cmd.Reason,
	))
}

func tellEnableGitlabRepo(
	state *State, cmd *CmdEnableGitlabRepo,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := state.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	// XXX Should be validated more strictly, using regex or so.
	gns := cmd.GitlabNamespace
	gns = strings.TrimRight(gns, "/")
	if !strings.Contains(gns, "/") {
		return nil, ErrGitlabNamespaceMissingSlash
	}

	if repo.GitlabNamespace != "" {
		if gns == repo.GitlabNamespace {
			return nil, nil
		}
		return nil, ErrConflictGitlabInit
	}

	return newEvents(state.Vid(), pbevents.NewRepoEnableGitlabAccepted(
		repo.Id, gns,
	))
}

func (cfg *SplitRootConfig) checkTell() error {
	if cfg == nil {
		return ErrInvalidSplitRootConfig
	}
	if cfg.MaxDepth < 0 || cfg.MaxDepth > 8 {
		return ErrInvalidSplitRootConfig
	}
	if cfg.MinDiskUsage < 0 {
		return ErrInvalidSplitRootConfig
	}
	if cfg.MaxDiskUsage < 0 {
		return ErrInvalidSplitRootConfig
	}
	return nil
}

func (a *SplitRootConfig) conflictsWith(b *SplitRootConfig) bool {
	return (a.MaxDepth > 0 && a.MaxDepth != b.MaxDepth) ||
		(a.MinDiskUsage > 0 && a.MinDiskUsage != b.MinDiskUsage) ||
		(a.MaxDiskUsage > 0 && a.MinDiskUsage != b.MinDiskUsage)
}

func tellCreateSplitRootConfig(
	state *State, cmd *CmdCreateSplitRootConfig,
) ([]events.Event, error) {
	cfg := cmd.Config
	if err := cfg.checkTell(); err != nil {
		return nil, err
	}

	if state.info == nil {
		return nil, ErrUninitialized
	}

	rootPath := strings.TrimRight(cmd.GlobalRoot, "/")
	rootSt, ok := state.roots[rootPath]
	if !ok {
		return nil, ErrUnknownRoot
	}

	cfgSt := rootSt.splitRootConfig
	if cfgSt != nil {
		if cfg.conflictsWith(cfgSt) {
			return nil, ErrSplitRootConfigExists
		}
		return nil, nil // idempotent
	}

	const GiB = 1 << 30
	evCfg := &pb.FsoSplitRootParams{
		MaxDepth:     3,
		MinDiskUsage: 20 * GiB,
		MaxDiskUsage: 400 * GiB,
	}
	if cfg.MaxDepth > 0 {
		evCfg.MaxDepth = cfg.MaxDepth
	}
	if cfg.MinDiskUsage > 0 {
		evCfg.MinDiskUsage = cfg.MinDiskUsage
	}
	if cfg.MaxDiskUsage > 0 {
		evCfg.MaxDiskUsage = cfg.MaxDiskUsage
	}
	return newEvents(state.Vid(),
		pbevents.NewSplitRootEnabled(rootPath),
		pbevents.NewSplitRootParamsUpdated(rootPath, evCfg),
	)
}

func tellUpdateSplitRootConfig(
	state *State, cmd *CmdUpdateSplitRootConfig,
) ([]events.Event, error) {
	cfg := cmd.Config
	if err := cfg.checkTell(); err != nil {
		return nil, err
	}

	if state.info == nil {
		return nil, ErrUninitialized
	}

	rootPath := strings.TrimRight(cmd.GlobalRoot, "/")
	rootSt, ok := state.roots[rootPath]
	if !ok {
		return nil, ErrUnknownRoot
	}

	cfgSt := rootSt.splitRootConfig
	if cfgSt == nil {
		return nil, ErrNoSplitRootConfig
	}

	evCfg := &pb.FsoSplitRootParams{
		MaxDepth:     cfgSt.MaxDepth,
		MinDiskUsage: cfgSt.MinDiskUsage,
		MaxDiskUsage: cfgSt.MaxDiskUsage,
	}
	update := false
	if cfg.MaxDepth > 0 && cfg.MaxDepth != evCfg.MaxDepth {
		evCfg.MaxDepth = cfg.MaxDepth
		update = true
	}
	if cfg.MinDiskUsage > 0 && cfg.MinDiskUsage != evCfg.MinDiskUsage {
		evCfg.MinDiskUsage = cfg.MinDiskUsage
		update = true
	}
	if cfg.MaxDiskUsage > 0 && cfg.MaxDiskUsage != evCfg.MaxDiskUsage {
		evCfg.MaxDiskUsage = cfg.MaxDiskUsage
		update = true
	}
	if !update {
		return nil, nil // idempotent
	}
	return newEvents(state.Vid(),
		pbevents.NewSplitRootParamsUpdated(rootPath, evCfg),
	)
}

func tellDeleteSplitRootConfig(
	state *State, cmd *CmdDeleteSplitRootConfig,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	rootPath := strings.TrimRight(cmd.GlobalRoot, "/")
	rootSt, ok := state.roots[rootPath]
	if !ok {
		return nil, ErrUnknownRoot
	}

	cfgSt := rootSt.splitRootConfig
	if cfgSt == nil {
		return nil, ErrNoSplitRootConfig
	}

	return newEvents(state.Vid(),
		pbevents.NewSplitRootDisabled(rootPath),
	)
}

func tellSetPathFlags(
	st *State, cmd *CmdSetPathFlags,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	path := slashpath.Clean(cmd.Path)
	if !slashpath.IsAbs(path) {
		return nil, ErrMalformedPath
	}

	f := st.pathFlags[path]
	if f|cmd.Flags == f {
		return nil, nil // idempotent
	}

	return newEvents(
		st.Vid(),
		pbevents.NewPathFlagSet(path, cmd.Flags),
	)
}

func tellUnsetPathFlags(
	st *State, cmd *CmdUnsetPathFlags,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	path := slashpath.Clean(cmd.Path)
	if !slashpath.IsAbs(path) {
		return nil, ErrMalformedPath
	}

	f := st.pathFlags[path]
	if f&^cmd.Flags == f {
		return nil, nil // idempotent
	}

	return newEvents(
		st.Vid(),
		pbevents.NewPathFlagUnset(path, cmd.Flags),
	)
}

// `MayFreezeRepo()` is used in `BeginFreezeRepo()` to check preconditions
// before initializing a freeze-repo workflow.
func (st *State) MayFreezeRepo(repoId uuid.I) (ok bool, reason string) {
	repo, ok := st.reposById[repoId]
	if !ok {
		return false, "unknown repo"
	}
	return st.mayFreezeRepoInfo(repo)
}

// Only leaf repos may be frozen.
func (st *State) mayFreezeRepoInfo(repo *RepoInfo) (ok bool, reason string) {
	switch repo.StorageTier {
	case StorageOnline:
		break // Data that is online can be frozen.
	case StorageFreezeFailed:
		break // A failed freeze can be retried.
	default:
		return false, "conflicting repo storage state"
	}
	if len(st.ReposPrefix(repo.GlobalPath)) != 1 {
		return false, "not a leaf repo"
	}
	return true, ""
}

func tellBeginFreezeRepo(
	st *State, cmd *CmdBeginFreezeRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	switch repo.StorageTier {
	case StorageOnline:
		break // ok to begin freeze.
	case StorageFreezing:
		if repo.storageWorkflowId != cmd.WorkflowId {
			return nil, ErrConflictWorkflow
		}
		return nil, nil // idempotent
	case StorageFreezeFailed:
		break // A failed freeze can be retried.
	default:
		return nil, ErrConflictWorkflow
	}

	if ok, _ = st.mayFreezeRepoInfo(repo); !ok {
		return nil, ErrCannotFreezeRepo
	}

	return newEvents(
		st.Vid(),
		pbevents.NewFreezeRepoStarted2(cmd.RepoId, cmd.WorkflowId),
	)
}

func tellCommitFreezeRepo(
	st *State, cmd *CmdCommitFreezeRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	if repo.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictWorkflow
	}
	switch repo.StorageTier {
	case StorageFreezing:
		break // ok to commit if operation in progress.
	case StorageFrozen:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewFreezeRepoCompleted2Ok(cmd.RepoId, cmd.WorkflowId),
	)
}

func tellAbortFreezeRepo(
	st *State, cmd *CmdAbortFreezeRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	if repo.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictWorkflow
	}
	switch repo.StorageTier {
	case StorageFreezing:
		break // ok to abort if operation in progress.
	case StorageFreezeFailed:
		// XXX Maybe check that `StatusCode` is idempotent.
		return nil, nil // idempotent
	default:
		return nil, ErrConflictWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewFreezeRepoCompleted2Error(
			cmd.RepoId, cmd.WorkflowId, cmd.Code,
		),
	)
}

// `MayUnfreezeRepo()` is used in `BeginUnfreezeRepo()` to check preconditions
// before initializing a freeze-repo workflow.
func (st *State) MayUnfreezeRepo(repoId uuid.I) (ok bool, reason string) {
	repo, ok := st.reposById[repoId]
	if !ok {
		return false, "unknown repo"
	}
	return st.mayUnfreezeRepoInfo(repo)
}

// Any frozen repo may be unfrozen.
func (st *State) mayUnfreezeRepoInfo(repo *RepoInfo) (ok bool, reason string) {
	switch repo.StorageTier {
	case StorageFrozen:
		break // Data that is frozen can be unfrozen.
	case StorageUnfreezeFailed:
		break // A failed unfreeze can be retried.
	default:
		return false, "conflicting repo storage state"
	}
	return true, ""
}

func tellBeginUnfreezeRepo(
	st *State, cmd *CmdBeginUnfreezeRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	switch repo.StorageTier {
	case StorageFrozen:
		break // ok to begin unfreeze.
	case StorageUnfreezing:
		if repo.storageWorkflowId != cmd.WorkflowId {
			return nil, ErrConflictWorkflow
		}
		return nil, nil // idempotent
	case StorageUnfreezeFailed:
		break // A failed unfreeze can be retried.
	default:
		return nil, ErrConflictWorkflow
	}

	if ok, _ = st.mayUnfreezeRepoInfo(repo); !ok {
		return nil, ErrCannotUnfreezeRepo
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnfreezeRepoStarted2(cmd.RepoId, cmd.WorkflowId),
	)
}

func tellCommitUnfreezeRepo(
	st *State, cmd *CmdCommitUnfreezeRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	if repo.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictWorkflow
	}
	switch repo.StorageTier {
	case StorageUnfreezing:
		break // ok to commit if operation in progress.
	case StorageUnfreezeFailed:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnfreezeRepoCompleted2Ok(cmd.RepoId, cmd.WorkflowId),
	)
}

func tellAbortUnfreezeRepo(
	st *State, cmd *CmdAbortUnfreezeRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	if repo.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictWorkflow
	}
	switch repo.StorageTier {
	case StorageUnfreezing:
		break // ok to abort if operation in progress.
	case StorageUnfreezeFailed:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnfreezeRepoCompleted2Error(
			cmd.RepoId, cmd.WorkflowId, cmd.Code,
		),
	)
}

// `MayArchiveRepo()` is used in `BeginArchiveRepo()` to check preconditions
// before initializing an archive-repo workflow.
func (st *State) MayArchiveRepo(repoId uuid.I) (ok bool, reason string) {
	repo, ok := st.reposById[repoId]
	if !ok {
		return false, "unknown repo"
	}
	return st.mayArchiveRepoInfo(repo)
}

// Only leaf repos may be archived.
//
// Repos at the root toplevel may not be archived to avoid potential confusion
// with other services, e.g. Samba or NFS might export the directory;
// bcpfs-perms could concurrently check the directory.
func (st *State) mayArchiveRepoInfo(repo *RepoInfo) (ok bool, reason string) {
	switch repo.StorageTier {
	case StorageFrozen:
		break // Data that is frozen can be archived.
	case StorageArchiveFailed:
		break // A failed archive can be retried.
	default:
		return false, "conflicting repo storage state"
	}
	if _, ok := st.Root(repo.GlobalPath); ok {
		return false, "cannot archive repo at root toplevel"
	}
	if len(st.ReposPrefix(repo.GlobalPath)) != 1 {
		return false, "not a leaf repo"
	}
	return true, ""
}

func tellBeginArchiveRepo(
	st *State, cmd *CmdBeginArchiveRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	switch repo.StorageTier {
	case StorageFrozen:
		break // ok to begin archive.
	case StorageArchiving:
		if repo.storageWorkflowId != cmd.WorkflowId {
			return nil, ErrConflictWorkflow
		}
		return nil, nil // idempotent
	case StorageArchiveFailed:
		break // A failed freeze can be retried.
	default:
		return nil, ErrConflictWorkflow
	}

	if ok, _ = st.mayArchiveRepoInfo(repo); !ok {
		return nil, ErrCannotArchiveRepo
	}

	return newEvents(
		st.Vid(),
		pbevents.NewArchiveRepoStarted(cmd.RepoId, cmd.WorkflowId),
	)
}

func tellCommitArchiveRepo(
	st *State, cmd *CmdCommitArchiveRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	if repo.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictWorkflow
	}
	switch repo.StorageTier {
	case StorageArchiving:
		break // ok to commit if operation in progress.
	case StorageFrozen:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewArchiveRepoCompletedOk(cmd.RepoId, cmd.WorkflowId),
	)
}

func tellAbortArchiveRepo(
	st *State, cmd *CmdAbortArchiveRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	if repo.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictWorkflow
	}
	switch repo.StorageTier {
	case StorageArchiving:
		break // ok to abort if operation in progress.
	case StorageArchiveFailed:
		// XXX Maybe check that `StatusCode` is idempotent.
		return nil, nil // idempotent
	default:
		return nil, ErrConflictWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewArchiveRepoCompletedError(
			cmd.RepoId, cmd.WorkflowId, cmd.Code,
		),
	)
}

// `MayUnarchiveRepo()` is used in `BeginUnarchiveRepo()` to check
// preconditions before initializing an archive-repo workflow.
func (st *State) MayUnarchiveRepo(repoId uuid.I) (ok bool, reason string) {
	repo, ok := st.reposById[repoId]
	if !ok {
		return false, "unknown repo"
	}
	return st.mayUnarchiveRepoInfo(repo)
}

// See `mayArchiveRepoInfo()` for general rules.
func (st *State) mayUnarchiveRepoInfo(repo *RepoInfo) (ok bool, reason string) {
	switch repo.StorageTier {
	case StorageArchived:
		break // Data that is archived can be unarchived.
	case StorageUnarchiveFailed:
		break // A failed unarchive can be retried.
	default:
		return false, "conflicting repo storage state"
	}
	if _, ok := st.Root(repo.GlobalPath); ok {
		return false, "cannot unarchive repo at root toplevel"
	}
	if len(st.ReposPrefix(repo.GlobalPath)) != 1 {
		return false, "not a leaf repo"
	}
	return true, ""
}

func tellBeginUnarchiveRepo(
	st *State, cmd *CmdBeginUnarchiveRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	switch repo.StorageTier {
	case StorageArchived:
		break // ok to begin unarchive.
	case StorageUnarchiving:
		if repo.storageWorkflowId != cmd.WorkflowId {
			return nil, ErrConflictWorkflow
		}
		return nil, nil // idempotent
	case StorageUnarchiveFailed:
		break // A failed freeze can be retried.
	default:
		return nil, ErrConflictWorkflow
	}

	if ok, _ = st.mayUnarchiveRepoInfo(repo); !ok {
		return nil, ErrCannotUnarchiveRepo
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnarchiveRepoStarted(cmd.RepoId, cmd.WorkflowId),
	)
}

func tellCommitUnarchiveRepo(
	st *State, cmd *CmdCommitUnarchiveRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	if repo.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictWorkflow
	}
	switch repo.StorageTier {
	case StorageUnarchiving:
		break // ok to commit if operation in progress.
	case StorageFrozen:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnarchiveRepoCompletedOk(
			cmd.RepoId, cmd.WorkflowId,
		),
	)
}

func tellAbortUnarchiveRepo(
	st *State, cmd *CmdAbortUnarchiveRepo,
) ([]events.Event, error) {
	if st.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := st.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	if repo.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictWorkflow
	}
	switch repo.StorageTier {
	case StorageUnarchiving:
		break // ok to abort if operation in progress.
	case StorageUnarchiveFailed:
		// XXX Maybe check that `StatusCode` is idempotent.
		return nil, nil // idempotent
	default:
		return nil, ErrConflictWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnarchiveRepoCompletedError(
			cmd.RepoId, cmd.WorkflowId, cmd.Code,
		),
	)
}

func findRootInfoForRepo(roots map[string]*rootState, path string) *RootInfo {
	st := findRootStateForRepo(roots, path)
	if st == nil {
		return nil
	}
	return &st.info
}

func findRootStateForRepo(
	roots map[string]*rootState, path string,
) *rootState {
	for _, st := range roots {
		if pathIsEqualOrBelowPrefix(path, st.info.GlobalRoot) {
			return st
		}
	}
	return nil
}

func findParentRepo(
	repos map[string]*RepoInfo, path string,
) *RepoInfo {
	for path != "/" {
		if r, ok := repos[path]; ok {
			return r
		}
		path = slashpath.Dir(path)
	}
	return nil
}

// `path` and `prefix` both without trailing slash.
func pathIsEqualOrBelowPrefix(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	// Equal or slash right after prefix.
	return len(path) == len(prefix) || path[len(prefix)] == '/'
}

// `path` and `prefix` both without trailing slash.
func pathIsBelowPrefixStrict(path, prefix string) bool {
	// `path` must have at least an additional `/x`, i.e. it must be at
	// least two longer than `prefix`.
	return pathIsEqualOrBelowPrefix(path, prefix) &&
		len(path) >= len(prefix)+2
}

func (r *RootInfo) hostPath(gpath string) string {
	if !strings.HasPrefix(gpath, r.GlobalRoot) {
		panic("global path not below root")
	}
	return slashpath.Join(
		r.HostRoot, strings.TrimPrefix(gpath, r.GlobalRoot),
	)
}

func tellConfirmRepo(
	state *State, cmd *CmdConfirmRepo,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	inf, ok := state.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	if inf.Confirmed {
		// Already confirmed.
		return nil, nil
	}

	return newEvents(state.Vid(), pbevents.NewRepoAdded(
		inf.Id[:],
		inf.GlobalPath,
		cmd.RepoEventId,
	))
}

func (b *Behavior) tellBeginMoveRepo(
	state *State, cmd *CmdBeginMoveRepo,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := state.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}
	if !repo.Confirmed {
		return nil, ErrCannotMoveUnconfirmed
	}

	workflowId := cmd.WorkflowId
	if workflowId == uuid.Nil {
		return nil, ErrMalformedWorkflowId
	}
	if repo.hasActiveMoveRepo() {
		// XXX Maybe check idempotency.
		return nil, ErrConflictWorkflow
	}
	if workflowId == repo.moveRepoWorkflow {
		return nil, ErrWorkflowReuse
	}

	reason, err := b.pre.isAllowedAsUnusedWorkflowId(workflowId)
	if err != nil {
		return nil, err
	}
	if reason != "" {
		return nil, &WorkflowIdError{Reason: reason}
	}

	gpath := strings.TrimRight(cmd.NewGlobalPath, "/")
	if cmd.IsUnchangedGlobalPath {
		if gpath != repo.GlobalPath {
			return nil, ErrPathChanged
		}
	} else {
		if gpath == repo.GlobalPath {
			return nil, ErrPathUnchanged
		}
		if _, ok := state.reposByName[gpath]; ok {
			return nil, ErrConflictRepoInit
		}
	}

	root := findRootStateForRepo(state.roots, gpath)
	if root == nil {
		return nil, ErrUnknownRoot
	}

	return newEvents(state.Vid(), pbevents.NewPbRepoMoveAccepted(
		&pbevents.EvRepoMoveAccepted{
			RepoId:        repo.Id,
			WorkflowId:    workflowId,
			NewGlobalPath: gpath,
		},
	))
}

func (b *Behavior) tellCommitMoveRepo(
	state *State, cmd *CmdCommitMoveRepo,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	repo, ok := state.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	workflowId := cmd.WorkflowId
	if workflowId == uuid.Nil {
		return nil, ErrMalformedWorkflowId
	}
	if workflowId != repo.moveRepoWorkflow {
		return nil, ErrConflictWorkflow
	}
	if !repo.hasActiveMoveRepo() {
		return nil, ErrWorkflowTerminated
	}

	if cmd.GlobalPath != repo.newGlobalPath {
		return nil, ErrMismatchGlobalPath
	}

	ev := &pbevents.EvRepoMoved{
		RepoId:      repo.Id,
		RepoEventId: cmd.RepoEventId,
		WorkflowId:  workflowId,
		GlobalPath:  cmd.GlobalPath,
	}
	return newEvents(state.Vid(), pbevents.NewPbRepoMoved(ev))
}

func tellPostShadowRepoMoveStarted(
	state *State, cmd *CmdPostShadowRepoMoveStarted,
) ([]events.Event, error) {
	if state.info == nil {
		return nil, ErrUninitialized
	}

	inf, ok := state.reposById[cmd.RepoId]
	if !ok {
		return nil, ErrUnknownRepo
	}

	// Don't handle idempotency, assuming that the caller posts each event
	// only once.  But, as a safety measure, check that the caller does not
	// post the latest event again.  A more complete check could maintain
	// more history and check that the event is new against several latest
	// repo events.
	if cmd.RepoEventId == inf.lastRepoEventId {
		return nil, ErrRepeatedPost
	}

	return newEvents(state.Vid(), pbevents.NewShadowRepoMoveStarted(
		cmd.RepoId,
		cmd.RepoEventId,
		cmd.WorkflowId,
	))
}

type Registry struct {
	engine *events.Engine
}

func New(journal *events.Journal, p Preconditions) *Registry {
	eng := events.NewEngine(journal, &Behavior{pre: p})
	return &Registry{engine: eng}
}

func (r *Registry) Init(
	id uuid.I, info *Info,
) (ulid.I, error) {
	cmd := CmdInitRegistry(*info)
	return r.engine.TellIdVid(id, NoVC, &cmd)
}

func (r *Registry) EnableEphemeralWorkflows(
	id uuid.I, vid ulid.I, ephemeralWorkflowsId uuid.I,
) (ulid.I, error) {
	cmd := &CmdEnableEphemeralWorkflows{
		EphemeralWorkflowsId: ephemeralWorkflowsId,
	}
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) EnablePropagateRootAcls(
	id uuid.I, vid ulid.I,
) (ulid.I, error) {
	cmd := &CmdEnablePropagateRootAcls{}
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) InitRoot(
	id uuid.I, vid ulid.I, cmd *CmdInitRoot,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) RemoveRoot(
	id uuid.I, vid ulid.I, root string,
) (ulid.I, error) {
	cmd := &CmdRemoveRoot{root}
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) EnableGitlab(
	id uuid.I, vid ulid.I, cmd *CmdEnableGitlab,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) DisableGitlab(
	id uuid.I, vid ulid.I, root string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdDisableGitlab{GlobalRoot: root})
}

func (r *Registry) SetRepoNaming(
	id uuid.I, vid ulid.I, naming *pb.FsoRepoNaming,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdSetRepoNaming{Naming: naming})
}

func (r *Registry) PatchRepoNaming(
	id uuid.I, vid ulid.I, namingPatch *pb.FsoRepoNaming,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdPatchRepoNaming{
		NamingPatch: namingPatch,
	})
}

func (r *Registry) EnableDiscoveryPaths(
	id uuid.I, vid ulid.I, globalRoot string, paths []DepthPath,
) (ulid.I, error) {
	globalRoot = strings.TrimRight(globalRoot, "/")
	return r.engine.TellIdVid(id, vid, &CmdEnableDiscoveryPaths{
		GlobalRoot: globalRoot,
		DepthPaths: paths,
	})
}

func (r *Registry) SetRepoInitPolicy(
	id uuid.I, vid ulid.I, policy *pb.FsoRepoInitPolicy,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdSetRepoInitPolicy{
		Policy: policy,
	})
}

// `UpdateRootArchiveRecipients()` sets the archive GPG keys, enabling encryption.
func (r *Registry) UpdateRootArchiveRecipients(
	id uuid.I, vid ulid.I, globalRoot string, keys gpg.Fingerprints,
) (*State, error) {
	cmd := &CmdUpdateRootArchiveRecipients{
		GlobalRoot: globalRoot,
		Keys:       keys,
	}
	st, err := r.engine.TellIdVidState(id, vid, cmd)
	if err != nil {
		return nil, err
	}
	return st.(*State), nil
}

// `DeleteRootArchiveRecipients()` disables archive encryption.
func (r *Registry) DeleteRootArchiveRecipients(
	id uuid.I, vid ulid.I, globalRoot string,
) (ulid.I, error) {
	cmd := &CmdDeleteRootArchiveRecipients{
		GlobalRoot: globalRoot,
	}
	return r.engine.TellIdVid(id, vid, cmd)
}

// `UpdateRootShadowBackupRecipients()` sets the archive GPG keys, enabling encryption.
func (r *Registry) UpdateRootShadowBackupRecipients(
	id uuid.I, vid ulid.I, globalRoot string, keys gpg.Fingerprints,
) (*State, error) {
	cmd := &CmdUpdateRootShadowBackupRecipients{
		GlobalRoot: globalRoot,
		Keys:       keys,
	}
	st, err := r.engine.TellIdVidState(id, vid, cmd)
	if err != nil {
		return nil, err
	}
	return st.(*State), nil
}

// `DeleteRootShadowBackupRecipients()` disables archive encryption.
func (r *Registry) DeleteRootShadowBackupRecipients(
	id uuid.I, vid ulid.I, globalRoot string,
) (ulid.I, error) {
	cmd := &CmdDeleteRootShadowBackupRecipients{
		GlobalRoot: globalRoot,
	}
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) InitRepo(
	id uuid.I, vid ulid.I, cmd *CmdInitRepo,
) (newVid ulid.I, repoId uuid.I, err error) {
	cmd, err = cmd.ensureId()
	if err != nil {
		return ulid.Nil, uuid.Nil, err
	}

	newVid, err = r.engine.TellIdVid(id, vid, cmd)
	if err != nil {
		return ulid.Nil, uuid.Nil, err
	}

	return newVid, cmd.Id, nil
}

func (r *Registry) ReinitRepo(
	id uuid.I, vid ulid.I, cmd *CmdReinitRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) EnableGitlabRepo(
	id uuid.I, vid ulid.I, repoId uuid.I, gitlabNamespace string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdEnableGitlabRepo{
		RepoId:          repoId,
		GitlabNamespace: gitlabNamespace,
	})
}

func (r *Registry) ConfirmRepo(
	id uuid.I, vid ulid.I, repoId uuid.I, repoEventId ulid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdConfirmRepo{
		RepoId:      repoId,
		RepoEventId: repoEventId,
	})
}

func (r *Registry) BeginMoveRepo(
	id uuid.I, vid ulid.I, cmd *CmdBeginMoveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) CommitMoveRepo(
	id uuid.I, vid ulid.I, cmd *CmdCommitMoveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `PostShadowRepoMoveStarted()` is called by Nogfsoregd in `replicated.go` to
// duplicate `EV_FSO_SHADOW_REPO_MOVE_STARTED` repo events to the registry, so
// that Nogfsostad learns about them by watching the registry.
func (r *Registry) PostShadowRepoMoveStarted(
	id uuid.I, vid ulid.I,
	repoId uuid.I, repoEventId ulid.I, workflowId uuid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdPostShadowRepoMoveStarted{
		RepoId:      repoId,
		RepoEventId: repoEventId,
		WorkflowId:  workflowId,
	})
}

func (r *Registry) CreateSplitRootConfig(
	id uuid.I, vid ulid.I, root string, cfg *SplitRootConfig,
) (*State, error) {
	s, err := r.engine.TellIdVidState(id, vid, &CmdCreateSplitRootConfig{
		GlobalRoot: root,
		Config:     cfg,
	})
	if err != nil {
		return nil, err
	}
	return s.(*State), nil
}

func (r *Registry) UpdateSplitRootConfig(
	id uuid.I, vid ulid.I, root string, cfg *SplitRootConfig,
) (*State, error) {
	s, err := r.engine.TellIdVidState(id, vid, &CmdUpdateSplitRootConfig{
		GlobalRoot: root,
		Config:     cfg,
	})
	if err != nil {
		return nil, err
	}
	return s.(*State), nil
}

func (r *Registry) DeleteSplitRootConfig(
	id uuid.I, vid ulid.I, root string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdDeleteSplitRootConfig{
		GlobalRoot: root,
	})
}

func (r *Registry) SetPathFlags(
	id uuid.I, vid ulid.I, path string, flags uint32,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdSetPathFlags{
		Path:  path,
		Flags: flags,
	})
}

func (r *Registry) UnsetPathFlags(
	id uuid.I, vid ulid.I, path string, flags uint32,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdUnsetPathFlags{
		Path:  path,
		Flags: flags,
	})
}

func (r *Registry) BeginFreezeRepo(
	id uuid.I, vid ulid.I, cmd *CmdBeginFreezeRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) CommitFreezeRepo(
	id uuid.I, vid ulid.I, cmd *CmdCommitFreezeRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) AbortFreezeRepo(
	id uuid.I, vid ulid.I, cmd *CmdAbortFreezeRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) BeginUnfreezeRepo(
	id uuid.I, vid ulid.I, cmd *CmdBeginUnfreezeRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) CommitUnfreezeRepo(
	id uuid.I, vid ulid.I, cmd *CmdCommitUnfreezeRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) AbortUnfreezeRepo(
	id uuid.I, vid ulid.I, cmd *CmdAbortUnfreezeRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) BeginArchiveRepo(
	id uuid.I, vid ulid.I, cmd *CmdBeginArchiveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) CommitArchiveRepo(
	id uuid.I, vid ulid.I, cmd *CmdCommitArchiveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) AbortArchiveRepo(
	id uuid.I, vid ulid.I, cmd *CmdAbortArchiveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) BeginUnarchiveRepo(
	id uuid.I, vid ulid.I, cmd *CmdBeginUnarchiveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) CommitUnarchiveRepo(
	id uuid.I, vid ulid.I, cmd *CmdCommitUnarchiveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) AbortUnarchiveRepo(
	id uuid.I, vid ulid.I, cmd *CmdAbortUnarchiveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Registry) FindId(id uuid.I) (*State, error) {
	s, err := r.engine.FindId(id)
	if err != nil {
		return nil, err
	}
	if s.Vid() == events.EventEpoch {
		return nil, ErrUninitialized
	}
	return s.(*State), nil
}

func (s *State) Name() string { return s.info.Name }

func (s *State) NumRoots() int { return len(s.roots) }
func (s *State) NumRepos() int { return len(s.reposByName) }

func (s *State) Roots() []*RootInfo {
	var roots []*RootInfo
	for _, r := range s.roots {
		roots = append(roots, &r.info)
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].GlobalRoot < roots[j].GlobalRoot
	})
	return roots
}

func (s *State) Root(root string) (*RootInfo, bool) {
	r, ok := s.roots[root]
	if !ok {
		return nil, false
	}
	return &r.info, ok
}

func (s *State) HasRoot(root string) bool {
	_, ok := s.Root(root)
	return ok
}

func (s *State) SplitRootConfig(root string) (*SplitRootConfig, bool) {
	r, ok := s.roots[root]
	if !ok {
		return nil, false
	}
	if r.splitRootConfig == nil {
		return nil, false
	}
	return r.splitRootConfig, true
}

func (s *State) RepoRoot(id uuid.I) (*RootInfo, error) {
	r, err := s.repoRootState(id)
	if err != nil {
		return nil, err
	}
	return &r.info, nil
}

func (s *State) RepoAclPolicy(id uuid.I) (*pb.RepoAclPolicy, error) {
	r, err := s.repoRootState(id)
	if err != nil {
		return nil, err
	}

	switch s.repoAclPolicy {
	case pb.RepoAclPolicy_P_NO_ACLS:
		return &pb.RepoAclPolicy{
			Policy: pb.RepoAclPolicy_P_NO_ACLS,
		}, nil
	case pb.RepoAclPolicy_P_PROPAGATE_ROOT_ACLS:
		return &pb.RepoAclPolicy{
			Policy: pb.RepoAclPolicy_P_PROPAGATE_ROOT_ACLS,
			FsoRootInfo: &pb.FsoRootInfo{
				GlobalRoot: r.info.GlobalRoot,
				Host:       r.info.Host,
				HostRoot:   r.info.HostRoot,
			},
		}, nil
	default:
		panic("logic error")
	}
}

func (s *State) repoRootState(id uuid.I) (*rootState, error) {
	repo, ok := s.reposById[id]
	if !ok {
		return nil, ErrUnknownRepo
	}
	gpath := repo.GlobalPath
	for _, r := range s.roots {
		if pathIsEqualOrBelowPrefix(gpath, r.info.GlobalRoot) {
			return r, nil
		}
	}
	// If the repo is in the registry, there must be a root for it.
	panic("logic error")
}

func (s *State) Repos() []*RepoInfo {
	var repos []*RepoInfo
	for _, r := range s.reposByName {
		repos = append(repos, r)
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].GlobalPath < repos[j].GlobalPath
	})
	return repos
}

// `ReposPrefix(prefix)` returns repos whose global path is equal or below
// `prefix`.  `prefix` must not have a trailing slash.
func (s *State) ReposPrefix(prefix string) []*RepoInfo {
	var repos []*RepoInfo
	for gpath, r := range s.reposByName {
		if pathIsEqualOrBelowPrefix(gpath, prefix) {
			repos = append(repos, r)
		}
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].GlobalPath < repos[j].GlobalPath
	})
	return repos
}

func (s *State) RepoById(id uuid.I) (*RepoInfo, bool) {
	inf, ok := s.reposById[id]
	return inf, ok
}

func (s *State) RepoByPath(path string) (*RepoInfo, bool) {
	inf, ok := s.reposByName[path]
	return inf, ok
}

func (s *State) PathFlagsPrefix(prefix string) map[string]uint32 {
	flags := make(map[string]uint32)
	for p, f := range s.pathFlags {
		if pathIsEqualOrBelowPrefix(p, prefix) {
			flags[p] = f
		}
	}
	return flags
}
