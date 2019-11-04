package pbevents

import (
	"bytes"
	slashpath "path"
	"strconv"
	"strings"

	"github.com/nogproject/nog/backend/internal/configmap"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `RegistryEvent_EV_FSO_REGISTRY_ADDED` aka `EvRegistryAdded`.
type EvRegistryAdded struct {
	pb.FsoRegistryInfo
}

func (EvRegistryAdded) RegistryEvent() {}

func NewRegistryAdded(name string) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event: pb.RegistryEvent_EV_FSO_REGISTRY_ADDED,
		FsoRegistryInfo: &pb.FsoRegistryInfo{
			Name: name,
		},
	}
}

func fromPbRegistryAdded(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REGISTRY_ADDED {
		panic("invalid event")
	}
	ev := &EvRegistryAdded{
		FsoRegistryInfo: *evpb.FsoRegistryInfo,
	}
	return ev, nil
}

// `RegistryEvent_EV_EPHEMERAL_WORKFLOWS_ENABLED` aka
// EvEphemeralWorkflowsEnabled`.
type EvEphemeralWorkflowsEnabled struct {
	EphemeralWorkflowsId uuid.I
}

func (EvEphemeralWorkflowsEnabled) RegistryEvent() {}

func NewEphemeralWorkflowsEnabled(
	ephemeralWorkflowsId uuid.I,
) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event:                pb.RegistryEvent_EV_EPHEMERAL_WORKFLOWS_ENABLED,
		EphemeralWorkflowsId: ephemeralWorkflowsId[:],
	}
}

func fromPbEphemeralWorkflowsEnabled(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_EPHEMERAL_WORKFLOWS_ENABLED {
		panic("invalid event")
	}
	id, err := uuid.FromBytes(evpb.EphemeralWorkflowsId)
	if err != nil {
		return nil, err
	}
	return &EvEphemeralWorkflowsEnabled{
		EphemeralWorkflowsId: id,
	}, nil
}

// `EV_FSO_REPO_ACL_POLICY_UPDATED` aka `EvRepoPolicyUpdated` sets the policy
// that is used to control ACLs on repo real files.
type EvRepoAclPolicyUpdated struct {
	Policy pb.RepoAclPolicy_Policy
}

func (EvRepoAclPolicyUpdated) RegistryEvent() {}

func NewRepoAclPolicyUpdated(
	policy pb.RepoAclPolicy_Policy,
) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event: pb.RegistryEvent_EV_FSO_REPO_ACL_POLICY_UPDATED,
		RepoAclPolicy: &pb.RepoAclPolicy{
			Policy: policy,
		},
	}
}

func fromPbRepoAclPolicyUpdated(evpb pb.RegistryEvent) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REPO_ACL_POLICY_UPDATED {
		return nil, ErrInvalidEvent
	}

	pol := evpb.RepoAclPolicy
	if pol == nil {
		return nil, ErrInvalidEvent
	}

	ppol := pol.Policy
	switch ppol {
	case pb.RepoAclPolicy_P_NO_ACLS: // ok
	case pb.RepoAclPolicy_P_PROPAGATE_ROOT_ACLS: // ok
	default:
		return nil, ErrInvalidEvent
	}

	return &EvRepoAclPolicyUpdated{
		Policy: ppol,
	}, nil
}

// `RegistryEvent_EV_FSO_ROOT_ADDED` aka `EvRootAdded`.
type EvRootAdded struct {
	pb.FsoRootInfo
}

func (EvRootAdded) RegistryEvent() {}

func NewRootAdded(inf *pb.FsoRootInfo) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event:       pb.RegistryEvent_EV_FSO_ROOT_ADDED,
		FsoRootInfo: inf,
	}
}

func fromPbRootAdded(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_ROOT_ADDED {
		panic("invalid event")
	}
	ev := &EvRootAdded{
		FsoRootInfo: *evpb.FsoRootInfo,
	}
	return ev, nil
}

// `RegistryEvent_EV_FSO_ROOT_REMOVED` aka `EvRootRemoved`.
type EvRootRemoved struct {
	GlobalRoot string
}

func (EvRootRemoved) RegistryEvent() {}

