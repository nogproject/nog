package nogfsodomd

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/unixdomainspb"
	"github.com/nogproject/nog/backend/pkg/getent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

type Config struct {
	Domain        string
	GroupPrefixes []string
	Conn          *grpc.ClientConn
	SysRPCCreds   credentials.PerRPCCredentials
}

type Syncer struct {
	lg       Logger
	domain   string
	prefixes []string
	conn     *grpc.ClientConn
	creds    grpc.CallOption
}

func New(lg Logger, cfg *Config) *Syncer {
	return &Syncer{
		lg:       lg,
		domain:   cfg.Domain,
		prefixes: cfg.GroupPrefixes,
		conn:     cfg.Conn,
		creds:    grpc.PerRPCCredentials(cfg.SysRPCCreds),
	}
}

func (syn *Syncer) Sync(ctx context.Context) error {
	err := syn.syncDomain(ctx)
	switch {
	case err == context.Canceled:
		return err
	case err != nil:
		syn.lg.Errorw(
			"Failed to sync Unix domain.",
			"err", err,
		)
		return err
	default:
		return nil
	}
}

func (syn *Syncer) syncDomain(ctx context.Context) error {
	c := pb.NewUnixDomainsClient(syn.conn)

	getUnixDomain := func() (
		[]byte,
		[]byte,
		[]*pb.UnixDomainUser,
		[]*pb.UnixDomainGroup,
		error,
	) {
		i := &pb.GetUnixDomainI{
			DomainName: syn.domain,
		}
		o, err := c.GetUnixDomain(ctx, i, syn.creds)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return o.DomainId, o.DomainVid, o.Users, o.Groups, err
	}

	domId, domVid, domUsers, domGroups, err := getUnixDomain()
	if err != nil {
		return err
	}

	createGroup := func(
		vid []byte, g getent.Group,
	) ([]byte, error) {
		i := &pb.CreateUnixGroupI{
			DomainId:  domId,
			DomainVid: vid,
			Name:      g.Group,
			Gid:       g.Gid,
		}
		o, err := c.CreateUnixGroup(ctx, i, syn.creds)
		if err != nil {
			return nil, err
		}
		return o.DomainVid, nil
	}

	createUser := func(
		vid []byte, u getent.Passwd,
	) ([]byte, error) {
		i := &pb.CreateUnixUserI{
			DomainId:  domId,
			DomainVid: vid,
			Name:      u.User,
			Uid:       u.Uid,
			Gid:       u.Gid,
		}
		o, err := c.CreateUnixUser(ctx, i, syn.creds)
		if err != nil {
			return nil, err
		}
		return o.DomainVid, nil
	}

	addGroupUser := func(
		vid []byte, g getent.Group, u getent.Passwd,
	) ([]byte, error) {
		i := &pb.AddUnixGroupUserI{
			DomainId:  domId,
			DomainVid: vid,
			Gid:       g.Gid,
			Uid:       u.Uid,
		}
		o, err := c.AddUnixGroupUser(ctx, i, syn.creds)
		if err != nil {
			return nil, err
		}
		return o.DomainVid, nil
	}

	removeGroupUser := func(
		vid []byte, g *pb.UnixDomainGroup, u *pb.UnixDomainUser,
	) ([]byte, error) {
		i := &pb.RemoveUnixGroupUserI{
			DomainId:  domId,
			DomainVid: vid,
			Gid:       g.Gid,
			Uid:       u.Uid,
		}
		o, err := c.RemoveUnixGroupUser(ctx, i, syn.creds)
		if err != nil {
			return nil, err
		}
		return o.DomainVid, nil
	}

	deleteUser := func(
		vid []byte, u *pb.UnixDomainUser,
	) ([]byte, error) {
		i := &pb.DeleteUnixUserI{
			DomainId:  domId,
			DomainVid: vid,
			Uid:       u.Uid,
		}
		o, err := c.DeleteUnixUser(ctx, i, syn.creds)
		if err != nil {
			return nil, err
		}
		return o.DomainVid, nil
	}

	deleteGroup := func(
		vid []byte, g *pb.UnixDomainGroup,
	) ([]byte, error) {
		i := &pb.DeleteUnixGroupI{
			DomainId:  domId,
			DomainVid: vid,
			Gid:       g.Gid,
		}
		o, err := c.DeleteUnixGroup(ctx, i, syn.creds)
		if err != nil {
			return nil, err
		}
		return o.DomainVid, nil
	}

	groups, err := getent.Groups(ctx)
	if err != nil {
		return err
	}
	groups = getent.SelectGroups(groups, syn.prefixes)
	groups, err = getent.DedupGroups(groups)
	if err != nil {
		return err
	}

	passwds, err := getent.Passwds(ctx)
	if err != nil {
		return err
	}
	passwds = getent.SelectPasswds(passwds, groups)

	// Catch obvious configuration errors.
	if len(groups) == 0 {
		return &GetentError{
			Reason: "no groups selected",
		}
	}
	if len(passwds) == 0 {
		return &GetentError{
			Reason: "no passwd entries selected",
		}
	}

	usersByName := make(map[string]getent.Passwd)
	usersByUid := make(map[uint32]getent.Passwd)
	for _, u := range passwds {
		usersByName[u.User] = u
		usersByUid[u.Uid] = u
	}

	domUsersByUid := make(map[uint32]*pb.UnixDomainUser)
	for _, u := range domUsers {
		domUsersByUid[u.Uid] = u
	}

	// Add groups.
	domGids := make(map[uint32]struct{})
	for _, g := range domGroups {
		domGids[g.Gid] = struct{}{}
	}
	for _, g := range groups {
		if _, ok := domGids[g.Gid]; !ok {
			v, err := createGroup(domVid, g)
			if err != nil {
				return err
			}
			domVid = v
			syn.lg.Infow(
				"Created group.",
				"group", g.Group,
				"GID", g.Gid,
			)
		}
	}

	// Add users.
	for _, u := range passwds {
		if _, ok := domUsersByUid[u.Uid]; !ok {
			v, err := createUser(domVid, u)
			if err != nil {
				return err
			}
			domVid = v
			syn.lg.Infow(
				"Created user.",
				"user", u.User,
				"UID", u.Uid,
				"GID", u.Gid,
			)
		}
	}

	// Add users to groups.
	domGidUids := make(map[uint32]map[uint32]struct{})
	for _, g := range domGroups {
		uids := make(map[uint32]struct{})
		for _, u := range g.Uids {
			uids[u] = struct{}{}
		}
		domGidUids[g.Gid] = uids
	}
	for _, g := range groups {
		for _, username := range g.Users {
			u, ok := usersByName[username]
			// Ignore users that are not in passwds.
			if !ok {
				continue
			}
			// Do not add to primary group.  The primary group was
			// handled when the user was added.
			if u.Gid == g.Gid {
				continue
			}
			if _, ok := domGidUids[g.Gid][u.Uid]; !ok {
				v, err := addGroupUser(domVid, g, u)
				if err != nil {
					return err
				}
				domVid = v
				syn.lg.Infow(
					"Added group user.",
					"group", g.Group,
					"GID", u.Gid,
					"user", u.User,
					"UID", u.Uid,
				)
			}
		}
	}

	// Remove users from groups.
	gidUids := make(map[uint32]map[uint32]struct{})
	for _, g := range groups {
		uids := make(map[uint32]struct{})
		for _, username := range g.Users {
			u, ok := usersByName[username]
			if !ok {
				// Ignore users that are not in passwds.
				continue
			}
			uids[u.Uid] = struct{}{}
		}
		gidUids[g.Gid] = uids
	}
	for _, g := range domGroups {
		for _, uid := range g.Uids {
			u, ok := domUsersByUid[uid]
			if !ok {
				return &DomainLogicError{
					Reason: "missing group user",
				}
			}
			// Do not remove from primary group.  The primary group
			// will be handled when the user is removed.
			if u.Gid == g.Gid {
				continue
			}
			if _, ok := gidUids[g.Gid][u.Uid]; !ok {
				v, err := removeGroupUser(domVid, g, u)
				if err != nil {
					return err
				}
				domVid = v
				syn.lg.Infow(
					"Removed group user.",
					"group", g.Group,
					"GID", u.Gid,
					"user", u.User,
					"UID", u.Uid,
				)
			}
		}
	}

	// Remove users.
	for _, u := range domUsers {
		if _, ok := usersByUid[u.Uid]; !ok {
			v, err := deleteUser(domVid, u)
			if err != nil {
				return err
			}
			domVid = v
			syn.lg.Infow(
				"Deleted user.",
				"user", u.User,
				"UID", u.Uid,
			)
		}
	}

	// Remove groups.
	gids := make(map[uint32]struct{})
	for _, g := range groups {
		gids[g.Gid] = struct{}{}
	}
	for _, g := range domGroups {
		if _, ok := gids[g.Gid]; !ok {
			v, err := deleteGroup(domVid, g)
			if err != nil {
				return err
			}
			domVid = v
			syn.lg.Infow(
				"Deleted group.",
				"group", g.Group,
				"GID", g.Gid,
			)
		}
	}

	return nil
}
