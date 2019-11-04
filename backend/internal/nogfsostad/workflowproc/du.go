package workflowproc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nogproject/nog/backend/pkg/execx"
)

var duTool = execx.MustLookTool(execx.ToolSpec{
	"du",
	[]string{"--version"},
	"du (GNU coreutils)",
})

// Ensure English output, e.g. error messages.
//
// See <http://perlgeek.de/en/article/set-up-a-clean-utf8-environment> for
// relevant env variables.  But use `C.UTF-8` instead of `en_US.UTF-8`.  Most
// distros seem to include `C.UTF-8` now as a fallback.  There is an ongoing
// discussion to add `C.UTF-8` to glibc,
// <https://sourceware.org/bugzilla/show_bug.cgi?id=17318>.  Ubuntu and Docker
// container base images come with only the locales `C`, `C.UTF-8`, and
// `POSIX`.
func du0Command(ctx context.Context, dir string, args ...string) *exec.Cmd {
	args = append(args, "-0", ".")
	cmd := exec.CommandContext(ctx, duTool.Path, args...)
	cmd.Env = append(os.Environ(),
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
		"LANGUAGE=C.UTF-8",
	)
	cmd.Dir = dir
	return cmd
}

func Scan0s(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		// We have a full 0-terminated line.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func parseDuLine(line string) (int64, string, error) {
	fs := strings.SplitN(line, "\t", 2)
	if len(fs) != 2 {
		return 0, "", errors.New("wrong number of fields")
	}
	usage := fs[0]
	path := fs[1]

	kb, err := strconv.ParseInt(usage, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse du size: %s", err)
	}

	if strings.HasPrefix(path, "./") {
		path = path[2:]
	}
	if path == "" {
		path = "."
	}

	return kb * 1024, path, nil
}
