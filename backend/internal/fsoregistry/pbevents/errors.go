package pbevents

import (
	"errors"
	"fmt"
)

var ErrUnknownEventType = errors.New("unknown event type")
var ErrMalformedGPGFingerprint = errors.New("malformed GPG key fingerprint")
var ErrDuplicateGPGFingerprint = errors.New("duplicate GPG key fingerprints")
var ErrMissingRootInfo = errors.New("missing root info")
var ErrMalformedRootInfo = errors.New("malformed root info")
var ErrInvalidEvent = errors.New("invalid event")
var ErrMalformedRepoNamingNil = errors.New("nil repo naming not allowed")
var ErrMalformedRepoNamingRule = errors.New("unknown repo naming rule")
var ErrMalformedRepoNamingEmptyGlobalRoot = errors.New("malformed repo naming: empty global root")
var ErrNamingConfigNil = errors.New("naming config is nil")
var ErrMissingLevel = errors.New("missing `level`")
var ErrLevelWrongType = errors.New("`level` has wrong type")
var ErrLevelNotInteger = errors.New("`level` is not an integer")
var ErrLevelOutOfRange = errors.New("`level` out of range")
var ErrIgnoreHasWrongType = errors.New("`ignore` value has wrong type")
var ErrIgnoreListEmpty = errors.New("`ignore` list is empty")
var ErrUnexpectedConfigField = errors.New("unexpected config fields")
var ErrMissingPatterns = errors.New("missing `patterns`")
var ErrPatternsWrongType = errors.New("`patterns` has wrong type")
var ErrPolicyNil = errors.New("invalid nil policy")
var ErrUnknownRepoNamingPolicy = errors.New("unknown repo naming policy")
var ErrMissingGloblist = errors.New("missing globlist")

type PatternInvalidError struct {
	Pattern string
}

func (err *PatternInvalidError) Error() string {
	return fmt.Sprintf("invalid pattern `%s`", err.Pattern)
}

type PatternInvalidActionError struct {
	Pattern string
}

func (err *PatternInvalidActionError) Error() string {
	return fmt.Sprintf("invalid action in `%s`", err.Pattern)
}

type PatternInvalidGlobError struct {
	Pattern string
}

func (err *PatternInvalidGlobError) Error() string {
	return fmt.Sprintf("invalid glob pattern in `%s`", err.Pattern)
}

type DepthPathInvalidError struct {
	Path   string
	Reason string
	Err    error
}

func (err *DepthPathInvalidError) Error() string {
	msg := fmt.Sprintf("invalid depth path `%s`: %s", err.Path, err.Reason)
	if err.Err == nil {
		return msg
	}
	return msg + ": " + err.Err.Error()
}

type ConfigMapFieldError struct {
	Field  string
	Reason string
}

func (err *ConfigMapFieldError) Error() string {
	return fmt.Sprintf(
		"invalid config field `%s`: %s",
		err.Field, err.Reason,
	)
}

type ParseError struct {
	What string
	Err  error
}

func (err *ParseError) Error() string {
	return fmt.Sprintf("invalid %s: %v", err.What, err.Err)
}
