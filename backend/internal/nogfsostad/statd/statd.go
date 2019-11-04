// Package `statd`: GRPC service `nogfso.Stat`.
package statd

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad/shadows"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/rate"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

var (
	maxStatSoftTimeout           = 10 * time.Second
	maxReinitSoftTimeout         = maxStatSoftTimeout
	maxShaSoftTimeout            = 1 * time.Minute
	maxRefreshContentSoftTimeout = 10 * time.Second
)

const (
	statQueueSize           = 30
	shaQueueSize            = 10
	refreshContentQueueSize = 30
)

type Server struct {
	lg Logger
	// `Server` allows parallel operations, relying on `Processor` to
	// serialize operations per repo.
	proc              Processor
	authn             auth.Authenticator
	authz             auth.Authorizer
	sysRPCCreds       grpc.CallOption
	doStatC           chan doFunc
	doShaC            chan doFunc
	doRefreshContentC chan doFunc
	// Control sha concurrency separately, since it may cause substantial
	// I/O, which could slow down the machine if not properly limited.
	nConcurrent    int
	nConcurrentSha int

	limiter *rate.Limiter
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
	Infow(msg string, kv ...interface{})
}

type User struct {
	Name  string
	Email string
}

type Processor interface {
	GlobalRepoPath(repoId uuid.I) (string, bool)
	StatStatus(
		ctx context.Context,
		repoId uuid.I,
		fn shadows.StatStatusFunc,
	) error
	StatRepo(
		ctx context.Context,
		repo uuid.I,
		author User,
		opts shadows.StatOptions,
	) error
	ShaRepo(ctx context.Context, repo uuid.I, author User) error
	RefreshContent(ctx context.Context, repo uuid.I, author User) error
	ReinitSubdirTracking(
		ctx context.Context,
		repo uuid.I,
		author User,
		subdirTracking pb.SubdirTracking,
	) error
}

type doFunc func(context.Context, *grpc.ClientConn)

func New(
	lg Logger,
	authn auth.Authenticator,
	authz auth.Authorizer,
	proc Processor,
	sysRPCCreds credentials.PerRPCCredentials,
) *Server {
	return &Server{
		lg:                lg,
		proc:              proc,
		authn:             authn,
		authz:             authz,
		sysRPCCreds:       grpc.PerRPCCredentials(sysRPCCreds),
		doStatC:           make(chan doFunc, statQueueSize),
		doShaC:            make(chan doFunc, shaQueueSize),
		doRefreshContentC: make(chan doFunc, refreshContentQueueSize),
		nConcurrent:       6,
		nConcurrentSha:    2,

		limiter: rate.NewLimiter(lg, rate.Config{
			Name:    "stat.statd",
			MinRate: 2,
			MaxRate: 200,
			Burst:   10,
			Tau:     5 * time.Second,
		}),
	}
}

// `Process()` asynchronously processes stat and sha requests.
func (srv *Server) Process(ctx context.Context, conn *grpc.ClientConn) error {
	var wg sync.WaitGroup

	wg.Add(srv.nConcurrent)
	procForever := func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case do := <-srv.doStatC:
				do(ctx, conn)
			case do := <-srv.doRefreshContentC:
				do(ctx, conn)
			}
		}
	}
	for i := 0; i < srv.nConcurrent; i++ {
		go procForever()
	}

	wg.Add(srv.nConcurrentSha)
	procForeverSha := func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case do := <-srv.doShaC:
				do(ctx, conn)
			}
		}
	}
	for i := 0; i < srv.nConcurrentSha; i++ {
		go procForeverSha()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.limiter.Regulate(ctx)
	}()

	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func (srv *Server) StatStatus(
	i *pb.StatStatusI, ostream pb.Stat_StatStatusServer,
) error {
	ctx := ostream.Context()

	repoId, err := srv.authRepoId(ctx, AAFsoRefreshRepo, i.Repo)
	if err != nil {
		return err
	}

	const batchSize = 64
	o := &pb.StatStatusO{}
	clear := func() {
		o.Paths = make([]*pb.PathStatus, 0, batchSize)
	}
	clear()

	flush := func() error {
		err := ostream.Send(o)
		clear()
		return err
	}

	maybeFlush := func() error {
		if len(o.Paths) < batchSize {
			return nil
		}
		return flush()
	}

	if err := srv.proc.StatStatus(
		ctx, repoId,
		func(ps pb.PathStatus) error {
			o.Paths = append(o.Paths, &ps)
			return maybeFlush()
		},
	); err != nil {
		return err
	}

	return flush()
}

