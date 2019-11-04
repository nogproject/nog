package shadows

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	slashpath "path"
	"runtime"
	"strings"

	git "github.com/libgit2/git2go"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/gitstat"
	yaml "gopkg.in/yaml.v2"
)

type ListStatTreeFunc func(info pb.PathInfo) error

func (fs *Filesystem) ListStatTree(
	ctx context.Context,
	shadowPath string,
	gitCommit []byte,
	prefix string,
	callback ListStatTreeFunc,
) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}

	repo, err := git.OpenRepository(shadowPath)
	if err != nil {
		return err
	}

	coId := git.NewOidFromBytes(gitCommit)
	if coId == nil {
		return errors.New("invalid commit id")
	}

	co, err := repo.LookupCommit(coId)
	if err != nil {
		return err
	}

	tree, err := co.Tree()
	if err != nil {
		return err
	}

	// XXX Not yet implemented: If prefix != "", find subtree and start
	// walk there.
	if prefix != "" {
		return errors.New("non-empty prefix unimplemented")
	}

	var walkErr error
	setWalkErr := func(path string, err error) {
		walkErr = fmt.Errorf("walk error at `%s`: %s", path, err)
	}

	// <https://libgit2.github.com/libgit2/#HEAD/group/tree/git_tree_walk>
	// Return codes:
	const (
		WalkContinue = 0
		WalkSkip     = 1
		WalkStop     = -1
	)
	// `path` is the tree path with trailing slash, empty string for root.
	walkFn := func(path string, ent *git.TreeEntry) int {
		if isIgnoredTreeEntry(ent) {
			return WalkSkip
		}

		fullPath := slashpath.Join(path, ent.Name)

		if ent.Name == ".nogtree" {
			inf := pb.PathInfo{
				Mode: uint32(gitstat.ModeDir),
			}
			if path == "" {
				inf.Path = "."
			} else {
				inf.Path = path[:len(path)-1]
			}

			stat, err := readStatBlob(repo, ent.Id)
			if err != nil {
				setWalkErr(fullPath, err)
				return WalkStop
			}
			inf.Mtime = stat.Mtime
			// The root may have nogbundle stat details.
			if path == "" {
				inf.Size = stat.Size
				inf.Dirs = stat.Dirs
				inf.Files = stat.Files
				inf.Links = stat.Links
				inf.Others = stat.Others
			}

			if err := callback(inf); err != nil {
				setWalkErr(fullPath, err)
				return WalkStop
			}
			return WalkContinue
		}

		if ent.Type == git.ObjectTree {
			// `callback()` is called when visiting the subtree
			// `.nogtree`.
			return WalkContinue
		}

		inf := pb.PathInfo{
			Path: fullPath,
		}
		switch ent.Filemode {
		case git.FilemodeBlobExecutable:
			fallthrough
		case git.FilemodeBlob:
			stat, err := readStatBlob(repo, ent.Id)
			if err != nil {
				setWalkErr(fullPath, err)
				return WalkStop
			}

			// `git-fso` converts gitlink submodule commits to stat
			// blobs with a `submodule` field.  Return them as
			// `ModeGitlink`.  Return nogbundles as `ModeDir`.
			// Both have a total size and counts for dirs, files,
			// links and others; while regular files only have a
			// size.
			if stat.isSubmodule() {
				inf.Mode = uint32(gitstat.ModeGitlink)
				inf.Gitlink = stat.Submodule
				inf.Mtime = stat.Mtime
				inf.Size = stat.Size
				inf.Dirs = stat.Dirs
				inf.Files = stat.Files
				inf.Links = stat.Links
				inf.Others = stat.Others
			} else if stat.isNogbundle() {
				inf.Mode = uint32(gitstat.ModeDir)
				inf.Mtime = stat.Mtime
				inf.Size = stat.Size
				inf.Dirs = stat.Dirs
				inf.Files = stat.Files
				inf.Links = stat.Links
				inf.Others = stat.Others
			} else {
				inf.Mode = uint32(gitstat.ModeRegular)
				inf.Mtime = stat.Mtime
				inf.Size = stat.Size
			}

		case git.FilemodeLink:
			inf.Mode = uint32(gitstat.ModeSymlink)
			blob, err := repo.LookupBlob(ent.Id)
			if err != nil {
				setWalkErr(fullPath, err)
				return WalkStop
			}
			inf.Symlink = string(blob.Contents())

		case git.FilemodeCommit:
			// This should not happend, because `git-fso` converts
			// gitlink submodule commits to stat blobs with a
			// `submodule` field.  See above.  Handle it anyway.
			inf.Mode = uint32(gitstat.ModeGitlink)
			inf.Gitlink = ent.Id[:]

		default:
			// `git.FilemodeTree` has been handled above switch.
			err = fmt.Errorf(
				"unhandled git object type `%s`", ent.Type,
			)
			setWalkErr(fullPath, err)
			return WalkStop
		}

		if err := callback(inf); err != nil {
			setWalkErr(fullPath, err)
			return WalkStop
		}
		return WalkContinue
	}

	/* The `KeepAlive(tree)` below avoids potential segfaults like:

	```
	[signal SIGSEGV: segmentation violation]
	github.com/libgit2/git2go._Cfunc_git_tree_entry_name()
		github.com/libgit2/git2go/_obj/_cgo_gotypes.go:7182
	github.com/libgit2/git2go.newTreeEntry.func1()
		/go/src/github.com/libgit2/git2go/tree.go:43
	github.com/libgit2/git2go.newTreeEntry()
		/go/src/github.com/libgit2/git2go/tree.go:43
	github.com/libgit2/git2go.CallbackGitTreeWalk()
		/go/src/github.com/libgit2/git2go/tree.go:130
	[...]
	github.com/libgit2/git2go._Cfunc__go_git_treewalk()
		github.com/libgit2/git2go/_obj/_cgo_gotypes.go:1764
	github.com/libgit2/git2go.Tree.Walk.func1()
		/go/src/github.com/libgit2/git2go/tree.go:143
	github.com/libgit2/git2go.Tree.Walk()
		/go/src/github.com/libgit2/git2go/tree.go:143
	```

	It is not entirely clear why it is needed.  `git2go/tree.c: func (t
	Tree) Walk(...)` contains a `KeepAlive(t)` that should be equivalent.

	One hypothesis is that the `KeepAlive()` in git2go is ineffective,
	because `Walk()` has a value receiver but `tree` here is a pointer.
	Maybe calling a value receiver with a pointer causes `KeepAlive()` to
	become ineffective in the callee.  Go might believe that the caller
	passed a pointer, so its the callers responsibility to keep it alive.
	There is no evidence for this hypothesis.

	# See also

	git2go KeepAlive PR <https://github.com/libgit2/git2go/pull/393>.

	The Go 1.9.3 release notes mention an issue with `runtime.KeepAlive()`.
	<https://github.com/golang/go/issues/23477>: "#22458 cmd/compile:
	runtime.KeepAlive doesn't work. CL 74210 per andybons; cmd/compile: fix
	runtime.KeepAlive".  The issue seemed related.  But upgrading to 1.9.3
	did not fix the segfault.

	*/
	tree.Walk(walkFn)
	runtime.KeepAlive(tree)
	return walkErr
}

