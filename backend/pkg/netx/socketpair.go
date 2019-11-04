package netx

import (
	"net"
	"os"
	"syscall"
)

func FileSocketpair() (spc [2]*os.File, err error) {
	sp, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return spc, err
	}
	spf := [2]*os.File{
		os.NewFile(uintptr(sp[0]), ""),
		os.NewFile(uintptr(sp[1]), ""),
	}
	return spf, nil
}

func UnixSocketpair() (spc [2]*net.UnixConn, err error) {
	spf, err := FileSocketpair()
	if err != nil {
		return spc, err
	}
	// `net.FileConn()` dups the file descriptor.  Close the original file
	// descriptors, so that `net.Conn.Close()` closes the socket.
	defer func() {
		_ = spf[0].Close()
		_ = spf[1].Close()
	}()

	for i := 0; i < 2; i++ {
		c, err := net.FileConn(spf[i])
		if err != nil {
			if i > 0 {
				_ = spc[0].Close()
				spc[0] = nil
			}
			return spc, err
		}
		spc[i] = c.(*net.UnixConn)
	}

	return spc, nil
}