func NewRootRemoved(root string) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event: pb.RegistryEvent_EV_FSO_ROOT_REMOVED,
		FsoRootInfo: &pb.FsoRootInfo{
			GlobalRoot: root,
		},
	}
}

func fromPbRootRemoved(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_ROOT_REMOVED {
		panic("invalid event")
	}
	ev := &EvRootRemoved{
		GlobalRoot: evpb.FsoRootInfo.GlobalRoot,
	}
	return ev, nil
}

// `RegistryEvent_EV_FSO_ROOT_UPDATED` aka `EvRootUpdated`.
type EvRootUpdated struct {
	pb.FsoRootInfo
}

func (EvRootUpdated) RegistryEvent() {}

func NewRootUpdated(inf *pb.FsoRootInfo) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event:       pb.RegistryEvent_EV_FSO_ROOT_UPDATED,
		FsoRootInfo: inf,
	}
}

func fromPbRootUpdated(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_ROOT_UPDATED {
		panic("invalid event")
	}
	ev := &EvRootUpdated{
		FsoRootInfo: *evpb.FsoRootInfo,
	}
	return ev, nil
}

// `EV_FSO_REPO_NAMING_UPDATED` aka `EvRepoNamingUpdated` sets the naming
// convention that is used to discover untracked repos below a root.
//
// Fields:
//
//  - `fso_repo_naming.global_root`, `GlobalRoot`:  The global path of the
//    root.
//  - `fso_repo_naming.rule`, `Rule`: The rule name.
//  - `fso_repo_naming.config`, `Config`: A config map that controls rule
//    details.  The structure of the config depends on the rule.
//
// See `ValidateRepoNaming()` for available rules and config map structures.
//
// The config can be patched with `EV_FSO_REPO_NAMING_CONFIG_UPDATED`.
type EvRepoNamingUpdated struct {
	pb.FsoRepoNaming
}

func (EvRepoNamingUpdated) RegistryEvent() {}

func NewRepoNamingUpdated(n *pb.FsoRepoNaming) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event:         pb.RegistryEvent_EV_FSO_REPO_NAMING_UPDATED,
		FsoRepoNaming: n,
	}
}

func fromPbRepoNamingUpdated(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REPO_NAMING_UPDATED {
		panic("invalid event")
	}
	naming := evpb.FsoRepoNaming
	if err := ValidateRepoNaming(naming); err != nil {
		return nil, err
	}
	ev := &EvRepoNamingUpdated{FsoRepoNaming: *naming}
	return ev, nil
}

func ValidateRepoNaming(naming *pb.FsoRepoNaming) error {
	if naming == nil {
		return ErrMalformedRepoNamingNil
	}

	switch naming.Rule {
	case "Stdtools2017":
		return validateRepoNamingStdtools2017(naming)
	case "SubdirLevel":
		return validateRepoNamingSubdirLevel(naming)
	case "PathPatterns":
		return validateRepoNamingPathPatterns(naming)
	default:
		return ErrMalformedRepoNamingRule
	}
}

// `EV_FSO_REPO_NAMING_CONFIG_UPDATED` aka `EvRepoNamingConfigUpdated` patches
// a naming configuration that has been previously set with
// `EV_FSO_REPO_NAMING_UPDATED`.
//
// The meaning of 'patch' depends on the current naming rule.  Naming rules
// should avoid `O(n^2)` event storage.  Example: An ingnore list would be
// incrementally built by repeatedly adding paths.  Each path is stored only
// once.  If the entire list was instead stored each time, the total amount of
// events would require `O(n^2)` storage.
//
// Fields:
//
//  - `fso_repo_naming.global_root`, `GlobalRoot`: The global path of the root
//    for which the naming config is updated.
//  - `fso_repo_naming.rule`, `Rule`: The rule name.  It must equal the rule
//    that has been previously set with `EV_FSO_REPO_NAMING_UPDATED`.
//  - `fso_repo_naming.config`, `ConfigPatch`: The config patch.
//
// See `ValidateRepoNamingPatch()` for valid rule-config combinations.
type EvRepoNamingConfigUpdated struct {
	GlobalRoot  string
	Rule        string
	ConfigPatch pb.ConfigMap
}

func (EvRepoNamingConfigUpdated) RegistryEvent() {}

