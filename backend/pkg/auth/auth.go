package auth

import (
	"context"
	"fmt"
)

type Identity map[string]interface{}

type UnixIdentity struct {
	Domain     string
	Username   string
	Groupnames []string
}

type UnixIdentities []UnixIdentity

func (ids UnixIdentities) FindDomain(dom string) (UnixIdentity, bool) {
	for _, id := range ids {
		if id.Domain == dom {
			return id, true
		}
	}
	return UnixIdentity{}, false
}

type Action string

func (a Action) String() string {
	return string(a)
}

type ActionDetails map[string]interface{}

type ScopedAction struct {
	Action
	Details ActionDetails
}

func (sa ScopedAction) String() string {
	return fmt.Sprintf("{%s %s}", sa.Action, sa.Details)
}

type SimpleScope struct {
	Action string `json:"action"`
	Path   string `json:"path,omitempty"`
	Name   string `json:"name,omitempty"`
}

type Scope struct {
	Actions []string
	Paths   []string
	Names   []string
}

type Authenticator interface {
	Authenticate(context.Context) (Identity, error)
}

type Authorizer interface {
	Authorize(Identity, Action, ActionDetails) error
}

type AnyAuthorizer interface {
	AuthorizeAny(Identity, ...ScopedAction) error
}
