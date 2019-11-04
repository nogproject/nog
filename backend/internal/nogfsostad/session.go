package nogfsostad

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nogproject/nog/backend/internal/fsoauthz"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

type SessionConfig struct {
	// IsInitRepoAllowed()
	Prefixes             []string
	Hosts                []string
	InitLimits           *InitLimits
	SessionName          string
	TransportCredentials credentials.TransportCredentials
	Authenticator        auth.Authenticator
	Authorizer           auth.Authorizer
	SysRPCCreds          credentials.PerRPCCredentials
}

type Session struct {
	lg          Logger
	prefixes    []string
	hosts       map[string]bool
	statd       pb.StatServer
	gitnogd     pb.GitNogServer
	gitnogtreed pb.GitNogTreeServer
	discoveryd  pb.DiscoveryServer

	tarttd pb.TarttServer

	testUdoD pb.TestUdoServer

	// IsInitRepoAllowed() limits.
	initLimits *InitLimits

	tlsName     string
	tls         credentials.TransportCredentials
	authn       auth.Authenticator
	authz       auth.Authorizer
	sysRPCCreds grpc.CallOption
}

func NewSession(
	lg Logger,
	statd pb.StatServer,
	gitnogd pb.GitNogServer,
	gitnogtreed pb.GitNogTreeServer,
	discoveryd pb.DiscoveryServer,
	tarttd pb.TarttServer,
	testUdoD pb.TestUdoServer,
	cfg *SessionConfig,
) *Session {
	var prefixes []string
	for _, p := range cfg.Prefixes {
		// Ensure trailing slash.
		p = strings.TrimRight(p, "/") + "/"
		prefixes = append(prefixes, p)
	}

	hset := make(map[string]bool)
	for _, h := range cfg.Hosts {
		hset[h] = true
	}

	return &Session{
		lg:          lg,
		prefixes:    prefixes,
		hosts:       hset,
		statd:       statd,
		gitnogd:     gitnogd,
		gitnogtreed: gitnogtreed,
		discoveryd:  discoveryd,

		tarttd: tarttd,

		testUdoD: testUdoD,

		initLimits: cfg.InitLimits,

		tlsName:     cfg.SessionName,
		tls:         cfg.TransportCredentials,
		authn:       cfg.Authenticator,
		authz:       cfg.Authorizer,
		sysRPCCreds: grpc.PerRPCCredentials(cfg.SysRPCCreds),
	}
}

