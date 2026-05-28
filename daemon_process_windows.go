//go:build windows

package main

import (
	"os"
	"os/exec"
)

func startBackground(cmd *exec.Cmd) error {
	return cmd.Start()
}

func fallbackStopProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