func NewRepoNamingConfigUpdated(patch *pb.FsoRepoNaming) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event:         pb.RegistryEvent_EV_FSO_REPO_NAMING_CONFIG_UPDATED,
		FsoRepoNaming: patch,
	}
}

func fromPbRepoNamingConfigUpdated(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REPO_NAMING_CONFIG_UPDATED {
		panic("invalid event")
	}
	naming := evpb.FsoRepoNaming
	if err := ValidateRepoNamingPatch(naming); err != nil {
		return nil, err
	}
	ev := &EvRepoNamingConfigUpdated{
		GlobalRoot:  naming.GlobalRoot,
		Rule:        naming.Rule,
		ConfigPatch: *naming.Config,
	}
	return ev, nil
}

func ValidateRepoNamingPatch(naming *pb.FsoRepoNaming) error {
	if naming == nil {
		return ErrMalformedRepoNamingNil
	}

	if naming.GlobalRoot == "" {
		return ErrMalformedRepoNamingEmptyGlobalRoot
	}

	switch naming.Rule {
	case "Stdtools2017":
		return validateRepoNamingNonNilIgnoreListOnly(naming)
	case "SubdirLevel":
		return validateRepoNamingNonNilIgnoreListOnly(naming)
	case "PathPatterns":
		return validateRepoNamingPatchPathPatterns(naming)
	default:
		return ErrMalformedRepoNamingRule
	}
}

func validateRepoNamingStdtools2017(naming *pb.FsoRepoNaming) error {
	cfgPb := naming.Config
	if cfgPb == nil {
		return nil
	}
	return validateConfigSingleStringList(cfgPb, "ignore")
}

func validateRepoNamingSubdirLevel(naming *pb.FsoRepoNaming) error {
	cfgPb := naming.Config
	if cfgPb == nil {
		return ErrNamingConfigNil
	}

	cfg, err := configmap.ParsePb(cfgPb)
	if err != nil {
		return err
	}

	nExpected := 1
	if iface, ok := cfg["level"]; !ok {
		return ErrMissingLevel
	} else if val, ok := iface.(float64); !ok {
		return ErrLevelWrongType
	} else {
		level := int(val)
		if float64(level) != val {
			return ErrLevelNotInteger
		}
		if level < 1 || level > 4 {
			return ErrLevelOutOfRange
		}
	}

	if iface, ok := cfg["ignore"]; ok {
		val, ok := iface.([]string)
		if !ok {
			return ErrIgnoreHasWrongType
		}
		if len(val) == 0 {
			return ErrIgnoreListEmpty
		}
		nExpected += 1
	}

	if len(cfg) != nExpected {
		return ErrUnexpectedConfigField
	}

	return nil
}

func validateRepoNamingPathPatterns(naming *pb.FsoRepoNaming) error {
	cfgPb := naming.Config
	if cfgPb == nil {
		return ErrNamingConfigNil
	}

	cfg, err := configmap.ParsePb(cfgPb)
	if err != nil {
		return err
	}

	iface, ok := cfg["patterns"]
	if !ok {
		return ErrMissingPatterns
	}

	patterns, ok := iface.([]string)
	if !ok {
		return ErrPatternsWrongType
	}

	for _, pat := range patterns {
		toks := strings.SplitN(pat, " ", 2)
		if len(toks) != 2 {
			return &PatternInvalidError{Pattern: pat}
		}
		action := toks[0]
		glob := toks[1]

		switch action {
		case "enter":
		case "repo":
		case "superrepo":
		case "ignore":
		default:
			return &PatternInvalidActionError{Pattern: pat}
		}

		// Test glob with nonempty path to ensure that `Match()` does
		// inspecting the pattern.
		if _, err := slashpath.Match(glob, "/a/b/c"); err != nil {
			return &PatternInvalidGlobError{Pattern: pat}
		}
	}

	if len(cfg) != 1 {
		return ErrUnexpectedConfigField
	}

	return nil
}

func validateRepoNamingNonNilIgnoreListOnly(naming *pb.FsoRepoNaming) error {
	cfgPb := naming.Config
	if cfgPb == nil {
		return ErrNamingConfigNil
	}
	return validateConfigSingleStringList(cfgPb, "ignore")
}

