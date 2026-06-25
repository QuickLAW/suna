package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/alanchenchen/suna/internal/guisd"
)

// runGUI 启动 GUI companion 进程：HTTP 服务 + 前端 + daemon 代理。
// 文件浏览和终端不依赖 daemon；Agent 聊天需要 daemon 在线。
func runGUI() {
	// 尝试确保 daemon 在运行（Agent 聊天需要）。失败时仍可使用文件和终端。
	ensureDaemonRunning()

	// 连接 daemon；连接失败不阻塞启动，只是禁用聊天功能。
	var proxy *guisd.AgentProxy
	if p, err := guisd.NewAgentProxy(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: daemon not available: %s\n", err)
		fmt.Fprintf(os.Stderr, "File browser and terminal will work. Chat features disabled.\n\n")
	} else {
		proxy = p
		defer proxy.Close()
	}

	// 使用随机可用端口，避免冲突。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot listen: %s\n", err)
		os.Exit(1)
	}
	addr := listener.Addr().String()

	server := guisd.NewServer(addr, proxy)
	url := "http://" + addr

	// 尝试自动打开浏览器。
	openBrowser(url)
	fmt.Printf("Suna GUI is running at %s\n", url)
	fmt.Print("Press Ctrl+C to stop.\n\n")

	// 等待中断信号。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down GUI...")
		listener.Close()
	}()

	if err := server.Serve(listener); err != nil {
		// listener 关闭后返回的错误是正常的退出。
	}
}

// openBrowser 尝试用系统默认浏览器打开 URL。失败时静默忽略。
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}
