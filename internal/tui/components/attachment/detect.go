package attachment

import (
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/alanchenchen/suna/internal/protocol"
)

func DetectImagePaste(raw string) (PendingImagePaste, bool, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return PendingImagePaste{}, false, false
	}
	if p, ok := detectDataImageURI(trimmed); ok {
		return p, true, false
	}
	if LooksLikeLargeBase64(trimmed) {
		return PendingImagePaste{}, false, true
	}
	if p, ok := detectImageURL(trimmed); ok {
		p.Raw = raw
		return p, true, false
	}
	if p, ok := detectImagePath(trimmed); ok {
		p.Raw = raw
		return p, true, false
	}
	return PendingImagePaste{}, false, false
}

func detectImagePath(raw string) (PendingImagePaste, bool) {
	path := strings.Trim(raw, "'\"")
	path = strings.ReplaceAll(path, "\\ ", " ")
	path = expandTilde(path)
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || !IsImageName(path) {
		return PendingImagePaste{}, false
	}
	return PendingImagePaste{SourceKind: protocol.AttachmentKindPath, Path: path, Name: filepath.Base(path), MimeType: ImageMimeFromName(path), Size: info.Size()}, true
}

func detectImageURL(raw string) (PendingImagePaste, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || !IsImageName(u.Path) {
		return PendingImagePaste{}, false
	}
	name := filepath.Base(u.Path)
	if name == "." || name == "/" || name == "" {
		name = "remote-image"
	}
	return PendingImagePaste{SourceKind: protocol.AttachmentKindURL, URL: raw, Name: name, MimeType: ImageMimeFromName(u.Path)}, true
}

func detectDataImageURI(raw string) (PendingImagePaste, bool) {
	if !strings.HasPrefix(raw, "data:image/") {
		return PendingImagePaste{}, false
	}
	idx := strings.Index(raw, ";base64,")
	if idx < 0 {
		return PendingImagePaste{}, false
	}
	mimeType := raw[len("data:"):idx]
	encoded := raw[idx+len(";base64,"):]
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > MaxPastedImageBytes {
		return PendingImagePaste{}, false
	}
	ext := ExtFromMime(mimeType)
	return PendingImagePaste{SourceKind: "data_uri", Name: "pasted-image" + ext, MimeType: mimeType, Size: int64(len(data)), Data: data}, true
}

func IsImageName(name string) bool { return ImageMimeFromName(name) != "" }

func ImageMimeFromName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return ""
	}
}

func ExtFromMime(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func LooksLikeLargeBase64(s string) bool {
	if len(s) < 1024 || strings.ContainsAny(s, " \n\t") {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '+' && r != '/' && r != '=' {
			return false
		}
	}
	return true
}
