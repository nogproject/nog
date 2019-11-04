package privileges

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/udopb"
)

type Privilege interface {
	Release()
}

type UdoStat interface {
	Privilege
	Stat(ctx context.Context, path string) (*pb.UdoStatO, error)
}

type UdoStatPrivileges interface {
	AcquireUdoStat(
		ctx context.Context, username string,
	) (UdoStat, error)
}

type UdoChattr interface {
	Privilege
	ChattrSetImmutable(ctx context.Context, path string) error
	ChattrUnsetImmutable(ctx context.Context, path string) error
	ChattrTreeSetImmutable(ctx context.Context, path string) error
	ChattrTreeUnsetImmutable(ctx context.Context, path string) error
}

type UdoChattrPrivileges interface {
	AcquireUdoChattr(
		ctx context.Context, username string,
	) (UdoChattr, error)
}

type UdoAclBash interface {
	Privilege
	PropagateAcls(ctx context.Context, src, dest string) error
}

type UdoAclBashPrivileges interface {
	AcquireUdoAclBash(
		ctx context.Context, username string,
	) (UdoAclBash, error)
}

type UdoRename interface {
	Privilege
	Rename(ctx context.Context, oldPath, newPath string) error
}

type UdoRenamePrivileges interface {
	AcquireUdoRename(
		ctx context.Context, username string,
	) (UdoRename, error)
}
