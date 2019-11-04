package netx

import (
	"errors"
	"net"
	"os"
)

var ErrClosed = errors.New("closed")

// `FdConn(fd)` converts a file descriptor into a `net.Conn`.  The original
// file descriptor will always be closed, even if the conversion fails.
func FdConn(fd uintptr) (net.Conn, error) {
	// `f.Close()`, because `net.FileConn()` dups the file descriptor.  It
	// also sets the socket to non-blocking.
	f := os.NewFile(fd, "")
	defer func() { _ = f.Close() }()
	return net.FileConn(f)
}

// `ConnectedConnListener` is a `net.Listener` that returns one connection on
// the first call to `Accept()`.  Further calls to `Accept()` block until the
// listener is closed.
type ConnectedConnListener struct {
	// `conn` is a channel of size 1 that contains the connection.  The
	// first receive returns the connection.  Further receives block until
	// the channel is closed by `ConnectedConnListener.Close()`.
	conn chan net.Conn
	addr net.Addr
}

// `ListenConnectedConn()` wraps a connected `net.Conn` such that is can be
// used as a `net.Listener`.
func ListenConnectedConn(conn net.Conn) *ConnectedConnListener {
	lis := &ConnectedConnListener{
		conn: make(chan net.Conn, 1),
		addr: conn.LocalAddr(),
	}
	lis.conn <- conn
	return lis
}

func (lis *ConnectedConnListener) Accept() (net.Conn, error) {
	conn := <-lis.conn
	if conn == nil {
		return nil, ErrClosed
	}
	return conn, nil
}

func (lis *ConnectedConnListener) Close() error {
	close(lis.conn)
	return nil
}

func (lis *ConnectedConnListener) Addr() net.Addr {
	return lis.addr
}
