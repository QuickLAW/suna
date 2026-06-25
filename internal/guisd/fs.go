package guisd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// maxUploadBytes 限制单次文件写入的 body 大小，避免内存耗尽。
const maxUploadBytes = 10 * 1024 * 1024 // 10MB

// FileEntry 是文件树中的一个节点。
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// alwaysHiddenDirs 总是隐藏的目录名，避免在文件树中展示噪音。
var alwaysHiddenDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"__pycache__":  true,
	".DS_Store":    true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".nuxt":        true,
	"target":       true,
}

// resolveWorkspacePath 将用户传入的路径解析为绝对路径，并确保不逃逸 workspace。
// 相对路径基于 workspace（Server.cwd）解析；~ 展开为用户主目录但仍受 workspace 约束。
// 返回的路径是清洗后的绝对路径。如果路径逃逸 workspace 返回错误。
func (s *Server) resolveWorkspacePath(path string) (string, error) {
	if path == "" {
		return s.cwd, nil
	}
	// 展开 ~ 前缀
	path = expandUserPath(path)
	// 相对路径基于 workspace 解析
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.cwd, path)
	}
	path = filepath.Clean(path)
	// 检查是否在 workspace 内
	if !isPathInside(s.cwd, path) {
		return "", &fsError{Code: http.StatusForbidden, Msg: "path outside workspace: " + path}
	}
	return path, nil
}

// isPathInside 判断 target 是否在 root 目录内（含 root 自身）。
func isPathInside(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel)
}

// loadGitignore 读取 dir 下的 .gitignore 文件（如果存在），解析为规则。
func loadGitignore(dir string) gitignoreRules {
	var rules gitignoreRules
	f, err := os.Open(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return rules
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 跳过否定模式（!开头），简化处理
		if strings.HasPrefix(line, "!") {
			continue
		}
		rules.patterns = append(rules.patterns, line)
	}
	return rules
}

// gitignoreRules 保存一个目录下 .gitignore 的简化匹配规则。
type gitignoreRules struct {
	patterns []string // 简单 glob 模式
}

// match 判断给定名称是否被 .gitignore 规则匹配。
func (r gitignoreRules) match(name string, isDir bool) bool {
	for _, p := range r.patterns {
		// 目录限定模式（以 / 结尾）
		if strings.HasSuffix(p, "/") {
			if !isDir {
				continue
			}
			p = strings.TrimSuffix(p, "/")
		}
		// 去掉前导 /
		p = strings.TrimPrefix(p, "/")
		// 精确匹配
		if p == name {
			return true
		}
		// path.Match glob 匹配
		if ok, _ := filepath.Match(p, name); ok {
			return true
		}
	}
	return false
}

// fsWalker 抽象文件遍历的状态：当前目录、累积规则、上限与回调。
// 抽出来后 list / search 复用同一份遍历逻辑，避免 DRY 违反。
type fsWalker struct {
	cwd      string
	maxFiles int // 最多遍历的文件数（list）
	skipDirs map[string]bool
	onEntry  func(relPath string, fullPath string, isDir bool) bool
	stopped  bool
	visited  int
}

// walk 通用遍历入口。
func (w *fsWalker) walk(root string) {
	rootRules := loadGitignore(root)
	w.walkDir(root, rootRules)
}

func (w *fsWalker) walkDir(dir string, parentRules gitignoreRules) {
	if w.stopped {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if w.stopped {
			return
		}
		name := entry.Name()
		if w.skipDirs[name] {
			continue
		}
		if parentRules.match(name, entry.IsDir()) {
			continue
		}
		full := filepath.Join(dir, name)
		if entry.IsDir() {
			childRules := loadGitignore(full)
			merged := gitignoreRules{patterns: append(append([]string{}, parentRules.patterns...), childRules.patterns...)}
			// 调用方可以选择在子目录继续；目录也参与计数（出于统计目的）。
			w.walkDir(full, merged)
			continue
		}
		rel, err := filepath.Rel(w.cwd, full)
		if err != nil {
			rel = full
		}
		if !w.onEntry(filepath.ToSlash(rel), full, false) {
			w.stopped = true
			return
		}
	}
}

// ===== HTTP handlers =====