func validateRepoNamingPatchPathPatterns(naming *pb.FsoRepoNaming) error {
	cfgPb := naming.Config
	if cfgPb == nil {
		return ErrNamingConfigNil
	}
	ps, err := validateConfigSingleStringListGet(cfgPb, "enabledPaths")
	if err != nil {
		return err
	}

	for _, p := range ps {
		toks := strings.SplitN(p, " ", 2)
		if len(toks) != 2 {
			return &DepthPathInvalidError{
				Path:   p,
				Reason: "malformed",
			}
		}

		depth, err := strconv.ParseUint(toks[0], 10, 64)
		if err != nil {
			return &DepthPathInvalidError{
				Path:   p,
				Reason: "invalid depth",
				Err:    err,
			}
		}

		if depth > 2 {
			return &DepthPathInvalidError{
				Path:   p,
				Reason: "depth to great",
			}
		}
	}

	return nil
}

func validateConfigSingleStringList(cfgPb *pb.ConfigMap, key string) error {
	_, err := validateConfigSingleStringListGet(cfgPb, key)
	return err
}

func validateConfigSingleStringListGet(
	cfgPb *pb.ConfigMap, key string,
) ([]string, error) {
	cfg, err := configmap.ParsePb(cfgPb)
	if err != nil {
		return nil, err
	}

	valIface, ok := cfg[key]
	if !ok {
		return nil, &ConfigMapFieldError{
			Field:  key,
			Reason: "missing field",
		}
	}
	val, ok := valIface.([]string)
	if !ok {
		return nil, &ConfigMapFieldError{
			Field:  key,
			Reason: "value has wrong type",
		}
	}
	if len(val) == 0 {
		return nil, &ConfigMapFieldError{
			Field:  key,
			Reason: "empty list",
		}
	}

	if len(cfg) != 1 {
		return nil, ErrUnexpectedConfigField
	}

	return val, nil
}

// `EV_FSO_REPO_INIT_POLICY_UPDATED` aka `EvRepoInitPolicyUpdated` sets the
// policy that is used to determine repo initialization options.
//
// Fields:
//
//  - `fso_repo_init_policy.global_root`, `GlobalRoot`:  The global path of the
//    root.
//  - `fso_repo_init_policy.policy`, `Policy`: The policy type.
//
// If policy type `IPOL_SUBDIR_TRACKING_GLOBLIST`:
//
//  - `fso_repo_init_policy.subdir_tracking_globlist`,
//    `SubdirTrackingGloblist`: lists pairs of glob patterns and
//    `SubdirTracking`.  The first matching glob decides the `SubdirTracking`.
//
// See `ValidateRepoInitPolicy()` for details.
type EvRepoInitPolicyUpdated struct {
	pb.FsoRepoInitPolicy
}

func (EvRepoInitPolicyUpdated) RegistryEvent() {}

func NewRepoInitPolicyUpdated(p *pb.FsoRepoInitPolicy) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event:             pb.RegistryEvent_EV_FSO_REPO_INIT_POLICY_UPDATED,
		FsoRepoInitPolicy: p,
	}
}

func fromPbRepoInitPolicyUpdated(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REPO_INIT_POLICY_UPDATED {
		panic("invalid event")
	}
	policy := evpb.FsoRepoInitPolicy
	if err := ValidateRepoInitPolicy(policy); err != nil {
		return nil, err
	}
	ev := &EvRepoInitPolicyUpdated{FsoRepoInitPolicy: *policy}
	return ev, nil
}

func ValidateRepoInitPolicy(policy *pb.FsoRepoInitPolicy) error {
	if policy == nil {
		return ErrPolicyNil
	}

	switch policy.Policy {
	case pb.FsoRepoInitPolicy_IPOL_SUBDIR_TRACKING_GLOBLIST:
		return validateRepoInitPolicySubdirTrackingGloblist(policy)
	default:
		return ErrUnknownRepoNamingPolicy
	}
}

func validateRepoInitPolicySubdirTrackingGloblist(
	policy *pb.FsoRepoInitPolicy,
) error {
	globs := policy.SubdirTrackingGloblist
	if len(globs) < 1 {
		return ErrMissingGloblist
	}

	for _, g := range globs {
		pat := g.Pattern
		// Test glob with nonempty path to ensure that `Match()` does
		// inspecting the pattern.
		if _, err := slashpath.Match(pat, "/a/b/c"); err != nil {
			return &PatternInvalidGlobError{
				Pattern: pat,
			}
		}
	}

	return nil
}

