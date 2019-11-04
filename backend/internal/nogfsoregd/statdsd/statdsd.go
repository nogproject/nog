package statdsd

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nogproject/nog/backend/internal/fsorepos"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Server struct {
	// Atomic.  Placed first in the struct to ensure proper alignment on
	// any arch; see <https://godoc.org/sync/atomic#pkg-note-bug>.
	seq uint64

	lg  Logger
	ctx context.Context
	wg  sync.WaitGroup

	advertiseAddr string
	tls           credentials.TransportCredentials
	authn         auth.Authenticator
	authz         auth.Authorizer

	mu        sync.Mutex
	connSlots map[uint64]chan<- net.Conn
	sessions  map[uint64]*session

	repos *fsorepos.Repos
}

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

func New(
	lg Logger,
	advertiseAddr string,
	tls credentials.TransportCredentials,
	authn auth.Authenticator,
	authz auth.Authorizer,
	repos *fsorepos.Repos,
) *Server {
	return &Server{
		lg:            lg,
		advertiseAddr: advertiseAddr,
		tls:           tls,
		authn:         authn,
		authz:         authz,
		connSlots:     make(map[uint64]chan<- net.Conn),
		sessions:      make(map[uint64]*session),
		repos:         repos,
	}
}

func (srv *Server) Serve(ctx context.Context, lis net.Listener) error {
	if srv.ctx != nil {
		panic("server context already initialized")
	}
	srv.ctx = ctx

	var isClosing int32
	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()
		for {
			conn, err := lis.Accept()
			if err != nil {
				if atomic.LoadInt32(&isClosing) == 0 {
					srv.lg.Errorw(
						"accept failed", "err", err,
					)
				}
				return
			}

			slot, err := readRgrpcHello(conn)
			if err != nil {
				srv.lg.Warnw(
					"failed to accept RGRPC",
					"err", err,
				)
				conn.Close()
				continue
			}

			srv.mu.Lock()
			connSlot, ok := srv.connSlots[slot]
			delete(srv.connSlots, slot)
			srv.mu.Unlock()
			if !ok {
				srv.lg.Errorw(
					"rejected unexpected RGRPC slot",
					"slot", slot,
				)
				conn.Close()
				continue
			}

			select {
			case <-srv.ctx.Done():
				return
			case connSlot <- conn:
				close(connSlot)
			}
		}
	}()

	select {
	case <-srv.ctx.Done():
		atomic.StoreInt32(&isClosing, 1)
		lis.Close()
		srv.wg.Wait()
		return srv.ctx.Err()
	}
}

// `readRgrpcHello()` reads the first line unbuffered, including crlf, parses
// it and returns the slot number.  The underlying connection can then be
// passed to GRPC to start the TLS handshake.
//
// The magic hello line for reverse GRPC is:
//
// ```
// "RGRPC" <space> <slot-number-ascii-decimal> <cr> <lf>
// ```
//
func readRgrpcHello(r io.Reader) (uint64, error) {
	// How much bytes to read?
	//
	// "RGRPC ": 6 bytes
	// slot uint64: max ceil(64 / 10 * 3) digits: 1 to 20 bytes
	// crlf: 2 bytes
	// --------------------
	// total: 9 to 28 bytes
	//
	// The max is actually smaller; see `maxSafeInteger`.  But we use an
	// upper limit here, just in case we later reconsider the
	// `maxSafeInteger` limit.
	//
	const magic = "RGRPC "
	const minMagicLineLength = len(magic) + 1 + 2
	const maxMagicLineLength = len(magic) + 20 + 2

	dat := make([]byte, maxMagicLineLength)
	n, err := readLine(r, dat)
	if err != nil {
		return 0, err
	}

	if n < minMagicLineLength {
		return 0, errors.New("magic line too short")
	}

	if string(dat[0:len(magic)]) != magic {
		return 0, errors.New("invalid magic")
	}

	if dat[n-2] != '\r' || dat[n-1] != '\n' {
		return 0, errors.New("line does not end with crlf")
	}

	slot, err := strconv.ParseUint(string(dat[len(magic):n-2]), 10, 64)
	if err != nil {
		return 0, err
	}

	return slot, nil
}

func readLine(r io.Reader, dat []byte) (n int, err error) {
	for i := range dat {
		for {
			nn, err := r.Read(dat[i : i+1])
			n += nn // nn is 0 or 1.
			if err != nil {
				return n, err
			}
			// GoDoc `io.Reader.Read()` states that `nn=0,err=nil`
			// must be handled.
			if nn == 0 {
				continue
			}
			break
		}
		if dat[i] == '\n' {
			return n, nil
		}
	}
	return n, errors.New("missing end of line")
}

type session struct {
	conn      *grpc.ClientConn
	slot      uint64
	ourToken  []byte
	peerName  string
	peerToken []byte
	prefixes  []string
}

