package main

import (
	"bufio"
	"errors"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

var ErrMalformedManifest = errors.New("malformed manifest")

type Manifest struct {
	fileSet map[string]struct{}
}

// Some duplication with `backend/internal/nogfsostad/shadows/tartt.go`.  See
// `readManifest()` there for further details.
func readManifest(r io.Reader) (*Manifest, error) {
	// Line format: `<key> <colon> <value> <space> <space> <file>`.
	// Specifically:
	//
	// ```
	// size:<int>  foo.dat
	// sha256:<hex>  foo.dat
	// sha512:<hex>  foo.dat
	// ```
	//
	s := bufio.NewScanner(r)
	s.Split(bufio.ScanLines)

	fileSet := make(map[string]struct{})
	for s.Scan() {
		line := s.Text()

		lineFields := strings.SplitN(line, " ", 3)
		if len(lineFields) != 3 {
			return nil, ErrMalformedManifest
		}
		file := lineFields[2]
		fileSet[file] = struct{}{}
	}

	return &Manifest{fileSet: fileSet}, nil
}

func (mf *Manifest) HasFile(name string) bool {
	_, ok := mf.fileSet[name]
	return ok
}

func (mf *Manifest) Glob(pattern string) ([]string, error) {
	matched := make([]string, 0)
	for name, _ := range mf.fileSet {
		ok, err := filepath.Match(pattern, name)
		if err != nil {
			return nil, err
		}
		if ok {
			matched = append(matched, name)
		}
	}
	sort.Strings(matched)
	return matched, nil
}
