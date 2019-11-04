package ulid

import (
	crand "crypto/rand"
	"time"

	"github.com/oklog/ulid"
)

// `I` is an `oklog/ulid.ULID`.
type I = ulid.ULID

// `Nil` is the all-zero null value.
var Nil I

// `ulid.One` can be used as a sentinel value.
var One = ulid.MustParse("00000000000000000000000001")

// `ulid.Two` can be used as a sentinel value.
var Two = ulid.MustParse("00000000000000000000000002")

// funcs
var Parse = ulid.Parse

func New() (I, error) {
	return ulid.New(ulid.Now(), crand.Reader)
}

func ParseBytes(data []byte) (I, error) {
	var id I
	if data == nil {
		return id, nil
	}
	err := id.UnmarshalBinary(data)
	return id, err
}

func Time(id I) time.Time {
	ms := id.Time()
	s := ms / 1000
	ns := (ms % 1000) * 1000 * 1000
	return time.Unix(int64(s), int64(ns))
}

const RFC3339Milli = "2006-01-02T15:04:05.000Z07:00"

func TimeString(id I) string {
	return Time(id).Format(RFC3339Milli)
}
