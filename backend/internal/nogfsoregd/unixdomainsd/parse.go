package unixdomainsd

import (
	"regexp"

	"github.com/nogproject/nog/backend/internal/fsomain"
	"github.com/nogproject/nog/backend/internal/unixdomains"
	"github.com/nogproject/nog/backend/pkg/regexpx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Unix domain names must be all uppercase, start with a letter.  Hyphens are
// allowed, but not at the end.  The rule is inspired by the DNS LDH rule.
var rgxUnixDomainName = regexp.MustCompile(regexpx.Verbose(`
	^
	[A-Z]([A-Z0-9-]*[A-Z0-9])?
	$
`))

func checkUnixDomainName(s string) error {
	if !rgxUnixDomainName.MatchString(s) {
		return ErrMalformedUnixDomainName
	}
	return nil
}

func checkGid(gid uint32) error {
	if gid == 0 {
		return ErrGid0Forbidden
	}
	return nil
}

func checkUid(uid uint32) error {
	if uid == 0 {
		return ErrUid0Forbidden
	}
	return nil
}

func parseVid(b []byte) (ulid.I, error) {
	if b == nil {
		return ulid.Nil, ErrMissingVid
	}
	return parseVidNoNil(b)
}

func parseMainVid(b []byte) (ulid.I, error) {
	if b == nil {
		return fsomain.NoVC, nil
	}
	return parseVidNoNil(b)
}

func parseUnixDomainVid(b []byte) (ulid.I, error) {
	if b == nil {
		return unixdomains.NoVC, nil
	}
	return parseVidNoNil(b)
}

func parseVidNoNil(b []byte) (ulid.I, error) {
	vid, err := ulid.ParseBytes(b)
	if err != nil {
		err := status.Errorf(
			codes.InvalidArgument, "malformed vid: %s", err,
		)
		return ulid.Nil, err
	}
	return vid, nil
}

func parseDomainId(b []byte) (uuid.I, error) {
	id, err := uuid.FromBytes(b)
	if err != nil {
		err = status.Errorf(
			codes.InvalidArgument, "malformed domain id: %v", err,
		)
		return uuid.Nil, err
	}
	return id, nil
}
