package main

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var buildPath = "./build/"
var targetPath = buildPath + "bin/"
var packPath = buildPath + "pack/"
var sourcePath = "./"

func main() {
	if len(os.Args) < 2 {
		log.Fatal("build args error")
	}
	err := os.RemoveAll(buildPath)
	if err != nil {
		log.Fatal(err)
	}
	subCmd := os.Args[1]
	if len(os.Args) > 2 {
		sourcePath = os.Args[2]
	}
	if subCmd == "build-cur" {
		mustBuild(runtime.GOOS)
	} else if subCmd == "build-cur-pack" {
		mustBuild(runtime.GOOS)
		mustPack(targetPath, packPath + "release_"+ runtime.GOOS + ".tar.gz")
	} else if subCmd == "build-all" {
		mustBuild("darwin")
		mustBuild("linux")
		mustBuild("windows")
	} else if subCmd == "build-all-pack" {
		mustBuild("darwin")
		mustBuild("linux")
		mustBuild("windows")
		mustPack(targetPath, packPath + "release_all.tar.gz")
	} else {
		log.Fatal("sub command error")
	}
}

func mustBuild(goos string) {
	outputPath := targetPath + goos + "/"
	err := os.MkdirAll(outputPath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", outputPath, "-v", sourcePath + "bin/...")
	goBin, _ := filepath.Abs(targetPath)
	cmd.Env = append(cmd.Env, "GOBIN="+goBin)
	cmd.Env = append(cmd.Env, "GOOS="+goos)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GOBIN=") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}

	mustExec(cmd)
}

func mustPack(targetPath string, dst string) {
	err := os.MkdirAll(filepath.Dir(dst), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	fw, err := os.Create(dst)
	if err != nil {
		log.Fatal(err)
	}
	defer fw.Close()
	tw := tar.NewWriter(fw)
	defer tw.Close()
	err = filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("walk error - %s\n", err)
			return err
		}
		if path == targetPath {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			fmt.Printf("FileInfoHeader error - %s\n", err)
			return err
		}
		hdr.Name = strings.TrimPrefix(path, strings.TrimPrefix(targetPath, "./"))
		if err := tw.WriteHeader(hdr); err != nil {
			fmt.Printf("WriteHeader error - %s\n", err)
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		// 打开文件
		fr, err := os.Open(path)
		defer fr.Close()
		if err != nil {
			fmt.Printf("Open error - %s\n", err)
			return err
		}

		// copy 文件数据到 tw
		_, err = io.Copy(tw, fr)
		if err != nil {
			fmt.Printf("Copy error - %s\n", err)
			return err
		}

		//log.Printf("成功打包 %s ，共写入了 %d 字节的数据\n", path, n)

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func mustExec(cmd *exec.Cmd) {
	fmt.Println(">>>", strings.Join(cmd.Args, " "))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