func (srv *Server) Hello(
	ctx context.Context, i *pb.StatdsHelloI,
) (*pb.StatdsHelloO, error) {
	if err := srv.authNameLocal(ctx, AAFsoSession, i.Name); err != nil {
		return nil, err
	}

	slot := atomic.AddUint64(&srv.seq, 1)

	// Better be conservative and limit slot numbers to integers that are
	// safe when stored as IEEE floats, as in JavaScript.  Panic on
	// overflow, since it should never happen anyway.
	const maxSafeInteger = (1<<53 - 1)
	if slot > maxSafeInteger {
		panic("slot overflow")
	}

	token, err := newToken()
	if err != nil {
		err := status.Errorf(
			codes.ResourceExhausted, "failed to create token",
		)
		return nil, err
	}

	connSlot := make(chan net.Conn)
	srv.mu.Lock()
	srv.connSlots[slot] = connSlot
	srv.mu.Unlock()

	se := &session{
		slot:      slot,
		ourToken:  token,
		peerName:  i.Name,
		peerToken: i.SessionToken,
		prefixes:  i.Prefixes,
	}
	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()
		srv.runSession(se, connSlot)
	}()

	return &pb.StatdsHelloO{
		CallbackAddr: srv.advertiseAddr,
		CallbackSlot: slot,
		SessionToken: token,
	}, nil
}

func (srv *Server) runSession(se *session, connSlot <-chan net.Conn) {
	cleanupSession := func() {
		srv.mu.Lock()
		delete(srv.connSlots, se.slot)
		delete(srv.sessions, se.slot)
		srv.mu.Unlock()
		if se.conn != nil {
			se.conn.Close()
		}
	}
	defer cleanupSession()

	// Total timeout for dial + initial ping.
	ctx, cancel := context.WithTimeout(srv.ctx, 7*time.Second)
	defer cancel()

	dialer := func(addr string, wait time.Duration) (net.Conn, error) {
		if connSlot == nil {
			return nil, errors.New("repeated dial")
		}
		select {
		case conn := <-connSlot:
			connSlot = nil
			return conn, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	conn, err := grpc.DialContext(
		ctx,
		se.peerName,
		grpc.WithTransportCredentials(srv.tls),
		grpc.WithDialer(dialer),
	)
	if err != nil {
		srv.lg.Warnw(
			"Failed to dial session.",
			"err", err,
			"module", "statdsd",
			"slot", se.slot,
			"prefixes", se.prefixes,
		)
		return
	}
	se.conn = conn

	c := pb.NewStatdsCallbackClient(conn)
	o, err := c.Ping(ctx, &pb.StatdsCallbackPingI{
		SessionToken: se.peerToken,
	})
	if err != nil {
		srv.lg.Warnw(
			"Initial session ping failed.",
			"err", err,
			"module", "statdsd",
			"slot", se.slot,
			"prefixes", se.prefixes,
		)
		return
	}
	if !bytes.Equal(o.SessionToken, se.ourToken) {
		srv.lg.Errorw(
			"Invalid initial session token.",
			"module", "statdsd",
			"slot", se.slot,
			"prefixes", se.prefixes,
		)
		return
	}

	cancel() // Release init timeout.

	srv.mu.Lock()
	srv.sessions[se.slot] = se
	srv.mu.Unlock()
	srv.lg.Infow(
		"New nogfsostad session.",
		"module", "statdsd",
		"slot", se.slot,
		"prefixes", se.prefixes,
	)

	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
Loop:
	for {
		select {
		case <-tick.C:
			ctx, cancel := context.WithTimeout(
				srv.ctx, 2*time.Second,
			)
			o, err := c.Ping(ctx, &pb.StatdsCallbackPingI{
				SessionToken: se.peerToken,
			})
			cancel() // Release timeout.
			if err != nil {
				srv.lg.Warnw(
					"Session ping failed.",
					"err", err,
					"module", "statdsd",
				)
				break Loop
			}
			if !bytes.Equal(o.SessionToken, se.ourToken) {
				srv.lg.Errorw(
					"Invalid session token.",
					"err", err,
					"module", "statdsd",
				)
				break Loop
			}
		case <-srv.ctx.Done():
			break Loop
		}
	}

	// Deferred `cleanupSession()` does the actual cleanup.
	srv.lg.Infow(
		"Removed nogfsostad session.",
		"module", "statdsd",
		"slot", se.slot,
		"prefixes", se.prefixes,
	)
}

func newToken() ([]byte, error) {
	b := make([]byte, 16)
	_, err := crand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (srv *Server) findSessionByPath(globalPath string) *session {
	path := ensureTrailingSlash(globalPath)
	srv.mu.Lock()
	defer srv.mu.Unlock()
	for _, s := range srv.sessions {
		for _, pfx := range s.prefixes {
			if strings.HasPrefix(path, pfx) {
				return s
			}
		}
	}
	return nil
}

func copyMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, md)
}

func ensureTrailingSlash(s string) string {
	if s == "" {
		return "/"
	}
	if s[len(s)-1] == '/' {
		return s
	}
	return s + "/"
}
