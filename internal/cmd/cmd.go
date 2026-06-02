package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

func Run(src string, tags string) *exec.Cmd {
	return goCmd(src, tags, "run", ".")
}

func Build(src, dest string, tags string) *exec.Cmd {
	outPath := filepath.Join(dest, "app")
	if outPath == "" {
		outPath = filepath.Join(dest, "app")
	}

	return goCmd(src, tags, "build", "-o", outPath, ".")
}

// goCmd creates a configured go command.
func goCmd(dir string, tags string, subCmd string, args ...string) *exec.Cmd {
	cmdArgs := []string{subCmd}

	if tags != "" {
		cmdArgs = append(cmdArgs, "-tags", tags)
	}

	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext( //nolint:gosec // Command is constructed from controlled input
		context.Background(),
		"go",
		cmdArgs...,
	)

	cmd.Dir = dir
	cmd.Env = os.Environ()
	setProcessGroup(cmd)

	return cmd
}
