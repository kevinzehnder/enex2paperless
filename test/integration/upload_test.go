//go:build integration

package integration

import (
	"enex2paperless/internal/config"
	"enex2paperless/pkg/enex"
	"testing"
	"time"
)

func TestDocumentProcessing(t *testing.T) {
	tests := []struct {
		name                  string
		enexFile              string
		configModifier        func(*config.Config)
		processOpts           enex.ProcessOptions
		minNotesExpected      int
		minUploadsExpected    int
		verifyDocument        *documentVerification
		skipVerifyInPaperless bool
	}{
		{
			name:     "basic document upload",
			enexFile: "test.enex",
			processOpts: enex.ProcessOptions{
				ConcurrentWorkers: 1,
				OutputFolder:      "",
				RetryPromptFunc:   nil,
			},
			minNotesExpected:   1,
			minUploadsExpected: 1,
			verifyDocument: &documentVerification{
				title: "Test PDF Note",
				tags:  []string{"SampleTag"},
			},
		},
		{
			name:     "document with multiple tags",
			enexFile: "test.enex",
			configModifier: func(cfg *config.Config) {
				cfg.AdditionalTags = []string{"IntegrationTest", "AutomatedUpload"}
			},
			processOpts: enex.ProcessOptions{
				ConcurrentWorkers: 1,
				OutputFolder:      "",
				RetryPromptFunc:   nil,
			},
			minNotesExpected:   1,
			minUploadsExpected: 1,
			verifyDocument: &documentVerification{
				title: "Test PDF Note",
				tags:  []string{"SampleTag", "IntegrationTest", "AutomatedUpload"},
			},
		},
		{
			name:     "file type filtering - PDF only",
			enexFile: "filetypes.enex",
			configModifier: func(cfg *config.Config) {
				cfg.FileTypes = []string{"pdf"}
			},
			processOpts: enex.ProcessOptions{
				ConcurrentWorkers: 1,
				OutputFolder:      "",
				RetryPromptFunc:   nil,
			},
			minNotesExpected:      1,
			minUploadsExpected:    1,
			skipVerifyInPaperless: true,
		},
		{
			name:     "zip file processing",
			enexFile: "zip.enex",
			configModifier: func(cfg *config.Config) {
				cfg.FileTypes = []string{"any"}
			},
			processOpts: enex.ProcessOptions{
				ConcurrentWorkers: 1,
				OutputFolder:      "",
				RetryPromptFunc:   nil,
			},
			minNotesExpected:      1,
			minUploadsExpected:    1,
			skipVerifyInPaperless: true,
		},
		{
			name:     "concurrent uploads",
			enexFile: "test.enex",
			processOpts: enex.ProcessOptions{
				ConcurrentWorkers: 3,
				OutputFolder:      "",
				RetryPromptFunc:   nil,
			},
			minNotesExpected:   1,
			minUploadsExpected: 1,
			verifyDocument: &documentVerification{
				title: "Test PDF Note",
				tags:  []string{"SampleTag"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// get test config
			cfg := GetTestConfig(t)
			SkipIfPaperlessUnavailable(t, cfg)

			// apply config modifications
			if tt.configModifier != nil {
				tt.configModifier(&cfg)
			}

			client := GetPaperlessClient(t, cfg)

			// cleanup now and after test
			CleanupTestInstance(t, client)
			defer CleanupTestInstance(t, client)

			// Create EnexFile with injected config
			enexPath := GetAssetPath(tt.enexFile)
			enexFile := enex.NewEnexFile(enexPath, cfg)

			// Process the ENEX file
			result, err := enexFile.Process(tt.processOpts)
			if err != nil {
				t.Fatalf("Failed to process enex file: %v", err)
			}

			// Verify minimum expectations
			if result.NotesProcessed < tt.minNotesExpected {
				t.Errorf("Expected at least %d notes processed, got %d",
					tt.minNotesExpected, result.NotesProcessed)
			}

			if result.FilesUploaded < tt.minUploadsExpected {
				t.Errorf("Expected at least %d files uploaded, got %d",
					tt.minUploadsExpected, result.FilesUploaded)
			}

			// Verify document in Paperless if specified
			if tt.verifyDocument != nil && !tt.skipVerifyInPaperless {
				doc, err := client.WaitForDocument(tt.verifyDocument.title, 30*time.Second)
				if err != nil {
					t.Fatalf("Document not found in Paperless: %v", err)
				}

				if doc.Title != tt.verifyDocument.title {
					t.Errorf("Expected document title '%s', got '%s'",
						tt.verifyDocument.title, doc.Title)
				}

				// Verify tags
				for _, tagName := range tt.verifyDocument.tags {
					AssertDocumentHasTag(t, client, doc, tagName)
				}

				t.Logf("Successfully verified document: %s (ID: %d)", doc.Title, doc.ID)
			}

			// Log results
			t.Logf("Processed %d notes, uploaded %d files (workers: %d)",
				result.NotesProcessed,
				result.FilesUploaded,
				tt.processOpts.ConcurrentWorkers)
		})
	}
}

// documentVerification specifies expected document properties
type documentVerification struct {
	title string
	tags  []string
}
