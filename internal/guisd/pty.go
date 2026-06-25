package guisd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"runtime"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// ptyBufSize 是 PTY 读缓冲大小。32KB 在保留低延迟的同时减少 read/write 系统调用次数。
const ptyBufSize = 32 * 1024

// ptyResizeMsg 是前端发送的终端尺寸调整消息。
type ptyResizeMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// resizeMarker 是 resize 消息中的关键字段子串，用于快速识别消息类型。
// 用 []byte 形式避免每次转换 string。
var resizeMarker = []byte(`"resize"`)

// handlePTY 为一个 WebSocket 连接创建独立的终端会话。
// Unix 系统使用真正的 PTY；Windows 使用命令执行降级（非交互式）。
func handlePTY(conn *websocket.Conn, cwd string) {
	if runtime.GOOS == "windows" {
		handlePTYWindows(conn, cwd)
		return
	}
	handlePTYUnix(conn, cwd)
}

// handlePTYUnix 在 Unix 系统上使用真正的 PTY 终端。
// 每个 WebSocket 连接对应一个独立的 shell 进程，互不干扰。
func handlePTYUnix(conn *websocket.Conn, cwd string) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}

	cmd := exec.Command(shell)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		_ = conn.WriteJSON(map[string]any{"error": "failed to start terminal: " + err.Error()})
		return
	}
	defer func() {
		_ = ptmx.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// goroutine A：PTY 输出 → WebSocket。
	// 读写都用较大缓冲减少系统调用；写失败意味着客户端断连，立即 cancel 让主循环退出。
	go func() {
		buf := make([]byte, ptyBufSize)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.TextMessage, buf[:n]); werr != nil {
					cancel()
					return
				}
			}
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// 主循环：WebSocket 输入 → PTY（解析 resize 消息后转发）
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
			continue
		}
		// 快速识别 resize 消息：检查首字符 + 关键字段子串，避免完整 JSON 解析。
		if isResizeMsg(data) {
			var rm ptyResizeMsg
			if json.Unmarshal(data, &rm) == nil && rm.Type == "resize" {
				_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rm.Rows), Cols: uint16(rm.Cols)})
			}
			continue
		}
		// 普通输入直接写入 PTY
		if _, err := ptmx.Write(data); err != nil {
			break
		}
		// 优先响应 cancel（来自 PTY 关闭 / 写错误）
		select {
		case <-ctx.Done():
			goto cleanup
		default:
		}
	}

cleanup:
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	// 等 shell 真正退出，避免僵尸进程；同时检查退出错误（不影响清理流程）。
	_ = cmd.Wait()
}

// isResizeMsg 快速判断 WebSocket 消息是否是 resize JSON。
// 用 bytes.Contains 跳过 string 转换。
func isResizeMsg(data []byte) bool {
	return len(data) > 0 && data[0] == '{' && bytes.Contains(data, resizeMarker)
}

// handlePTYWindows 在 Windows 上提供命令执行降级（非交互式）。
// 由于 creack/pty 不支持 Windows，这里用 cmd /c 逐行执行命令。
// 后端负责 echo 字符和处理基础编辑（退格），前端无感知。
func handlePTYWindows(conn *websocket.Conn, cwd string) {
	// 通知前端进入非交互模式（前端可据此调整提示）
	_ = conn.WriteJSON(map[string]any{"mode": "non-interactive"})

	// 欢迎横幅
	banner := "\x1b[1;34m╭─────────────────────────────────────╮\x1b[0m\r\n" +
		"\x1b[1;34m│              Suna 终端              │\x1b[0m\r\n" +
		"\x1b[1;34m╰─────────────────────────────────────╯\x1b[0m\r\n" +
		"\x1b[2m[Windows 非交互模式 — 输入命令后按回车执行]\x1b[0m\r\n\r\n"
	_ = conn.WriteMessage(websocket.TextMessage, []byte(banner))

	var lineBuf []byte

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
			continue
		}
		// 忽略 resize 消息（非交互模式下无意义）
		if isResizeMsg(data) {
			continue
		}

		// 逐字节处理：echo + 缓冲 + 命令执行
		for _, b := range data {
			switch {
			case b == '\r' || b == '\n':
				// 换行：执行缓冲中的命令
				_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n"))
				cmdLine := string(bytes.TrimSpace(lineBuf))
				lineBuf = lineBuf[:0]
				if cmdLine == "" {
					continue
				}
				if cmdLine == "exit" || cmdLine == "quit" {
					_ = conn.WriteMessage(websocket.TextMessage, []byte("\x1b[2m[terminal closed]\x1b[0m\r\n"))
					return
				}
				output := execWindowsCommand(cmdLine, cwd)
				_ = conn.WriteMessage(websocket.TextMessage, []byte(output))

			case b == 0x7f || b == '\b':
				// 退格/DEL：删除缓冲最后一个字符，echo "\b \b" 擦除显示
				if len(lineBuf) > 0 {
					lineBuf = lineBuf[:len(lineBuf)-1]
					_ = conn.WriteMessage(websocket.TextMessage, []byte("\b \b"))
				}

			case b >= 0x20:
				// 可打印字符：加入缓冲并 echo
				lineBuf = append(lineBuf, b)
				_ = conn.WriteMessage(websocket.TextMessage, []byte{b})

			default:
				// 其他控制字符（Ctrl+C 等）忽略
			}
		}
	}
}

// execWindowsCommand 在 Windows 上执行命令并返回带 ANSI 着色的输出。
func execWindowsCommand(cmdLine, cwd string) string {
	cmd := exec.Command("cmd", "/c", cmdLine)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	result := string(out)
	if err != nil {
		result += "\r\n\x1b[31m[error: " + err.Error() + "]\x1b[0m"
	}
	return result + "\r\n"
}
