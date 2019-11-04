package unixdomains

import (
	"regexp"
	"sort"

	"github.com/nogproject/nog/backend/internal/events"
	uxev "github.com/nogproject/nog/backend/internal/unixdomains/events"
	pb "github.com/nogproject/nog/backend/internal/unixdomainspb"
	"github.com/nogproject/nog/backend/pkg/regexpx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

var NoVC = events.NoVC
var RetryNoVC = events.RetryNoVC

type State struct {
	id  uuid.I
	vid ulid.I

	name string

	groupsByName map[string]*group
	groupsByGid  map[uint32]*group
	usersByName  map[string]*user
	usersByUid   map[uint32]*user
}

// The bools indicate which part of the state has been duplicated.
type Advancer struct {
	state  bool // The state itself.
	groups bool
	users  bool
}

type group struct {
	group string
	gid   uint32
	uids  []uint32
}

func (g *group) hasUid(uid uint32) bool {
	for _, guid := range g.uids {
		if uid == guid {
			return true
		}
	}
	return false
}

type user struct {
	user string
	uid  uint32
	gids []uint32
}

type CmdInit struct {
	Name string
}

type CmdCreateGroup struct {
	Group string
	Gid   uint32
}

type CmdCreateUser struct {
	User string
	Uid  uint32
	Gid  uint32
}

type CmdAddGroupUser struct {
	Gid uint32
	Uid uint32
}

type CmdRemoveGroupUser struct {
	Gid uint32
	Uid uint32
}

type CmdDeleteUser struct {
	Uid uint32
}

type CmdDeleteGroup struct {
	Gid uint32
}

func (*State) AggregateState() {}

func (*CmdInit) AggregateCommand()            {}
func (*CmdCreateGroup) AggregateCommand()     {}
func (*CmdCreateUser) AggregateCommand()      {}
func (*CmdAddGroupUser) AggregateCommand()    {}
func (*CmdRemoveGroupUser) AggregateCommand() {}
func (*CmdDeleteUser) AggregateCommand()      {}
func (*CmdDeleteGroup) AggregateCommand()     {}

func (s *State) Id() uuid.I        { return s.id }
func (s *State) Vid() ulid.I       { return s.vid }
func (s *State) SetVid(vid ulid.I) { s.vid = vid }

type Behavior struct{}
type Event struct{ uxev.Event }

func (Behavior) NewState(id uuid.I) events.State { return &State{id: id} }
func (Behavior) NewEvent() events.Event          { return &Event{} }
func (Behavior) NewAdvancer() events.Advancer    { return &Advancer{} }

func (ev *Event) UnmarshalProto(data []byte) error {
	if err := ev.Event.UnmarshalProto(data); err != nil {
		return err
	}
	switch ev.Event.PbUnixDomainEvent().Event {
	default:
		return &EventTypeError{}
	case pb.UnixDomainEvent_EV_UNIX_DOMAIN_CREATED:
	case pb.UnixDomainEvent_EV_UNIX_GROUP_CREATED:
	case pb.UnixDomainEvent_EV_UNIX_USER_CREATED:
	case pb.UnixDomainEvent_EV_UNIX_GROUP_USER_ADDED:
	case pb.UnixDomainEvent_EV_UNIX_GROUP_USER_REMOVED:
	case pb.UnixDomainEvent_EV_UNIX_USER_DELETED:
	case pb.UnixDomainEvent_EV_UNIX_GROUP_DELETED:
	case pb.UnixDomainEvent_EV_UNIX_DOMAIN_DELETED:
	}
	return nil
}

func (a *Advancer) Advance(s events.State, ev events.Event) events.State {
	st := s.(*State)

	if !a.state {
		dup := *st
		st = &dup
		a.state = true
	}

	detachGroups := func() {
		if a.groups {
			return
		}
		st.groupsByName = dupGroupsByName(st.groupsByName)
		st.groupsByGid = dupGroupsByGid(st.groupsByGid)
		a.groups = true
	}

	detachUsers := func() {
		if a.users {
			return
		}
		st.usersByName = dupUsersByName(st.usersByName)
		st.usersByUid = dupUsersByUid(st.usersByUid)
		a.users = true
	}

	var evpb *pb.UnixDomainEvent
	switch x := ev.(type) {
	case *Event: // Event from `UnmarshalProto()`
		evpb = x.PbUnixDomainEvent()
	case *uxev.Event: // Event from `Tell()`
		evpb = x.PbUnixDomainEvent()
	default:
		panic("invalid event")
	}
	switch x := uxev.MustParsePbUnixDomainEvent(evpb).(type) {
	case *uxev.EvDomainCreated:
		st.name = x.Name
		st.groupsByName = make(map[string]*group)
		st.groupsByGid = make(map[uint32]*group)
		st.usersByName = make(map[string]*user)
		st.usersByUid = make(map[uint32]*user)
		return st

	case *uxev.EvGroupCreated:
		detachGroups()
		g := &group{group: x.Group, gid: x.Gid}
		st.groupsByName[g.group] = g
		st.groupsByGid[g.gid] = g
		return st

	case *uxev.EvUserCreated:
		// Update user state.
		detachUsers()
		u := &user{user: x.User, uid: x.Uid, gids: []uint32{x.Gid}}
		st.usersByName[u.user] = u
		st.usersByUid[u.uid] = u
		// Update group state.
		detachGroups()
		g := *st.groupsByGid[x.Gid] // copy
		g.uids = append(g.uids, x.Uid)
		st.groupsByName[g.group] = &g
		st.groupsByGid[g.gid] = &g
		return st

	case *uxev.EvGroupUserAdded:
		// Update user state.
		detachUsers()
		u := *st.usersByUid[x.Uid] // copy
		u.gids = append(u.gids, x.Gid)
		st.usersByName[u.user] = &u
		st.usersByUid[u.uid] = &u
		// Update group state.
		detachGroups()
		g := *st.groupsByGid[x.Gid] // copy
		g.uids = append(g.uids, x.Uid)
		st.groupsByName[g.group] = &g
		st.groupsByGid[g.gid] = &g
		return st

	case *uxev.EvGroupUserRemoved:
		// Update user state.
		detachUsers()
		u := *st.usersByUid[x.Uid] // copy
		u.gids = deleteGid(u.gids, x.Gid)
		st.usersByName[u.user] = &u
		st.usersByUid[u.uid] = &u
		// Update group state.
		detachGroups()
		g := *st.groupsByGid[x.Gid] // copy
		g.uids = deleteUid(g.uids, x.Uid)
		st.groupsByName[g.group] = &g
		st.groupsByGid[g.gid] = &g
		return st

	case *uxev.EvUserDeleted:
		// Update user state.
		detachUsers()
		u := st.usersByUid[x.Uid]
		delete(st.usersByUid, u.uid)
		delete(st.usersByName, u.user)
		// Update group state.
		detachGroups()
		g := *st.groupsByGid[u.gids[0]] // copy
		g.uids = deleteUid(g.uids, u.uid)
		st.groupsByName[g.group] = &g
		st.groupsByGid[g.gid] = &g
		return st

	case *uxev.EvGroupDeleted:
		detachGroups()
		g := st.groupsByGid[x.Gid]
		delete(st.groupsByGid, g.gid)
		delete(st.groupsByName, g.group)
		return st

	default:
		panic("invalid event")
	}
}

func dupGroupsByName(src map[string]*group) map[string]*group {
	dup := make(map[string]*group)
	for k, v := range src {
		dup[k] = v
	}
	return dup
}

func dupGroupsByGid(src map[uint32]*group) map[uint32]*group {
	dup := make(map[uint32]*group)
	for k, v := range src {
		dup[k] = v
	}
	return dup
}

func dupUsersByName(src map[string]*user) map[string]*user {
	dup := make(map[string]*user)
	for k, v := range src {
		dup[k] = v
	}
	return dup
}

func dupUsersByUid(src map[uint32]*user) map[uint32]*user {
	dup := make(map[uint32]*user)
	for k, v := range src {
		dup[k] = v
	}
	return dup
}

func deleteGid(gids []uint32, gid uint32) []uint32 {
	for i, v := range gids {
		if v == gid {
			return append(gids[0:i], gids[i+1:]...)
		}
	}
	return gids
}

var deleteUid = deleteGid

func (Behavior) Tell(
	s events.State, c events.Command,
) ([]events.Event, error) {
	st := s.(*State)
	switch cmd := c.(type) {
	case *CmdInit:
		return tellInit(st, cmd)
	case *CmdCreateGroup:
		return tellCreateGroup(st, cmd)
	case *CmdCreateUser:
		return tellCreateUser(st, cmd)
	case *CmdAddGroupUser:
		return tellAddGroupUser(st, cmd)
	case *CmdRemoveGroupUser:
		return tellRemoveGroupUser(st, cmd)
	case *CmdDeleteUser:
		return tellDeleteUser(st, cmd)
	case *CmdDeleteGroup:
		return tellDeleteGroup(st, cmd)
	default:
		return nil, &InvalidCommandError{}
	}
}

var rgxDomainName = regexp.MustCompile(regexpx.Verbose(`
	^ [a-zA-Z0-9_-]+ $
`))

func tellInit(st *State, cmd *CmdInit) ([]events.Event, error) {
	// The command can only be idempotent if the workflow has not advanced
	// beyond init.
	switch {
	case st.name == "":
		break // Init is allowed as the first command.
	default:
		if cmd.Name != st.name {
			return nil, &NotIdempotentError{}
		}
		return nil, nil // idempotent
	}

	if !rgxDomainName.MatchString(cmd.Name) {
		return nil, &ArgumentError{Reason: "malformed domain name"}
	}

	ev := &uxev.EvDomainCreated{
		Name: cmd.Name,
	}
	return wrapEventsNewEventsError(uxev.NewEvents(
		st.Vid(),
		uxev.NewPbDomainCreated(ev),
	))
}

var rgxGroupName = regexp.MustCompile(regexpx.Verbose(`
	^ [a-zA-Z0-9_-]+ $
`))

func tellCreateGroup(st *State, cmd *CmdCreateGroup) ([]events.Event, error) {
	if st.name == "" {
		return nil, &UninitializedError{}
	}

	if g, ok := st.groupsByGid[cmd.Gid]; ok {
		if cmd.Group != g.group {
			return nil, &GroupConflictError{
				AGroup: g.group,
				AGid:   g.gid,
				BGroup: cmd.Group,
				BGid:   cmd.Gid,
			}
		}
		return nil, nil // idempotent
	}

	if g, ok := st.groupsByName[cmd.Group]; ok {
		if cmd.Gid != g.gid {
			return nil, &GroupConflictError{
				AGroup: g.group,
				AGid:   g.gid,
				BGroup: cmd.Group,
				BGid:   cmd.Gid,
			}
		}
		return nil, nil // idempotent
	}

	if !rgxGroupName.MatchString(cmd.Group) {
		return nil, &ArgumentError{Reason: "malformed group name"}
	}

	return wrapEventsNewEventsError(uxev.NewEvents(
		st.Vid(),
		uxev.NewPbGroupCreated(cmd.Group, cmd.Gid),
	))
}

var rgxUserName = regexp.MustCompile(regexpx.Verbose(`
	^ [a-zA-Z0-9_-]+ $
`))

func (cmd *CmdCreateUser) isEqualUser(u *user) bool {
	return cmd.User == u.user &&
		cmd.Uid == u.uid &&
		cmd.Gid == u.gids[0]
}

func tellCreateUser(st *State, cmd *CmdCreateUser) ([]events.Event, error) {
	if st.name == "" {
		return nil, &UninitializedError{}
	}

	if !rgxUserName.MatchString(cmd.User) {
		return nil, &ArgumentError{Reason: "malformed user name"}
	}

	u := st.usersByName[cmd.User]
	if u == nil {
		u = st.usersByUid[cmd.Uid]
	}
	if u != nil {
		if !cmd.isEqualUser(u) {
			return nil, &UserConflictError{
				AUser: u.user,
				AUid:  u.uid,
				AGid:  u.gids[0],
				BUser: cmd.User,
				BUid:  cmd.Uid,
				BGid:  cmd.Gid,
			}
		}
		return nil, nil // idempotent
	}

	if _, ok := st.groupsByGid[cmd.Gid]; !ok {
		return nil, &MissingGroupError{Gid: cmd.Gid}
	}

	return wrapEventsNewEventsError(uxev.NewEvents(
		st.Vid(),
		uxev.NewPbUserCreated(&uxev.EvUserCreated{
			User: cmd.User,
			Uid:  cmd.Uid,
			Gid:  cmd.Gid,
		}),
	))
}

func tellAddGroupUser(
	st *State, cmd *CmdAddGroupUser,
) ([]events.Event, error) {
	if st.name == "" {
		return nil, &UninitializedError{}
	}

	if _, ok := st.usersByUid[cmd.Uid]; !ok {
		return nil, &MissingUserError{Uid: cmd.Uid}
	}

	g, ok := st.groupsByGid[cmd.Gid]
	if !ok {
		return nil, &MissingGroupError{Gid: cmd.Gid}
	}

	if g.hasUid(cmd.Uid) {
		return nil, nil // idempotent
	}

	return wrapEventsNewEventsError(uxev.NewEvents(
		st.Vid(),
		uxev.NewPbGroupUserAdded(&uxev.EvGroupUserAdded{
			Uid: cmd.Uid,
			Gid: cmd.Gid,
		}),
	))
}

func tellRemoveGroupUser(
	st *State, cmd *CmdRemoveGroupUser,
) ([]events.Event, error) {
	if st.name == "" {
		return nil, &UninitializedError{}
	}

	u, ok := st.usersByUid[cmd.Uid]
	if !ok {
		return nil, &MissingUserError{Uid: cmd.Uid}
	}
	if cmd.Gid == u.gids[0] {
		return nil, &CannotRemovePrimaryGroupError{Gid: cmd.Gid}
	}

	g, ok := st.groupsByGid[cmd.Gid]
	if !ok {
		return nil, &MissingGroupError{Gid: cmd.Gid}
	}

	if !g.hasUid(cmd.Uid) {
		return nil, nil // idempotent
	}

	return wrapEventsNewEventsError(uxev.NewEvents(
		st.Vid(),
		uxev.NewPbGroupUserRemoved(&uxev.EvGroupUserRemoved{
			Uid: cmd.Uid,
			Gid: cmd.Gid,
		}),
	))
}

func tellDeleteUser(st *State, cmd *CmdDeleteUser) ([]events.Event, error) {
	if st.name == "" {
		return nil, &UninitializedError{}
	}

	u := st.usersByUid[cmd.Uid]
	if u == nil {
		return nil, nil // idempotent
	}
	if len(u.gids) > 1 {
		return nil, &PreconditionError{
			Reason: "cannot delete user with secondary groups",
		}
	}

	return wrapEventsNewEventsError(uxev.NewEvents(
		st.Vid(),
		uxev.NewPbUserDeleted(cmd.Uid),
	))
}

func tellDeleteGroup(st *State, cmd *CmdDeleteGroup) ([]events.Event, error) {
	if st.name == "" {
		return nil, &UninitializedError{}
	}

	g := st.groupsByGid[cmd.Gid]
	if g == nil {
		return nil, nil // idempotent
	}
	if len(g.uids) > 0 {
		return nil, &PreconditionError{
			Reason: "cannot delete non-empty group",
		}
	}

	return wrapEventsNewEventsError(uxev.NewEvents(
		st.Vid(),
		uxev.NewPbGroupDeleted(cmd.Gid),
	))
}

type UnixDomains struct {
	engine *events.Engine
}

func New(journal *events.Journal) *UnixDomains {
	return &UnixDomains{
		engine: events.NewEngine(journal, Behavior{}),
	}
}

func (r *UnixDomains) FindId(id uuid.I) (*State, error) {
	st, err := r.engine.FindId(id)
	if err != nil {
		return nil, &JournalError{Err: err}
	}
	if st.Vid() == events.EventEpoch {
		return nil, &UninitializedError{}
	}
	return st.(*State), nil
}

func (r *UnixDomains) Init(id uuid.I, cmd *CmdInit) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, NoVC, cmd))
}

