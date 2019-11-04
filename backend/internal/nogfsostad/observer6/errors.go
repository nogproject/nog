package observer6

import (
	"context"
	"fmt"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/errorsx"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `ConfigErrorMessageTruncateLength` limits the length of error messages when
// storing them on a repo.  Longer messages are truncated.  The limit must be
// smaller than the maximum length that nogfsoregd accepts.
const ConfigErrorMessageTruncateLength = 120

func isStoredError(err error) bool {
	_, ok := err.(interface {
		StoredError()
	})
	return ok
}

func isStrongError(err error) bool {
	_, ok := err.(interface {
		StrongError()
	})
	return ok
}

func isWeakError(err error) bool {
	_, ok := err.(interface {
		WeakError()
	})
	return ok
}

func isCanceled(err error) bool {
	return err == context.Canceled
}

func unwrap(err error) error {
	if w, ok := err.(errorsx.Wrapper); ok {
		return w.Unwrap()
	}
	return err
}

func truncateErrorMessage(s string) string {
	if len(s) <= ConfigErrorMessageTruncateLength {
		return s
	}
	return s[0:ConfigErrorMessageTruncateLength-3] + "..."
}

func (o *RegistryObserver) storeRepoError(
	ctx context.Context, repo uuid.I, repoError error,
) error {
	// Prefix with time, so that the message is likely to be unique.
	ts := time.Now().UTC().Format(time.RFC3339)
	emsg := truncateErrorMessage(fmt.Sprintf("%s %s", ts, repoError))

	c := pb.NewReposClient(o.conn)
	_, err := c.SetRepoError(
		ctx,
		&pb.SetRepoErrorI{
			Repo:         repo[:],
			ErrorMessage: emsg,
		},
		o.sysRPCCreds,
	)
	if err != nil {
		return err
	}

	o.lg.Errorw(
		"Stored repo error.",
		"module", "nogfsostad",
		"repo", repo,
		"err", emsg,
	)

	return nil
}

type retryWeakRepoErrorHandler struct {
	lg        Logger
	storer    repoErrorStorer
	retryWeak int
}

func (o *RegistryObserver) newRepoErrorHandler() repoErrorHandler {
	return &retryWeakRepoErrorHandler{
		lg:     o.lg,
		storer: o,
	}
}

func (h *retryWeakRepoErrorHandler) handleRepoError(
	ctx context.Context, repoId uuid.I, repoError error,
) (bool, error) {
	if _, ok := errorsx.AsPred(repoError, isCanceled); ok {
		return false, nil
	}

	// Ignore known error.
	if _, ok := errorsx.AsPred(repoError, isStoredError); ok {
		h.lg.Warnw(
			"Ignored stored repo error.",
			"module", "nogfsostad",
			"repoId", repoId.String(),
			"err", repoError,
		)
		h.retryWeak = 0
		return true, nil
	}

	// Store strong error immediately.
	if _, ok := errorsx.AsPred(repoError, isStrongError); ok {
		if err := h.storer.storeRepoError(
			ctx, repoId, unwrap(repoError),
		); err != nil {
			return false, err
		}
		h.retryWeak = 0
		return true, nil
	}

	// Retry weak errors a few times, then store.
	if _, ok := errorsx.AsPred(repoError, isWeakError); ok {
		if h.retryWeak <= ConfigMaxRetryWeak {
			h.retryWeak++
			return false, nil
		}
		if err := h.storer.storeRepoError(
			ctx, repoId, unwrap(repoError),
		); err != nil {
			return false, err
		}
		h.retryWeak = 0
		return true, nil
	}

	return false, nil
}
