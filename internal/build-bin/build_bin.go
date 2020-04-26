package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var targetPath = "./build/bin/"

func main() {
	if len(os.Args) < 2 {
		log.Fatal("build args error")
	}
	subCmd := os.Args[1]
	if subCmd == "install" {
		mustInstall()
	} else if subCmd == "install-pack" {
		mustInstall()
		mustPack()
	} else {
		log.Fatal("sub command error")
	}
}

func mustInstall() {
	mustExec(exec.Command("rm", "-rf", targetPath))

	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "install")
	goBin, _ := filepath.Abs(targetPath)
	cmd.Env = append(cmd.Env, "GOBIN="+goBin)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GOBIN=") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	if runtime.GOARCH == "arm64" {
		cmd.Args = append(cmd.Args, "-p", "1")
	}
	cmd.Args = append(cmd.Args, "-v")
	cmd.Args = append(cmd.Args, "./bin/...")

	mustExec(cmd)
}

func mustPack() {
	cmd := exec.Command("/bin/bash", "-c", fmt.Sprintf("cd %s && tar -zcvf release.tar.gz ./*", targetPath))

	mustExec(cmd)
}

func mustExec(cmd *exec.Cmd) {
	fmt.Println(">>>", strings.Join(cmd.Args, " "))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