type statInfo struct {
	Name      string      `yaml:"name"`
	Size      int64       `yaml:"size"`
	Mtime     int64       `yaml:"mtime"`
	Submodule gitOidBytes `yaml:"submodule"`
	Dirs      int64       `yaml:"dirs"`
	Files     int64       `yaml:"files"`
	Links     int64       `yaml:"links"`
	Others    int64       `yaml:"others"`
}

func (st statInfo) isSubmodule() bool {
	return st.Submodule != nil
}

func (st statInfo) isNogbundle() bool {
	return st.Submodule == nil && st.Dirs > 0
}

// `gitOidBytes` implements `UnmarshalYAML()` that expects a hex-encoded Git
// object id.
type gitOidBytes []byte

func readStatBlob(repo *git.Repository, oid *git.Oid) (statInfo, error) {
	blob, err := repo.LookupBlob(oid)
	if err != nil {
		return statInfo{}, err
	}

	var stat statInfo
	err = yaml.Unmarshal(blob.Contents(), &stat)
	return stat, err
}

func (out *gitOidBytes) UnmarshalYAML(
	unmarshal func(interface{}) error,
) error {
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}

	bin, err := hex.DecodeString(str)
	if err != nil {
		return err
	}

	*out = bin
	return nil
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

func isIgnoredTreeEntry(ent *git.TreeEntry) bool {
	if strings.HasPrefix(ent.Name, ".git") {
		return true
	}
	if strings.HasPrefix(ent.Name, ".nog") && ent.Name != ".nogtree" {
		return true
	}
	return false
}
