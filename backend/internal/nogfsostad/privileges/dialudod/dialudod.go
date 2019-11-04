package dialudod

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/daemons"
	"github.com/nogproject/nog/backend/pkg/grpc/ucred"
	"github.com/nogproject/nog/backend/pkg/pwd"
	"google.golang.org/grpc"
)

type Privileges struct {
	daemons   *daemons.Daemons
	socketDir string
}

func New(daemons *daemons.Daemons, socketDir string) *Privileges {
	return &Privileges{
		daemons:   daemons,
		socketDir: socketDir,
	}
}

func (ps *Privileges) udod(
	ctx context.Context, username string,
) (*daemons.Daemon, error) {
	return ps.daemons.Start(
		ctx,
		daemons.Key(fmt.Sprintf("udod(username=%s)", username)),
		func(ctx2 context.Context) (*daemons.Egg, error) {
			return dial(ctx2, username, ps.socketDir)
		},
	)
}

func unixDialer(addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", addr, timeout)
}

type UnknownUserError struct {
	Username string
}

func (err *UnknownUserError) Error() string {
	return fmt.Sprintf("unknown user `%s`", err.Username)
}

func dial(
	ctx context.Context, username string, socketDir string,
) (egg *daemons.Egg, err error) {
	pw := pwd.Getpwnam(username)
	if pw == nil {
		return nil, &UnknownUserError{Username: username}
	}
	uid := pw.UID

	sockPath := filepath.Join(
		socketDir, fmt.Sprintf("udod-%s.sock", username),
	)
	creds := &ucred.SoPeerCred{
		Authorizer: ucred.NewUidAuthorizer(uid),
		Logger:     nil,
	}
	conn, err := grpc.Dial(
		sockPath,
		grpc.WithDialer(unixDialer),
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return nil, err
	}

	return &daemons.Egg{Cmd: nil, Conn: conn}, nil
}
