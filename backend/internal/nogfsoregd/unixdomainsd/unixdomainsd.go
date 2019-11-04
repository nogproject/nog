package unixdomainsd

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsomain"
	"github.com/nogproject/nog/backend/internal/unixdomains"
	pb "github.com/nogproject/nog/backend/internal/unixdomainspb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// Canceling the server `ctx` stops streaming connections.  Use it together
// with `grpc.Server.GracefulStop()`:
//
// ```
// cancel() // non-blocking
// gsrv.GracefulStop() // blocking
// ```
//
type Server struct {
	ctx      context.Context
	lg       Logger
	authn    auth.Authenticator
	authz    auth.AnyAuthorizer
	main     *fsomain.Main
	mainId   uuid.I
	domains  *unixdomains.UnixDomains
	domainsJ *events.Journal
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
}

func New(
	ctx context.Context,
	lg Logger,
	authn auth.Authenticator,
	authz auth.AnyAuthorizer,
	main *fsomain.Main,
	mainId uuid.I,
	domains *unixdomains.UnixDomains,
	domainsJ *events.Journal,
) *Server {
	return &Server{
		ctx:      ctx,
		lg:       lg,
		authn:    authn,
		authz:    authz,
		main:     main,
		mainId:   mainId,
		domains:  domains,
		domainsJ: domainsJ,
	}
}

func (srv *Server) CreateUnixDomain(
	ctx context.Context, i *pb.CreateUnixDomainI,
) (*pb.CreateUnixDomainO, error) {
	domainName := i.DomainName
	if err := checkUnixDomainName(domainName); err != nil {
		return nil, err
	}
	if err := srv.authName(ctx, AAInitUnixDomain, domainName); err != nil {
		return nil, err
	}

	mainVid, err := parseMainVid(i.MainVid)
	if err != nil {
		return nil, err
	}

	main, err := srv.main.FindId(srv.mainId)
	if err != nil {
		return nil, asMainGrpcError(err)
	}

	// Pre-check to avoid garbage in domains.
	if mainVid != fsomain.NoVC && mainVid != main.Vid() {
		return nil, ErrVersionConflict
	}
	if main.FindUnixDomainName(domainName) != nil {
		return nil, ErrDomainNameInUse
	}

	domainId, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	domainVid, err := srv.domains.Init(domainId, &unixdomains.CmdInit{
		Name: domainName,
	})
	if err != nil {
		return nil, asUnixDomainsError(err)
	}

	mainVid2, err := srv.main.AddUnixDomain(
		srv.mainId, mainVid, domainId, domainName,
	)
	if err != nil {
		return nil, asMainGrpcError(err)
	}

	o := &pb.CreateUnixDomainO{
		DomainId:  domainId[:],
		DomainVid: domainVid[:],
		MainVid:   mainVid2[:],
	}
	return o, nil
}

func (srv *Server) GetUnixDomain(
	ctx context.Context, i *pb.GetUnixDomainI,
) (*pb.GetUnixDomainO, error) {
	switch {
	case i.DomainName != "" && i.DomainId != nil:
		return nil, ErrMalformedRequest
	case i.DomainName != "":
		return srv.getUnixDomainByName(ctx, i)
	case i.DomainId != nil:
		return srv.getUnixDomainById(ctx, i)
	default:
		return nil, ErrMalformedRequest
	}
}

func (srv *Server) getUnixDomainByName(
	ctx context.Context, i *pb.GetUnixDomainI,
) (*pb.GetUnixDomainO, error) {
	domainName := i.DomainName
	if err := checkUnixDomainName(domainName); err != nil {
		return nil, err
	}
	if err := srv.authName(ctx, AAReadUnixDomain, domainName); err != nil {
		return nil, err
	}

	main, err := srv.main.FindId(srv.mainId)
	if err != nil {
		return nil, asMainGrpcError(err)
	}

	inf := main.FindUnixDomainName(domainName)
	if inf == nil {
		return nil, ErrDomainNotFound
	}
	domainId := inf.Id

	domain, err := srv.domains.FindId(domainId)
	if err != nil {
		return nil, asUnixDomainsError(err)
	}
	domainVid := domain.Vid()

	// Return only user name, UID, and GID.  The client must derive
	// secondary groups.
	us := domain.Users()
	uspb := make([]*pb.UnixDomainUser, 0, len(us))
	for _, u := range us {
		uspb = append(uspb, &pb.UnixDomainUser{
			User: u.User,
			Uid:  u.Uid,
			Gid:  u.Gid,
		})
	}

	gs := domain.Groups()
	gspb := make([]*pb.UnixDomainGroup, 0, len(gs))
	for _, g := range gs {
		gspb = append(gspb, &pb.UnixDomainGroup{
			Group: g.Group,
			Gid:   g.Gid,
			Uids:  g.Uids,
		})
	}

	o := &pb.GetUnixDomainO{
		DomainId:   domainId[:],
		DomainVid:  domainVid[:],
		DomainName: domainName,
		Users:      uspb,
		Groups:     gspb,
	}
	return o, nil
}