func (srv *Server) Stat(
	ctx context.Context, req *pb.StatI,
) (*pb.StatO, error) {
	repo, err := srv.authRepoId(ctx, AAFsoRefreshRepo, req.Repo)
	if err != nil {
		return nil, err
	}

	if !srv.limiter.L.Allow() {
		err = status.Errorf(
			codes.ResourceExhausted, "rate limit",
		)
		return nil, err
	}

	statOpts := shadows.StatOptions{}
	// Ignore unknown flags.
	if req.Flags&uint32(pb.StatI_F_MTIME_RANGE_ONLY) != 0 {
		statOpts.MtimeRangeOnly = true
	}

	// Record rate-limit feedback independent of context.
	timeout := time.AfterFunc(maxStatSoftTimeout, func() {
		srv.limiter.Excess()
	})

	isBlocking := (req.JobControl == pb.JobControl_JC_WAIT)
	var retC chan error
	if isBlocking {
		retC = make(chan error)
	}

	// Process the request in the background.  Weak errors are only logged
	// locally.  Other errors are also stored on the repo.
	do := func(doCtx context.Context, conn *grpc.ClientConn) {
		defer timeout.Stop()

		author := User{
			Name:  req.AuthorName,
			Email: req.AuthorEmail,
		}
		err := srv.proc.StatRepo(doCtx, repo, author, statOpts)
		// ok -> record rate limit success if quick enough.
		if err == nil && timeout.Stop() {
			srv.limiter.Success()
		}

		// Return to RPC if it isn't canceled.
		if isBlocking {
			select {
			case retC <- err:
				return
			case <-ctx.Done():
			}
		}
		// Otherwise log err, and store non-weak err on repo.
		switch err.(type) {
		case nil: // ok
		case interface {
			WeakError()
		}:
			// weak -> log.
			srv.lg.Errorw("StatRepo() weak error.", "err", err)
		default:
			// non-weak -> log and store.
			srv.lg.Errorw("StatRepo() failed.", "err", err)
			srv.storeError(doCtx, conn, repo, err)
		}
	}

	// Non-blocking put into stat queue.  Always process in the background,
	// so that `git-fso` can complete even if the RPC is canceled.
	select {
	case srv.doStatC <- do: // ok, queued.
	default:
		err := status.Errorf(
			codes.ResourceExhausted, "stat queue full",
		)
		srv.limiter.Excess()
		timeout.Stop()
		return nil, err
	}

	if isBlocking {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-retC:
			if err != nil {
				return nil, err
			}
		}
	}
	return &pb.StatO{}, nil
}