func (s *Server) handleFileTree(w http.ResponseWriter, r *http.Request) {
	root := r.URL.Query().Get("path")
	if root == "" {
		root = s.cwd
	}
	resolved, err := s.resolveWorkspacePath(root)
	if err != nil {
		writeFSError(w, err)
		return
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSONError(w, http.StatusNotFound, "directory not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 读取该目录下的 .gitignore 规则
	ignoreRules := loadGitignore(resolved)

	result := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		// 跳过始终隐藏的目录/文件
		if alwaysHiddenDirs[name] {
			continue
		}
		// 跳过 .gitignore 匹配的条目
		if ignoreRules.match(name, entry.IsDir()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, FileEntry{
			Name:    name,
			Path:    filepath.Join(resolved, name),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
		})
	}

	// 排序：目录在前，然后按名称（不区分大小写）
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	writeJSON(w, result)
}

func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSONError(w, http.StatusBadRequest, "path is required")
		return
	}
	resolved, err := s.resolveWorkspacePath(path)
	if err != nil {
		writeFSError(w, err)
		return
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSONError(w, http.StatusNotFound, "file not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if info.IsDir() {
		writeJSONError(w, http.StatusBadRequest, "path is a directory")
		return
	}

	// 限制读取大小，避免加载超大文件
	if info.Size() > 10*1024*1024 {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "file too large (max 10MB)")
		return
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"path":    resolved,
		"content": string(data),
		"size":    len(data),
		"modTime": info.ModTime().Format("2006-01-02 15:04:05"),
	})
}

// decodeJSONBody 解析请求体并处理 body 过大 / 解析错误等常见情况，
// 把底层 http.MaxBytesError 也归一为 413，避免给前端透出无意义错误。
func decodeJSONBody(w http.ResponseWriter, r *http.Request, max int64, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, max)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

// handleFileWrite 写入文件内容（原子写入：先写临时文件再 rename）。
func (s *Server) handleFileWrite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if !decodeJSONBody(w, r, maxUploadBytes, &req) {
		return
	}
	if req.Path == "" {
		writeJSONError(w, http.StatusBadRequest, "path is required")
		return
	}
	resolved, err := s.resolveWorkspacePath(req.Path)
	if err != nil {
		writeFSError(w, err)
		return
	}

	if err := atomicWriteFile(resolved, []byte(req.Content), 0644); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

// atomicWriteFile 原子性地写入文件：先写入同目录临时文件，再 rename 覆盖目标。
// 避免写入中途崩溃导致文件损坏。
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".suna-write-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	// 无论成功与否，清理临时文件
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// fsError 携带 HTTP 状态码的文件操作错误。
type fsError struct {
	Code int
	Msg  string
}

func (e *fsError) Error() string { return e.Msg }

// expandUserPath 展开 ~ 和 ~user 路径前缀。
func expandUserPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			if strings.HasPrefix(path, "~/") {
				return filepath.Join(home, path[2:])
			}
		}
	}
	return path
}

// ===== 文件操作 API（create/delete/rename）=====

// handleFileCreate 创建文件或目录。
// 请求体 JSON: {path, isDir}。如果目标已存在返回 409。
func (s *Server) handleFileCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path  string `json:"path"`
		IsDir bool   `json:"isDir"`
	}
	if !decodeJSONBody(w, r, 1024, &req) {
		return
	}
	if req.Path == "" {
		writeJSONError(w, http.StatusBadRequest, "path is required")
		return
	}
	resolved, err := s.resolveWorkspacePath(req.Path)
	if err != nil {
		writeFSError(w, err)
		return
	}
	// 已存在则冲突
	if _, err := os.Stat(resolved); err == nil {
		writeJSONError(w, http.StatusConflict, "path already exists")
		return
	}
	if req.IsDir {
		if err := os.MkdirAll(resolved, 0755); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		if err := atomicWriteFile(resolved, []byte{}, 0644); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, map[string]string{"status": "ok", "path": resolved})
}

// handleFileDelete 删除文件或目录（目录递归删除）。
// 请求体 JSON: {path}。
func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if !decodeJSONBody(w, r, 1024, &req) {
		return
	}
	if req.Path == "" {
		writeJSONError(w, http.StatusBadRequest, "path is required")
		return
	}
	resolved, err := s.resolveWorkspacePath(req.Path)
	if err != nil {
		writeFSError(w, err)
		return
	}
	// 禁止删除 workspace 根目录本身
	if samePath(resolved, s.cwd) {
		writeJSONError(w, http.StatusForbidden, "cannot delete workspace root")
		return
	}
	if err := os.RemoveAll(resolved); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// handleFileRename 移动/重命名文件或目录。
// 请求体 JSON: {oldPath, newPath}。
func (s *Server) handleFileRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPath string `json:"oldPath"`
		NewPath string `json:"newPath"`
	}
	if !decodeJSONBody(w, r, 4096, &req) {
		return
	}
	if req.OldPath == "" || req.NewPath == "" {
		writeJSONError(w, http.StatusBadRequest, "oldPath and newPath are required")
		return
	}
	oldResolved, err := s.resolveWorkspacePath(req.OldPath)
	if err != nil {
		writeFSError(w, err)
		return
	}
	newResolved, err := s.resolveWorkspacePath(req.NewPath)
	if err != nil {
		writeFSError(w, err)
		return
	}
	// 禁止移动 workspace 根目录
	if samePath(oldResolved, s.cwd) {
		writeJSONError(w, http.StatusForbidden, "cannot move workspace root")
		return
	}
	// 确保目标父目录存在
	if err := os.MkdirAll(filepath.Dir(newResolved), 0755); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.Rename(oldResolved, newResolved); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "newPath": newResolved})
}

