package dialsududod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/daemons"
	pb "github.com/nogproject/nog/backend/internal/udopb"
	"github.com/nogproject/nog/backend/pkg/netx"
	"google.golang.org/grpc"
)

var ErrDialedTwice = errors.New("dialed more than once")
var ErrWrongUdodUsername = errors.New("udod returned an unexpected username")

type Privileges struct {
	daemons *daemons.Daemons
	socket  string
}

func New(daemons *daemons.Daemons, socket string) *Privileges {
	return &Privileges{
		daemons: daemons,
		socket:  socket,
	}
}

func (p *Privileges) udod(
	ctx context.Context, username string,
) (*daemons.Daemon, error) {
	return p.daemons.Start(
		ctx,
		daemons.Key(fmt.Sprintf("udod(username=%s)", username)),
		func(ctx2 context.Context) (*daemons.Egg, error) {
			return dial(ctx2, username, p.socket)
		},
	)
}

func unixDialer(addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", addr, timeout)
}

func dial(
	ctx context.Context, username string, socketPath string,
) (egg *daemons.Egg, err error) {
	udodConn, err := startUdod(ctx, username, socketPath)
	if err != nil {
		return nil, err
	}

	// `grpc.Dial()` should not retry, because we use options `WithBlock(),
	// FailOnNonTempDialError()`.  Nonetheless, return the socket only for
	// the first dial, and return errors for further dials.
	first := make(chan struct{}, 1)
	first <- struct{}{}
	conn, err := grpc.DialContext(
		ctx,
		"",
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithDisableRetry(),
		grpc.WithDialer(func(string, time.Duration) (net.Conn, error) {
			select {
			case <-first:
				c := udodConn
				udodConn = nil
				return c, nil
			default:
				return nil, ErrDialedTwice
			}
		}),
	)
	if err != nil {
		return nil, err
	}
	if udodConn != nil {
		panic("dial did not use udodConn")
	}
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	// Check the daemon to detect problems early and return an error before
	// it is used.
	c := pb.NewUdoDaemonClient(conn)
	pingO, err := c.Ping(ctx, &pb.PingI{})
	if err != nil {
		return nil, err
	}
	if pingO.Username != username {
		return nil, ErrWrongUdodUsername
	}

	egg = &daemons.Egg{
		Cmd:       nil,
		Conn:      conn,
		IsManaged: true,
	}
	conn = nil
	return egg, nil
}

func startUdod(
	ctx context.Context, username string, socketPath string,
) (*net.UnixConn, error) {
	conn, err := dialUnixContext(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	// Ignore `Close()` errors.  After `ReadFdUnixConn()` succeeds, further
	// errors are irrelevant.
	defer func() { _ = conn.Close() }()

	if t, ok := ctx.Deadline(); ok {
		conn.SetDeadline(t)
	}
	w := json.NewEncoder(conn)
	if err := w.Encode(struct {
		Username string `json:"username"`
	}{
		Username: username,
	}); err != nil {
		return nil, err
	}

	return netx.ReadFdUnixConn(conn)
}

func dialUnixContext(ctx context.Context, path string) (*net.UnixConn, error) {
	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	return conn.(*net.UnixConn), nil
}
