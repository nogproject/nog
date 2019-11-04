package reposd

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrOperationMismatch = status.Error(
	codes.FailedPrecondition, "operation mismatch",
)
