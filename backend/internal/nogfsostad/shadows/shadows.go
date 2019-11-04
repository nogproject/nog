// Package `shadows`: FSO shadow repos.
package shadows

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/execx"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

type Config struct {
	ShadowRoot             string
	ShadowRootAlternatives []string
	GitFsoProgram          string
	TrimHostRoot           string
	GitCommitter           User
}

type Filesystem struct {
	lg           Logger
	root         string
	allRoots     []string
	tools        *tools
	trimHostRoot string
	gitCommitter User
}

type Logger interface {
	Infow(msg string, kv ...interface{})
}

type tools struct {
	git    *execx.Tool
	gitFso *execx.Tool
}

func New(lg Logger, cfg Config) (*Filesystem, error) {
	fs := Filesystem{
		lg:   lg,
		root: cfg.ShadowRoot,
		allRoots: append(
			[]string{cfg.ShadowRoot},
			cfg.ShadowRootAlternatives...,
		),
		trimHostRoot: ensureTrailingSlash(cfg.TrimHostRoot),
	}
	if cfg.GitCommitter.isEmpty() {
		fs.gitCommitter = User{
			Name:  "nogfsostad",
			Email: "nogfsostad@sys.nogproject.io",
		}
	} else {
		fs.gitCommitter = cfg.GitCommitter
	}

	if err := checkIsDir(fs.root); err != nil {
		return nil, err
	}

	var err error
	fs.tools, err = lookTools(cfg.GitFsoProgram)
	if err != nil {
		return nil, err
	}

	return &fs, nil
}

func lookTools(gitFsoPath string) (*tools, error) {
	ts := tools{}

	var err error
	ts.git, err = execx.LookTool(execx.ToolSpec{
		Program:   "git",
		CheckArgs: []string{"--version"},
		CheckText: "git version 2",
	})
	if err != nil {
		return nil, err
	}

	ts.gitFso, err = execx.LookTool(execx.ToolSpec{
		Program:   gitFsoPath,
		CheckArgs: []string{"--version"},
		CheckText: "git-fso-0.1.0",
	})
	if err != nil {
		return nil, err
	}

	return &ts, nil
}

type ShadowInfo struct {
	ShadowPath string
}

type User struct {
	Name  string
	Email string
}

func (u User) isEmpty() bool {
	return u.Name == ""
}

type SubdirTracking int

const (
	SubdirTrackingUnspecified SubdirTracking = iota
	EnterSubdirs
	BundleSubdirs
	IgnoreSubdirs
	IgnoreMost
)

type InitOptions struct {
	SubdirTracking SubdirTracking
}

// See NOE-18 for details.  Mangle UUID into shadow path to allow
// nested repos, like:
//
// ```
// real/foo/
// real/foo/bar/
//
// shadow/foo/<uuid>.fso/
// shadow/foo/bar/<uuid>.fso/
// ```
//
func (fs *Filesystem) ShadowPath(hostPath string, repoId uuid.I) string {
	return filepath.Join(
		fs.root,
		strings.TrimPrefix(hostPath, fs.trimHostRoot),
		fmt.Sprintf("%s.fso", repoId),
	)
}

