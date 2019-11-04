package unixdomainsd

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrShutdown = status.Errorf(
	codes.Unavailable, "shutdown",
)
var ErrMalformedRequest = status.Error(
	codes.InvalidArgument, "malformed request",
)
var ErrMissingVid = status.Error(
	codes.InvalidArgument, "missing vid",
)
var ErrMalformedUnixDomainName = status.Error(
	codes.InvalidArgument, "malformed Unix domain name",
)
var ErrDomainNameInUse = status.Error(
	codes.FailedPrecondition, "domain name already in use",
)
var ErrVersionConflict = status.Error(
	codes.FailedPrecondition, "version conflict",
)
var ErrDomainNotFound = status.Error(
	codes.NotFound, "Unix domain not found",
)
var ErrUserNotFound = status.Error(
	codes.NotFound, "Unix user not found",
)
var ErrNotYetImplemented = status.Error(
	codes.Unimplemented, "not yet implemented",
)
var ErrGid0Forbidden = status.Error(
	codes.InvalidArgument, "Unix GID 0 forbidden",
)
var ErrUid0Forbidden = status.Error(
	codes.InvalidArgument, "Unix UID 0 forbidden",
)

func asMainGrpcError(err error) error {
	if err == nil {
		return nil
	}
	return status.Errorf(codes.Unknown, "main error: %v", err)
}

func asUnixDomainsError(err error) error {
	if err == nil {
		return nil
	}
	return status.Errorf(codes.Unknown, "Unix domains error: %v", err)
}
