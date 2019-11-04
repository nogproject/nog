package fsorepos

import (
	"errors"
	"fmt"
)

var ErrCommandUnknown = errors.New("unknown command")
var ErrCommandConflict = errors.New("command conflict")
var ErrConflictShadowPath = errors.New("shadow path conflict")
var ErrConflictWorkflow = errors.New("workflow conflict")
var ErrNoGPGKeys = errors.New("no GPG keys")
var ErrDuplicateGPGKeys = errors.New("duplicate GPG keys")
var ErrInitConflict = errors.New("init conflict")
var ErrMalformedShadowBackupURL = errors.New("malformed shadow backup URL")
var ErrMalformedTarttURL = errors.New("malformed tartt URL")
var ErrMalformedWorkflowId = errors.New("malformed workflow ID")
var ErrMissingShadow = errors.New("missing shadow repo")
var ErrNotInitialized = errors.New("repo is not initialized")
var ErrNotInitializedShadowBackup = errors.New("shadow backup is not initialized")
var ErrShadowPathUnchanged = errors.New("unchanged shadow path")
var ErrUninitialized = errors.New("uninitialized repo")
var ErrWorkflowActive = errors.New("the workflow is already active")
var ErrWorkflowReuse = errors.New("workflow ID must not be reused")
var ErrInvalidErrorStatusCode = errors.New("invalid error status code")
var ErrInvalidErrorStatusMessage = errors.New("invalid error status message")
var ErrStatusMessageTooLong = errors.New("status message too long")
var ErrConflictRepoError = errors.New("cannot proceed due to repo error")
var ErrConflictStorageWorkflow = errors.New("storage workflow conflict")
var ErrEmptyTarPath = errors.New("empty tar path")
var ErrNoTarttRepo = errors.New("no tartt repo")
var ErrGitlabPathConflict = errors.New("conflicting GitLab path")
var ErrGitlabConfigInvalid = fmt.Errorf("invalid Gitlab config")
var ErrSubdirTrackingInvalid = errors.New("invalid SubdirTracking")
var ErrGitlabNamespaceInvalid = errors.New("invalid Gitlab namespace")
var ErrClearMessageEmpty = errors.New("empty clear message")
var ErrClearMessageMismatch = errors.New("clear message mismatch")

type EventDetailsError struct {
	Err error
}

func (err *EventDetailsError) Error() string {
	return fmt.Sprintf("invalid event details: %s", err.Err)
}
