// Package `ratelimit` wraps the subset of `github.com/juju/ratelimit` that
// other Nog packages use.
package ratelimit

import "github.com/juju/ratelimit"

type Bucket = ratelimit.Bucket

// funcs
var Reader = ratelimit.Reader
var Writer = ratelimit.Writer
var NewBucketWithRate = ratelimit.NewBucketWithRate
