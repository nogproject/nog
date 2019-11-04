package udodprivileges

import (
	"context"

	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/daemons"
	pb "github.com/nogproject/nog/backend/internal/udopb"
)

func NewUdoChattr(d *daemons.Daemon, username string) *UdoChattr {
	return &UdoChattr{
		daemon:   d,
		username: username,
		client:   pb.NewUdoChattrClient(d.Conn()),
	}
}

type UdoChattr struct {
	daemon   *daemons.Daemon
	username string
	client   pb.UdoChattrClient
}

func (p *UdoChattr) Release() {
	p.daemon.Release()
	p.daemon = nil
}

func (p *UdoChattr) ChattrSetImmutable(
	ctx context.Context, path string,
) error {
	_, err := p.client.UdoChattrSetImmutable(
		ctx,
		&pb.UdoChattrSetImmutableI{
			Username: p.username,
			Path:     path,
		},
	)
	return err
}

func (p *UdoChattr) ChattrUnsetImmutable(
	ctx context.Context, path string,
) error {
	_, err := p.client.UdoChattrUnsetImmutable(
		ctx,
		&pb.UdoChattrUnsetImmutableI{
			Username: p.username,
			Path:     path,
		},
	)
	return err
}
func (p *UdoChattr) ChattrTreeSetImmutable(
	ctx context.Context, path string,
) error {
	_, err := p.client.UdoChattrTreeSetImmutable(
		ctx,
		&pb.UdoChattrTreeSetImmutableI{
			Username: p.username,
			Path:     path,
		},
	)
	return err
}

func (p *UdoChattr) ChattrTreeUnsetImmutable(
	ctx context.Context, path string,
) error {
	_, err := p.client.UdoChattrTreeUnsetImmutable(
		ctx,
		&pb.UdoChattrTreeUnsetImmutableI{
			Username: p.username,
			Path:     path,
		},
	)
	return err
}
