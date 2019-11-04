package getent

import (
	"github.com/nogproject/nog/backend/pkg/execx"
)

var getentTool = execx.MustLookTool(execx.ToolSpec{
	Program:   "getent",
	CheckArgs: []string{"--version"},
	CheckText: "getent",
})
