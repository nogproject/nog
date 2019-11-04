package sudoudod

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/nogproject/nog/backend/internal/nogfsostad/privileges/daemons"
	pb "github.com/nogproject/nog/backend/internal/udopb"
	"github.com/nogproject/nog/backend/pkg/execx"
	"github.com/nogproject/nog/backend/pkg/netx"
	"google.golang.org/grpc"
)

var ErrDialedTwice = errors.New("dialed more than once")
var ErrWrongUdodUsername = errors.New("udod returned an unexpected username")

type Privileges struct {
	daemons *daemons.Daemons
}

func New(daemons *daemons.Daemons) *Privileges {
	return &Privileges{
		daemons: daemons,
	}
}

func (ps *Privileges) udod(
	ctx context.Context, username string,
) (*daemons.Daemon, error) {
	return ps.daemons.Start(
		ctx,
		daemons.Key(fmt.Sprintf("udod(username=%s)", username)),
		func(ctx2 context.Context) (*daemons.Egg, error) {
			program := "nogfsostaudod-fd"
			if username == "root" {
				program = "nogfsostasuod-fd"
			}
			tool, err := execx.LookTool(execx.ToolSpec{
				Program:   program,
				CheckArgs: []string{"--version"},
				CheckText: program,
			})
			if err != nil {
				return nil, err
			}
			return sudoStart(
				ctx2, username,
				tool.Path, "--conn-fd=3",
			)
		},
	)
}

func sudoStart(
	ctx context.Context, username string, program string, args ...string,
) (egg *daemons.Egg, err error) {
	parentConn, childSock, err := socketpair()
	if err != nil {
		return nil, err
	}
	// If the sockets have not been used, clean up as much as possible.
	defer func() {
		if parentConn != nil {
			_ = parentConn.Close()
		}
		if childSock != nil {
			_ = childSock.Close()
		}
	}()

	sudoArgs := []string{
		// Don't ask for password.
		"-n",
		// Keep fd 3 open.  Sudo must be configured to allow `-C`; see
		// usage.
		"-C", "4",
		// Run as user.
		"-u", username,
	}
	sudoArgs = append(sudoArgs, program)
	sudoArgs = append(sudoArgs, args...)
	cmd := exec.Command("sudo", sudoArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{childSock}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// The child has duped the socket.  Close the parent file descriptor,
	// so that dial will fail quickly if the child dies.
	_ = childSock.Close()
	childSock = nil
	// If the connection to the child fails, try to kill it and wait in the
	// background to avoid zombies.  We cannot reliably kill the child,
	// because it may run as a different user.  If the conn failed,
	// however, it is likely that the child will exit.  We nonetheless wait
	// in the background to avoid blocking the current goroutine if the
	// child does not exit.  Better leak a goroutine than blocking the
	// current goroutine.
	defer func() {
		if cmd != nil {
			go func() {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}()
		}
	}()

	// `grpc.Dial()` should not retry with `WithBlock(),
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
				c := parentConn
				parentConn = nil
				return c, nil
			default:
				return nil, ErrDialedTwice
			}
		}),
	)
	if err != nil {
		return nil, err
	}
	if parentConn != nil {
		panic("dial did not use parentConn")
	}

	// `eggTmp` now owns `cmd` and `conn`.  If `eggTmp` is not returned due
	// to an error, terminate it to close `conn` and stop `cmd`.
	eggTmp := &daemons.Egg{Cmd: cmd, Conn: conn}
	cmd = nil
	conn = nil
	defer func() {
		if eggTmp != nil {
			go terminateEgg(eggTmp)
		}
	}()

	// Check the daemon to detect problems early and return an error before
	// it is used.
	c := pb.NewUdoDaemonClient(eggTmp.Conn)
	pingO, err := c.Ping(ctx, &pb.PingI{})
	if err != nil {
		return nil, err
	}
	if pingO.Username != username {
		return nil, ErrWrongUdodUsername
	}

	egg = eggTmp
	eggTmp = nil
	return egg, nil
}

// `go terminateEgg()` runs in the background without timeout in order to avoid
// zombie processes, deliberately leaking a goroutine if the child does not
// exit.
//
// `terminateEgg()` should perhaps analyze the error and retry `Terminate()` if
// the error is temporary.
func terminateEgg(e *daemons.Egg) {
	c := pb.NewUdoDaemonClient(e.Conn)
	_, _ = c.Terminate(context.Background(), &pb.TerminateI{})
	_ = e.Conn.Close()
	_ = e.Cmd.Wait()
}

func socketpair() (parent net.Conn, child *os.File, err error) {
	sp, err := netx.UnixSocketpair()
	if err != nil {
		return nil, nil, err
	}

	parent = sp[0]
	// `File()` dups the file descriptor.  Close the original.
	child, err = sp[1].File()
	_ = sp[1].Close()
	if err != nil {
		_ = parent.Close()
		return nil, nil, err
	}
	return parent, child, nil
}
