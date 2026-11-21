// Run from the repository root: go run ./scripts/build-collectors
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func build() error {
	root := "data/releases"
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	for _, target := range []struct{ os, arch, suffix string }{{"linux", "amd64", ""}, {"linux", "arm64", ""}, {"darwin", "arm64", ""}, {"darwin", "amd64", ""}, {"windows", "amd64", ".exe"}} {
		name := "collector-" + target.os + "-" + target.arch + target.suffix
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", filepath.Join(root, name), "./tier1")
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "GOOS=") && !strings.HasPrefix(v, "GOARCH=") && !strings.HasPrefix(v, "CGO_ENABLED=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "GOOS="+target.os, "GOARCH="+target.arch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		fmt.Println("Built", name)
	}
	data, err := os.ReadFile("config/allowlist.yaml")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "allowlist.yaml"), data, 0600); err != nil {
		return err
	}
	fmt.Println("Copied allowlist.yaml")
	return nil
}
