// +build darwin linux

package shell

import (
	"os/exec"
)

func GetCmd(s string) (*exec.Cmd, error) {
	return exec.Command("/cmd/bash", "-c", s), nil
}
