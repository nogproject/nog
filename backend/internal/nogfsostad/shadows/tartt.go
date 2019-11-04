package shadows

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	slashpath "path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	git "github.com/libgit2/git2go"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/regexpx"
	yaml "gopkg.in/yaml.v2"
)

var ErrMalformedTspath = errors.New("malformed tspath")
var ErrMalformedManifest = errors.New("malformed manifest")

type ListTarsFunc func(info pb.TarInfo) error

func (fs *Filesystem) TarttHead(
	ctx context.Context, shadowPath string,
) (*pb.TarttHeadO, error) {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return nil, err
	}

	o := &pb.TarttHeadO{}
	var err error
	const branch = "master-tartt"
	o.Commit, o.Author, o.Committer, err = fs.gitShowHead(
		ctx, shadowPath, fmt.Sprintf("refs/heads/%s", branch),
	)
	if err != nil {
		err := fmt.Errorf("failed to show `%s`: %s", branch, err)
		return nil, err
	}
	if o.Commit == nil {
		err := fmt.Errorf("missing branch `%s`", branch)
		return nil, err
	}
	return o, nil
}

func (fs *Filesystem) ListTars(
	ctx context.Context,
	shadowPath string,
	gitCommit []byte,
	callback ListTarsFunc,
) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}

	repo, err := git.OpenRepository(shadowPath)
	if err != nil {
		return err
	}
	defer repo.Free()

	coId := git.NewOidFromBytes(gitCommit)
	if coId == nil {
		return errors.New("invalid commit id")
	}

	co, err := repo.LookupCommit(coId)
	if err != nil {
		return err
	}

	return listTarsCommit(ctx, repo, co, callback)
}

func listTarsCommit(
	ctx context.Context,
	repo *git.Repository,
	co *git.Commit,
	callback ListTarsFunc,
) error {
	tree, err := co.Tree()
	if err != nil {
		return err
	}

	var errWalk error
	setErrWalk := func(path string, err error) {
		errWalk = fmt.Errorf("walk error at `%s`: %s", path, err)
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
		if ent.Name != "manifest.shasums" {
			return WalkContinue
		}

		tspath, ts, tarType, err := parseGitTarttTarPath(path)
		if err != nil {
			// Ignore malformed path.  Maybe setErrWalk() instead.
			return WalkContinue
		}

		info := pb.TarInfo{
			Path:    tspath,
			TarType: tarType,
			Time:    ts.Unix(),
		}

		info.Manifest, err = loadManifest(repo, ent.Id)
		if err != nil {
			setErrWalk(slashpath.Join(path, ent.Name), err)
			return WalkStop
		}

		if err := callback(info); err != nil {
			setErrWalk(slashpath.Join(path, ent.Name), err)
			return WalkStop
		}

		return WalkContinue
	}

	// The `KeepAlive(tree)` below avoids potential segfaults.  See
	// `./tree.go` for details.
	tree.Walk(walkFn)
	runtime.KeepAlive(tree)
	return errWalk
}

// See `tartt/tree.go` for discussion of timestamp formats.  Briefly, v1 was
// not compliant with ISO 8601.  v2 is compliant with ISO 8601 basic format.
const (
	timestampTimeFormat1 = "2006-01-02T150405Z"
	timestampTimeFormat2 = "20060102T150405Z"
)

var rgxTarDirPath = regexp.MustCompile(regexpx.Verbose(`
	^
	stores /
	( [^/]+ ) /
	(
		(?:
			(?: [0-9]{4}-[0-9]{2}-[0-9]{2} | [0-9]{8} )T[0-9]{6}Z /
			[a-z0-9]+ /
		)*
	)
	( (?: [0-9]{4}-[0-9]{2}-[0-9]{2} | [0-9]{8} )T[0-9]{6}Z ) /
	( full | patch ) /
	$
`))

