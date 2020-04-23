package listener

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
)

// 阻塞式执行shell命令，等待执行完毕并返回结果
func ExecShell(s string) (string, error) {
	cmd := exec.Command("/bin/bash", "-c", s)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return ``, err
	}
	return out.String(), err
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
