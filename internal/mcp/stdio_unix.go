//go:build !windows

package mcp

import (
	"os"
	"os/exec"
	"syscall"
)

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessTree(p *os.Process) {
	if p == nil {
		return
	}
	// 负 pid 会向进程组发送信号，确保 stdio MCP server 的子进程一起退出。
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
}
