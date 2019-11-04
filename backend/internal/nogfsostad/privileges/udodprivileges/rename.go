package udodprivileges

import (
	"context"

	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/daemons"
	pb "github.com/nogproject/nog/backend/internal/udopb"
)

func NewUdoRename(d *daemons.Daemon, username string) *UdoRename {
	return &UdoRename{
		daemon:   d,
		username: username,
		client:   pb.NewUdoRenameClient(d.Conn()),
	}
}

type UdoRename struct {
	daemon   *daemons.Daemon
	username string
	client   pb.UdoRenameClient
}

func (p *UdoRename) Release() {
	p.daemon.Release()
	p.daemon = nil
}

func (p *UdoRename) Rename(
	ctx context.Context, oldPath, newPath string,
) error {
	_, err := p.client.UdoRename(ctx, &pb.UdoRenameI{
		Username: p.username,
		OldPath:  oldPath,
		NewPath:  newPath,
	})
	return err
}
