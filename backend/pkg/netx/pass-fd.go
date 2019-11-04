package netx

import (
	"errors"
	"net"
	"os"

	"github.com/ftrvxmtrx/fd"
)

var ErrFdGetInvalid = errors.New("fd.Get() returned invalid result")
var ErrReadNonUnixConn = errors.New("received file is not a UnixConn")

func WriteFdFile(via *net.UnixConn, f *os.File) error {
	return fd.Put(via, f)
}

func ReadFdFile(via *net.UnixConn) (*os.File, error) {
	f, err := fd.Get(via, 1, nil)
	if err != nil {
		return nil, err
	}
	if len(f) != 1 {
		return nil, ErrFdGetInvalid
	}
	return f[0], nil
}

func WriteFdUnixConn(via *net.UnixConn, conn *net.UnixConn) error {
	f, err := conn.File()
	if err != nil {
		return err
	}
	// `conn.File()` duped the file descriptor.  Close the duplicate.
	defer func() { _ = f.Close() }()
	return WriteFdFile(via, f)
}

func ReadFdUnixConn(via *net.UnixConn) (*net.UnixConn, error) {
	f, err := ReadFdFile(via)
	if err != nil {
		return nil, err
	}
	// `net.FileConn()` dups the file descriptor.  Close the original.
	defer func() { _ = f.Close() }()

	conn, err := net.FileConn(f)
	if err != nil {
		return nil, err
	}
	uconn, ok := conn.(*net.UnixConn)
	if !ok {
		_ = conn.Close()
		return nil, ErrReadNonUnixConn
	}

	return uconn, nil
}
