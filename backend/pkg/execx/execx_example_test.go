package execx_test

import (
	"fmt"
	"os/exec"

	"github.com/nogproject/nog/backend/pkg/execx"
)

var bash = execx.MustLookTool(execx.ToolSpec{
	Program:   "bash",
	CheckArgs: []string{"--version"},
	CheckText: "GNU bash",
})

func Example() {
	out, _ := exec.Command(bash.Path, "-c", "echo hello").Output()
	fmt.Println(string(out))
	// Output:
	// hello
}
