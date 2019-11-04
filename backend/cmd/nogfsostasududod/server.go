package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"

	"github.com/nogproject/nog/backend/pkg/execx"
	"github.com/nogproject/nog/backend/pkg/netx"
)

type Server struct {
	lg              Logger
	auth            Auther
	nogfsostaudodFd *execx.Tool
	nogfsostasuodFd *execx.Tool
}

func NewServer(lg Logger, auth Auther, version string) (*Server, error) {
	nogfsostaudodFd, err := execx.LookTool(execx.ToolSpec{
		Program:   "nogfsostaudod-fd",
		CheckArgs: []string{"--version"},
		CheckText: fmt.Sprintf("nogfsostaudod-fd-%s", version),
	})
	if err != nil {
		return nil, err
	}

	nogfsostasuodFd, err := execx.LookTool(execx.ToolSpec{
		Program:   "nogfsostasuod-fd",
		CheckArgs: []string{"--version"},
		CheckText: fmt.Sprintf("nogfsostasuod-fd-%s", version),
	})
	if err != nil {
		return nil, err
	}

	if auth == nil {
		auth = &nullAuther{}
	}

	return &Server{
		lg:              lg,
		auth:            auth,
		nogfsostaudodFd: nogfsostaudodFd,
		nogfsostasuodFd: nogfsostasuodFd,
	}, nil
}

func (srv *Server) Serve(ctx context.Context, lis *net.UnixListener) error {
	var wg sync.WaitGroup

	errC := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := lis.AcceptUnix()
			if err != nil {
				errC <- err
				close(errC)
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := srv.handleRequest(ctx, conn)
				if err != nil {
					srv.lg.Errorw(
						"Failed handling request.",
						"err", err,
					)
				}
			}()
		}
	}()

	select {
	case err := <-errC:
		_ = lis.Close()
		wg.Wait()
		return err

	case <-ctx.Done():
		// Ignore `errC` after closing listener.
		err := lis.Close()
		wg.Wait()
		if err != nil {
			return err
		}
		return ctx.Err()
	}
}

func (srv *Server) handleRequest(
	ctx context.Context, conn *net.UnixConn,
) error {
	// Clean up conn if it was not closed below.
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	if err := srv.auth.Auth(conn); err != nil {
		return err
	}

	r := json.NewDecoder(conn)
	var i struct {
		Username string `json:"username"`
	}
	if err := r.Decode(&i); err != nil {
		return err
	}
	username := i.Username

	socks, err := netx.FileSocketpair()
	if err != nil {
		return err
	}
	// Clean up sockets that were not closed below.
	defer func() {
		for _, s := range socks {
			if s != nil {
				_ = s.Close()
			}
		}
	}()

	if err := netx.WriteFdFile(conn, socks[0]); err != nil {
		return err
	}
	_ = socks[0].Close()
	socks[0] = nil

	if err := conn.Close(); err != nil {
		return err
	}
	conn = nil

	program := srv.nogfsostaudodFd.Path
	if username == "root" {
		program = srv.nogfsostasuodFd.Path
	}
	cmd := exec.CommandContext(
		ctx,
		"sudo",
		// Don't ask for password.
		"-n",
		// Keep fd 3 open.  Sudo must be configured to allow `-C`; see
		// usage.
		"-C", "4",
		// Run as user.
		"-u", username,
		// Program with args.
		program, "--conn-fd=3",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{socks[1]}
	if err := cmd.Start(); err != nil {
		return err
	}
	// The child has duped the socket.  Close the parent file descriptor,
	// so that dial will fail quickly if the child dies.
	_ = socks[1].Close()
	socks[1] = nil

	srv.lg.Infow(
		"Started nogfsostaxxxd-fd.",
		"username", username,
		"program", program,
	)

	err = cmd.Wait()
	srv.lg.Infow(
		"nogfsostaxxxd-fd terminated.",
		"username", username,
		"program", program,
	)
	return err
}