// `RegistryEvent_EV_FSO_REPO_ACCEPTED` aka `EvRepoAccepted`.
type EvRepoAccepted struct {
	pb.FsoRepoInfo
}

func (EvRepoAccepted) RegistryEvent() {}

func NewRepoAccepted(inf *pb.FsoRepoInfo) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event:       pb.RegistryEvent_EV_FSO_REPO_ACCEPTED,
		FsoRepoInfo: inf,
	}
}

func fromPbRepoAccepted(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REPO_ACCEPTED {
		panic("invalid event")
	}
	ev := &EvRepoAccepted{
		FsoRepoInfo: *evpb.FsoRepoInfo,
	}
	return ev, nil
}

// `RegistryEvent_EV_FSO_REPO_ADDED` aka `EvRepoAdded`.
type EvRepoAdded struct {
	pb.FsoRepoInfo
}

func (EvRepoAdded) RegistryEvent() {}

func NewRepoAdded(
	repoId []byte, globalPath string, repoEventId ulid.I,
) pb.RegistryEvent {
	ev := pb.RegistryEvent{
		Event: pb.RegistryEvent_EV_FSO_REPO_ADDED,
		FsoRepoInfo: &pb.FsoRepoInfo{
			Id:         repoId,
			GlobalPath: globalPath,
		},
	}
	if repoEventId != ulid.Nil {
		ev.RepoEventId = repoEventId[:]
	}
	return ev
}

func fromPbRepoAdded(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REPO_ADDED {
		panic("invalid event")
	}
	ev := &EvRepoAdded{
		FsoRepoInfo: *evpb.FsoRepoInfo,
	}
	return ev, nil
}

// `RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED` aka `EvRepoReinitAccepted`.
type EvRepoReinitAccepted struct {
	RepoId     []byte
	GlobalPath string
	Reason     string
}

func (EvRepoReinitAccepted) RegistryEvent() {}

func NewRepoReinitAccepted(
	repoId []byte, globalPath, reason string,
) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event: pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED,
		FsoRepoInfo: &pb.FsoRepoInfo{
			Id:         repoId,
			GlobalPath: globalPath,
		},
		FsoRepoReinitReason: reason,
	}
}

func fromPbRepoReinitAccepted(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED {
		panic("invalid event")
	}
	ev := &EvRepoReinitAccepted{
		RepoId:     evpb.FsoRepoInfo.Id,
		GlobalPath: evpb.FsoRepoInfo.GlobalPath,
		Reason:     evpb.FsoRepoReinitReason,
	}
	return ev, nil
}

// `EV_FSO_SHADOW_REPO_MOVE_STARTED` aka `EvShadowRepoMoveStarted` refers to a
// corresponding repo event.
type EvShadowRepoMoveStarted struct {
	RepoId      uuid.I
	RepoEventId ulid.I
	WorkflowId  uuid.I
}

func (EvShadowRepoMoveStarted) RegistryEvent() {}

func NewShadowRepoMoveStarted(
	repoId uuid.I, repoEventId ulid.I, workflowId uuid.I,
) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event:       pb.RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED,
		RepoId:      repoId[:],
		RepoEventId: repoEventId[:],
		WorkflowId:  workflowId[:],
	}
}

func fromPbShadowRepoMoveStarted(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED {
		panic("invalid event")
	}
	repoId, err := uuid.FromBytes(evpb.RepoId)
	if err != nil {
		return nil, err
	}
	repoEventId, err := ulid.ParseBytes(evpb.RepoEventId)
	if err != nil {
		return nil, err
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, err
	}
	ev := &EvShadowRepoMoveStarted{
		RepoId:      repoId,
		RepoEventId: repoEventId,
		WorkflowId:  workflowId,
	}
	return ev, nil
}

// `RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED` aka `EvRepoMoveAccepted` starts a
// move-repo workflow, which changes the real repo and the shadow repo
// location.  See package `moverepowf` for details.
type EvRepoMoveAccepted struct {
	RepoId        uuid.I
	WorkflowId    uuid.I
	NewGlobalPath string
}