func (se *Session) Process(
	ctx context.Context, conn *grpc.ClientConn,
) error {
	var wg sync.WaitGroup

	c := pb.NewStatdsClient(conn)
	serveOnce := func() error {
		token, err := newToken()
		if err != nil {
			return err
		}

		o, err := c.Hello(
			ctx,
			&pb.StatdsHelloI{
				Name:         se.tlsName,
				SessionToken: token,
				Prefixes:     se.prefixes,
			},
			se.sysRPCCreds,
		)
		if err != nil {
			return err
		}

		// Don't set `grpc.keepalive` options, because reverse GRPC
		// uses an explicit ping on each session.  Maybe we could avoid
		// the explicit ping and instead rely on the gRPC keepalive
		// mechanism.  But it is not immediately clear how to do this
		// for a reverse GRPC connection.
		//
		// `keepalive.PermitWithoutStream` might do the trick, maybe
		// together with an initial ping.
		gsrv := grpc.NewServer(grpc.Creds(se.tls))
		cbd := &callbackServer{
			se:        se,
			ourToken:  token,
			peerToken: o.SessionToken,
			pingCh:    make(chan struct{}, 1),
			authn:     se.authn,
			authz:     se.authz,
		}
		pb.RegisterStatdsCallbackServer(gsrv, cbd)
		pb.RegisterStatServer(gsrv, se.statd)
		pb.RegisterGitNogServer(gsrv, se.gitnogd)
		pb.RegisterDiscoveryServer(gsrv, se.discoveryd)
		pb.RegisterGitNogTreeServer(gsrv, se.gitnogtreed)
		pb.RegisterTarttServer(gsrv, se.tarttd)
		if se.testUdoD != nil {
			pb.RegisterTestUdoServer(gsrv, se.testUdoD)
		}
		lis, err := callbackListen(ctx, o.CallbackAddr, o.CallbackSlot)
		if err != nil {
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			gsrv.Serve(lis)
		}()

		se.lg.Infow(
			"Started session callback server.",
			"callbackAddr", o.CallbackAddr,
			"slot", o.CallbackSlot,
		)

		for {
			timeout := time.NewTimer(20 * time.Second)
			select {
			case <-cbd.pingCh:
				timeout.Stop()
			case <-timeout.C:
				gsrv.GracefulStop()
				return errors.New("missing ping")
			case <-ctx.Done():
				gsrv.GracefulStop()
				return ctx.Err()
			}
		}
	}

	wg.Add(1)
	serveForever := func() {
		defer wg.Done()
		for {
			err := serveOnce()
			// Non-blocking check whether canceled.
			select {
			default:
			case <-ctx.Done():
				return
			}

			wait := 20 * time.Second
			se.lg.Errorw(
				"Will retry session2.",
				"module", "nogfsostad",
				"err", err,
				"retryIn", wait,
			)
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}
	go serveForever()

	select {
	case <-ctx.Done():
		wg.Wait()
		return ctx.Err()
	}
}

type addr string

func (a addr) Network() string { return "rgrpc" }
func (a addr) String() string  { return string(a) }

type callbackListener struct {
	conn chan net.Conn
}

func (l *callbackListener) Accept() (net.Conn, error) {
	conn := <-l.conn
	if conn == nil {
		return nil, errors.New("closed")
	}
	return conn, nil
}

func (l *callbackListener) Close() error {
	close(l.conn)
	return nil
}

func (l *callbackListener) Addr() net.Addr {
	return addr("inaccessible")
}

func callbackListen(
	ctx context.Context, addr string, slot uint64,
) (net.Listener, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	hello := fmt.Sprintf("RGRPC %d\r\n", slot)
	if _, err := io.WriteString(conn, hello); err != nil {
		return nil, err
	}

	l := &callbackListener{make(chan net.Conn, 1)}
	l.conn <- conn
	return l, nil
}

type callbackServer struct {
	se        *Session
	ourToken  []byte
	peerToken []byte
	pingCh    chan struct{}
	authn     auth.Authenticator
	authz     auth.Authorizer
}

func (srv *callbackServer) Ping(
	ctx context.Context, i *pb.StatdsCallbackPingI,
) (*pb.StatdsCallbackPingO, error) {
	// No auth.

	if !bytes.Equal(i.SessionToken, srv.ourToken) {
		err := status.Errorf(
			codes.Unauthenticated, "invalid session token",
		)
		return nil, err
	}

	select {
	default: // non-blocking
	case srv.pingCh <- struct{}{}:
	}

	return &pb.StatdsCallbackPingO{
		SessionToken: srv.peerToken,
	}, nil
}

func newToken() ([]byte, error) {
	b := make([]byte, 16)
	_, err := crand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

const AAFsoInitRepo = fsoauthz.AAFsoInitRepo

func (srv *callbackServer) IsInitRepoAllowed(
	ctx context.Context, i *pb.IsInitRepoAllowedI,
) (*pb.IsInitRepoAllowedO, error) {
	euid, err := srv.authn.Authenticate(ctx)
	if err != nil {
		return nil, err
	}
	if err := srv.authz.Authorize(
		euid, AAFsoInitRepo, map[string]interface{}{"path": i.Repo},
	); err != nil {
		return nil, err
	}

	if len(i.HostPath) < 1 || i.HostPath[0] != '/' {
		err := status.Errorf(
			codes.InvalidArgument, "malformed host path",
		)
		return nil, err
	}

	if !srv.se.hosts[i.FileHost] {
		err := status.Errorf(
			codes.FailedPrecondition, "unknown file host",
		)
		return nil, err
	}

	if !srv.se.isEqualOrBelowKnownPrefix(i.Repo) {
		err := status.Errorf(
			codes.FailedPrecondition, "invalid repo location",
		)
		return nil, err
	}

	if !isDir(i.HostPath) {
		reason := fmt.Sprintf("`%s` is not a directory", i.HostPath)
		return &pb.IsInitRepoAllowedO{
			IsAllowed: false,
			Reason:    reason,
		}, nil
	}

	lim := srv.se.initLimits.find(i.Repo)
	reason := checkInitLimit(
		i.SubdirTracking, i.HostPath, lim,
	)
	return &pb.IsInitRepoAllowedO{
		IsAllowed: reason == "",
		Reason:    reason,
	}, nil
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return st.IsDir()
}

func (se *Session) isEqualOrBelowKnownPrefix(repo string) bool {
	path := ensureTrailingSlash(repo)
	for _, pfx := range se.prefixes {
		if strings.HasPrefix(path, pfx) {
			return true
		}
	}
	return false
}
