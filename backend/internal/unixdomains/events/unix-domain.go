package events

import (
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/unixdomainspb"
	"github.com/nogproject/nog/backend/pkg/ulid"
)

var ErrUnknownUnixDomainEvent = errors.New("unknown UnixDomainEvent type")

type Event struct {
	id     ulid.I
	parent ulid.I
	pb     pb.UnixDomainEvent
}

func NewEvents(
	parent ulid.I, pbs ...pb.UnixDomainEvent,
) ([]events.Event, error) {
	evs := make([]events.Event, 0, len(pbs))
	for _, pb := range pbs {
		id, err := ulid.New()
		if err != nil {
			return nil, err
		}
		e := &Event{id: id, parent: parent, pb: pb}
		e.pb.Id = e.id[:]
		e.pb.Parent = e.parent[:]
		evs = append(evs, e)
		parent = id
	}
	return evs, nil
}

func (e *Event) MarshalProto() ([]byte, error) {
	return proto.Marshal(&e.pb)
}

func (e *Event) UnmarshalProto(data []byte) error {
	var err error
	if err = proto.Unmarshal(data, &e.pb); err != nil {
		return err
	}
	if e.id, err = ulid.ParseBytes(e.pb.Id); err != nil {
		return err
	}
	if e.parent, err = ulid.ParseBytes(e.pb.Parent); err != nil {
		return err
	}
	if _, err := ParsePbUnixDomainEvent(&e.pb); err != nil {
		err := errors.New("invalid event details")
		return err
	}
	return nil
}

func (e *Event) Id() ulid.I     { return e.id }
func (e *Event) Parent() ulid.I { return e.parent }

// Receiver by value.
func (e Event) WithId(id ulid.I) events.Event {
	e.id = id
	e.pb.Id = e.id[:]
	return &e
}

// Receiver by value.
func (e Event) WithParent(parent ulid.I) events.Event {
	e.parent = parent
	e.pb.Parent = e.parent[:]
	return &e
}

func (e *Event) PbUnixDomainEvent() *pb.UnixDomainEvent {
	return &e.pb
}

type UnixDomainEvent interface {
	UnixDomainEvent()
}

// `ParsePbUnixDomainEvent()` converts a protobuf struct to an `Ev*` struct.
func ParsePbUnixDomainEvent(
	evpb *pb.UnixDomainEvent,
) (ev UnixDomainEvent, err error) {
	switch evpb.Event {
	case pb.UnixDomainEvent_EV_UNIX_DOMAIN_CREATED:
		return fromPbDomainCreated(evpb)
	case pb.UnixDomainEvent_EV_UNIX_GROUP_CREATED:
		return fromPbGroupCreated(evpb)
	case pb.UnixDomainEvent_EV_UNIX_USER_CREATED:
		return fromPbUserCreated(evpb)
	case pb.UnixDomainEvent_EV_UNIX_GROUP_USER_ADDED:
		return fromPbGroupUserAdded(evpb)
	case pb.UnixDomainEvent_EV_UNIX_GROUP_USER_REMOVED:
		return fromPbGroupUserRemoved(evpb)
	case pb.UnixDomainEvent_EV_UNIX_USER_DELETED:
		return fromPbUserDeleted(evpb)
	case pb.UnixDomainEvent_EV_UNIX_GROUP_DELETED:
		return fromPbGroupDeleted(evpb)

	// `UnixDomainEvent_EV_UNIX_DOMAIN_DELETED` not implemented.

	default:
		return nil, ErrUnknownUnixDomainEvent
	}
}

func MustParsePbUnixDomainEvent(evpb *pb.UnixDomainEvent) UnixDomainEvent {
	ev, err := ParsePbUnixDomainEvent(evpb)
	if err != nil {
		panic(err)
	}
	return ev
}

// `UnixDomainEvent_EV_UNIX_DOMAIN_CREATED` aka `EvDomainCreated`.
type EvDomainCreated struct {
	Name string
}

func (*EvDomainCreated) UnixDomainEvent() {}

func NewPbDomainCreated(ev *EvDomainCreated) pb.UnixDomainEvent {
	if ev.Name == "" {
		panic("empty Name")
	}
	evpb := pb.UnixDomainEvent{
		Event:          pb.UnixDomainEvent_EV_UNIX_DOMAIN_CREATED,
		UnixDomainName: ev.Name,
	}
	return evpb
}

func fromPbDomainCreated(evpb *pb.UnixDomainEvent) (UnixDomainEvent, error) {
	if evpb.Event != pb.UnixDomainEvent_EV_UNIX_DOMAIN_CREATED {
		panic("invalid event")
	}
	ev := &EvDomainCreated{
		Name: evpb.UnixDomainName,
	}
	return ev, nil
}

// `UnixDomainEvent_EV_UNIX_GROUP_CREATED` aka `EvGroupCreated`.
type EvGroupCreated struct {
	Group string
	Gid   uint32
}

func (*EvGroupCreated) UnixDomainEvent() {}

func NewPbGroupCreated(group string, gid uint32) pb.UnixDomainEvent {
	if group == "" {
		panic("empty Group")
	}
	evpb := pb.UnixDomainEvent{
		Event:     pb.UnixDomainEvent_EV_UNIX_GROUP_CREATED,
		UnixGroup: group,
		UnixGid:   gid,
	}
	return evpb
}