func (EvRepoMoveAccepted) RegistryEvent() {}

func NewPbRepoMoveAccepted(ev *EvRepoMoveAccepted) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event:      pb.RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED,
		WorkflowId: ev.WorkflowId[:],
		FsoRepoInfo: &pb.FsoRepoInfo{
			Id:         ev.RepoId[:],
			GlobalPath: ev.NewGlobalPath,
		},
	}
}

func fromPbRepoMoveAccepted(evpb pb.RegistryEvent) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED {
		panic("invalid event")
	}
	repoId, err := uuid.FromBytes(evpb.FsoRepoInfo.Id)
	if err != nil {
		return nil, &ParseError{What: "repo ID", Err: err}
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvRepoMoveAccepted{
		RepoId:        repoId,
		WorkflowId:    workflowId,
		NewGlobalPath: evpb.FsoRepoInfo.GlobalPath,
	}, nil
}

// `RegistryEvent_EV_FSO_REPO_MOVED` aka `EvRepoMoved` is part of the move-repo
// workflow.  See package `moverepowf` for details.
type EvRepoMoved struct {
	RepoId      uuid.I
	RepoEventId ulid.I
	WorkflowId  uuid.I
	GlobalPath  string
}

func (EvRepoMoved) RegistryEvent() {}

func NewPbRepoMoved(ev *EvRepoMoved) pb.RegistryEvent {
	return pb.RegistryEvent{
		Event:       pb.RegistryEvent_EV_FSO_REPO_MOVED,
		WorkflowId:  ev.WorkflowId[:],
		RepoEventId: ev.RepoEventId[:],
		FsoRepoInfo: &pb.FsoRepoInfo{
			Id:         ev.RepoId[:],
			GlobalPath: ev.GlobalPath,
		},
	}
}

func fromPbRepoMoved(evpb pb.RegistryEvent) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REPO_MOVED {
		panic("invalid event")
	}
	repoId, err := uuid.FromBytes(evpb.FsoRepoInfo.Id)
	if err != nil {
		return nil, &ParseError{What: "repo ID", Err: err}
	}
	repoEventId, err := ulid.ParseBytes(evpb.RepoEventId)
	if err != nil {
		return nil, &ParseError{What: "repo event ID", Err: err}
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvRepoMoved{
		RepoId:      repoId,
		RepoEventId: repoEventId,
		WorkflowId:  workflowId,
		GlobalPath:  evpb.FsoRepoInfo.GlobalPath,
	}, nil
}

// `RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED` aka
// `EvRepoEnableGitlabAccepted`.
type EvRepoEnableGitlabAccepted struct {
	RepoId          uuid.I
	GitlabNamespace string
}

func (EvRepoEnableGitlabAccepted) RegistryEvent() {}

func NewRepoEnableGitlabAccepted(
	repoId uuid.I, gitlabNamespace string,
) pb.RegistryEvent {
	ty := pb.RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED
	return pb.RegistryEvent{
		Event:           ty,
		RepoId:          repoId[:],
		GitlabNamespace: gitlabNamespace,
	}
}

func fromPbRepoEnableGitlabAccepted(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED {
		panic("invalid event")
	}
	repoId, err := uuid.FromBytes(evpb.RepoId)
	if err != nil {
		return nil, err
	}
	ev := &EvRepoEnableGitlabAccepted{
		RepoId:          repoId,
		GitlabNamespace: evpb.GitlabNamespace,
	}
	return ev, nil
}

// `RegistryEvent_EV_FSO_SPLIT_ROOT_ENABLED` aka `EvSplitRootEnabled`.
type EvSplitRootEnabled struct {
	GlobalRoot string
}

func (EvSplitRootEnabled) RegistryEvent() {}

func NewSplitRootEnabled(
	root string,
) pb.RegistryEvent {
	ty := pb.RegistryEvent_EV_FSO_SPLIT_ROOT_ENABLED
	return pb.RegistryEvent{
		Event: ty,
		FsoSplitRootParams: &pb.FsoSplitRootParams{
			GlobalRoot: root,
		},
	}
}

