package udodprivileges

import (
	"context"

	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/daemons"
	pb "github.com/nogproject/nog/backend/internal/udopb"
)

func NewUdoAclBash(d *daemons.Daemon, username string) *UdoAclBash {
	return &UdoAclBash{
		daemon:   d,
		username: username,
		client:   pb.NewUdoAclBashClient(d.Conn()),
	}
}

type UdoAclBash struct {
	daemon   *daemons.Daemon
	username string
	client   pb.UdoAclBashClient
}

func (p *UdoAclBash) Release() {
	p.daemon.Release()
	p.daemon = nil
}

func (p *UdoAclBash) PropagateAcls(
	ctx context.Context,
	src, dst string,
) error {
	_, err := p.client.UdoBashPropagateAcls(
		ctx,
		&pb.UdoBashPropagateAclsI{
			Username: p.username,
			Source:   src,
			Target:   dst,
		},
	)
	return err
}