// ===== 文件列表 API（Ctrl+P 快速打开）=====

// maxListResults 限制 /api/fs/list 返回的文件数量，避免超大项目响应过慢。
const maxListResults = 500

// handleFileList 递归列出 workspace 内所有文件路径（不含目录），用于前端 Ctrl+P 模糊搜索。
// 排除 alwaysHiddenDirs 和 .gitignore 匹配的条目。
func (s *Server) handleFileList(w http.ResponseWriter, r *http.Request) {
	results := make([]string, 0, 128)
	walker := &fsWalker{
		cwd:      s.cwd,
		skipDirs: alwaysHiddenDirs,
		onEntry: func(relPath, _ string, _ bool) bool {
			results = append(results, relPath)
			return len(results) < maxListResults
		},
	}
	walker.walk(s.cwd)
	sort.Strings(results)
	writeJSON(w, map[string]any{"files": results, "truncated": len(results) >= maxListResults})
}

// ===== 全局搜索 API（Ctrl+Shift+F）=====

// maxSearchMatches 限制单文件最大匹配数，避免超大文件刷屏。
const maxSearchMatches = 200

// maxSearchFileSize 限制搜索时单文件最大字节数（2MB），跳过二进制大文件。
const maxSearchFileSize = 2 * 1024 * 1024

// SearchMatch 是一次搜索命中的结果。
type SearchMatch struct {
	Path    string      `json:"path"`
	Matches []SearchHit `json:"matches"`
}

// SearchHit 是单条匹配的行号和内容。
type SearchHit struct {
	Line    int    `json:"line"`    // 1-based 行号
	Column  int    `json:"column"`  // 0-based 列号
	Preview string `json:"preview"` // 该行内容（已 trim）
}

// handleFileSearch 在 workspace 内全局搜索文件内容。
// 查询参数: query（必填）、isRegex（可选，默认 false）、ignoreCase（可选，默认 true）。
func (s *Server) handleFileSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		writeJSONError(w, http.StatusBadRequest, "query is required")
		return
	}
	isRegex := r.URL.Query().Get("isRegex") == "true"
	ignoreCase := r.URL.Query().Get("ignoreCase") != "false" // 默认 true

	// 构建正则
	pattern := query
	if ignoreCase && !isRegex {
		pattern = "(?i)" + regexp.QuoteMeta(query)
	} else if !isRegex {
		pattern = regexp.QuoteMeta(query)
	} else if ignoreCase {
		pattern = "(?i)" + query
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid regex: "+err.Error())
		return
	}

	results := make([]SearchMatch, 0, 32)
	walker := &fsWalker{
		cwd:      s.cwd,
		skipDirs: alwaysHiddenDirs,
		onEntry: func(relPath, fullPath string, _ bool) bool {
			info, err := os.Stat(fullPath)
			if err != nil || info.Size() > maxSearchFileSize {
				return true
			}
			hits := searchInFile(fullPath, re)
			if len(hits) > 0 {
				results = append(results, SearchMatch{Path: relPath, Matches: hits})
			}
			return true
		},
	}
	walker.walk(s.cwd)
	writeJSON(w, map[string]any{"results": results, "total": len(results)})
}

// searchInFile 在单个文件中搜索正则匹配，返回最多 maxSearchMatches 条命中。
func searchInFile(path string, re *regexp.Regexp) []SearchHit {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// 简单二进制检测：包含 NULL 字节则跳过
	if bytes.IndexByte(data, 0) >= 0 {
		return nil
	}
	hits := make([]SearchHit, 0, 8)
	lineNum := 0
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		lineNum++
		if len(hits) >= maxSearchMatches {
			break
		}
		loc := re.FindIndex(line)
		if loc != nil {
			preview := strings.TrimSpace(string(line))
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			hits = append(hits, SearchHit{
				Line:    lineNum,
				Column:  loc[0],
				Preview: preview,
			})
		}
	}
	return hits
}

// samePath 跨平台判断两个路径是否相同（Windows 不区分大小写）。
func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if filepath.Separator == '\\' {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// writeFSError 统一处理 resolveWorkspacePath 返回的错误。
func writeFSError(w http.ResponseWriter, err error) {
	if fe, ok := err.(*fsError); ok {
		writeJSONError(w, fe.Code, fe.Msg)
		return
	}
	writeJSONError(w, http.StatusBadRequest, err.Error())
}
