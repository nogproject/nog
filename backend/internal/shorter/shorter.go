// vim: sw=8

// Package `shorter` is a name shorting service backed by MongoDB.
package shorter

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	mgo "gopkg.in/mgo.v2"
	bson "gopkg.in/mgo.v2/bson"
)

const (
	KeyId  = "_id"
	KeyUri = "uri"

	MinAbbrvLen = 11
)

// `Names` maintains a mapping between long names and short ids in a Mongo
// collection.
//
// `Shorten()` returns a known id or creates a new `<IdPrefix>xX...` id based
// on a hash of `<UriPrefix>:<namespace>:<name>`.  Each shortener can be
// assigned a different uppercase `IdPrefix` to ensure that ids are globally
// unique.  `x` is always a lowercase alphanumeric character to separate the
// prefix from the hash.  The rest `X...` of the hash can contain uppercase or
// lowercase alphanumeric characters.  The `UriPrefix` is usually left at its
// default value `ngn`, for NoG Name.
//
// `Find()` works like `Shorten()` but returns an error if the name does not
// yet exist.
//
// The shortener uses approximately `MinAbbrvLen*6 - 1 = 65` bits of a secure
// hash for `xX`, so that ids are likely to be unique even without the prefix.
// A name shortener creates longer ids if it detects name collisions.
type Names struct {
	idPrefix  string
	uriPrefix string
	names     *mgo.Collection

	lock  sync.Mutex // Protects `cache`.
	cache map[string]string
}

type NamesConfig struct {
	Collection string
	IdPrefix   string
	UriPrefix  string
}

func NewNames(conn *mgo.Session, cfg *NamesConfig) (*Names, error) {
	names := conn.DB("").C(cfg.Collection)
	if err := names.EnsureIndex(mgo.Index{
		Key:    []string{KeyUri},
		Unique: true,
	}); err != nil {
		err = fmt.Errorf("failed to create index: %s", err)
		return nil, err
	}

	ipfx := cfg.IdPrefix
	if ipfx == "" {
		ipfx = "T"
	}

	upfx := cfg.UriPrefix
	if upfx == "" {
		// `ngn` for NoG Name.
		upfx = "ngn:"
	}
	if upfx[len(upfx)-1] != ':' {
		upfx += ":"
	}

	return &Names{
		idPrefix:  ipfx,
		uriPrefix: upfx,
		names:     names,
		cache:     make(map[string]string),
	}, nil
}

type doc struct {
	Id  string `bson:"_id"`
	Uri string `bson:"uri"`
}

func (ns *Names) Shorten(namespace, name string) (string, error) {
	createYes := true
	return ns.shorten(namespace, name, createYes)
}

func (ns *Names) Find(namespace, name string) (string, error) {
	createNo := false
	return ns.shorten(namespace, name, createNo)
}

func (ns *Names) shorten(
	namespace, name string, create bool,
) (string, error) {
	var id string
	uri := fmt.Sprintf("%s%s:%s", ns.uriPrefix, namespace, name)
	var code string
	var abbrv int

	ns.lock.Lock()
	id = ns.cache[uri]
	ns.lock.Unlock()
	if id != "" {
		return id, nil
	}

Loop:
	for {
		var d doc
		err := ns.names.Find(bson.M{
			KeyUri: uri,
		}).Select(bson.M{
			KeyId: 1,
		}).One(&d)
		switch err {
		case nil:
			id = d.Id
			break Loop
		case mgo.ErrNotFound:
			if !create {
				return "", nil
			}
			// Ignore and try to insert.
		default:
			return "", err
		}

		if code == "" {
			sum := sha256.Sum256([]byte(uri))
			enc := base64.URLEncoding.WithPadding(base64.NoPadding)
			code = enc.EncodeToString(sum[:])
			// Avoid potentially confusing `-` and `_`.
			code = strings.Replace(code, "-", "y", -1)
			code = strings.Replace(code, "_", "z", -1)
			// Always start lowercase.
			code = strings.ToLower(code[0:1]) + code[1:]
			// Prepend prefix.
			code = ns.idPrefix + code

			// Approximately `MinAbbrvLen * 6 - 1` bits; not
			// exactly due to remapping of potentially confusing
			// letters.
			abbrv = len(ns.idPrefix) + MinAbbrvLen
		}

		// A collision is practically impossible.  We test it anyway.
		if abbrv > len(code) {
			err := fmt.Errorf("hash collision")
			return "", err
		}

		err = ns.names.Insert(&doc{
			Id:  code[0:abbrv],
			Uri: uri,
		})
		if err != nil && !mgo.IsDup(err) {
			return "", err
		}
		// If err == nil, loop once more to `Find()` the key, although
		// the loop could break here with the inserted id.  But
		// inserting a new key is relatively expensive anyway.  An
		// additional `Find()` seems acceptable.

		abbrv++
	}

	ns.lock.Lock()
	ns.cache[uri] = id
	ns.lock.Unlock()

	return id, nil
}

// Deprecated: Don't use `ListShortAll()` to find names.  Instead use a higher
// service that has the list.
func (ns *Names) ListShortAll(namespace string) ([]string, error) {
	uriPrefix := fmt.Sprintf("^%s%s:", ns.uriPrefix, namespace)
	iter := ns.names.Find(bson.M{
		KeyUri: bson.M{"$regex": uriPrefix},
	}).Select(bson.M{
		KeyId: 1,
	}).Iter()

	ids := make([]string, 0)
	var d doc
	for iter.Next(&d) {
		ids = append(ids, d.Id)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}

	return ids, nil
}
