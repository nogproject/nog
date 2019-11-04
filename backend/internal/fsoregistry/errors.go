package fsoregistry

import (
	"errors"
	"fmt"
)

var ErrNotInitialized = errors.New("repo is not initialized")
var ErrConflictInit = errors.New("init conflict")
var ErrCannotMoveUnconfirmed = errors.New("cannot move unconfirmed repo")
var ErrConflictRepoInit = errors.New("repo already initialized at path")
var ErrConflictWorkflow = errors.New("workflow conflict")
var ErrMalformedEphemeralWorkflowsId = errors.New("malformed ephemeral workflows ID")
var ErrConflictEphemeralWorkflowsId = errors.New("ephemeral workflows ID conflict")
var ErrMalformedWorkflowId = errors.New("malformed workflow ID")
var ErrMismatchGlobalPath = errors.New("global path mismatch")
var ErrPathChanged = errors.New("changed path")
var ErrPathUnchanged = errors.New("unchanged path")
var ErrRepeatedPost = errors.New("repeated post")
var ErrUninitialized = errors.New("uninitialized registry")
var ErrUnknownRepo = errors.New("unknown repo")
var ErrUnknownRoot = errors.New("unknown root")
var ErrWorkflowReuse = errors.New("workflow ID must not be reused")
var ErrWorkflowTerminated = errors.New("workflow has terminated")
var ErrInvalidSplitRootConfig = errors.New("invalid split root config")
var ErrSplitRootConfigExists = errors.New("split root config already exists")
var ErrNoSplitRootConfig = errors.New("no split root config")
var ErrMalformedPath = errors.New("malformed path")
var ErrNoGPGKeys = errors.New("no GPG keys")
var ErrDuplicateGPGKeys = errors.New("duplicate GPG keys")
var ErrCannotFreezeRepo = errors.New("cannot freeze repo")
var ErrCannotUnfreezeRepo = errors.New("cannot unfreeze repo")
var ErrParentRepoImmutable = errors.New("parent repo immutable")
var ErrCannotArchiveRepo = errors.New("cannot archive repo")
var ErrCannotUnarchiveRepo = errors.New("cannot unarchive repo")
var ErrNamingRuleMismatch = errors.New("naming rule mismatch")
var ErrPathNotStrictlyBelow = errors.New("path not strictly below root")
var ErrPathDepthOutOfRange = errors.New("path depth out of range")
var ErrRootWithoutInitPolicy = errors.New("root has no repo init policy")
var ErrRootWithoutRepoNaming = errors.New("root has no repo naming")
var ErrCreatorNameMissing = errors.New("missing creator name")
var ErrCreatorEmailMissing = errors.New("missing creator email")
var ErrCommandUnknown = errors.New("unknown command")
var ErrGitlabNamespaceMissingSlash = errors.New("Gitlab namespace missing slash")
var ErrReasonEmpty = errors.New("empty reason")
var ErrReasonAlreadyApplied = errors.New("reason already applied")
var ErrConflictGitlabInit = errors.New("GitLab already initialized with conflicting namespace")

type InitRepoDenyError struct {
	Reason string
}

func (err *InitRepoDenyError) Error() string {
	return fmt.Sprintf("init repo denied: %s", err.Reason)
}

type EventDetailsError struct {
	Err error
}

func (err *EventDetailsError) Error() string {
	return fmt.Sprintf("invalid event details: %s", err.Err)
}

type WorkflowIdError struct {
	Reason string
}

func (err *WorkflowIdError) Error() string {
	return err.Reason
}

type EnablePathRuleError struct {
	Rule string
}

func (err *EnablePathRuleError) Error() string {
	return fmt.Sprintf(
		"naming rule `%s` does not support enabling paths",
		err.Rule,
	)
}

type InternalError struct {
	What string
	Err  error
}

func (err *InternalError) Error() string {
	return fmt.Sprintf("internal error: %s: %s", err.What, err.Err)
}
