package dialsududod

/*

This file must be kept in sync in packages `sudoudod`, `dialudod`, and
`dialsududod`.  The original is in `sudoudod`.  To generate the other files:

    sed 's/^package sudoudod/package dialudod/' \
        backend/internal/nogfsostad/privileges/sudoudod/udodprivileges.go \
        > backend/internal/nogfsostad/privileges/dialudod/udodprivileges.go

    sed 's/^package sudoudod/package dialsududod/' \
        backend/internal/nogfsostad/privileges/sudoudod/udodprivileges.go \
        > backend/internal/nogfsostad/privileges/dialsududod/udodprivileges.go

*/

import (
	"context"

	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/privileges"
	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/udodprivileges"
)

func (ps *Privileges) AcquireUdoStat(
	ctx context.Context, username string,
) (privileges.UdoStat, error) {
	d, err := ps.udod(ctx, username)
	if err != nil {
		return nil, err
	}
	return udodprivileges.NewUdoStat(d, username), nil
}

func (ps *Privileges) AcquireUdoChattr(
	ctx context.Context, username string,
) (privileges.UdoChattr, error) {
	d, err := ps.udod(ctx, username)
	if err != nil {
		return nil, err
	}
	return udodprivileges.NewUdoChattr(d, username), nil
}

func (ps *Privileges) AcquireUdoAclBash(
	ctx context.Context, username string,
) (privileges.UdoAclBash, error) {
	d, err := ps.udod(ctx, username)
	if err != nil {
		return nil, err
	}
	return udodprivileges.NewUdoAclBash(d, username), nil
}

func (ps *Privileges) AcquireUdoRename(
	ctx context.Context, username string,
) (privileges.UdoRename, error) {
	d, err := ps.udod(ctx, username)
	if err != nil {
		return nil, err
	}
	return udodprivileges.NewUdoRename(d, username), nil
}
