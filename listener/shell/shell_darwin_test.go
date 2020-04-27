// +build linux darwin dragonfly freebsd netbsd openbsd

package shell

import (
	"github.com/pefish/anywherectl/internal/test"
	"os/exec"
	"testing"
)

func TestExecShell(t *testing.T) {
	cmd := new(exec.Cmd)
	result, err := ExecShell(cmd, `echo "test"`)
	test.Nil(t, err)
	test.Equal(t, "test\n", result)

	cmd = new(exec.Cmd)
	result1, err := ExecShell(cmd, `tetetet && echo $?`)
	test.NotNil(t, err)
	test.Equal(t, "exit status 127", err.Error())
	test.Equal(t, "", result1)

	cmd = new(exec.Cmd)
	result2, err := ExecShell(cmd,`echo "test" && echo $?`)
	test.Nil(t, err)
	test.Equal(t, "test\n0\n", result2)

}

