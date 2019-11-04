package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/nogproject/nog/backend/pkg/execx"
)

var tarTool = execx.MustLookTool(execx.ToolSpec{
	Program:   "tar",
	CheckArgs: []string{"--version"},
	CheckText: "tar (GNU tar)",
})

var cpTool = execx.MustLookTool(execx.ToolSpec{
	Program:   "cp",
	CheckArgs: []string{"--version"},
	CheckText: "cp (GNU coreutils)",
})

var awkTool = execx.MustLookTool(execx.ToolSpec{
	Program:   "awk",
	CheckArgs: []string{"--version"},
	CheckText: "GNU Awk",
})

var tarttStoreTool = execx.MustLookTool(execx.ToolSpec{
	Program:   "tartt-store",
	CheckArgs: []string{"--version"},
	CheckText: "tartt-store-",
})

var tarttIsDirTool = execx.MustLookTool(execx.ToolSpec{
	Program:   "tartt-is-dir",
	CheckArgs: []string{"--version"},
	CheckText: "tartt-is-dir-",
})

var gpg2Tool = execx.MustLookTool(execx.ToolSpec{
	Program:   "gpg2",
	CheckArgs: []string{"--version"},
	CheckText: "gpg (GnuPG) 2.",
})

type TarFeatures struct {
	ListedIncrementalMtime bool
}

func detectTarFeatures() (*TarFeatures, error) {
	cmd := exec.Command(tarTool.Path, "--help")
	cmd.Env = append(os.Environ(),
		// Ensure English.
		//
		// See
		// <http://perlgeek.de/en/article/set-up-a-clean-utf8-environment>
		// for relevant variables.  But use `C.UTF-8` instead of
		// `en_US.UTF-8`.  Most distros seem to include `C.UTF-8` now
		// as a fallback.  There is an ongoing discussion to add
		// `C.UTF-8` to glibc,
		// <https://sourceware.org/bugzilla/show_bug.cgi?id=17318>.
		// Ubuntu and Docker container base images come with only the
		// locales `C`, `C.UTF-8`, and `POSIX`.
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
		"LANGUAGE=C.UTF-8",
	)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	help := string(out)

	f := &TarFeatures{}
	if strings.Contains(help, "--listed-incremental-mtime=FILE") {
		f.ListedIncrementalMtime = true
	}

	return f, nil
}