func (srv *Server) getUnixDomainById(
	ctx context.Context, i *pb.GetUnixDomainI,
) (*pb.GetUnixDomainO, error) {
	domain, err := srv.authUnixDomainIdState(
		ctx, AAWriteUnixDomain, i.DomainId,
	)
	if err != nil {
		return nil, err
	}

	_ = domain

	return nil, ErrNotYetImplemented
}

func (srv *Server) GetUnixUser(
	ctx context.Context, i *pb.GetUnixUserI,
) (*pb.GetUnixUserO, error) {
	domainName := i.DomainName
	if err := checkUnixDomainName(domainName); err != nil {
		return nil, err
	}
	if err := srv.authName(ctx, AAReadUnixDomain, domainName); err != nil {
		return nil, err
	}

	main, err := srv.main.FindId(srv.mainId)
	if err != nil {
		return nil, asMainGrpcError(err)
	}

	inf := main.FindUnixDomainName(domainName)
	if inf == nil {
		return nil, ErrDomainNotFound
	}
	domainId := inf.Id

	domain, err := srv.domains.FindId(domainId)
	if err != nil {
		return nil, asUnixDomainsError(err)
	}
	domainVid := domain.Vid()

	u, ok := domain.FindUser(i.User)
	if !ok {
		return nil, ErrUserNotFound
	}

	groups := make([]string, 0, len(u.Gids))
	for _, gid := range u.Gids {
		g, _ := domain.FindGid(gid)
		groups = append(groups, g.Group)
	}

	o := &pb.GetUnixUserO{
		DomainId:   domainId[:],
		DomainVid:  domainVid[:],
		DomainName: domainName,
		User:       u.User,
		Group:      groups[0],
		Groups:     groups,
		Uid:        u.Uid,
		Gid:        u.Gid,
		Gids:       u.Gids,
	}
	return o, nil
}

