// Package `uuid` is a subset of `google/uuid`.  See GoDoc
// <https://godoc.org/github.com/google/uuid>.
package uuid

import (
	"fmt"

	"github.com/google/uuid"
)

// `I` is a `google/uuid.UUID`.
type I = uuid.UUID

var (
	// vars
	Nil = uuid.Nil

	// funcs
	Must      = uuid.Must
	NewSHA1   = uuid.NewSHA1
	NewRandom = uuid.NewRandom
	Parse     = uuid.Parse
)

func FromBytes(data []byte) (I, error) {
	if len(data) != 16 {
		return Nil, fmt.Errorf("invalid data size")
	}
	var i I
	copy(i[:], data[:])
	return i, nil
}
