package acls

import (
	"context"

	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/privileges"
)

type AclPropagator interface {
	PropagateAcls(ctx context.Context, src, dst string) error
}

type UdoBashPrivileges interface {
	privileges.UdoAclBashPrivileges
}

type UdoBash struct {
	privs UdoBashPrivileges
}

func NewUdoBash(privs UdoBashPrivileges) *UdoBash {
	return &UdoBash{
		privs: privs,
	}
}

func (ub *UdoBash) PropagateAcls(
	ctx context.Context,
	src, dst string,
) error {
	sudo, err := ub.privs.AcquireUdoAclBash(ctx, "root")
	if err != nil {
		return err
	}
	defer sudo.Release()
	return sudo.PropagateAcls(ctx, src, dst)
}
