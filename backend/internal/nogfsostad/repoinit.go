package nogfsostad

import (
	"fmt"

	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

type RepoInfo struct {
	Id         uuid.I
	Vid        ulid.I
	GlobalPath string
	HostPath   string
	ShadowPath string
	GitlabSsh  string
}

// `strongError` indicates that the error is likely to be permanent.  Retrying
// is unlikely to resolve it.  Example: A locally missing file.
//
// `strongError` implements `interface{ StrongError() }`, which can be used to
// detect strong errors in type assertions.
type strongError struct {
	err error
}

// `weakError` indicates that the error might disappear during retry.  Example:
// A problem with a remote service that may temporary.
//
// `weakError` implements `interface{ WeakError() }`, which can be used to
// detect weak errors in type assertions.
type weakError struct {
	err error
}

// `storedError` indicates that the error has been stored before.  It must be
// resolved by an admin.  Example: A repeated `strongError` has been stored on
// the repo.  The repo will be ignored until an admin clears the error.
//
// `storedError` implements `interface{ StoredError() }`, which can be used to
// detect stored errors in type assertions.
type storedError struct {
	err error
}

func asStrongError(err error) error    { return &strongError{err: err} }
func (err *strongError) StrongError()  {}
func (err *strongError) Unwrap() error { return err.err }
func (err *strongError) Error() string {
	return fmt.Sprintf("strong error: %v", err.err)
}

func asWeakError(err error) error    { return &weakError{err: err} }
func (err *weakError) WeakError()    {}
func (err *weakError) Unwrap() error { return err.err }
func (err *weakError) Error() string {
	return fmt.Sprintf("weak error: %v", err.err)
}

func asStoredError(err error) error    { return &storedError{err: err} }
func (err *storedError) StoredError()  {}
func (err *storedError) Unwrap() error { return err.err }
func (err *storedError) Error() string {
	return fmt.Sprintf("stored error: %v", err.err)
}