func fromPbGroupCreated(evpb *pb.UnixDomainEvent) (UnixDomainEvent, error) {
	if evpb.Event != pb.UnixDomainEvent_EV_UNIX_GROUP_CREATED {
		panic("invalid event")
	}
	ev := &EvGroupCreated{
		Group: evpb.UnixGroup,
		Gid:   evpb.UnixGid,
	}
	return ev, nil
}

// `UnixDomainEvent_EV_UNIX_USER_CREATED` aka `EvUserCreated`.
type EvUserCreated struct {
	User string
	Uid  uint32
	Gid  uint32
}

func (*EvUserCreated) UnixDomainEvent() {}

func NewPbUserCreated(ev *EvUserCreated) pb.UnixDomainEvent {
	if ev.User == "" {
		panic("empty User")
	}
	evpb := pb.UnixDomainEvent{
		Event:    pb.UnixDomainEvent_EV_UNIX_USER_CREATED,
		UnixUser: ev.User,
		UnixUid:  ev.Uid,
		UnixGid:  ev.Gid,
	}
	return evpb
}

func fromPbUserCreated(evpb *pb.UnixDomainEvent) (UnixDomainEvent, error) {
	if evpb.Event != pb.UnixDomainEvent_EV_UNIX_USER_CREATED {
		panic("invalid event")
	}
	ev := &EvUserCreated{
		User: evpb.UnixUser,
		Uid:  evpb.UnixUid,
		Gid:  evpb.UnixGid,
	}
	return ev, nil
}

// `UnixDomainEvent_EV_UNIX_GROUP_USER_ADDED` aka `EvGroupUserAdded`.
type EvGroupUserAdded struct {
	Gid uint32
	Uid uint32
}

func (*EvGroupUserAdded) UnixDomainEvent() {}

func NewPbGroupUserAdded(ev *EvGroupUserAdded) pb.UnixDomainEvent {
	evpb := pb.UnixDomainEvent{
		Event:   pb.UnixDomainEvent_EV_UNIX_GROUP_USER_ADDED,
		UnixGid: ev.Gid,
		UnixUid: ev.Uid,
	}
	return evpb
}

func fromPbGroupUserAdded(evpb *pb.UnixDomainEvent) (UnixDomainEvent, error) {
	if evpb.Event != pb.UnixDomainEvent_EV_UNIX_GROUP_USER_ADDED {
		panic("invalid event")
	}
	ev := &EvGroupUserAdded{
		Gid: evpb.UnixGid,
		Uid: evpb.UnixUid,
	}
	return ev, nil
}

// `UnixDomainEvent_EV_UNIX_GROUP_USER_REMOVED` aka `EvGroupUserRemoved`.
type EvGroupUserRemoved struct {
	Gid uint32
	Uid uint32
}

func (*EvGroupUserRemoved) UnixDomainEvent() {}

func NewPbGroupUserRemoved(ev *EvGroupUserRemoved) pb.UnixDomainEvent {
	evpb := pb.UnixDomainEvent{
		Event:   pb.UnixDomainEvent_EV_UNIX_GROUP_USER_REMOVED,
		UnixGid: ev.Gid,
		UnixUid: ev.Uid,
	}
	return evpb
}

func fromPbGroupUserRemoved(evpb *pb.UnixDomainEvent) (UnixDomainEvent, error) {
	if evpb.Event != pb.UnixDomainEvent_EV_UNIX_GROUP_USER_REMOVED {
		panic("invalid event")
	}
	ev := &EvGroupUserRemoved{
		Gid: evpb.UnixGid,
		Uid: evpb.UnixUid,
	}
	return ev, nil
}

// `UnixDomainEvent_EV_UNIX_USER_DELETED` aka `EvUserDeleted`.
type EvUserDeleted struct {
	Uid uint32
}

func (*EvUserDeleted) UnixDomainEvent() {}

func NewPbUserDeleted(uid uint32) pb.UnixDomainEvent {
	evpb := pb.UnixDomainEvent{
		Event:   pb.UnixDomainEvent_EV_UNIX_USER_DELETED,
		UnixUid: uid,
	}
	return evpb
}

func fromPbUserDeleted(evpb *pb.UnixDomainEvent) (UnixDomainEvent, error) {
	if evpb.Event != pb.UnixDomainEvent_EV_UNIX_USER_DELETED {
		panic("invalid event")
	}
	ev := &EvUserDeleted{
		Uid: evpb.UnixUid,
	}
	return ev, nil
}

// `UnixDomainEvent_EV_UNIX_GROUP_DELETED` aka `EvGroupDeleted`.
type EvGroupDeleted struct {
	Gid uint32
}

func (*EvGroupDeleted) UnixDomainEvent() {}

func NewPbGroupDeleted(gid uint32) pb.UnixDomainEvent {
	evpb := pb.UnixDomainEvent{
		Event:   pb.UnixDomainEvent_EV_UNIX_GROUP_DELETED,
		UnixGid: gid,
	}
	return evpb
}

func fromPbGroupDeleted(evpb *pb.UnixDomainEvent) (UnixDomainEvent, error) {
	if evpb.Event != pb.UnixDomainEvent_EV_UNIX_GROUP_DELETED {
		panic("invalid event")
	}
	ev := &EvGroupDeleted{
		Gid: evpb.UnixGid,
	}
	return ev, nil
}
