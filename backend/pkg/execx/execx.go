// Package `execx` provides utility functions that supplement the stdlib
// package `os/exec`.
//
// `MustLookTool()` reliably locates external command line tools during program
// startup.
package execx

import (
	"fmt"
	"os/exec"
	"strings"
)

// `ToolSpec` is used to tell `MustLookTool()` how to look for an external
// tool.
type ToolSpec struct {
	Program   string
	CheckArgs []string
	CheckText string
}

type Tool struct {
	Path string
}

func LookTool(s ToolSpec) (*Tool, error) {
	path, err := exec.LookPath(s.Program)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to find path of `%s`: %v", s.Program, err,
		)
	}

	o, err := exec.Command(path, s.CheckArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to execute `%s %s`: %v", path,
			strings.Join(s.CheckArgs, ", "), err,
		)
	}
	if !strings.Contains(string(o), s.CheckText) {
		return nil, fmt.Errorf(
			"`%s %s` did not print `%s`.", s.Program,
			strings.Join(s.CheckArgs, ", "), s.CheckText,
		)
	}

	return &Tool{path}, nil
}

// `MustLookTool()` tries to run `s.Program` with `s.CheckArgs` and verifies
// that its output contains `s.CheckText`.  If anything fails, `MustLookTool()`
// panics.
func MustLookTool(s ToolSpec) *Tool {
	t, err := LookTool(s)
	if err != nil {
		msg := fmt.Sprintf("%v", err)
		panic(msg)
	}
	return t
}
