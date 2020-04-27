// +build windows

package shell

import (
	"bytes"
	"os/exec"
)

func ExecShell(cmd *exec.Cmd, s string) (string, error) {
	*cmd = *exec.Command("cmd", "/C", s)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		return "", err
	}
	err := cmd.Wait()
	if err != nil {
		return ``, err
	}
	return out.String(), err
}

