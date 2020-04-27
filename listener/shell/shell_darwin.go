// +build darwin

package shell

import (
	"bufio"
	"fmt"
	"os/exec"
)

func GetCmd(s string) (*exec.Cmd, error) {
	return exec.Command("/bin/bash", "-c", s), nil
}


func ExecShellStream(s string) {
	cmd := exec.Command("/bin/bash", "-c", s)
	reader_, _ := cmd.StdoutPipe()
	cmd.Start()
	reader := bufio.NewReader(reader_)
	for {
		line, _, _ := reader.ReadLine()
		fmt.Println(string(line))
	}
	cmd.Wait()
}