func parseGitTarttTarPath(
	path string,
) (tspath string, ts time.Time, tarType pb.TarInfo_TarType, err error) {
	m := rgxTarDirPath.FindStringSubmatch(path)
	if m == nil {
		err = ErrMalformedTspath
		return tspath, ts, tarType, err
	}
	store := m[1]
	tsPrefix := m[2]
	tsDir := m[3]
	tarDir := m[4]

	ts, err = time.Parse(timestampTimeFormat2, tsDir)
	if err != nil {
		// If parsing also fails with legacy v1 format, return the v2
		// error.  Otherwise, accept the v1 format and clear the error.
		t, err1 := time.Parse(timestampTimeFormat1, tsDir)
		if err1 != nil {
			err = ErrMalformedTspath
			return tspath, ts, tarType, err
		}
		ts, err = t, nil
	}

	switch tarDir {
	case "full":
		tarType = pb.TarInfo_TAR_FULL
	case "patch":
		tarType = pb.TarInfo_TAR_PATCH
	default:
		err = ErrMalformedTspath
		return tspath, ts, tarType, err
	}

	if tsPrefix == "" {
		tspath = fmt.Sprintf("%s/%s", store, tsDir)
	} else {
		tspath = fmt.Sprintf("%s/%s%s", store, tsPrefix, tsDir)
	}
	return tspath, ts, tarType, err
}

func loadManifest(
	repo *git.Repository, oid *git.Oid,
) ([]*pb.TarManifestEntry, error) {
	blob, err := repo.LookupBlob(oid)
	if err != nil {
		return nil, err
	}
	return readManifest(bytes.NewReader(blob.Contents()))
}

func readManifest(r io.Reader) ([]*pb.TarManifestEntry, error) {
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

	var manifest []*pb.TarManifestEntry
	byName := make(map[string]*pb.TarManifestEntry)
	for s.Scan() {
		line := s.Text()

		lineFields := strings.SplitN(line, " ", 3)
		if len(lineFields) != 3 {
			return nil, ErrMalformedManifest
		}
		kvText := lineFields[0]
		file := lineFields[2]

		kvFields := strings.SplitN(kvText, ":", 2)
		if len(kvFields) != 2 {
			return nil, ErrMalformedManifest
		}
		k := kvFields[0]
		v := kvFields[1]

		entry, ok := byName[file]
		if !ok {
			entry = &pb.TarManifestEntry{File: file}
			manifest = append(manifest, entry)
			byName[file] = entry
		}

		switch k {
		case "size":
			i, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, ErrMalformedManifest
			}
			entry.Size = i
		case "sha256":
			if len(v) != 256/8*2 {
				return nil, ErrMalformedManifest
			}
			sha, err := hex.DecodeString(v)
			if err != nil {
				return nil, ErrMalformedManifest
			}
			entry.Sha256 = sha
		case "sha512":
			if len(v) != 512/8*2 {
				return nil, ErrMalformedManifest
			}
			sha, err := hex.DecodeString(v)
			if err != nil {
				return nil, ErrMalformedManifest
			}
			entry.Sha512 = sha
		}
	}

	return manifest, s.Err()
}

func (fs *Filesystem) GetTarttconfig(
	ctx context.Context,
	shadowPath string,
	gitCommit []byte,
) ([]byte, error) {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return nil, err
	}

	repo, err := git.OpenRepository(shadowPath)
	if err != nil {
		return nil, err
	}
	defer repo.Free()

	coId := git.NewOidFromBytes(gitCommit)
	if coId == nil {
		err := errors.New("invalid commit id")
		return nil, err
	}

	co, err := repo.LookupCommit(coId)
	if err != nil {
		return nil, err
	}

	tree, err := co.Tree()
	if err != nil {
		return nil, err
	}

	ent := tree.EntryByName("tarttconfig.yml")
	if ent == nil {
		err := errors.New("missing `tarttconfig.yml`")
		return nil, err
	}

	blob, err := repo.LookupBlob(ent.Id)
	if err != nil {
		return nil, err
	}

	return blob.Contents(), nil
}

type TarttIsFrozenArchiveInfo struct {
	IsFrozenArchive bool
	Reason          string

	StatAuthorName  string
	StatAuthorEmail string
	StatAuthorDate  time.Time
	Dirs            int64
	Files           int64
	FilesSize       int64
	Links           int64
	Others          int64
	MtimeMin        time.Time
	MtimeMax        time.Time

	TarPath string
	TarTime time.Time
}

type nogtreeInfo struct {
	Size     int64  `yaml:"size"`
	Dirs     int64  `yaml:"dirs"`
	Files    int64  `yaml:"files"`
	Links    int64  `yaml:"links"`
	Others   int64  `yaml:"others"`
	MtimeMin int64  `yaml:"mtime_min"`
	MtimeMax int64  `yaml:"mtime_max"`
	Attrs    string `yaml:"attrs"`
}

