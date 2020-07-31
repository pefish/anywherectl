// +build windows

package shell

import (
	"os/exec"
)

func GetCmd(s string) (*exec.Cmd, error) {
	return exec.Command("cmd", "/C", s), nil
}