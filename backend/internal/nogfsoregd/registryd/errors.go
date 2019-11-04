package registryd

import (
	"github.com/nogproject/nog/backend/internal/fsoregistry"
	"github.com/nogproject/nog/backend/internal/workflows/archiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/freezerepowf"
	"github.com/nogproject/nog/backend/internal/workflows/pingregistrywf"
	"github.com/nogproject/nog/backend/internal/workflows/splitrootwf"
	"github.com/nogproject/nog/backend/internal/workflows/unarchiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/unfreezerepowf"
	"github.com/nogproject/nog/backend/pkg/errorsx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrShutdown = status.Error(
	codes.Unavailable, "shutdown",
)
var ErrVersionConflict = status.Error(
	codes.FailedPrecondition, "version conflict",
)
var ErrMalformedVid = status.Error(
	codes.InvalidArgument, "malformed VID",
)
var ErrMalformedWorkflowId = status.Error(
	codes.InvalidArgument, "malformed workflow ID",
)
var ErrMalformedJobControl = status.Error(
	codes.InvalidArgument, "malformed job control",
)
var ErrMalformedAfter = status.Error(
	codes.InvalidArgument, "malformed after",
)
var ErrMalformedPath = status.Error(
	codes.InvalidArgument, "malformed path",
)
var ErrMissingConfig = status.Error(
	codes.InvalidArgument, "missing config",
)
var ErrUnknownRoot = status.Errorf(
	codes.NotFound, "%s", fsoregistry.ErrUnknownRoot,
)
var ErrPartialResult = status.Error(
	codes.Unknown, "partial result",
)
var ErrUnknownWorkflow = status.Error(
	codes.NotFound, "unknown workflow",
)
var ErrForeignRegistryWorkflow = status.Error(
	codes.PermissionDenied, "registry does not own workflow",
)
var ErrDenyUnknownWorkflowType = status.Error(
	codes.PermissionDenied, "deny access to unknown workflow type",
)
var ErrParsePb = status.Error(
	codes.Internal, "failed to parse protobuf",
)
var ErrInconsistentRegistryState = status.Error(
	codes.Internal, "inconsistent registry state",
)
var ErrUnknownWorkflowState = status.Error(
	codes.Internal, "unknown workflow state",
)
var ErrSplitRootDisabled = status.Error(
	codes.FailedPrecondition, "split root disabled",
)
var ErrPathOutsideRoot = status.Error(
	codes.FailedPrecondition, "path outside of root",
)
var ErrPathNotCandidate = status.Error(
	codes.FailedPrecondition, "path is not a candidate",
)
var ErrSplitRootDecisionUnimplemented = status.Error(
	codes.Unimplemented, "unimplemented split-root decision",
)
var ErrDefaultDeny = status.Error(
	codes.PermissionDenied, "default deny",
)

func asMainGrpcError(err error) error {
	if err == nil {
		return nil
	}
	return status.Errorf(codes.Unknown, "main error: %v", err)
}

func asRegistryGrpcError(err error) error {
	if err == nil {
		return nil
	}
	return status.Errorf(codes.Unknown, "registry error: %v", err)
}

func asWorkflowIndexGrpcError(err error) error {
	if err == nil {
		return nil
	}
	return status.Errorf(codes.Unknown, "workflow index error: %v", err)
}

func asWorkflowGrpcError(err error) error {
	if err == nil {
		return nil
	}
	return status.Errorf(codes.Unknown, "workflow error: %v", err)
}

func asPingRegistryWorkflowGrpcError(err error) error {
	if err == nil {
		return nil
	}
	msg := "ping-registry workflow: " + err.Error()
	return status.Error(codePingRegistry(err), msg)
}

func codePingRegistry(err error) codes.Code {
	isFailedPrecondition := func(err error) bool {
		switch err.(type) {
		case *pingregistrywf.StateConflictError:
			return true
		default:
			return false
		}
	}

	switch {
	case errorsx.IsPred(err, isFailedPrecondition):
		return codes.FailedPrecondition
	default:
		return codes.Unknown
	}
}

func asSplitRootWorkflowGrpcError(err error) error {
	if err == nil {
		return nil
	}
	msg := "split-root workflow: " + err.Error()
	return status.Error(codeSplitRoot(err), msg)
}

func codeSplitRoot(err error) codes.Code {
	isFailedPrecondition := func(err error) bool {
		switch err.(type) {
		case *splitrootwf.StateConflictError:
			return true
		default:
			return false
		}
	}

	isResourceExhausted := func(err error) bool {
		switch err.(type) {
		case *splitrootwf.ResourceExhaustedError:
			return true
		default:
			return false
		}
	}

	switch {
	case errorsx.IsPred(err, isFailedPrecondition):
		return codes.FailedPrecondition
	case errorsx.IsPred(err, isResourceExhausted):
		return codes.ResourceExhausted
	default:
		return codes.Unknown
	}
}

func asFreezeRepoWorkflowGrpcError(err error) error {
	if err == nil {
		return nil
	}
	msg := "freeze-repo workflow: " + err.Error()
	return status.Error(codeFreezeRepo(err), msg)
}

func codeFreezeRepo(err error) codes.Code {
	isFailedPrecondition := func(err error) bool {
		switch err.(type) {
		case *freezerepowf.StateConflictError:
			return true
		default:
			return false
		}
	}

	switch {
	case errorsx.IsPred(err, isFailedPrecondition):
		return codes.FailedPrecondition
	default:
		return codes.Unknown
	}
}

func asUnfreezeRepoWorkflowGrpcError(err error) error {
	if err == nil {
		return nil
	}
	msg := "unfreeze-repo workflow: " + err.Error()
	return status.Error(codeUnfreezeRepo(err), msg)
}

func codeUnfreezeRepo(err error) codes.Code {
	isFailedPrecondition := func(err error) bool {
		switch err.(type) {
		case *unfreezerepowf.StateConflictError:
			return true
		default:
			return false
		}
	}

	switch {
	case errorsx.IsPred(err, isFailedPrecondition):
		return codes.FailedPrecondition
	default:
		return codes.Unknown
	}
}

func asArchiveRepoWorkflowGrpcError(err error) error {
	if err == nil {
		return nil
	}
	msg := "archive-repo workflow: " + err.Error()
	return status.Error(codeArchiveRepo(err), msg)
}

func codeArchiveRepo(err error) codes.Code {
	isFailedPrecondition := func(err error) bool {
		switch err.(type) {
		case *archiverepowf.StateConflictError:
			return true
		default:
			return false
		}
	}

	switch {
	case errorsx.IsPred(err, isFailedPrecondition):
		return codes.FailedPrecondition
	default:
		return codes.Unknown
	}
}

func asUnarchiveRepoWorkflowGrpcError(err error) error {
	if err == nil {
		return nil
	}
	msg := "unarchive-repo workflow: " + err.Error()
	return status.Error(codeUnarchiveRepo(err), msg)
}

func codeUnarchiveRepo(err error) codes.Code {
	isFailedPrecondition := func(err error) bool {
		switch err.(type) {
		case *unarchiverepowf.StateConflictError:
			return true
		default:
			return false
		}
	}

	switch {
	case errorsx.IsPred(err, isFailedPrecondition):
		return codes.FailedPrecondition
	default:
		return codes.Unknown
	}
}