func (srv *Server) Sha(
	ctx context.Context, req *pb.ShaI,
) (*pb.ShaO, error) {
	repo, err := srv.authRepoId(ctx, AAFsoRefreshRepo, req.Repo)
	if err != nil {
		return nil, err
	}

	if !srv.limiter.L.Allow() {
		err = status.Errorf(
			codes.ResourceExhausted, "rate limit",
		)
		return nil, err
	}

	// Record rate-limit feedback independent of context.
	timeout := time.AfterFunc(maxShaSoftTimeout, func() {
		srv.limiter.Excess()
	})

	isBlocking := (req.JobControl == pb.JobControl_JC_WAIT)
	var retC chan error
	if isBlocking {
		retC = make(chan error)
	}

	// Process the request in the background.  Weak errors are only logged
	// locally.  Other errors are also stored on the repo.
	do := func(doCtx context.Context, conn *grpc.ClientConn) {
		defer timeout.Stop()

		author := User{
			Name:  req.AuthorName,
			Email: req.AuthorEmail,
		}
		err := srv.proc.ShaRepo(doCtx, repo, author)
		// ok -> record rate limit success if quick enough.
		if err == nil && timeout.Stop() {
			srv.limiter.Success()
		}

		// Return to RPC if it isn't canceled.
		if isBlocking {
			select {
			case retC <- err:
				return
			case <-ctx.Done():
			}
		}
		// Otherwise log err, and store non-weak err on repo.
		switch err.(type) {
		case nil: // ok
		case interface {
			WeakError()
		}:
			// weak -> log.
			srv.lg.Errorw("ShaRepo() weak error.", "err", err)
		default:
			// non-weak -> log and store.
			srv.lg.Errorw("ShaRepo() failed.", "err", err)
			srv.storeError(doCtx, conn, repo, err)
		}
	}

	// Non-blocking put into sha queue.  Always process in the background,
	// so that `git-fso` can complete even if the RPC is canceled.
	select {
	case srv.doShaC <- do: // ok, queued.
	default:
		err := status.Errorf(
			codes.ResourceExhausted, "sha queue full",
		)
		srv.limiter.Excess()
		timeout.Stop()
		return nil, err
	}

	if isBlocking {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-retC:
			if err != nil {
				return nil, err
			}
		}
	}
	return &pb.ShaO{}, nil
}

func (srv *Server) RefreshContent(
	ctx context.Context, req *pb.RefreshContentI,
) (*pb.RefreshContentO, error) {
	repo, err := srv.authRepoId(ctx, AAFsoRefreshRepo, req.Repo)
	if err != nil {
		return nil, err
	}

	if !srv.limiter.L.Allow() {
		err = status.Errorf(
			codes.ResourceExhausted, "rate limit",
		)
		return nil, err
	}

	// Record rate-limit feedback independent of context.
	timeout := time.AfterFunc(maxRefreshContentSoftTimeout, func() {
		srv.limiter.Excess()
	})

	isBlocking := (req.JobControl == pb.JobControl_JC_WAIT)
	var retC chan error
	if isBlocking {
		retC = make(chan error)
	}

	// Process the request in the background.  Weak errors are only logged
	// locally.  Other errors are also stored on the repo.
	do := func(doCtx context.Context, conn *grpc.ClientConn) {
		defer timeout.Stop()

		author := User{
			Name:  req.AuthorName,
			Email: req.AuthorEmail,
		}
		err := srv.proc.RefreshContent(doCtx, repo, author)
		// ok -> record rate limit success if quick enough.
		if err == nil && timeout.Stop() {
			srv.limiter.Success()
		}

		// Return to RPC if it isn't canceled.
		if isBlocking {
			select {
			case retC <- err:
				return
			case <-ctx.Done():
			}
		}
		// Otherwise log err, and store non-weak err on repo.
		switch err.(type) {
		case nil: // ok
		case interface {
			WeakError()
		}:
			// weak -> log.
			srv.lg.Errorw(
				"RefreshContent() weak error.", "err", err,
			)
		default:
			// non-weak -> log and store.
			srv.lg.Errorw("RefreshContent() failed.", "err", err)
			srv.storeError(doCtx, conn, repo, err)
		}
	}

	// Non-blocking put into refresh content queue.  Always process in the
	// background, so that `git-fso` can complete even if the RPC is
	// canceled.
	select {
	case srv.doRefreshContentC <- do: // ok, queued.
	default:
		err := status.Errorf(
			codes.ResourceExhausted, "refresh content queue full",
		)
		srv.limiter.Excess()
		timeout.Stop()
		return nil, err
	}

	if isBlocking {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-retC:
			if err != nil {
				return nil, err
			}
		}
	}
	return &pb.RefreshContentO{}, nil
}

