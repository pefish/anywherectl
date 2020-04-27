// +build linux

package shell

import (
	"os/exec"
)

func GetCmd(s string) (*exec.Cmd, error) {
	return exec.Command("/bin/bash", "-c", s), nil
}