func (t nogtreeInfo) isImmutable() bool {
	return strings.Contains(t.Attrs, "i")
}

func (fs *Filesystem) TarttIsFrozenArchive(
	ctx context.Context, shadowPath string,
) (*TarttIsFrozenArchiveInfo, error) {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return nil, err
	}

	repo, err := git.OpenRepository(shadowPath)
	if err != nil {
		return nil, err
	}
	defer repo.Free()

	// `master-stat` must exist for an enabled repo -> return error.
	statBr, err := repo.LookupBranch("master-stat", git.BranchLocal)
	if err != nil {
		return nil, err
	}
	statObj, err := statBr.Peel(git.ObjectCommit)
	if err != nil {
		return nil, err
	}
	statCo, err := statObj.AsCommit()
	if err != nil {
		return nil, err
	}

	nogtree, err := loadToplevelNogtree(repo, statCo)
	if err != nil {
		return nil, err
	}
	if !nogtree.isImmutable() {
		return &TarttIsFrozenArchiveInfo{
			IsFrozenArchive: false,
			Reason:          "master-stat is not immutable",
		}, nil
	}

	// `master-tartt` may be missing for an enabled repo -> convert error
	// to info.
	tarttBr, err := repo.LookupBranch("master-tartt", git.BranchLocal)
	if err != nil {
		return &TarttIsFrozenArchiveInfo{
			IsFrozenArchive: false,
			Reason:          "master-tartt is missing",
		}, nil
	}
	tarttObj, err := tarttBr.Peel(git.ObjectCommit)
	if err != nil {
		return nil, err
	}
	tarttCo, err := tarttObj.AsCommit()
	if err != nil {
		return nil, err
	}
	statWhen := statCo.Committer().When

	// If `master-tartt` is newer than `master-stat`, it does not
	// necessarily contain an archive for the latest `master-stat`.  The
	// lastest `master-tartt` might be a GC commit.  But if `master-tartt`
	// is not newer, it certainly does not contain an archive for the
	// latest `master-stat`.
	if !tarttCo.Committer().When.After(statWhen) {
		return &TarttIsFrozenArchiveInfo{
			IsFrozenArchive: false,
			Reason:          "tartt archive is not up to date",
		}, nil
	}

	// Check that the latest tartt archive is a full archive that is newer
	// than master-stat.
	var latestTar pb.TarInfo
	err = listTarsCommit(ctx, repo, tarttCo, func(info pb.TarInfo) error {
		latestTar = info
		return nil
	})
	if err != nil {
		return nil, err
	}
	if latestTar.TarType != pb.TarInfo_TAR_FULL {
		return &TarttIsFrozenArchiveInfo{
			IsFrozenArchive: false,
			Reason:          "not a full tartt archive",
		}, nil
	}
	if !time.Unix(latestTar.Time, 0).After(statWhen) {
		return &TarttIsFrozenArchiveInfo{
			IsFrozenArchive: false,
			Reason:          "tartt archive is not up to date",
		}, nil
	}

	author := statCo.Author()
	return &TarttIsFrozenArchiveInfo{
		IsFrozenArchive: true,

		StatAuthorName:  author.Name,
		StatAuthorEmail: author.Email,
		StatAuthorDate:  author.When,
		Dirs:            nogtree.Dirs,
		Files:           nogtree.Files,
		FilesSize:       nogtree.Size,
		Links:           nogtree.Links,
		Others:          nogtree.Others,
		MtimeMin:        time.Unix(nogtree.MtimeMin, 0),
		MtimeMax:        time.Unix(nogtree.MtimeMax, 0),

		TarPath: latestTar.Path,
		TarTime: time.Unix(latestTar.Time, 0),
	}, nil
}

var ErrMissingNogtree = errors.New("missing `.nogtree`")

func loadToplevelNogtree(
	repo *git.Repository, commit *git.Commit,
) (*nogtreeInfo, error) {
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	ent := tree.EntryByName(".nogtree")
	if ent == nil {
		return nil, ErrMissingNogtree
	}

	blob, err := repo.LookupBlob(ent.Id)
	if err != nil {
		return nil, err
	}

	var inf nogtreeInfo
	err = yaml.Unmarshal(blob.Contents(), &inf)
	return &inf, err
}