func (fs *Filesystem) Init(
	hostPath string, author User, repoId uuid.I, opts InitOptions,
) (*ShadowInfo, error) {
	if err := checkIsDir(hostPath); err != nil {
		return nil, err
	}

	if strings.HasPrefix(hostPath, fs.root) {
		err := fmt.Errorf(
			"path `%s` is below shadow root `%s`",
			hostPath, fs.root,
		)
		return nil, err
	}

	shadow := fs.ShadowPath(hostPath, repoId)
	si := &ShadowInfo{ShadowPath: shadow}

	// If the shadow repo exists, it must have been successfully
	// initialized before, because shadow repos are moved atomically to
	// their permanent location.
	if isDir(shadow) {
		return si, nil
	}

	// Limit to user write, leave rest to umask.
	parent := filepath.Dir(shadow)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return nil, err
	}
	// Don't use `ioutil.TempDir()`, because we want `perms=0755`.
	tmp := filepath.Join(
		parent,
		fmt.Sprintf(
			"%s.__tmp__%d__",
			filepath.Base(shadow),
			time.Now().UnixNano(),
		),
	)
	if err := os.Mkdir(tmp, 0755); err != nil {
		err = fmt.Errorf("failed to create tempdir: %v", err)
		return nil, err
	}
	defer func() {
		err := os.RemoveAll(tmp)
		_ = err // `tmp` may have moved to its final location.
	}()

	args := []string{"init", "--observe", hostPath}
	switch opts.SubdirTracking {
	case EnterSubdirs:
		// Nothing to append.  Enter is the default.
	case BundleSubdirs:
		args = append(args, "--bundle-subdirs")
	case IgnoreSubdirs:
		args = append(args, "--ignore-subdirs")
	case IgnoreMost:
		args = append(args, "--ignore-most")
	default:
		panic("unknown SubdirTracking")
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, fs.tools.gitFso.Path, args...)
	cmd.Dir = tmp
	cmd.Env = fs.gitEnvAuthor(author)
	if out, err := cmd.CombinedOutput(); err != nil {
		err := fmt.Errorf(
			"git-fso init failed: %s; output: %s", err, out,
		)
		return nil, err
	}

	uuidPath := filepath.Join(tmp, ".git/fso/uuid")
	uuidData := []byte(fmt.Sprintf("%s\n", repoId.String()))
	if err := ioutil.WriteFile(uuidPath, uuidData, 0644); err != nil {
		err = fmt.Errorf("failed to save UUID: %s", err)
		return nil, err
	}

	if err := os.Rename(tmp, shadow); err != nil {
		err := fmt.Errorf("failed to move shadow repo: %s", err)
		return nil, err
	}

	return si, nil
}

type Oid [20]byte

func (fs *Filesystem) Ref(shadowPath, ref string) (Oid, error) {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return Oid{}, err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		fs.tools.git.Path, "rev-parse", "-q", "--verify", ref,
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf(
			"git rev-parse failed: %s; output: %s", err, out,
		)
		return Oid{}, err
	}
	out = bytes.TrimSpace(out)

	var oid Oid
	n, err := hex.Decode(oid[0:20], out)
	if err != nil {
		return Oid{}, err
	}
	if n != 20 {
		return Oid{}, errors.New("invalid Git object hex id")
	}

	return oid, nil
}

type StatOptions struct {
	MtimeRangeOnly bool
}

func (fs *Filesystem) Stat(
	shadowPath string, author User, opts StatOptions,
) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	args := []string{"stat"}
	if opts.MtimeRangeOnly {
		args = append(args, "--mtime-range-only")
	}
	cmd := exec.CommandContext(ctx, fs.tools.gitFso.Path, args...)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnvAuthor(author)
	if out, err := cmd.CombinedOutput(); err != nil {
		err := fmt.Errorf(
			"git-fso stat failed: %s; output: %s", err, out,
		)
		return err
	} else {
		fs.lg.Infow(
			"git-fso stat ok.",
			"shadow", shadowPath,
			"out", string(out),
		)
	}

	return nil
}

func (fs *Filesystem) Sha(shadowPath string, author User) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 120*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		fs.tools.gitFso.Path, "sha",
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnvAuthor(author)
	if out, err := cmd.CombinedOutput(); err != nil {
		err := fmt.Errorf(
			"git-fso sha failed: %s; output: %s", err, out,
		)
		return err
	} else {
		fs.lg.Infow(
			"git-fso sha ok.",
			"shadow", shadowPath,
			"out", string(out),
		)
	}

	return nil
}

func (fs *Filesystem) RefreshContent(shadowPath string, author User) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		fs.tools.gitFso.Path, "content",
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnvAuthor(author)
	if out, err := cmd.CombinedOutput(); err != nil {
		err := fmt.Errorf(
			"git-fso content failed: %s; output: %s", err, out,
		)
		return err
	} else {
		fs.lg.Infow(
			"git-fso content ok.",
			"shadow", shadowPath,
			"out", string(out),
		)
	}

	return nil
}

func (fs *Filesystem) Archive(shadowPath string, author User) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, fs.tools.gitFso.Path, "archive")
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnvAuthor(author)
	if out, err := cmd.CombinedOutput(); err != nil {
		err := fmt.Errorf(
			"git-fso archive failed: %s; output: %s", err, out,
		)
		return err
	} else {
		fs.lg.Infow(
			"git-fso archive ok.",
			"shadow", shadowPath,
			"out", string(out),
		)
	}

	return nil
}

