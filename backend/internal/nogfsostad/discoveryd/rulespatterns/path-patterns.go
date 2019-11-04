package rulespatterns

import (
	"errors"
	"os"
	slashpath "path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nogproject/nog/backend/internal/nogfsostad/discoveryd/rules"
)

var ErrMalformedPattern = errors.New("malformed pattern")
var ErrInvalidAction = errors.New("invalid action")
var ErrMalformedDepthPath = errors.New("malformed depth path")
var ErrTooDeep = errors.New("too many subdirs in depth path")

type Config struct {
	Patterns     []string
	EnabledPaths []string
}

type action int

const (
	actEnter action = 1 + iota
	actRepo
	actSuperRepo
	actIgnore
)

type pattern struct {
	action action
	glob   string
}

// `rulespatterns.Finder` applies a list of patterns to identify repo
// candidates.  Create with `NewFinder()`.  Example `Patterns`:
//
//      superrepo .
//      superrepo foo
//           repo foo/overview
//          enter foo/data
//           repo foo/data/*
//         ignore foo/*
//         ignore *
//
// `EnabledPaths` is a list of `<depth> <path>` pairs that are prepended before
// the patterns as follows:
//
//      0 foo  ->  "superrepo foo"
//      1 foo  ->  "superrepo foo", "superrepo foo/*"
//      2 foo  ->  "superrepo foo", "superrepo foo/*", "superrepo foo/*/*"
//
type Finder struct {
	patterns []pattern
}

func NewFinder(cfg Config) (*Finder, error) {
	patterns := make([]pattern, 0)

	for _, depthPath := range cfg.EnabledPaths {
		toks := strings.SplitN(depthPath, " ", 2)
		if len(toks) != 2 {
			return nil, ErrMalformedDepthPath
		}
		depth, err := strconv.ParseUint(toks[0], 10, 32)
		if err != nil {
			return nil, ErrMalformedDepthPath
		}
		// The limit here must be greater or equal to the limit in
		// `fsorepos/pbevents/pbevents.go`.
		if depth > 3 {
			return nil, ErrTooDeep
		}

		glob := escapeGlob(toks[1])
		for i := 0; i <= int(depth); i++ {
			patterns = append(patterns, pattern{
				action: actSuperRepo,
				glob:   glob,
			})
			glob += "/*"
		}
	}

	for i, pat := range cfg.Patterns {
		toks := strings.SplitN(pat, " ", 2)
		if len(toks) != 2 {
			return nil, ErrMalformedPattern
		}
		actString := toks[0]
		glob := toks[1]

		// Unless the first pattern explicitly handles the root, add a
		// rule to enter it.
		if i == 0 && glob != "." {
			patterns = append(patterns, pattern{
				action: actEnter,
				glob:   ".",
			})
		}

		var a action
		switch actString {
		case "enter":
			a = actEnter
		case "repo":
			a = actRepo
		case "superrepo":
			a = actSuperRepo
		case "ignore":
			a = actIgnore
		default:
			return nil, ErrInvalidAction
		}
		patterns = append(patterns, pattern{
			action: a,
			glob:   glob,
		})
	}

	return &Finder{patterns}, nil
}

// The special glob chars that need be escaped are `*?[\`, see
// <https://godoc.org/path#Match>.
var rgxGlobSpecial = regexp.MustCompile(`[*?\[\\]`)

func escapeGlob(path string) string {
	return rgxGlobSpecial.ReplaceAllString(path, `\$0`)
}

func (f *Finder) Find(
	root string, known map[string]bool, fns rules.FindHandlerFuncs,
) error {
	handleCandidate := func(p string) error {
		if fns.CandidateFn != nil {
			if err := fns.CandidateFn(p); err != nil {
				return err
			}
		}
		return filepath.SkipDir
	}

	handleCandidateEnter := func(p string) error {
		if fns.CandidateFn != nil {
			if err := fns.CandidateFn(p); err != nil {
				return err
			}
		}
		return nil
	}

	handleIgnore := func(p string) error {
		if fns.IgnoreFn != nil {
			if err := fns.IgnoreFn(p); err != nil {
				return err
			}
		}
		return filepath.SkipDir
	}

	handleKnown := func(p string) error {
		if fns.KnownFn != nil {
			if err := fns.KnownFn(p); err != nil {
				return err
			}
		}
		return filepath.SkipDir
	}

	handleKnownEnter := func(p string) error {
		if fns.KnownFn != nil {
			if err := fns.KnownFn(p); err != nil {
				return err
			}
		}
		return nil
	}

	handleAction := func(act action, relpath string) error {
		switch act {
		case actEnter:
			return nil
		case actRepo:
			if known[relpath] {
				return handleKnown(relpath)
			}
			return handleCandidate(relpath)
		case actSuperRepo:
			if known[relpath] {
				return handleKnownEnter(relpath)
			}
			return handleCandidateEnter(relpath)
		case actIgnore:
			if known[relpath] {
				return handleKnown(relpath)
			}
			return handleIgnore(relpath)
		default:
			panic("invalid action")
		}
	}

	root = ensureTrailingSlash(root)
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}

		relpath := strings.TrimPrefix(path, root)
		if relpath == "" {
			relpath = "."
		}

		// Ignore hidden by Unix convention.
		if relpath != "." {
			_, basename := filepath.Split(relpath)
			if basename[0] == '.' {
				return handleIgnore(relpath)
			}
		}

		for _, pat := range f.patterns {
			matched, err := slashpath.Match(pat.glob, relpath)
			if err != nil {
				continue // Silently ignore invalid patterns.
			}
			if matched {
				return handleAction(pat.action, relpath)
			}
		}

		// The default is to ignore.
		return handleIgnore(relpath)
	}

	return filepath.Walk(root, walkFn)
}

func ensureTrailingSlash(s string) string {
	if s == "" {
		return "/"
	}
	if s[len(s)-1] == '/' {
		return s
	}
	return s + "/"
}
