package ucred

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// `ErrDisabled` is used by `NoneAuthorizer` to indicate that a service cannot
// be used.
var ErrDisabled = status.Error(codes.PermissionDenied, "service disabled")

// `ErrUcredMissing` indicates that a context has no ucred.
var ErrMissingUcred = status.Error(codes.Unauthenticated, "missing ucred")

// `AnyAuthorizer.Authorize()` authorizes every context, even if the context
// has no ucred.
type AnyAuthorizer struct{}

// `AnyAuthorizer.Authorize()` always returns `nil`.
func (a *AnyAuthorizer) Authorize(ctx context.Context) error {
	return nil
}

// `AnyAuthorizer.AuthorizeInfo()` always returns `nil`.
func (a *AnyAuthorizer) AuthorizeInfo(*AuthInfo) error {
	return nil
}

// `NoneAuthorizer.Authorize()` rejects every context.
type NoneAuthorizer struct{}

// `NoneAuthorizer.Authorize()` always returns `ErrDisabled`.
func (a *NoneAuthorizer) Authorize(ctx context.Context) error {
	return ErrDisabled
}

// `NoneAuthorizer.AuthorizeInfo()` always returns `ErrDisabled`.
func (a *NoneAuthorizer) AuthorizeInfo(*AuthInfo) error {
	return ErrDisabled
}

// `UidAuthorizer.Authorize(ctx)` authorizes a context if it has a ucred UID
// that is in the list of allowed uids.
type UidAuthorizer struct {
	uids map[uint32]struct{}
}

// `NewUidAuthorizer(uids)` creates an `UidAuthorizer` that authorizes if ucred
// matches `uids`, and rejects all other ucreds.
func NewUidAuthorizer(uids ...uint32) *UidAuthorizer {
	a := &UidAuthorizer{uids: map[uint32]struct{}{}}
	for _, uid := range uids {
		a.uids[uid] = struct{}{}
	}
	return a
}

// `UidAuthorizer.Authorize(ctx)` returns an error unless a valid ucred that
// matches the list of uids is in the `ctx`.
func (a *UidAuthorizer) Authorize(ctx context.Context) error {
	info, ok := FromContext(ctx)
	if !ok {
		return ErrMissingUcred
	}
	return a.AuthorizeInfo(info)
}

// `UidAuthorizer.AuthorizeInfo(info)` returns an error unless `info` matches
// the `UidAuthorizer` list of uids.
func (a *UidAuthorizer) AuthorizeInfo(info *AuthInfo) error {
	uid := info.Ucred.Uid
	if _, ok := a.uids[uid]; ok {
		return nil
	}
	return status.Errorf(codes.PermissionDenied, "denied uid:%d", uid)
}