func (fs *Filesystem) ReinitSubdirTracking(
	shadowPath string, author User, subdirTracking SubdirTracking,
) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}

	args := []string{"reinit"}
	switch subdirTracking {
	case EnterSubdirs:
		args = append(args, "--enter-subdirs")
	case BundleSubdirs:
		args = append(args, "--bundle-subdirs")
	case IgnoreSubdirs:
		args = append(args, "--ignore-subdirs")
	case IgnoreMost:
		args = append(args, "--ignore-most")
	default:
		panic("unknown subdirTracking")
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, fs.tools.gitFso.Path, args...)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnvAuthor(author)
	if out, err := cmd.CombinedOutput(); err != nil {
		err := fmt.Errorf(
			"git-fso reinit failed: %s; output: %s", err, out,
		)
		return err
	} else {
		fs.lg.Infow(
			"git-fso reinit ok.",
			"shadow", shadowPath,
			"out", string(out),
		)
	}

	return nil
}

func (fs *Filesystem) Push(shadowPath string, remote string) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		fs.tools.git.Path, "push", "--all", remote,
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		err := fmt.Errorf(
			"git push failed: %s; output: %s", err, out,
		)
		return err
	} else {
		fs.lg.Infow(
			"git push ok.",
			"shadow", shadowPath,
			"out", string(out),
		)
	}

	return nil
}

func (fs *Filesystem) Head(
	ctx context.Context, shadowPath string,
) (*pb.HeadO, error) {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return nil, err
	}

	ref := func(b string) string { return fmt.Sprintf("refs/heads/%s", b) }

	heads := pb.HeadGitCommits{}
	o := &pb.HeadO{}
	var err error

	branch := "master-stat"
	heads.Stat, o.StatAuthor, o.StatCommitter, err = fs.gitShowHead(
		ctx, shadowPath, ref(branch),
	)
	if err != nil {
		err := fmt.Errorf("failed to show `%s`: %s", branch, err)
		return nil, err
	}

	branch = "master-sha"
	heads.Sha, o.ShaAuthor, o.ShaCommitter, err = fs.gitShowHead(
		ctx, shadowPath, ref(branch),
	)
	if err != nil {
		err := fmt.Errorf("failed to show `%s`: %s", branch, err)
		return nil, err
	}

	branch = "master-meta"
	heads.Meta, o.MetaAuthor, o.MetaCommitter, err = fs.gitShowHead(
		ctx, shadowPath, ref(branch),
	)
	if err != nil {
		err := fmt.Errorf("failed to show `%s`: %s", branch, err)
		return nil, err
	}

	branch = "master-content"
	heads.Content, o.ContentAuthor, o.ContentCommitter, err = fs.gitShowHead(
		ctx, shadowPath, ref(branch),
	)
	if err != nil {
		err := fmt.Errorf("failed to show `%s`: %s", branch, err)
		return nil, err
	}

	o.CommitId, err = headsSha(&heads)
	if err != nil {
		err := fmt.Errorf("headsSha() failed: %s", err)
		return nil, err
	}

	o.GitCommits = &heads

	return o, nil
}

func (fs *Filesystem) Summary(
	ctx context.Context, shadowPath string,
) (*pb.SummaryO, error) {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return nil, err
	}

	head, err := fs.Head(ctx, shadowPath)
	if err != nil {
		return nil, err
	}

	o := &pb.SummaryO{}
	o.CommitId = head.CommitId

	cmd := exec.CommandContext(
		ctx,
		fs.tools.git.Path, "ls-tree", "-r", "-z",
		"refs/heads/master-stat",
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf(
			"git show failed: %s; output: %s", err, out,
		)
		return nil, err
	}

	var nFiles int64
	dirs := make(map[string]bool)
	var nOther int64

	nullByte := []byte{0}
	out = bytes.TrimSuffix(out, nullByte)
	for _, l := range bytes.Split(out, nullByte) {
		tabFields := strings.SplitN(string(l), "\t", 2)
		if len(tabFields) != 2 {
			err := fmt.Errorf("failed to parse git ls-tree")
			return nil, err
		}
		spaceFields := strings.Split(tabFields[0], " ")
		if len(spaceFields) != 3 {
			err := fmt.Errorf("failed to parse git ls-tree")
			return nil, err
		}
		name := tabFields[1]
		typ := spaceFields[1]

		if isHiddenName(name) {
			continue
		}

		dirs[path.Dir(name)] = true
		switch typ {
		case "blob":
			nFiles++
		default:
			nOther++
		}
	}

	o.NumFiles = nFiles
	delete(dirs, ".") // Don't count root dir.
	o.NumDirs = int64(len(dirs))
	o.NumOther = nOther

	return o, nil
}

