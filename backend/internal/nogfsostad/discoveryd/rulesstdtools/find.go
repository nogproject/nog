package rulesstdtools

import (
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nogproject/nog/backend/internal/nogfsostad/discoveryd/rules"
	"github.com/nogproject/nog/backend/pkg/regexpx"
)

// `Finder2017` implements the Stdrepo naming rules up to 2017, including
// timeless repos, but excluding the non-standard locations from
// `cfg_projectPathMap`.  See `vissys_stdtools_2017` for details, in
// particular:
//
// ```
// $ grep rePath.*= -- lib/_toolslib.sh
// rePath2013='^/<ProjectsRoot>/[^/]+/[0-9]{4}/[0-9]{4}(-[0-9]{2})?_[^/_]+$'
// rePath2014='^/<ProjectsRoot>/[^/]+/[0-9]{4}/([0-9]{2}_)?[^/_]+$'
// rePath2016='^/<ProjectsRoot>/[^/]+/[0-9]{4}/[^/_]+(_[0-9]{2})?$'
// rePathTimeless='^/<ProjectsRoot>/[^/]+/[a-zA-Z][a-zA-Z0-9-]*$'
//
// $ grep -A 3 ^cfg_projectPathMap -- lib/_toolslib.sh
// ...
// ```
//
// As a special case, `Finder2017` also returns the toplevel project directory
// itself.  We always want a toplevel super-repo to ensure that all files are
// backed up somewhere, even if the repos at deeper levels do not fully cover
// the project.  The init policy `bundle-subdirs` is usually used to track only
// subdir summaries and thus keep the number of paths that are tracked in the
// toplevel repo reasonable.
type Finder2017 struct {
	// `ProjectsRoot` is the toplevel stdrepo project dir, with trailing
	// slash.  Candidate paths must be below.
	ProjectsRoot string
	rootDepth    int

	IgnorePatterns []string

	rgxYearDir      *regexp.Regexp
	rgx2013Repo     *regexp.Regexp
	rgx2014To15Repo *regexp.Regexp
	rgx2016To19Repo *regexp.Regexp
	rgxTimelessRepo *regexp.Regexp
}

// The regular expressions should be reasonably robust:
//
//  - match without trailing slash.
//  - use specific `[a-z...]` patterns for repo shortnames.
//  - use specific year and month patterns.
//

// `patMonth` is the month `01..12` pattern; use a non-capturing group to avoid
// potential confusion.
const patMonth = "(?:0[1-9]|1[0-2])"

// `patShortname` is the shortname pattern.
const patShortname = "[a-zA-Z][a-zA-Z0-9-]*"

func NewFinder(
	projectsRoot string,
	ignorePatterns []string,
) (*Finder2017, error) {
	projectsRoot = ensureTrailingSlash(projectsRoot)
	f := &Finder2017{
		ProjectsRoot:   projectsRoot,
		rootDepth:      dirDepth(projectsRoot),
		IgnorePatterns: ignorePatterns,
	}

	patProjects := `^` + regexp.QuoteMeta(projectsRoot)

	f.rgxYearDir = regexp.MustCompile(patProjects + regexpx.Verbose(`
		[^/]+ /
		201[3-9]
		$
	`))

	f.rgx2013Repo = regexp.MustCompile(patProjects + regexpx.Verbose(`
		[^/]+ /
		2013/
		2013 (-`+patMonth+`)? _`+patShortname+`
		$
	`))

	f.rgx2014To15Repo = regexp.MustCompile(patProjects + regexpx.Verbose(`
		[^/]+ /
		201[4-5]/
		(`+patMonth+`_)? `+patShortname+`
		$
	`))

	f.rgx2016To19Repo = regexp.MustCompile(patProjects + regexpx.Verbose(`
		[^/]+ /
		201[6-9]/
		`+patShortname+`(_`+patMonth+`)?
		$
	`))

	f.rgxTimelessRepo = regexp.MustCompile(patProjects + regexpx.Verbose(`
		[^/]+ /
		`+patShortname+`
		$
	`))

	return f, nil
}

func (f *Finder2017) isYearDir(path string) bool {
	return f.rgxYearDir.MatchString(path)
}

func (f *Finder2017) isTimefulRepo(path string) bool {
	return f.rgx2016To19Repo.MatchString(path) ||
		f.rgx2014To15Repo.MatchString(path) ||
		f.rgx2013Repo.MatchString(path)
}

func (f *Finder2017) isTimelessRepo(path string) bool {
	return f.rgxTimelessRepo.MatchString(path)
}

func (f *Finder2017) Find(
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

	root = ensureTrailingSlash(root)
	if !strings.HasPrefix(root, f.ProjectsRoot) {
		return handleIgnore(".")
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil // Continue.
		}

		relpath := strings.TrimPrefix(path, root)
		if relpath == "" {
			relpath = "."
		}

		if f.ignorePath(relpath) {
			return handleIgnore(relpath)
		}

		switch dirDepth(path) {
		case f.rootDepth: // `.../projects`.
			return nil // Enter.

		case f.rootDepth + 1: // `.../projects/<project>`.
			// Return the toplevel itself as a special case
			// candidate, see comment at `Finder2017 struct`, but
			// also enter to find the stdrepos.
			if known[relpath] {
				return handleKnownEnter(relpath)
			}
			return handleCandidateEnter(relpath)

		case f.rootDepth + 2: // `.../projects/<project>/<sub>`.
			if f.isYearDir(path) {
				return nil // Enter.
			}
			if known[relpath] {
				return handleKnown(relpath)
			}
			if f.isTimelessRepo(path) {
				return handleCandidate(relpath)
			}

		case f.rootDepth + 3: // `.../projects/<project>/<year>/<sub>`.
			if known[relpath] {
				return handleKnown(relpath)
			}
			if f.isTimefulRepo(path) {
				return handleCandidate(relpath)
			}

		default:
			err := fmt.Errorf(
				"logic error: must not reach dir `%s`", path,
			)
			panic(err)
		}

		return handleIgnore(relpath)
	}

	return filepath.Walk(root, walkFn)
}

func (f *Finder2017) ignorePath(path string) bool {
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

func ensureTrailingSlash(s string) string {
	if s == "" {
		return "/"
	}
	if s[len(s)-1] == '/' {
		return s
	}
	return s + "/"
}

// `dirDepth(abspath)` return the depth of the directory `abspath`:
//
// ```
// dirDepth("/") == 0
// dirDepth("/exorg") == 1
// dirDepth("/exorg/") == 1
// dirDepth("/exorg/data") == 2
// ...
// ```
//
func dirDepth(path string) int {
	n := strings.Count(path, "/")
	if n == 0 {
		panic("invalid abspath")
	}
	if path[len(path)-1] == '/' { // ends with slash.
		n -= 1
	}
	return n
}
