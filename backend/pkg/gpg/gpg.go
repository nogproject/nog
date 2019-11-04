package gpg

import (
	"bytes"
	"encoding/hex"
	"errors"
	"sort"
)

type Fingerprint [20]byte
type Fingerprints []Fingerprint

var ErrMalformedFingerprint = errors.New("malformed GPG key fingerprint")

func ParseFingerprintHex(s string) (p Fingerprint, err error) {
	if hex.DecodedLen(len(s)) != len(p) {
		return p, ErrMalformedFingerprint
	}
	n, err := hex.Decode(p[:], []byte(s))
	if err != nil {
		return p, err
	}
	if n != len(p) {
		return p, ErrMalformedFingerprint
	}
	return p, nil
}

func ParseFingerprintsHex(ss ...string) (Fingerprints, error) {
	if len(ss) == 0 {
		return nil, nil
	}
	ps := make(Fingerprints, 0, len(ss))
	for _, s := range ss {
		p, err := ParseFingerprintHex(s)
		if err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	ps.Sort()
	return ps, nil
}

func ParseFingerprintBytes(b []byte) (p Fingerprint, err error) {
	if len(p) != len(p) {
		return p, ErrMalformedFingerprint
	}
	copy(p[:], b)
	return p, nil
}

func ParseFingerprintsBytes(bs ...[]byte) (Fingerprints, error) {
	if len(bs) == 0 {
		return nil, nil
	}
	ps := make(Fingerprints, 0, len(bs))
	for _, b := range bs {
		p, err := ParseFingerprintBytes(b)
		if err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	ps.Sort()
	return ps, nil
}

func (fs Fingerprints) Sort() {
	sort.Slice(fs, func(i, j int) bool {
		return bytes.Compare(fs[i][:], fs[j][:]) < 0
	})
}

func (fs Fingerprints) Bytes() [][]byte {
	bs := make([][]byte, 0, len(fs))
	for i := range fs {
		bs = append(bs, fs[i][:])
	}
	return bs
}

func (fs Fingerprints) HasDuplicate() bool {
	for i := 0; i < len(fs); i++ {
		for j := i + 1; j < len(fs); j++ {
			if fs[i] == fs[j] {
				return true
			}
		}
	}
	return false
}

func (as Fingerprints) Has(b Fingerprint) bool {
	for _, a := range as {
		if b == a {
			return true
		}
	}
	return false
}

func (as Fingerprints) Equal(bs Fingerprints) bool {
	if len(as) != len(bs) {
		return false
	}
	for _, b := range bs {
		if !as.Has(b) {
			return false
		}
	}
	return true
}