func (fs *Filesystem) Meta(
	ctx context.Context, shadowPath string,
) (*pb.MetaO, error) {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return nil, err
	}

	heads, err := fs.gitHeads(ctx, shadowPath)
	if err != nil {
		return nil, err
	}

	o := &pb.MetaO{}
	o.CommitId, err = headsSha(heads)
	if err != nil {
		err := fmt.Errorf("headsSha() failed: %s", err)
		return nil, err
	}

	metaJson := []byte("{}")
	if heads.Meta != nil {
		blob, err := fs.getFileContent(
			ctx, shadowPath, heads.Meta, ".nogtree",
		)
		if err != nil {
			return nil, err
		}
		if blob != nil {
			j, err := recodeMetaYamlToJson(blob)
			if err != nil {
				return nil, err
			}
			metaJson = j
		}
	}

	o.MetaJson = metaJson
	return o, nil
}

func (fs *Filesystem) Content(
	ctx context.Context, shadowPath, contentPath string,
) (*pb.ContentO, error) {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return nil, err
	}

	heads, err := fs.gitHeads(ctx, shadowPath)
	if err != nil {
		return nil, err
	}

	o := &pb.ContentO{}
	o.CommitId, err = headsSha(heads)
	if err != nil {
		err := fmt.Errorf("headsSha() failed: %s", err)
		return nil, err
	}

	blob, err := fs.getFileContent(
		ctx, shadowPath, heads.Content, contentPath,
	)
	if err != nil {
		return nil, err
	}
	if blob == nil {
		err := errors.New("unknown path")
		return nil, err
	}
	o.Content = blob

	return o, nil
}

func (fs *Filesystem) GitGc(ctx context.Context, shadowPath string) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}
	return fs.gitGc(ctx, shadowPath)
}

func (fs *Filesystem) getFileContent(
	ctx context.Context, shadowPath string, head []byte, path string,
) ([]byte, error) {
	tree, err := fs.gitLsTreeShallow(ctx, shadowPath, head)
	if err != nil {
		return nil, err
	}
	info, ok := tree[path]
	if !ok {
		return nil, nil
	}

	fields := strings.Split(info, " ")
	if len(fields) != 3 {
		err := fmt.Errorf("failed to parse git ls-tree")
		return nil, err
	}
	blobId := fields[2]

	return fs.gitCatFileBlob(ctx, shadowPath, blobId)
}

func (fs *Filesystem) gitRevParseBytes(
	ctx context.Context, shadowPath, ref string,
) ([]byte, error) {
	cmd := exec.CommandContext(
		ctx,
		fs.tools.git.Path, "rev-parse", "-q", "--verify", ref,
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf(
			"git rev-parse failed: %s; output: %s", err, out,
		)
		return nil, err
	}
	out = bytes.TrimSpace(out)

	head := make([]byte, hex.DecodedLen(len(out)))
	if _, err := hex.Decode(head, out); err != nil {
		err := fmt.Errorf("failed to decode commit hash: %s", err)
		return nil, err
	}

	return head, nil
}

func (fs *Filesystem) gitUpdateRef(
	ctx context.Context, shadowPath, ref string,
	new, old []byte, message string,
) error {
	cmd := exec.CommandContext(
		ctx,
		fs.tools.git.Path, "update-ref",
		"-m", message,
		ref, hex.EncodeToString(new), hex.EncodeToString(old),
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf(
			"git update-ref failed: %s; output: %s", err, out,
		)
		return err
	}
	return nil
}

