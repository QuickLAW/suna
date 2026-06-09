//go:build windows

package mcp

import (
	"os"
	"os/exec"
)

func setProcessGroup(cmd *exec.Cmd) {
	// Windows 没有 Unix 进程组语义，关闭时退化为终止 MCP server 主进程。
}

func killProcessTree(p *os.Process) {
	if p == nil {
		return
	}
	_ = p.Kill()
}
