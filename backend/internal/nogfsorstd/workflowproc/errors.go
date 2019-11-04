package workflowproc

import (
	"errors"
	"strings"
)

// `ConfigErrorMessageTruncateLength` limits the length of error messages when
// storing them on a repo.  Longer messages are truncated.  The limit must be
// smaller than the maximum length that nogfsoregd accepts.
const ConfigErrorMessageTruncateLength = 120

var ErrUnknownEvent = errors.New("unknown event")
var ErrWrongHost = errors.New("wrong host")

func truncateErrorMessage(s string) string {
	if len(s) <= ConfigErrorMessageTruncateLength {
		return s
	}
	return s[0:ConfigErrorMessageTruncateLength-3] + "..."
}

func errorContainsAny(err error, substrs []string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, substr := range substrs {
		if strings.Contains(msg, substr) {
			return true
		}
	}
	return false
}
