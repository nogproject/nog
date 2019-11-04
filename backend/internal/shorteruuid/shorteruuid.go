// vim: sw=8

// Package `shorteruuid` is a name shorting service using RFC 4122 version 5
// SHA-1-based UUIDs.
package shorteruuid

import (
	"encoding/base64"
	"fmt"

	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `nsNogName` is the UUID that identifies the Nog names SHA-1-UUID namespace.
var nsNogName = uuid.Must(uuid.Parse("ecb02ec6-006d-429f-a378-9392612f9c61"))

type Names struct {
	ns uuid.I
}

type NamesConfig struct {
	Namespace uuid.I
}

func NewNames(cfg *NamesConfig) *Names {
	var ns uuid.I
	if cfg != nil {
		ns = cfg.Namespace
	}
	if ns == uuid.Nil {
		ns = nsNogName
	}
	return &Names{ns: ns}
}

func NewNogNames() *Names {
	return &Names{ns: nsNogName}
}

func (ns *Names) UUID(namespace, name string) uuid.I {
	data := []byte(fmt.Sprintf("%s:%s", namespace, name))
	return uuid.NewSHA1(ns.ns, data)
}

// DEPRECATED.  Only for compatibility with package `shorter`.  Use `UUID()`
// instead.
func (ns *Names) Shorten(namespace, name string) string {
	id := ns.UUID(namespace, name)
	return base64.RawURLEncoding.EncodeToString(id[:])
}
