package paperless

import (
	"enex2paperless/internal/config"
	"net/http"
)

// PaperlessFile represents a file to be uploaded to Paperless-NGX
type PaperlessFile struct {
	Title    string
	FileName string
	MimeType string
	Data     []byte
	Created  string
	Tags     []string
	client   *http.Client
	config   config.Config
	TagIds   []int
}

// NewPaperlessFile creates a new PaperlessFile instance
func NewPaperlessFile(title, fileName, mimeType, created string, data []byte, tags []string, cfg config.Config) *PaperlessFile {
	return &PaperlessFile{
		Title:    title,
		FileName: fileName,
		MimeType: mimeType,
		Data:     data,
		Created:  created,
		Tags:     tags,
		client:   getSharedClient(),
		config:   cfg,
	}
}
