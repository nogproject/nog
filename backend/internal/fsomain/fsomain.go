// vim: sw=8

/*

Package `fsomain` implements an event-sourced aggregate that contains the FSO
filesystem observer main root entity.  The main root is accessed via a
wellknown name, usually `main`.  To get to a repo, traverse entities:

    fsomain -> fsoregistry -> fsorepos

The main root contains a list of registries.  Registry details are in separate
entities, implemented by package `fsoregistry`.  Registries contain lists of
repos.  Repo details are in separate entities, implemented by package
`fsorepos`.

*/
package fsomain

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/golang/protobuf/proto"
	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/fsomainpb"
	"github.com/nogproject/nog/backend/pkg/regexpx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

var ErrCommandUnknown = errors.New("unknown command")
var ErrUninitialized = errors.New("uninitialized")
var ErrDomainNameInUse = errors.New("domain name already in used")
var ErrMalformedDomainName = errors.New("malformed unix domain name")

// `NoVC` is a sentinel value that can be passed in place of `vid` to indicate
// that concurrency version checks are skipped.
var NoVC = events.NoVC

type State struct {
	id  uuid.I
	vid ulid.I

	name          string
	registries    []RegistryInfo
	domainsByName map[string]UnixDomainInfo
}

type RegistryInfo struct {
	Name      string
	Confirmed bool
}

type UnixDomainInfo struct {
	Id   uuid.I
	Name string
}

type Event struct {
	id     ulid.I
	parent ulid.I
	pb     pb.Event
}

type CmdInitMain struct {
	Name string
}

type CmdInitRegistry struct {
	Name string
}

type CmdConfirmRegistry struct {
	Name string
}

type CmdAddUnixDomain struct {
	DomainId   uuid.I
	DomainName string
}

func (*State) AggregateState()                {}
func (*CmdInitMain) AggregateCommand()        {}
func (*CmdInitRegistry) AggregateCommand()    {}
func (*CmdConfirmRegistry) AggregateCommand() {}
func (*CmdAddUnixDomain) AggregateCommand()   {}

func (s *State) Id() uuid.I        { return s.id }
func (s *State) Vid() ulid.I       { return s.vid }
func (s *State) SetVid(vid ulid.I) { s.vid = vid }

