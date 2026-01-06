package enex

import (
	"enex2paperless/internal/config"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/spf13/afero"
)

type EnexFile struct {
	EnExport
	Fs                afero.Fs
	client            *http.Client
	config            config.Config
	NumNotes, Uploads atomic.Uint32
	NoteChannel       chan Note
	FailedNoteChannel chan Note
	FailedNoteSignal  chan bool
	FilePath          string
}

func NewEnexFile(filePath string, cfg config.Config) *EnexFile {
	return &EnexFile{
		Fs: afero.NewOsFs(),
		client: &http.Client{
			Timeout: time.Second * 10,
		},
		config:            cfg,
		NoteChannel:       make(chan Note),
		FailedNoteChannel: make(chan Note),
		FailedNoteSignal:  make(chan bool),
		FilePath:          filePath,
	}
}

type EnExport struct {
	ExportDate  string `xml:"export-date,attr"`
	Application string `xml:"application,attr"`
	Version     string `xml:"version,attr"`
	Notes       []Note `xml:"note"`
}

type Note struct {
	Title          string     `xml:"title"`
	Content        string     `xml:"content"`
	Created        string     `xml:"created"`
	Updated        string     `xml:"updated"`
	Tags           []string   `xml:"tag"`
	NoteAttributes NoteAttr   `xml:"note-attributes"`
	Resources      []Resource `xml:"resource"`
}

type NoteAttr struct {
	Location string `xml:"location"`
	// Add other attributes here
}

type Resource struct {
	Data               string             `xml:"data"`
	Mime               string             `xml:"mime"`
	Width              int                `xml:"width,omitempty"`
	Height             int                `xml:"height,omitempty"`
	ResourceAttributes ResourceAttributes `xml:"resource-attributes"`
}

type ResourceAttributes struct {
	SourceURL       string  `xml:"source-url,omitempty"`
	Timestamp       string  `xml:"timestamp,omitempty"`
	FileName        string  `xml:"file-name,omitempty"`
	Latitude        float64 `xml:"latitude,omitempty"`
	Longitude       float64 `xml:"longitude,omitempty"`
	Altitude        float64 `xml:"altitude,omitempty"`
	CameraMake      string  `xml:"camera-make,omitempty"`
	CameraModel     string  `xml:"camera-model,omitempty"`
	Attachment      bool    `xml:"attachment,omitempty"`
	ApplicationData string  `xml:"application-data,omitempty"`
}
