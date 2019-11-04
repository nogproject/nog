package ucred

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"

	"google.golang.org/grpc/credentials"
)

type Logger interface {
	Warnw(msg string, kv ...interface{})
}

var ErrUnimplemented = errors.New("unimplemented")
var ErrNoAuthorizer = errors.New("missing authorizer")

// A `ConnAuthorizer` is set on `SoPeerCred` to accept connection during
// `ServerHandshake()`.
type ConnAuthorizer interface {
	AuthorizeInfo(*AuthInfo) error
}

// `SoPeerCred` implements `grpc/credentials.TransportCredentials` for use as a
// `grpc.Creds()` server option or a `grpc.WithTransportCredentials()` client
// dial option.
type SoPeerCred struct {
	Authorizer ConnAuthorizer
	Logger     Logger
}

func (cr *SoPeerCred) warnw(msg string, kv ...interface{}) {
	if cr.Logger != nil {
		kv = append([]interface{}{
			"module", "ucred",
		}, kv...)
		cr.Logger.Warnw(msg, kv...)
	}
}

// `ClientHandshake()` does the same as `ServerHandshake()`.
func (creds *SoPeerCred) ClientHandshake(
	ctx context.Context, authority string, conn net.Conn,
) (net.Conn, credentials.AuthInfo, error) {
	return creds.handshake(conn)
}

// `ServerHandshake()` uses `SO_PEERCRED`, see man page `socket(7), to get the
// client ucred.  It then checks that the ucred is authorized and stores it as
// `AuthInfo` on the context, from where it can later be retrieved with
// `FromContext(ctx)` to authorize individual gRPC operations.
//
// If ucred is not authorized, `ServerHandshake()` returns an error to gRPC,
// which will close the connection.  The server logs a warning.  The client
// receives `code = Unavailable desc = transport`.
func (creds *SoPeerCred) ServerHandshake(
	conn net.Conn,
) (net.Conn, credentials.AuthInfo, error) {
	return creds.handshake(conn)
}

func (creds *SoPeerCred) handshake(
	conn net.Conn,
) (net.Conn, credentials.AuthInfo, error) {
	uconn, ok := conn.(*net.UnixConn)
	if !ok {
		err := fmt.Errorf("not a Unix connection")
		creds.warnw("Handshake denied.", "err", err)
		return nil, nil, err
	}

	fp, err := uconn.File()
	if err != nil {
		err = fmt.Errorf("failed to get fd: %s", err)
		creds.warnw("Handshake denied.", "err", err)
		return nil, nil, err
	}
	defer func() { _ = fp.Close() }()

	cred, err := syscall.GetsockoptUcred(
		int(fp.Fd()), syscall.SOL_SOCKET, syscall.SO_PEERCRED,
	)
	if err != nil {
		err = fmt.Errorf("SO_PEERCRED failed: %s", err)
		creds.warnw("Handshake denied.", "err", err)
		return nil, nil, err
	}

	auth := AuthInfo{Ucred: *cred}
	if creds.Authorizer == nil {
		err := ErrNoAuthorizer
		creds.warnw("Handshake denied.", "err", err)
		return nil, nil, err
	}
	if err := creds.Authorizer.AuthorizeInfo(&auth); err != nil {
		creds.warnw("Handshake denied.", "err", err)
		return nil, nil, err
	}

	return conn, auth, nil
}

// `Info()` returns something moderately useful.  It was not obvious that it
// needs to do more.
func (creds *SoPeerCred) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "peercred",
		ServerName:       "localhost",
	}
}

// Dummy implementation that returns self, which should be ok, because
// `SoPeerCred` is immutable.
func (creds *SoPeerCred) Clone() credentials.TransportCredentials {
	return creds
}

// Dummy implementation.
func (creds *SoPeerCred) OverrideServerName(string) error {
	return ErrUnimplemented
}
