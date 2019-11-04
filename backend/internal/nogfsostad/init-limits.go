package nogfsostad

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/execx"
)

type InitLimitsConfig struct {
	MaxFiles     uint64
	MaxBytes     uint64
	PrefixLimits []PathInitLimit
	RepoLimits   []PathInitLimit
}

type PathInitLimit struct {
	Path     string
	MaxFiles uint64
	MaxBytes uint64
}

type InitLimits struct {
	maxFiles     uint64
	maxBytes     uint64
	prefixLimits []PathInitLimit
	repoLimits   map[string]PathInitLimit
}

func NewInitLimits(cfg *InitLimitsConfig) *InitLimits {
	// Ensure trailing slash to limit matching to subdirs.
	prefixLimits := make([]PathInitLimit, 0, len(cfg.PrefixLimits))
	for _, l := range cfg.PrefixLimits {
		l.Path = ensureTrailingSlash(l.Path)
		prefixLimits = append(prefixLimits, l)
	}

	// Ensure trailing slash to avoid potential confusion.
	repoLimits := make(map[string]PathInitLimit)
	for _, l := range cfg.RepoLimits {
		l.Path = ensureTrailingSlash(l.Path)
		repoLimits[l.Path] = l
	}

	return &InitLimits{
		maxFiles:     cfg.MaxFiles,
		maxBytes:     cfg.MaxBytes,
		prefixLimits: prefixLimits,
		repoLimits:   repoLimits,
	}
}

func (ils *InitLimits) find(repo string) PathInitLimit {
	// Ensure trailing slash, so that prefix limit on `/project/foo/` is
	// selected for repo `/project/foo`.
	//
	// `NewInitLimits()` ensures trailing slashed on `repoLimits`.
	repo = ensureTrailingSlash(repo)

	lim, ok := ils.repoLimits[repo]
	if ok {
		return lim
	}

	for _, lim := range ils.prefixLimits {
		if strings.HasPrefix(repo, lim.Path) {
			return lim
		}
	}

	return PathInitLimit{
		MaxFiles: ils.maxFiles,
		MaxBytes: ils.maxBytes,
	}
}

// `checkInitLimit()` checks the init limit `lim`.  It returns the reason why
// the limit is violated or an empty string if the limit is satisfied.
func checkInitLimit(
	st pb.SubdirTracking,
	root string,
	lim PathInitLimit,
) string {
	switch st {
	// For `ignore-most`, skip the limits checks.  It is aconsidered safe
	// even with large trees, because it tracks only a fixed number of
	// files.
	case pb.SubdirTracking_ST_IGNORE_MOST:
		return "" // allow

	// For `bundle-subdirs` and `ignore-subdirs` check the number of files
	// in the directory, because a single directory can already contain an
	// unbounded number of files.  But ignore the file size, because we
	// want to allow super-repos of any size.
	case pb.SubdirTracking_ST_BUNDLE_SUBDIRS:
		fallthrough
	case pb.SubdirTracking_ST_IGNORE_SUBDIRS:
		ok, err := checkFilesLimitShallow(root, lim.MaxFiles)
		if err != nil {
			return fmt.Sprintf(
				"failed to count files in `%s`: %v",
				root, err,
			)
		}
		if !ok {
			return fmt.Sprintf(
				"more than %d files in `%s`",
				lim.MaxFiles, root,
			)
		}

		return "" // allow

	// For `enter-subdirs`, check the number of files and the total size
	// for the entire directory tree.
	case pb.SubdirTracking_ST_ENTER_SUBDIRS:
		ok, err := checkFilesLimitRecursive(root, lim.MaxFiles)
		if err != nil {
			return fmt.Sprintf(
				"failed to count files below `%s`: %v",
				root, err,
			)
		}
		if !ok {
			return fmt.Sprintf(
				"more than %d files below `%s`",
				lim.MaxFiles, root,
			)
		}

		ok, err = checkBytesLimit(root, lim.MaxBytes)
		if err != nil {
			return fmt.Sprintf(
				"failed to determine data size below `%s`: %v",
				root, err,
			)
		}
		if !ok {
			return fmt.Sprintf(
				"`%s` contains more than %d bytes",
				root, lim.MaxBytes,
			)
		}

		return "" // allow

	default:
		return "unknown subdir tracking"
	}
}

func checkFilesLimitShallow(root string, limit uint64) (bool, error) {
	if limit == 0 {
		return true, nil
	}

	fp, err := os.Open(root)
	if err != nil {
		return false, err
	}
	defer func() { _ = fp.Close() }()

	const MaxReadLen = 1024
	var k uint64
	for {
		names, err := fp.Readdirnames(MaxReadLen)
		k += uint64(len(names))
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
		if k > limit {
			break
		}
	}

	return k <= limit, nil
}

func checkFilesLimitRecursive(root string, limit uint64) (bool, error) {
	if limit == 0 {
		return true, nil
	}

	var k uint64
	var err error
	filepath.Walk(root, func(p string, i os.FileInfo, err2 error) error {
		if err2 != nil {
			err = err2
			return errors.New("stop")
		}
		k++
		if k > limit {
			return errors.New("stop")
		}
		return nil
	})

	if err != nil {
		return false, err
	}
	return k <= limit, nil
}

var duTool = execx.MustLookTool(execx.ToolSpec{
	Program:   "du",
	CheckArgs: []string{"--version"},
	CheckText: "du (GNU coreutils)",
})

func checkBytesLimit(root string, limit uint64) (bool, error) {
	if limit == 0 {
		return true, nil
	}

	ctx := context.Background()
	ctx, _ = context.WithTimeout(ctx, 5*time.Minute)
	cmd := exec.CommandContext(
		ctx, duTool.Path, "-sb", root,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf(
			"failed to run `du`: %v, output: %s", err, string(out),
		)
		return false, err
	}

	s, err := strconv.ParseInt(strings.Fields(string(out))[0], 10, 64)
	if err != nil {
		err := fmt.Errorf("failed to parse `du` output: %v", err)
		return false, err
	}
	if s < 0 {
		err := fmt.Errorf("`du` reported invalid size")
		return false, err
	}

	return uint64(s) <= limit, nil
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