// `newEvents()` could be extended to build a parent chain of multiple events.
func newEvents(parent ulid.I, pb pb.Event) ([]events.Event, error) {
	id, err := ulid.New()
	if err != nil {
		return nil, err
	}
	e := &Event{id: id, parent: parent, pb: pb}
	e.pb.Id = e.id[:]
	e.pb.Parent = e.parent[:]
	return []events.Event{e}, nil
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

func (e *Event) PbMainEvent() *pb.Event { return &e.pb }

type Behavior struct{}

func (Behavior) NewState(id uuid.I) events.State { return &State{id: id} }
func (Behavior) NewEvent() events.Event          { return &Event{} }
func (Behavior) NewAdvancer() events.Advancer    { return &Advancer{} }

type Advancer struct {
	main    bool
	domains bool
}

func (a *Advancer) Advance(s events.State, ev events.Event) events.State {
	evpb := ev.(*Event).pb
	st := s.(*State)

	if !a.main {
		dup := *st
		st = &dup
		a.main = true
	}

	detachDomains := func() {
		if a.domains {
			return
		}
		st.domainsByName = dupDomainsByName(st.domainsByName)
		a.domains = true
	}

	switch evpb.Event {
	case pb.Event_EV_FSO_MAIN_INITIALIZED:
		st.name = evpb.FsoMainName

	case pb.Event_EV_FSO_REGISTRY_ACCEPTED:
		st.registries = append(st.registries, RegistryInfo{
			Name: evpb.FsoRegistryName,
		})

	case pb.Event_EV_FSO_REGISTRY_CONFIRMED:
		name := evpb.FsoRegistryName
		for i, e := range st.registries {
			if e.Name == name {
				regs := st.registries[0:i]
				// Don't dup `e`.  It is a value.
				e.Confirmed = true
				regs = append(regs, e)
				regs = append(regs, st.registries[i+1:]...)
				st.registries = regs
				break
			}
		}

	case pb.Event_EV_UNIX_DOMAIN_ADDED:
		detachDomains()
		id, err := uuid.FromBytes(evpb.UnixDomainId)
		if err != nil {
			panic("malformed event")
		}
		name := evpb.UnixDomainName
		st.domainsByName[name] = UnixDomainInfo{
			Id:   id,
			Name: name,
		}

	default:
		panic("invalid event")
	}

	return st
}

func dupDomainsByName(
	src map[string]UnixDomainInfo,
) map[string]UnixDomainInfo {
	dup := make(map[string]UnixDomainInfo)
	for k, v := range src {
		dup[k] = v
	}
	return dup
}

func (Behavior) Tell(
	s events.State, c events.Command,
) ([]events.Event, error) {
	state := s.(*State)
	switch cmd := c.(type) {
	case *CmdInitMain:
		return tellInitMain(state, cmd)
	case *CmdInitRegistry:
		return tellInitRegistry(state, cmd)
	case *CmdConfirmRegistry:
		return tellConfirmRegistry(state, cmd)
	case *CmdAddUnixDomain:
		return tellAddUnixDomain(state, cmd)
	default:
		return nil, ErrCommandUnknown
	}
}

func tellInitMain(
	state *State, cmd *CmdInitMain,
) ([]events.Event, error) {
	// If already initialized, check that idempotent cmd.
	if state.name != "" {
		if cmd.Name != state.name {
			err := fmt.Errorf("init conflict")
			return nil, err
		}
		return nil, nil
	}

	return newEvents(state.Vid(), pb.Event{
		Event:       pb.Event_EV_FSO_MAIN_INITIALIZED,
		FsoMainName: cmd.Name,
	})
}

func tellInitRegistry(
	state *State, cmd *CmdInitRegistry,
) ([]events.Event, error) {
	if state.name == "" {
		err := fmt.Errorf("uninitialized")
		return nil, err
	}

	for _, e := range state.registries {
		if e.Name == cmd.Name {
			// Already initialized.
			return nil, nil
		}
	}

	return newEvents(state.Vid(), pb.Event{
		Event:           pb.Event_EV_FSO_REGISTRY_ACCEPTED,
		FsoRegistryName: cmd.Name,
	})
}

func tellConfirmRegistry(
	state *State, cmd *CmdConfirmRegistry,
) ([]events.Event, error) {
	if state.name == "" {
		err := fmt.Errorf("uninitialized")
		return nil, err
	}

	inf := state.findRegistryInfo(cmd.Name)
	if inf == nil {
		err := fmt.Errorf("unknown registry")
		return nil, err
	}
	if inf.Confirmed {
		// Already confirmed.
		return nil, nil
	}

	return newEvents(state.Vid(), pb.Event{
		Event:           pb.Event_EV_FSO_REGISTRY_CONFIRMED,
		FsoRegistryName: cmd.Name,
	})
}

func (s *State) findRegistryInfo(name string) *RegistryInfo {
	for _, e := range s.registries {
		if e.Name == name {
			return &e
		}
	}
	return nil
}

var rgxUnixDomainName = regexp.MustCompile(regexpx.Verbose(`
	^ [a-zA-Z0-9_-]+ $
`))

func tellAddUnixDomain(
	st *State, cmd *CmdAddUnixDomain,
) ([]events.Event, error) {
	if st.name == "" {
		return nil, ErrUninitialized
	}

	if st.FindUnixDomainName(cmd.DomainName) != nil {
		return nil, ErrDomainNameInUse
	}

	if !rgxUnixDomainName.MatchString(cmd.DomainName) {
		return nil, ErrMalformedDomainName
	}

	return newEvents(st.Vid(), pb.Event{
		Event:          pb.Event_EV_UNIX_DOMAIN_ADDED,
		UnixDomainId:   cmd.DomainId[:],
		UnixDomainName: cmd.DomainName,
	})
}

type Main struct {
	engine *events.Engine
}

func New(mainJ *events.Journal) *Main {
	eng := events.NewEngine(mainJ, Behavior{})
	return &Main{engine: eng}
}

func (r *Main) Init(
	id uuid.I, name string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, NoVC, &CmdInitMain{Name: name})
}

func (r *Main) InitRegistry(
	id uuid.I, vid ulid.I, name string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdInitRegistry{Name: name})
}

func (r *Main) ConfirmRegistry(
	id uuid.I, vid ulid.I, name string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdConfirmRegistry{Name: name})
}

func (r *Main) AddUnixDomain(
	id uuid.I, vid ulid.I, domainId uuid.I, domainName string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdAddUnixDomain{
		DomainId:   domainId,
		DomainName: domainName,
	})
}

func (r *Main) FindId(id uuid.I) (*State, error) {
	s, err := r.engine.FindId(id)
	if err != nil {
		return nil, err
	}
	return s.(*State), nil
}

func (s *State) NumRegistries() int { return len(s.registries) }

func (s *State) Registries() []RegistryInfo {
	return s.registries
}

func (st *State) FindUnixDomainName(name string) *UnixDomainInfo {
	inf, ok := st.domainsByName[name]
	if ok {
		return &inf
	} else {
		return nil
	}
}
