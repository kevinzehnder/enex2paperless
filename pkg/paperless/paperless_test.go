package paperless

import (
	"enex2paperless/internal/config"
	"testing"
)

// TestNewPaperlessFile tests the NewPaperlessFile constructor
func TestNewPaperlessFile(t *testing.T) {
	testCases := []struct {
		name     string
		title    string
		fileName string
		mimeType string
		created  string
		data     []byte
		tags     []string
		config   config.Config
	}{
		{
			name:     "basic pdf file",
			title:    "Test Document",
			fileName: "test.pdf",
			mimeType: "application/pdf",
			created:  "2022-01-01 12:00:00+00:00",
			data:     []byte("test data"),
			tags:     []string{"tag1", "tag2"},
			config: config.Config{
				PaperlessAPI: "http://localhost:8000",
				Token:        "test-token",
			},
		},
		{
			name:     "file with no tags",
			title:    "No Tags Document",
			fileName: "notags.pdf",
			mimeType: "application/pdf",
			created:  "2022-01-01 12:00:00+00:00",
			data:     []byte("test data"),
			tags:     []string{},
			config: config.Config{
				PaperlessAPI: "http://localhost:8000",
				Token:        "test-token",
			},
		},
		{
			name:     "empty data",
			title:    "Empty File",
			fileName: "empty.txt",
			mimeType: "text/plain",
			created:  "2022-01-01 12:00:00+00:00",
			data:     []byte{},
			tags:     []string{"empty"},
			config: config.Config{
				PaperlessAPI: "http://localhost:8000",
				Token:        "test-token",
			},
		},
		{
			name:     "image file",
			title:    "Test Image",
			fileName: "photo.jpg",
			mimeType: "image/jpeg",
			created:  "2022-01-01 12:00:00+00:00",
			data:     []byte("fake jpeg data"),
			tags:     []string{"photo", "vacation"},
			config: config.Config{
				PaperlessAPI: "http://localhost:8000",
				Token:        "test-token",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pf := NewPaperlessFile(
				tc.title,
				tc.fileName,
				tc.mimeType,
				tc.created,
				tc.data,
				tc.tags,
				tc.config,
			)

			if pf.Title != tc.title {
				t.Errorf("Title = %q, expected %q", pf.Title, tc.title)
			}

			if pf.FileName != tc.fileName {
				t.Errorf("FileName = %q, expected %q", pf.FileName, tc.fileName)
			}

			if pf.MimeType != tc.mimeType {
				t.Errorf("MimeType = %q, expected %q", pf.MimeType, tc.mimeType)
			}

			if pf.Created != tc.created {
				t.Errorf("Created = %q, expected %q", pf.Created, tc.created)
			}

			if len(pf.Data) != len(tc.data) {
				t.Errorf("Data length = %d, expected %d", len(pf.Data), len(tc.data))
			}

			if len(pf.Tags) != len(tc.tags) {
				t.Errorf("Tags length = %d, expected %d", len(pf.Tags), len(tc.tags))
			}

			for i, tag := range tc.tags {
				if pf.Tags[i] != tag {
					t.Errorf("Tag[%d] = %q, expected %q", i, pf.Tags[i], tag)
				}
			}

			if pf.client == nil {
				t.Error("client should not be nil")
			}

			// TagIds is initialized as nil and populated later via processTags
			if len(pf.TagIds) != 0 {
				t.Error("TagIds should be empty initially")
			}
		})
	}
}