func (srv *Server) ReinitSubdirTracking(
	ctx context.Context, req *pb.ReinitSubdirTrackingI,
) (*pb.ReinitSubdirTrackingO, error) {
	subdirTracking := req.SubdirTracking
	switch subdirTracking {
	case pb.SubdirTracking_ST_ENTER_SUBDIRS: // ok
	case pb.SubdirTracking_ST_BUNDLE_SUBDIRS: // ok
	case pb.SubdirTracking_ST_IGNORE_SUBDIRS: // ok
	case pb.SubdirTracking_ST_IGNORE_MOST: // ok
	default:
		err := status.Error(
			codes.InvalidArgument, "invalid subdir_tracking",
		)
		return nil, err
	}

	repo, err := srv.authRepoId(ctx, AAFsoInitRepo, req.Repo)
	if err != nil {
		return nil, err
	}

	if !srv.limiter.L.Allow() {
		err = status.Errorf(
			codes.ResourceExhausted, "rate limit",
		)
		return nil, err
	}

	// Record rate-limit feedback independent of context.
	timeout := time.AfterFunc(maxReinitSoftTimeout, func() {
		srv.limiter.Excess()
	})

	isBlocking := (req.JobControl == pb.JobControl_JC_WAIT)
	var retC chan error
	if isBlocking {
		retC = make(chan error)
	}

	// Process the request in the background.  Weak errors are only logged
	// locally.  Other errors are also stored on the repo.
	do := func(doCtx context.Context, conn *grpc.ClientConn) {
		defer timeout.Stop()

		author := User{
			Name:  req.AuthorName,
			Email: req.AuthorEmail,
		}

		err := srv.proc.ReinitSubdirTracking(
			doCtx, repo, author, subdirTracking,
		)
		// ok -> record rate limit success if quick enough.
		if err == nil && timeout.Stop() {
			srv.limiter.Success()
		}

		// Return to RPC if it isn't canceled.
		if isBlocking {
			select {
			case retC <- err:
				return
			case <-ctx.Done():
			}
		}
		// Otherwise log err, and store non-weak err on repo.
		switch err.(type) {
		case nil: // ok
		case interface {
			WeakError()
		}:
			// weak -> log.
			srv.lg.Errorw(
				"ReinitSubdirTracking() weak error.",
				"err", err,
			)
		default:
			// non-weak -> log and store.
			srv.lg.Errorw(
				"ReinitSubdirTracking() failed.",
				"err", err,
			)
			srv.storeError(doCtx, conn, repo, err)
		}
	}

	// Non-blocking put into stat queue.  Always process in the background,
	// so that `git-fso` can complete even if the RPC is canceled.  Use the
	// stat queue, because reinit should be quicker than stat and is
	// usually followed by stat.
	select {
	case srv.doStatC <- do: // ok, execution queued.
	default:
		err := status.Errorf(
			codes.ResourceExhausted, "stat queue full",
		)
		srv.limiter.Excess()
		timeout.Stop()
		return nil, err
	}

	if isBlocking {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-retC:
			if err != nil {
				return nil, err
			}
		}
	}
	return &pb.ReinitSubdirTrackingO{}, nil
}

// Truncate to a smaller limit than nogfsoregd accepts.
const limitErrorMessageMaxLength = 120

func truncateErrorMessage(s string) string {
	if len(s) <= limitErrorMessageMaxLength {
		return s
	}
	return s[0:limitErrorMessageMaxLength-3] + "..."
}

func (srv *Server) storeError(
	ctx context.Context, conn *grpc.ClientConn, repo uuid.I, store error,
) {
	// Prefix with time, so that message is likely to be unique.
	ts := time.Now().UTC().Format(time.RFC3339)
	emsg := truncateErrorMessage(fmt.Sprintf("%s %s", ts, store))

	c := pb.NewReposClient(conn)
	_, err := c.SetRepoError(
		ctx,
		&pb.SetRepoErrorI{
			Repo:         repo[:],
			ErrorMessage: emsg,
		},
		srv.sysRPCCreds,
	)
	if err != nil {
		srv.lg.Errorw(
			"Failed to store as repo error.",
			"module", "nogfsostad",
			"err", err,
			"repo", repo,
			"repoErr", emsg,
		)
		return
	}
	srv.lg.Errorw(
		"Stored repo error.",
		"module", "nogfsostad",
		"repo", repo,
		"err", emsg,
	)
}
