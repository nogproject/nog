// vim: sw=8

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// `xVersion` and `xBuild` are injected by the `Makefile`.
var (
	xVersion string
	xBuild   string
	version  = fmt.Sprintf("tartt-is-dir-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(strings.TrimSpace(`
Usage:
  tartt-is-dir [--] <path>

''tartt-is-dir'' determines whether ''<path>'' is a directory.

Exit codes that indicate success:

* 0: ''<path>'' is a directory.
* 10: ''<path>'' is not a directory.

Exit codes that indicate a failure:

* 1: It could not be determined whether ''<path>'' is a directory, probably due
     to lack of permissions.

`))

func main() {
	var path string
	switch len(os.Args) {
	case 1:
		die("Missing argument.")
	case 2:
		if os.Args[1] == "-h" || os.Args[1] == "--help" {
			cmdHelp()
		}
		if os.Args[1] == "--version" {
			cmdVersion()
		}
		path = os.Args[1]
	case 3:
		if os.Args[1] != "--" {
			die("Invalid argument.")
		}
		path = os.Args[2]
	default:
		die("Too many arguments.")
	}
	cmdIsDir(path)
}

func cmdHelp() {
	fmt.Println(usage)
	os.Exit(0)
}

func cmdVersion() {
	fmt.Println(version)
	os.Exit(0)
}

func cmdIsDir(path string) {
	inf, err := os.Stat(path)
	if err != nil {
		exitStatErr(path, err)
	}
	exitStatInfo(path, inf)
}

func exitStatErr(path string, err error) {
	errno := err.(*os.PathError).Err
	if errno == syscall.ENOENT {
		fmt.Printf("`%s` does not exists.\n", path)
		os.Exit(10)
	}
	fmt.Printf(
		"Failed to decide whether `%s` is a directory: %v\n",
		path, err,
	)
	os.Exit(1)
}

func exitStatInfo(path string, inf os.FileInfo) {
	if inf.IsDir() {
		fmt.Printf("`%s` is a directory.\n", path)
		os.Exit(0)
	}
	fmt.Printf("`%s` is not a directory.\n", path)
	os.Exit(10)
}

func die(msg string) {
	fmt.Fprintf(os.Stderr, "fatal: %s\n", msg)
	os.Exit(1)
}
