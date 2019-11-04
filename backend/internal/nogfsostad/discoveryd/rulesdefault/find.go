package rulesdefault

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nogproject/nog/backend/internal/nogfsostad/discoveryd/rules"
)

type DirectSubdirFinder struct{}

func (f *DirectSubdirFinder) Find(
	root string, known map[string]bool, fns rules.FindHandlerFuncs,
) error {
	root = ensureTrailingSlash(root)

	nilFn := func(string) error { return nil }
	if fns.CandidateFn == nil {
		fns.CandidateFn = nilFn
	}
	if fns.IgnoreFn == nil {
		fns.IgnoreFn = nilFn
	}
	if fns.KnownFn == nil {
		fns.KnownFn = nilFn
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		path = strings.TrimPrefix(path, root)
		if path == "" {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if known[path] {
			if err := fns.KnownFn(path); err != nil {
				return err
			}
			return filepath.SkipDir
		}

		// Ignore hidden by Unix convention.
		if path[0] == '.' {
			if err := fns.IgnoreFn(path); err != nil {
				return err
			}
			return filepath.SkipDir
		}

		if err := fns.CandidateFn(path); err != nil {
			return err
		}
		return filepath.SkipDir
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