func (srv *Server) CreateUnixGroup(
	ctx context.Context, i *pb.CreateUnixGroupI,
) (*pb.CreateUnixGroupO, error) {
	domain, err := srv.authUnixDomainIdState(
		ctx, AAWriteUnixDomain, i.DomainId,
	)
	if err != nil {
		return nil, err
	}

	if err := checkGid(i.Gid); err != nil {
		return nil, err
	}

	vid, err := parseUnixDomainVid(i.DomainVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.domains.CreateGroup(
		domain.Id(), vid, i.Name, i.Gid,
	)
	if err != nil {
		return nil, asUnixDomainsError(err)
	}

	return &pb.CreateUnixGroupO{
		DomainVid: vid2[:],
	}, nil
}

func (srv *Server) DeleteUnixGroup(
	ctx context.Context, i *pb.DeleteUnixGroupI,
) (*pb.DeleteUnixGroupO, error) {
	domain, err := srv.authUnixDomainIdState(
		ctx, AAWriteUnixDomain, i.DomainId,
	)
	if err != nil {
		return nil, err
	}

	vid, err := parseUnixDomainVid(i.DomainVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.domains.DeleteGroup(
		domain.Id(), vid, i.Gid,
	)
	if err != nil {
		return nil, asUnixDomainsError(err)
	}

	return &pb.DeleteUnixGroupO{
		DomainVid: vid2[:],
	}, nil
}

func (srv *Server) CreateUnixUser(
	ctx context.Context, i *pb.CreateUnixUserI,
) (*pb.CreateUnixUserO, error) {
	domain, err := srv.authUnixDomainIdState(
		ctx, AAWriteUnixDomain, i.DomainId,
	)
	if err != nil {
		return nil, err
	}

	if err := checkUid(i.Uid); err != nil {
		return nil, err
	}

	vid, err := parseUnixDomainVid(i.DomainVid)
	if err != nil {
		return nil, err
	}

	cmd := &unixdomains.CmdCreateUser{
		User: i.Name,
		Uid:  i.Uid,
		Gid:  i.Gid,
	}
	vid2, err := srv.domains.CreateUser(domain.Id(), vid, cmd)
	if err != nil {
		return nil, asUnixDomainsError(err)
	}

	return &pb.CreateUnixUserO{
		DomainVid: vid2[:],
	}, nil
}

func (srv *Server) DeleteUnixUser(
	ctx context.Context, i *pb.DeleteUnixUserI,
) (*pb.DeleteUnixUserO, error) {
	domain, err := srv.authUnixDomainIdState(
		ctx, AAWriteUnixDomain, i.DomainId,
	)
	if err != nil {
		return nil, err
	}

	vid, err := parseUnixDomainVid(i.DomainVid)
	if err != nil {
		return nil, err
	}

	vid2, err := srv.domains.DeleteUser(domain.Id(), vid, i.Uid)
	if err != nil {
		return nil, asUnixDomainsError(err)
	}

	return &pb.DeleteUnixUserO{
		DomainVid: vid2[:],
	}, nil
}

func (srv *Server) AddUnixGroupUser(
	ctx context.Context, i *pb.AddUnixGroupUserI,
) (*pb.AddUnixGroupUserO, error) {
	domain, err := srv.authUnixDomainIdState(
		ctx, AAWriteUnixDomain, i.DomainId,
	)
	if err != nil {
		return nil, err
	}

	vid, err := parseUnixDomainVid(i.DomainVid)
	if err != nil {
		return nil, err
	}

	cmd := &unixdomains.CmdAddGroupUser{
		Gid: i.Gid,
		Uid: i.Uid,
	}
	vid2, err := srv.domains.AddGroupUser(domain.Id(), vid, cmd)
	if err != nil {
		return nil, asUnixDomainsError(err)
	}

	return &pb.AddUnixGroupUserO{
		DomainVid: vid2[:],
	}, nil
}

func (srv *Server) RemoveUnixGroupUser(
	ctx context.Context, i *pb.RemoveUnixGroupUserI,
) (*pb.RemoveUnixGroupUserO, error) {
	domain, err := srv.authUnixDomainIdState(
		ctx, AAWriteUnixDomain, i.DomainId,
	)
	if err != nil {
		return nil, err
	}

	vid, err := parseUnixDomainVid(i.DomainVid)
	if err != nil {
		return nil, err
	}

	cmd := &unixdomains.CmdRemoveGroupUser{
		Gid: i.Gid,
		Uid: i.Uid,
	}
	vid2, err := srv.domains.RemoveGroupUser(domain.Id(), vid, cmd)
	if err != nil {
		return nil, asUnixDomainsError(err)
	}

	return &pb.RemoveUnixGroupUserO{
		DomainVid: vid2[:],
	}, nil
}

func (srv *Server) UnixDomainEvents(
	i *pb.UnixDomainEventsI,
	stream pb.UnixDomains_UnixDomainEventsServer,
) error {
	// `ctx.Done()` indicates client close, see
	// <https://groups.google.com/d/msg/grpc-io/C0rAhtCUhSs/SzFDLGqiCgAJ>.
	ctx := stream.Context()

	domain, err := srv.authUnixDomainIdState(
		ctx, AAReadUnixDomain, i.DomainId,
	)
	if err != nil {
		return err
	}
	id := domain.Id()

	after := events.EventEpoch
	if i.After != nil {
		a, err := parseUnixDomainVid(i.After)
		if err != nil {
			return err
		}
		after = a
	}

	updated := make(chan uuid.I, 1)
	updated <- id // Trigger initial Find().

	var ticks <-chan time.Time
	if i.Watch {
		srv.domainsJ.Subscribe(updated, id)
		defer srv.domainsJ.Unsubscribe(updated)

		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-srv.ctx.Done():
			return ErrShutdown
		case <-updated:
		case <-ticks:
		}

		iter := srv.domainsJ.Find(id, after)
		var ev unixdomains.Event
		for iter.Next(&ev) {
			after = ev.Id() // Update tail for restart.

			evpb := ev.PbUnixDomainEvent()
			rsp := &pb.UnixDomainEventsO{
				Events: []*pb.UnixDomainEvent{evpb},
			}
			if err := stream.Send(rsp); err != nil {
				_ = iter.Close()
				return err
			}
		}
		if err := iter.Close(); err != nil {
			// XXX Maybe add more detailed error case handling.
			err := status.Errorf(
				codes.Unknown, "journal error: %v", err,
			)
			return err
		}

		if !i.Watch {
			return nil
		}

		rsp := &pb.UnixDomainEventsO{
			WillBlock: true,
		}
		if err := stream.Send(rsp); err != nil {
			return err
		}
	}
}
