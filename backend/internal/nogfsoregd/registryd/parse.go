package registryd

import (
	"regexp"

	"github.com/nogproject/nog/backend/internal/fsomain"
	"github.com/nogproject/nog/backend/internal/fsoregistry"
	"github.com/nogproject/nog/backend/internal/workflows/archiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/durootwf"
	"github.com/nogproject/nog/backend/internal/workflows/freezerepowf"
	"github.com/nogproject/nog/backend/internal/workflows/pingregistrywf"
	"github.com/nogproject/nog/backend/internal/workflows/splitrootwf"
	"github.com/nogproject/nog/backend/internal/workflows/unarchiverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/unfreezerepowf"
	"github.com/nogproject/nog/backend/pkg/gpg"
	"github.com/nogproject/nog/backend/pkg/regexpx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var rgxDomainName = regexp.MustCompile(regexpx.Verbose(`
	^
	[a-z]([a-z0-9-]*[a-z0-9])?
	(\.[a-z]([a-z0-9-]*[a-z0-9])?)*
	$
`))

func checkRegistryName(s string) error {
	if !rgxDomainName.MatchString(s) {
		err := status.Errorf(
			codes.InvalidArgument,
			"malformed registry name: "+
				"`%s` does not match regex `%s`",
			s, rgxDomainName,
		)
		return err
	}
	return nil
}

func (srv *Server) parseRegistryName(name string) (uuid.I, error) {
	if err := checkRegistryName(name); err != nil {
		return uuid.Nil, err
	}
	id := srv.names.UUID(NsFsoRegistry, name)
	return id, nil
}

func (srv *Server) getRegistryState(name string) (*fsoregistry.State, error) {
	id, err := srv.parseRegistryName(name)
	if err != nil {
		return nil, err
	}

	s, err := srv.registry.FindId(id)
	if err != nil {
		return nil, asRegistryGrpcError(err)
	}

	return s, nil
}

func parseRepoId(b []byte) (uuid.I, error) {
	id, err := uuid.FromBytes(b)
	if err != nil {
		err = status.Errorf(
			codes.InvalidArgument, "malformed repo id: %v", err,
		)
		return uuid.Nil, err
	}
	return id, nil
}

func parseRegistryId(b []byte) (uuid.I, error) {
	id, err := uuid.FromBytes(b)
	if err != nil {
		err = status.Errorf(
			codes.InvalidArgument,
			"malformed registry id: %v", err,
		)
		return uuid.Nil, err
	}
	return id, nil
}

func parseWorkflowId(b []byte) (uuid.I, error) {
	id, err := uuid.FromBytes(b)
	if err != nil {
		err = status.Errorf(
			codes.InvalidArgument,
			"malformed workflow id: %v", err,
		)
		return uuid.Nil, err
	}
	return id, nil
}

func parseVid(b []byte) (ulid.I, error) {
	if b == nil {
		err := status.Error(codes.InvalidArgument, "missing vid")
		return ulid.Nil, err
	}
	return parseVidNoNil(b)
}

func parseMainVid(b []byte) (ulid.I, error) {
	if b == nil {
		return fsomain.NoVC, nil
	}
	return parseVidNoNil(b)
}

func parseRegistryVid(b []byte) (ulid.I, error) {
	if b == nil {
		return fsoregistry.NoVC, nil
	}
	return parseVidNoNil(b)
}

func parseDuRootVid(b []byte) (ulid.I, error) {
	if b == nil {
		return durootwf.NoVC, nil
	}
	return parseVidNoNil(b)
}

func parsePingRegistryVid(b []byte) (ulid.I, error) {
	if b == nil {
		return pingregistrywf.NoVC, nil
	}
	return parseVidNoNil(b)
}

func parseSplitRootVid(b []byte) (ulid.I, error) {
	if b == nil {
		return splitrootwf.NoVC, nil
	}
	return parseVidNoNil(b)
}

func parseFreezeRepoVid(b []byte) (ulid.I, error) {
	if b == nil {
		return freezerepowf.NoVC, nil
	}
	return parseVidNoNil(b)
}

func parseUnfreezeRepoVid(b []byte) (ulid.I, error) {
	if b == nil {
		return unfreezerepowf.NoVC, nil
	}
	return parseVidNoNil(b)
}

func parseArchiveRepoVid(b []byte) (ulid.I, error) {
	if b == nil {
		return archiverepowf.NoVC, nil
	}
	return parseVidNoNil(b)
}

func parseUnarchiveRepoVid(b []byte) (ulid.I, error) {
	if b == nil {
		return unarchiverepowf.NoVC, nil
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

func parseGPGFingerprintsBytes(bs [][]byte) (gpg.Fingerprints, error) {
	ps, err := gpg.ParseFingerprintsBytes(bs...)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s", err)
	}
	return ps, nil
}
