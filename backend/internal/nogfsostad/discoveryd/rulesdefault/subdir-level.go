package rulesdefault

import (
	"os"
	slashpath "path"
	"path/filepath"
	"strings"

	"github.com/nogproject/nog/backend/internal/nogfsostad/discoveryd/rules"
)

type SubdirLevelFinder struct {
	Level          int
	IgnorePatterns []string
}

func (f *SubdirLevelFinder) Find(
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

	root = ensureTrailingSlash(root)
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relpath := strings.TrimPrefix(path, root)
		if !info.IsDir() {
			return nil
		}
		if relpath == "" {
			return nil
		}
		if known[relpath] {
			return handleKnown(relpath)
		}

		// Ignore hidden by Unix convention.
		_, basename := filepath.Split(relpath)
		if basename[0] == '.' {
			return handleIgnore(relpath)
		}

		if f.ignorePath(relpath) {
			return handleIgnore(relpath)
		}

		if reldirDepth(relpath) == f.Level {
			return handleCandidate(relpath)
		}

		return nil
	}

	return filepath.Walk(root, walkFn)
}

func (f *SubdirLevelFinder) ignorePath(path string) bool {
	for _, pat := range f.IgnorePatterns {
		matched, err := slashpath.Match(pat, path)
		if err != nil {
			continue // Silently ignore invalid patterns.
		}
		if matched {
			return true
		}
	}
	return false
}

// `reldirDepth(relpath)` return the depth of the directory:
//
// ```
// reldirDepth("exorg") == 1
// reldirDepth("exorg/") == 1
// reldirDepth("exorg/data") == 2
// ...
// ```
//
func reldirDepth(relpath string) int {
	n := strings.Count(relpath, "/")
	if relpath[len(relpath)-1] == '/' { // ends with slash.
		n -= 1
	}
	return n + 1
}
