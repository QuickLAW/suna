package ipc

const maxIPCToolResultBytes = 16 * 1024

type ipcDisplayText struct {
	Text      string
	Truncated bool
	Bytes     int
}

func limitIPCToolResult(s string) ipcDisplayText {
	if len(s) <= maxIPCToolResultBytes {
		return ipcDisplayText{Text: s, Bytes: len(s)}
	}
	return ipcDisplayText{
		Text:      truncateUTF8(s, maxIPCToolResultBytes),
		Truncated: true,
		Bytes:     len(s),
	}
}

func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	end := 0
	for i := range s {
		if i > maxBytes {
			break
		}
		end = i
	}
	if end == 0 {
		return ""
	}
	return s[:end]
}