func fromPbSplitRootEnabled(evpb pb.RegistryEvent) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_SPLIT_ROOT_ENABLED {
		return nil, ErrInvalidEvent
	}

	params := evpb.FsoSplitRootParams
	if params == nil {
		return nil, ErrInvalidEvent
	}

	root := params.GlobalRoot
	if root == "" {
		return nil, ErrInvalidEvent
	}

	return &EvSplitRootEnabled{
		GlobalRoot: root,
	}, nil
}

// `RegistryEvent_EV_FSO_SPLIT_ROOT_PARAMS_UPDATED` aka
// `EvSplitRootParamsUpdated`.
type EvSplitRootParamsUpdated struct {
	GlobalRoot   string
	MaxDepth     int32
	MinDiskUsage int64
	MaxDiskUsage int64
}

func (EvSplitRootParamsUpdated) RegistryEvent() {}

func NewSplitRootParamsUpdated(
	root string,
	cfg *pb.FsoSplitRootParams,
) pb.RegistryEvent {
	ty := pb.RegistryEvent_EV_FSO_SPLIT_ROOT_PARAMS_UPDATED
	return pb.RegistryEvent{
		Event: ty,
		FsoSplitRootParams: &pb.FsoSplitRootParams{
			GlobalRoot:   root,
			MaxDepth:     cfg.MaxDepth,
			MinDiskUsage: cfg.MinDiskUsage,
			MaxDiskUsage: cfg.MaxDiskUsage,
		},
	}
}

func fromPbSplitRootParamsUpdated(evpb pb.RegistryEvent) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_SPLIT_ROOT_PARAMS_UPDATED {
		return nil, ErrInvalidEvent
	}

	params := evpb.FsoSplitRootParams
	if params == nil {
		return nil, ErrInvalidEvent
	}
	if params.GlobalRoot == "" {
		return nil, ErrInvalidEvent
	}
	if params.MaxDepth <= 0 {
		return nil, ErrInvalidEvent
	}
	if params.MinDiskUsage <= 0 {
		return nil, ErrInvalidEvent
	}
	if params.MaxDiskUsage <= 0 {
		return nil, ErrInvalidEvent
	}

	return &EvSplitRootParamsUpdated{
		GlobalRoot:   params.GlobalRoot,
		MaxDepth:     params.MaxDepth,
		MinDiskUsage: params.MinDiskUsage,
		MaxDiskUsage: params.MaxDiskUsage,
	}, nil
}

// `RegistryEvent_EV_FSO_SPLIT_ROOT_DISABLED` aka `EvSplitRootDisabled`.
type EvSplitRootDisabled struct {
	GlobalRoot string
}

func (EvSplitRootDisabled) RegistryEvent() {}

func NewSplitRootDisabled(
	root string,
) pb.RegistryEvent {
	ty := pb.RegistryEvent_EV_FSO_SPLIT_ROOT_DISABLED
	return pb.RegistryEvent{
		Event: ty,
		FsoSplitRootParams: &pb.FsoSplitRootParams{
			GlobalRoot: root,
		},
	}
}

func fromPbSplitRootDisabled(evpb pb.RegistryEvent) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_SPLIT_ROOT_DISABLED {
		return nil, ErrInvalidEvent
	}

	params := evpb.FsoSplitRootParams
	if params == nil {
		return nil, ErrInvalidEvent
	}

	root := params.GlobalRoot
	if root == "" {
		return nil, ErrInvalidEvent
	}

	return &EvSplitRootDisabled{
		GlobalRoot: root,
	}, nil
}

// `RegistryEvent_EV_FSO_PATH_FLAG_SET` aka `EvPathFlagSet`.
type EvPathFlagSet struct {
	Path  string
	Flags uint32
}

func (EvPathFlagSet) RegistryEvent() {}

func NewPathFlagSet(
	path string,
	flags uint32,
) pb.RegistryEvent {
	ty := pb.RegistryEvent_EV_FSO_PATH_FLAG_SET
	return pb.RegistryEvent{
		Event: ty,
		FsoPathFlag: &pb.FsoPathFlag{
			Path:  path,
			Flags: flags,
		},
	}
}

func fromPbPathFlagSet(evpb pb.RegistryEvent) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_PATH_FLAG_SET {
		return nil, ErrInvalidEvent
	}

	pf := evpb.FsoPathFlag
	if pf == nil {
		return nil, ErrInvalidEvent
	}

	return &EvPathFlagSet{
		Path:  pf.Path,
		Flags: pf.Flags,
	}, nil
}

