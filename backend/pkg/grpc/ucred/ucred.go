// Package `ucred` provides `SO_PEERCRED` auth for gRPC over a Unix domain
// socket.
//
// Package `github.com/nogproject/nog/backend/pkg/grpc/ucred` has been
// duplicated from `github.com/nogproject/bcpfs/pkg/grpc/ucred` and modified:
//
//  - It uses the `nog/backend` logger convention `Logger.Warnw()`.
//  - It supports `ClientHandshake()`.
//
// Both packages should perhaps be refactored to a common library package.
package ucred

import (
	"context"
	"syscall"

	"google.golang.org/grpc/peer"
)

// `AuthInfo.Ucred` is a field, so that further information, like TLS, could be
// added to `AuthInfo`.
type AuthInfo struct {
	Ucred syscall.Ucred
}

// `AuthType()` makes it an `grpc/credentials.AuthInfo` interface.
func (AuthInfo) AuthType() string {
	return "ucred"
}

// `FromContext()` returns the `ucred.AuthInfo` if it exists in `ctx`.  It uses
// `grpc/peer.FromContext()`.
func FromContext(ctx context.Context) (*AuthInfo, bool) {
	pr, ok := peer.FromContext(ctx)
	if !ok {
		return nil, false
	}
	info, ok := pr.AuthInfo.(AuthInfo)
	if !ok {
		return nil, false
	}
	return &info, true
}
