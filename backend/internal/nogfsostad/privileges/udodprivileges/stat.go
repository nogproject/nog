package udodprivileges

import (
	"context"

	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/daemons"
	pb "github.com/nogproject/nog/backend/internal/udopb"
)

func NewUdoStat(d *daemons.Daemon, username string) *UdoStat {
	return &UdoStat{
		daemon:   d,
		username: username,
		client:   pb.NewUdoStatClient(d.Conn()),
	}
}

type UdoStat struct {
	daemon   *daemons.Daemon
	username string
	client   pb.UdoStatClient
}

func (p *UdoStat) Release() {
	p.daemon.Release()
	p.daemon = nil
}

func (p *UdoStat) Stat(
	ctx context.Context, path string,
) (*pb.UdoStatO, error) {
	return p.client.UdoStat(ctx, &pb.UdoStatI{
		Username: p.username,
		Path:     path,
	})
}
