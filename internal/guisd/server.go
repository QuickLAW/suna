package guisd

import (
	"bufio"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/alanchenchen/suna/internal/logging"
)

//go:embed ui/*
var uiFS embed.FS

// upgrader 校验 Origin 头，防止跨站 WebSocket 劫持（CSWSH）。
// 只允许来自同一主机的请求（浏览器导航到 guisd 后从前端发起的 WS）。
var upgrader = websocket.Upgrader{
	CheckOrigin: checkWSOrigin,
}

// checkWSOrigin 校验 WebSocket 请求的 Origin 是否与 Host 同源。
// 拒绝来自任意第三方页面的 WebSocket 连接。
func checkWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// 非浏览器客户端（如 curl）无 Origin 头，允许通过
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	// 比较 host:port（不区分大小写）
	return strings.EqualFold(u.Host, r.Host)
}

// Server 是 guisd 的 HTTP 服务器，提供文件操作、PTY 终端和 Agent 聊天的 HTTP/WebSocket 接口。
type Server struct {
	proxy *AgentProxy
	cwd   string
	addr  string
}

// NewServer 创建 HTTP 服务器实例。
func NewServer(addr string, proxy *AgentProxy) *Server {
	cwd, _ := os.Getwd()
	return &Server{proxy: proxy, cwd: cwd, addr: addr}
}

// Handler 返回配置好路由的 http.Handler。
// 静态前端通过 http.FileServer 提供，避免手写路径拼接与 MIME 判断。
// 同时为嵌入资源加上 ETag / Cache-Control 友好的头信息。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// 静态前端：把 ui 子目录作为只读文件系统交给 http.FileServer。
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		// embed.FS 子目录不可能失败，保留 panic 作为启动期不变量保护。
		panic("guisd: embedded ui subdir missing: " + err.Error())
	}
	staticHandler := withCacheControl(http.FileServer(http.FS(sub)))
	mux.Handle("/", staticHandler)

	// 文件操作 API
	mux.HandleFunc("/api/fs/tree", method("GET", s.handleFileTree))
	mux.HandleFunc("/api/fs/read", method("GET", s.handleFileRead))
	mux.HandleFunc("/api/fs/write", method("POST", s.handleFileWrite))
	mux.HandleFunc("/api/fs/list", method("GET", s.handleFileList))
	mux.HandleFunc("/api/fs/search", method("GET", s.handleFileSearch))
	mux.HandleFunc("/api/fs/create", method("POST", s.handleFileCreate))
	mux.HandleFunc("/api/fs/delete", method("POST", s.handleFileDelete))
	mux.HandleFunc("/api/fs/rename", method("POST", s.handleFileRename))

	// WebSocket 路由：不过 method 中间件。
	mux.HandleFunc("/api/pty", s.handlePTYWS)
	mux.HandleFunc("/api/agent", s.handleAgentWS)

	// daemon 连接状态
	mux.HandleFunc("/api/status", method("GET", s.handleStatus))

	return loggingMiddleware(mux)
}

// loggingMiddleware 记录非静态请求的耗时与状态码，便于排查性能与错误。
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/static/") {
			// 静态资源不打日志（前端资源高频，重复日志反而掩盖异常）。
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		logging.Info("guisd", "http",
			logging.Event{"method": r.Method, "path": r.URL.Path, "status": rw.status, "ms": time.Since(start).Milliseconds()})
	})
}

// statusRecorder 捕获下游写入了什么状态码，用于日志。
// 同时为常见的接口（http.Hijacker / http.Flusher / http.CloseNotifier）显式 delegate，
// 否则把 http.ResponseWriter 接口值嵌进去时，方法提升只看接口本身，
// 不看运行时具体类型 *http.response 的额外方法集，
// 会导致 WebSocket 升级拿到 "response does not implement http.Hijacker"。
// 任何想把 ResponseWriter 包一层做日志/指标的人都会踩这个坑。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response does not implement http.Hijacker")
	}
	return hj.Hijack()
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// method 限制 handler 只接受指定方法，其他返回 405。
func method(m string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != m {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h(w, r)
	}
}

// withCacheControl 给静态资源加上简单的 ETag/Cache-Control，减少重复下载。
// 资源嵌入二进制本身不会变，浏览器可以放心长缓存。
func withCacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300, must-revalidate")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		h.ServeHTTP(w, r)
	})
}

// ListenAndServe 启动 HTTP 服务器。
func (s *Server) ListenAndServe() error {
	return http.ListenAndServe(s.addr, s.Handler())
}

// Addr 返回监听地址。
func (s *Server) Addr() string { return s.addr }

// Serve 在指定 listener 上提供 HTTP 服务。
func (s *Server) Serve(l net.Listener) error {
	return http.Serve(l, s.Handler())
}

func (s *Server) handlePTYWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Error("guisd", "pty_ws_upgrade_failed", err, logging.Event{"remote": r.RemoteAddr})
		return
	}
	defer conn.Close()
	handlePTY(conn, s.cwd)
}

func (s *Server) handleAgentWS(w http.ResponseWriter, r *http.Request) {
	if s.proxy == nil || !s.proxy.Connected() {
		http.Error(w, `{"error":"daemon not connected"}`, http.StatusServiceUnavailable)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Error("guisd", "agent_ws_upgrade_failed", err, logging.Event{"remote": r.RemoteAddr})
		return
	}
	defer conn.Close()
	s.proxy.HandleWebSocket(conn)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"daemonConnected": s.proxy != nil && s.proxy.Connected(),
		"cwd":             s.cwd,
	})
}

// writeJSON 写入 JSON 响应。
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logging.Error("guisd", "write_json_error", err, logging.Event{})
	}
}

// writeJSONError 写入 JSON 错误响应。
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
