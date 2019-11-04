package pbevents

import (
	"errors"
	"fmt"
)

var ErrUnknownEventType = errors.New("unknown event type")
var ErrMissingGitAuthor = errors.New("missing GitAuthor")
var ErrMalformedGitAuthorName = errors.New("malformed GitAuthor.Name")
var ErrMalformedGitAuthorEmail = errors.New("malformed GitAuthor.Email")
var ErrMalformedGPGFingerprint = errors.New("malformed GPG key fingerprint")

type ParseError struct {
	What string
	Err  error
}

func (err *ParseError) Error() string {
	return fmt.Sprintf("invalid %s: %v", err.What, err.Err)
}