func (fs *Filesystem) gitLsTreeShallow(
	ctx context.Context, shadowPath string, head []byte,
) (map[string]string, error) {
	cmd := exec.CommandContext(
		ctx,
		fs.tools.git.Path, "ls-tree", "-z", hex.EncodeToString(head),
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf(
			"git rev-parse failed: %s; output: %s", err, out,
		)
		return nil, err
	}
	nullByte := []byte{0}
	out = bytes.TrimSuffix(out, nullByte)

	tree := make(map[string]string)
	for _, l := range bytes.Split(out, nullByte) {
		tabFields := strings.SplitN(string(l), "\t", 2)
		if len(tabFields) != 2 {
			err := fmt.Errorf("failed to parse git ls-tree")
			return nil, err
		}
		info := tabFields[0]
		name := tabFields[1]
		tree[name] = info
	}

	return tree, nil
}

func (fs *Filesystem) gitHashObjectBlob(
	ctx context.Context, shadowPath string, data []byte,
) (string, error) {
	cmd := exec.CommandContext(
		ctx,
		fs.tools.git.Path, "hash-object", "-w", "--stdin",
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	go func() {
		defer stdin.Close()
		_, err := stdin.Write(data)
		if err != nil {
			panic(err)
		}
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(out)), nil
}

func (fs *Filesystem) gitCatFileBlob(
	ctx context.Context, shadowPath, oid string,
) ([]byte, error) {
	cmd := exec.CommandContext(
		ctx,
		fs.tools.git.Path, "cat-file", "blob", oid,
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf(
			"git cat-file failed: %s; output: %s", err, out,
		)
		return nil, err
	}
	return out, nil
}

func (fs *Filesystem) gitMktree(
	ctx context.Context, shadowPath string, tree map[string]string,
) (string, error) {
	cmd := exec.CommandContext(
		ctx,
		fs.tools.git.Path, "mktree", "-z",
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	mustWriteString := func(s string) {
		_, err := io.WriteString(stdin, s)
		if err != nil {
			panic(err)
		}
	}

	go func() {
		defer stdin.Close()
		for name, details := range tree {
			mustWriteString(details)
			mustWriteString("\t")
			mustWriteString(name)
			mustWriteString("\x00")
		}
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(out)), nil
}

func (fs *Filesystem) gitGc(ctx context.Context, shadowPath string) error {
	cmd := exec.CommandContext(ctx, fs.tools.git.Path, "gc")
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf(
			"git gc failed: %s; output: %s", err, out,
		)
		return err
	}
	return nil
}

func isHiddenName(n string) bool {
	return strings.HasPrefix(n, ".git") || strings.HasPrefix(n, ".nog")
}

func (fs *Filesystem) gitHeads(
	ctx context.Context, shadowPath string,
) (heads *pb.HeadGitCommits, err error) {
	heads = &pb.HeadGitCommits{}

	ref := func(b string) string { return fmt.Sprintf("refs/heads/%s", b) }

	branch := "master-stat"
	heads.Stat, _, _, err = fs.gitShowHead(ctx, shadowPath, ref(branch))
	if err != nil {
		err := fmt.Errorf("failed to show `%s`: %s", branch, err)
		return nil, err
	}

	branch = "master-sha"
	heads.Sha, _, _, err = fs.gitShowHead(ctx, shadowPath, ref(branch))
	if err != nil {
		err := fmt.Errorf("failed to show `%s`: %s", branch, err)
		return nil, err
	}

	branch = "master-meta"
	heads.Meta, _, _, err = fs.gitShowHead(ctx, shadowPath, ref(branch))
	if err != nil {
		err := fmt.Errorf("failed to show `%s`: %s", branch, err)
		return nil, err
	}

	branch = "master-content"
	heads.Content, _, _, err = fs.gitShowHead(ctx, shadowPath, ref(branch))
	if err != nil {
		err := fmt.Errorf("failed to show `%s`: %s", branch, err)
		return nil, err
	}

	return heads, nil
}

func (fs *Filesystem) gitShowHead(
	ctx context.Context, shadowPath, branch string,
) (head []byte, author, committer *pb.WhoDate, err error) {

	cmd := exec.CommandContext(
		ctx,
		fs.tools.git.Path, "rev-parse", "-q", "--verify", branch,
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	if err := cmd.Run(); err != nil {
		// All nil indicates that branch does not exist.
		return nil, nil, nil, nil
	}

	nullFormat := "%x00"
	nullByte := []byte{0}
	cmd = exec.CommandContext(
		ctx,
		fs.tools.git.Path, "show", "-s",
		"--pretty=format:"+strings.Join([]string{
			"%H",  // commit hash
			"%an", // author name
			"%ae", // author email
			"%aI", // author date, strict ISO
			"%cn", // committer name
			"%ce", // committer email
			"%cI", // committer date, strict ISO
		}, nullFormat),
		branch,
	)
	cmd.Dir = shadowPath
	cmd.Env = fs.gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf(
			"git show failed: %s; output: %s", err, out,
		)
		return nil, nil, nil, err
	}

	fields := bytes.Split(out, nullByte)
	if len(fields) != 7 {
		err := fmt.Errorf("failed to parse git show output")
		return nil, nil, nil, err
	}

	head = make([]byte, hex.DecodedLen(len(fields[0])))
	if _, err := hex.Decode(head, fields[0]); err != nil {
		err := fmt.Errorf("failed to decode commit hash: %s", err)
		return nil, nil, nil, err
	}
	author = &pb.WhoDate{
		Name:  string(fields[1]),
		Email: string(fields[2]),
		Date:  string(fields[3]),
	}
	committer = &pb.WhoDate{
		Name:  string(fields[4]),
		Email: string(fields[5]),
		Date:  string(fields[6]),
	}
	return head, author, committer, nil
}

func headsSha(heads *pb.HeadGitCommits) ([]byte, error) {
	buf, err := proto.Marshal(heads)
	if err != nil {
		err := fmt.Errorf("proto encoding heads failed")
		return nil, err
	}
	sha := sha512.Sum512_256(buf)
	return sha[:], nil
}

func (fs *Filesystem) checkShadowPath(shadowPath string) error {
	if !hasAnyPrefix(shadowPath, fs.allRoots) {
		err := fmt.Errorf(
			"path `%s` is not below any shadow root", shadowPath,
		)
		return err
	}
	return nil
}

func hasAnyPrefix(path string, prefixes []string) bool {
	for _, pfx := range prefixes {
		if strings.HasPrefix(path, pfx) {
			return true
		}
	}
	return false
}

func (fs *Filesystem) gitEnvAuthor(a User) []string {
	return gitEnvAuthorCommitter(a, fs.gitCommitter)
}

// Prefer `gitEnvAuthor()`.
func (fs *Filesystem) gitEnv() []string {
	return fs.gitEnvAuthor(fs.gitCommitter)
}

func gitEnvAuthorCommitter(a User, c User) []string {
	return append(
		os.Environ(),
		fmt.Sprintf("GIT_AUTHOR_NAME=%s", a.Name),
		fmt.Sprintf("GIT_AUTHOR_EMAIL=%s", a.Email),
		fmt.Sprintf("GIT_COMMITTER_NAME=%s", c.Name),
		fmt.Sprintf("GIT_COMMITTER_EMAIL=%s", c.Email),
	)
}

func checkIsDir(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		err := fmt.Errorf("`%s` exits, but not a directory", path)
		return err
	}
	return nil
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return st.IsDir()
}

// `recodeMeta()` expects an input JSON object with string keys and returns it
// as YAML with sorted keys, one per line like `<key>: <json-val>`.  Example
// line: `note: "foo"`.
func recodeMeta(in []byte) ([]byte, error) {
	meta := make(map[string]interface{})
	if err := json.Unmarshal(in, &meta); err != nil {
		err := fmt.Errorf("failed to decode meta JSON: %v", err)
		return nil, err
	}

	var out bytes.Buffer
	jout := json.NewEncoder(&out)
	jout.SetEscapeHTML(false)

	keys := make([]string, 0)
	for k, _ := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&out, "%s: ", k)
		if err := jout.Encode(meta[k]); err != nil {
			err := fmt.Errorf("failed to encode meta: %v", err)
			return nil, err
		}
	}

	return out.Bytes(), nil
}