// `RegistryEvent_EV_FSO_PATH_FLAG_UNSET` aka `EvPathFlagUnset`.
type EvPathFlagUnset struct {
	Path  string
	Flags uint32
}

func (EvPathFlagUnset) RegistryEvent() {}

func NewPathFlagUnset(
	path string,
	flags uint32,
) pb.RegistryEvent {
	ty := pb.RegistryEvent_EV_FSO_PATH_FLAG_UNSET
	return pb.RegistryEvent{
		Event: ty,
		FsoPathFlag: &pb.FsoPathFlag{
			Path:  path,
			Flags: flags,
		},
	}
}

func fromPbPathFlagUnset(evpb pb.RegistryEvent) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_PATH_FLAG_UNSET {
		return nil, ErrInvalidEvent
	}

	pf := evpb.FsoPathFlag
	if pf == nil {
		return nil, ErrInvalidEvent
	}

	return &EvPathFlagUnset{
		Path:  pf.Path,
		Flags: pf.Flags,
	}, nil
}

// `EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED` aka
// `EvRootArchiveRecipientsUpdated`.
type EvRootArchiveRecipientsUpdated struct {
	GlobalRoot string
	Keys       [][]byte
}

func (EvRootArchiveRecipientsUpdated) RegistryEvent() {}

func NewRootArchiveRecipientsUpdated(
	root string, keys [][]byte,
) pb.RegistryEvent {
	if err := checkGPGKeyFingerprints(keys); err != nil {
		panic(err)
	}
	ty := pb.RegistryEvent_EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED
	return pb.RegistryEvent{
		Event: ty,
		FsoRootInfo: &pb.FsoRootInfo{
			GlobalRoot: root,
		},
		FsoGpgKeyFingerprints: keys,
	}
}

func fromPbRootArchiveRecipientsUpdated(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	inf := evpb.FsoRootInfo
	if inf == nil {
		return nil, ErrMissingRootInfo
	}
	if inf.GlobalRoot == "" {
		return nil, ErrMalformedRootInfo
	}

	keys := evpb.FsoGpgKeyFingerprints
	if err := checkGPGKeyFingerprints(keys); err != nil {
		return nil, err
	}

	ev := &EvRootArchiveRecipientsUpdated{
		GlobalRoot: inf.GlobalRoot,
		Keys:       keys,
	}
	return ev, nil
}

// `EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED` aka
// `EvRootShadowBackupRecipientsUpdated`.
type EvRootShadowBackupRecipientsUpdated struct {
	GlobalRoot string
	Keys       [][]byte
}

func (EvRootShadowBackupRecipientsUpdated) RegistryEvent() {}

func NewRootShadowBackupRecipientsUpdated(
	root string, keys [][]byte,
) pb.RegistryEvent {
	if err := checkGPGKeyFingerprints(keys); err != nil {
		panic(err)
	}
	ty := pb.RegistryEvent_EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED
	return pb.RegistryEvent{
		Event: ty,
		FsoRootInfo: &pb.FsoRootInfo{
			GlobalRoot: root,
		},
		FsoGpgKeyFingerprints: keys,
	}
}

func fromPbRootShadowBackupRecipientsUpdated(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	inf := evpb.FsoRootInfo
	if inf == nil {
		return nil, ErrMissingRootInfo
	}
	if inf.GlobalRoot == "" {
		return nil, ErrMalformedRootInfo
	}

	keys := evpb.FsoGpgKeyFingerprints
	if err := checkGPGKeyFingerprints(keys); err != nil {
		return nil, err
	}

	ev := &EvRootShadowBackupRecipientsUpdated{
		GlobalRoot: inf.GlobalRoot,
		Keys:       keys,
	}
	return ev, nil
}

func checkGPGKeyFingerprints(keys [][]byte) error {
	for _, k := range keys {
		if len(k) != 20 {
			return ErrMalformedGPGFingerprint
		}
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if bytes.Equal(keys[i], keys[j]) {
				return ErrDuplicateGPGFingerprint
			}
		}
	}
	return nil
}
