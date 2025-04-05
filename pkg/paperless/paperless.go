package paperless

import (
	"net/http"
	"time"
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
	TagIds   []int
}

// NewPaperlessFile creates a new PaperlessFile instance
func NewPaperlessFile(title, fileName, mimeType, created string, data []byte, tags []string) *PaperlessFile {
	return &PaperlessFile{
		Title:    title,
		FileName: fileName,
		MimeType: mimeType,
		Data:     data,
		Created:  created,
		Tags:     tags,
		client: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}
