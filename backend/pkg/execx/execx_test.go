package execx_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nogproject/nog/backend/pkg/execx"
)

func TestLookToolSuccess(t *testing.T) {
	ls, err := execx.LookTool(execx.ToolSpec{
		Program:   "ls",
		CheckArgs: []string{"--version"},
		CheckText: "ls",
	})
	if err != nil {
		t.Fatalf("`LookTool()` failed: %v", err)
	}
	txt := "/bin/ls"
	if ls.Path != txt {
		t.Errorf("Expected path `%s`; got `%s`", txt, ls.Path)
	}
}

func TestLookToolFail(t *testing.T) {
	var (
		err error
		txt string
	)

	txt = "failed to find"
	_, err = execx.LookTool(execx.ToolSpec{
		Program:   "invalid",
		CheckArgs: []string{"--version"},
		CheckText: "ls",
	})
	if err == nil {
		t.Error("Expected error.")
	}
	if !strings.Contains(fmt.Sprintf("%v", err), txt) {
		t.Errorf("Expected error text `%s`; got `%v`.", txt, err)
	}

	txt = "failed to execute"
	_, err = execx.LookTool(execx.ToolSpec{
		Program:   "ls",
		CheckArgs: []string{"--invalid"},
		CheckText: "ls",
	})
	if err == nil {
		t.Error("Expected error.")
	}
	if !strings.Contains(fmt.Sprintf("%v", err), txt) {
		t.Errorf("Expected error text `%s`; got `%v`.", txt, err)
	}

	txt = "did not print"
	_, err = execx.LookTool(execx.ToolSpec{
		Program:   "ls",
		CheckArgs: []string{"--version"},
		CheckText: "invalid",
	})
	if err == nil {
		t.Error("Expected error.")
	}
	if !strings.Contains(fmt.Sprintf("%v", err), txt) {
		t.Errorf("Expected error text `%s`; got `%v`.", txt, err)
	}
}