func (r *UnixDomains) CreateGroup(
	id uuid.I, vid ulid.I, group string, gid uint32,
) (ulid.I, error) {
	cmd := &CmdCreateGroup{
		Group: group,
		Gid:   gid,
	}
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *UnixDomains) DeleteGroup(
	id uuid.I, vid ulid.I, gid uint32,
) (ulid.I, error) {
	cmd := &CmdDeleteGroup{Gid: gid}
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *UnixDomains) CreateUser(
	id uuid.I, vid ulid.I, cmd *CmdCreateUser,
) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *UnixDomains) DeleteUser(
	id uuid.I, vid ulid.I, uid uint32,
) (ulid.I, error) {
	cmd := &CmdDeleteUser{Uid: uid}
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *UnixDomains) AddGroupUser(
	id uuid.I, vid ulid.I, cmd *CmdAddGroupUser,
) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *UnixDomains) RemoveGroupUser(
	id uuid.I, vid ulid.I, cmd *CmdRemoveGroupUser,
) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (st *State) Name() string {
	return st.name
}

type User struct {
	User string
	Uid  uint32
	Gid  uint32
	Gids []uint32
}

func (st *State) Users() []User {
	us := make([]User, 0, len(st.usersByUid))
	for _, u := range st.usersByUid {
		us = append(us, User{
			User: u.user,
			Uid:  u.uid,
			Gid:  u.gids[0],
			Gids: u.gids,
		})
	}
	sort.Slice(us, func(i, j int) bool {
		return us[i].Uid < us[j].Uid
	})
	return us
}

type Group struct {
	Group string
	Gid   uint32
	Uids  []uint32
}

func (st *State) Groups() []Group {
	gs := make([]Group, 0, len(st.groupsByGid))
	for _, g := range st.groupsByGid {
		gs = append(gs, Group{
			Group: g.group,
			Gid:   g.gid,
			Uids:  g.uids,
		})
	}
	sort.Slice(gs, func(i, j int) bool {
		return gs[i].Gid < gs[j].Gid
	})
	return gs
}

func (st *State) FindUser(name string) (User, bool) {
	u, ok := st.usersByName[name]
	if !ok {
		return User{}, false
	}
	return User{
		User: u.user,
		Uid:  u.uid,
		Gid:  u.gids[0],
		Gids: u.gids,
	}, true
}

func (st *State) FindGid(gid uint32) (Group, bool) {
	g, ok := st.groupsByGid[gid]
	if !ok {
		return Group{}, false
	}
	return Group{
		Group: g.group,
		Gid:   g.gid,
		Uids:  g.uids,
	}, true
}
