package attachment

import "github.com/alanchenchen/suna/internal/protocol"

const MaxPastedImageBytes = 10 * 1024 * 1024

type Item struct {
	Type       string
	SourceKind string
	Path       string
	URL        string
	Name       string
	MimeType   string
	Size       int64
}

type PendingImagePaste struct {
	Raw        string
	SourceKind string
	Path       string
	URL        string
	Name       string
	MimeType   string
	Size       int64
	Data       []byte
}

func (a Item) ToPart() protocol.MessagePart {
	return protocol.MessagePart{Type: "image", Source: protocol.AttachmentRef{Kind: a.SourceKind, Path: a.Path, URL: a.URL, MimeType: a.MimeType, Name: a.Name, Size: a.Size}}
}
