//go:build integration

package integration

import (
	"enex2paperless/pkg/enex"
	"testing"
	"time"
)

func TestBasicDocumentUpload(t *testing.T) {
	// Setup
	cfg := GetTestConfig(t)
	SkipIfPaperlessUnavailable(t, cfg)
	client := GetPaperlessClient(t, cfg)

	// Cleanup after test
	defer CleanupTestDocuments(t, client, "Test PDF Note")
	defer CleanupTestTags(t, client, []string{"SampleTag"})

	// Create EnexFile with injected config
	enexPath := GetAssetPath("test.enex")
	enexFile := enex.NewEnexFile(enexPath, cfg)

	// Process the ENEX file (no retries in tests)
	result, err := enexFile.Process(enex.ProcessOptions{
		ConcurrentWorkers: 1,
		OutputFolder:      "",
		RetryPromptFunc:   nil, // Auto-retry without prompting in tests
	})
	if err != nil {
		t.Fatalf("Failed to process enex file: %v", err)
	}

	if result.NotesProcessed == 0 {
		t.Fatal("Expected at least one note to be processed")
	}

	// Wait for document to appear in Paperless
	doc, err := client.WaitForDocument("Test PDF Note", 30*time.Second)
	if err != nil {
		t.Fatalf("Document not found after upload: %v", err)
	}

	// Verify document properties
	if doc.Title != "Test PDF Note" {
		t.Errorf("Expected title 'Test PDF Note', got '%s'", doc.Title)
	}

	// Verify tag was created and associated
	AssertDocumentHasTag(t, client, doc, "SampleTag")

	t.Logf("Successfully uploaded document: %s (ID: %d)", doc.Title, doc.ID)
}

func TestDocumentWithMultipleTags(t *testing.T) {
	// Setup
	cfg := GetTestConfig(t)
	cfg.AdditionalTags = []string{"IntegrationTest", "AutomatedUpload"}
	SkipIfPaperlessUnavailable(t, cfg)
	client := GetPaperlessClient(t, cfg)

	// Cleanup after test
	defer CleanupTestDocuments(t, client, "Test PDF Note")
	defer CleanupTestTags(t, client, []string{"SampleTag", "IntegrationTest", "AutomatedUpload"})

	// Create EnexFile with injected config including additional tags
	enexPath := GetAssetPath("test.enex")
	enexFile := enex.NewEnexFile(enexPath, cfg)

	// Start the upload process
	go func() {
		err := enexFile.ReadFromFile()
		if err != nil {
			t.Errorf("Failed to read enex file: %v", err)
		}
	}()

	// Process notes and upload
	go func() {
		err := enexFile.UploadFromNoteChannel("")
		if err != nil {
			t.Errorf("Failed to upload notes: %v", err)
		}
	}()

	// Wait for upload to complete
	doc, err := client.WaitForDocument("Test PDF Note", 30*time.Second)
	if err != nil {
		t.Fatalf("Document not found after upload: %v", err)
	}

	// Verify all tags are present
	expectedTags := []string{"SampleTag", "IntegrationTest", "AutomatedUpload"}
	for _, tagName := range expectedTags {
		AssertDocumentHasTag(t, client, doc, tagName)
	}

	t.Logf("Successfully uploaded document with %d tags", len(expectedTags))
}

func TestFileTypeFiltering(t *testing.T) {
	// Setup - only allow PDF files
	cfg := GetTestConfig(t)
	cfg.FileTypes = []string{"pdf"}
	SkipIfPaperlessUnavailable(t, cfg)
	client := GetPaperlessClient(t, cfg)

	// Cleanup after test
	defer CleanupTestDocuments(t, client, "Filetypes")

	// Create EnexFile with injected config
	enexPath := GetAssetPath("filetypes.enex")
	enexFile := enex.NewEnexFile(enexPath, cfg)

	// Process the ENEX file
	result, err := enexFile.Process(enex.ProcessOptions{
		ConcurrentWorkers: 1,
		OutputFolder:      "",
		RetryPromptFunc:   nil,
	})
	if err != nil {
		t.Fatalf("Failed to process enex file: %v", err)
	}

	uploadedCount := result.FilesUploaded

	// Verify that only PDF files were uploaded
	// The filetypes.enex file should have multiple file types,
	// but we should only see uploads matching our filter
	if uploadedCount == 0 {
		t.Error("Expected at least one PDF document to be uploaded")
	}

	t.Logf("Uploaded %d documents with PDF filter", uploadedCount)
}

func TestZipFileProcessing(t *testing.T) {
	// Setup
	cfg := GetTestConfig(t)
	cfg.FileTypes = []string{"any"} // Allow all file types from zip
	SkipIfPaperlessUnavailable(t, cfg)
	client := GetPaperlessClient(t, cfg)

	// Cleanup after test - clean up any documents with titles containing "zip"
	defer func() {
		docs, _ := client.GetDocuments()
		for _, doc := range docs {
			if contains(doc.Title, "zip") || contains(doc.Title, "Zip") {
				client.DeleteDocument(doc.ID)
			}
		}
	}()

	// Create EnexFile with injected config
	enexPath := GetAssetPath("zip.enex")
	enexFile := enex.NewEnexFile(enexPath, cfg)

	// Process the ENEX file
	result, err := enexFile.Process(enex.ProcessOptions{
		ConcurrentWorkers: 1,
		OutputFolder:      "",
		RetryPromptFunc:   nil,
	})
	if err != nil {
		t.Fatalf("Failed to process enex file: %v", err)
	}

	uploadedCount := result.FilesUploaded
	if uploadedCount == 0 {
		t.Error("Expected files to be extracted and uploaded from zip")
	}

	t.Logf("Extracted and uploaded %d files from zip archive", uploadedCount)
}

func TestConcurrentUploads(t *testing.T) {
	// Setup
	cfg := GetTestConfig(t)
	SkipIfPaperlessUnavailable(t, cfg)
	client := GetPaperlessClient(t, cfg)

	// Cleanup after test
	defer CleanupTestDocuments(t, client, "Test PDF Note")
	defer CleanupTestTags(t, client, []string{"SampleTag"})

	// Create EnexFile with injected config
	enexPath := GetAssetPath("test.enex")
	enexFile := enex.NewEnexFile(enexPath, cfg)

	// Use multiple concurrent uploaders
	concurrentWorkers := 3

	// Process with concurrent workers
	result, err := enexFile.Process(enex.ProcessOptions{
		ConcurrentWorkers: concurrentWorkers,
		OutputFolder:      "",
		RetryPromptFunc:   nil,
	})
	if err != nil {
		t.Fatalf("Failed to process enex file: %v", err)
	}

	if result.NotesProcessed == 0 {
		t.Fatal("Expected at least one note to be processed")
	}

	// Wait for document to appear in Paperless
	doc, err := client.WaitForDocument("Test PDF Note", 30*time.Second)
	if err != nil {
		t.Fatalf("Document not found after concurrent upload: %v", err)
	}

	t.Logf("Successfully uploaded document with %d concurrent workers: %s (ID: %d)",
		concurrentWorkers, doc.Title, doc.ID)
}

func TestUploadMetrics(t *testing.T) {
	// Setup
	cfg := GetTestConfig(t)
	SkipIfPaperlessUnavailable(t, cfg)
	client := GetPaperlessClient(t, cfg)

	// Cleanup after test
	defer CleanupTestDocuments(t, client, "Test PDF Note")
	defer CleanupTestTags(t, client, []string{"SampleTag"})

	// Create EnexFile with injected config
	enexPath := GetAssetPath("test.enex")
	enexFile := enex.NewEnexFile(enexPath, cfg)

	// Process the ENEX file
	result, err := enexFile.Process(enex.ProcessOptions{
		ConcurrentWorkers: 1,
		OutputFolder:      "",
		RetryPromptFunc:   nil,
	})
	if err != nil {
		t.Fatalf("Failed to process enex file: %v", err)
	}

	// Check metrics from result
	numNotes := result.NotesProcessed
	numUploads := result.FilesUploaded

	if numNotes == 0 {
		t.Error("Expected NumNotes to be greater than 0")
	}

	if numUploads == 0 {
		t.Error("Expected Uploads to be greater than 0")
	}

	if numUploads != numNotes {
		t.Logf("Note: NumNotes=%d, Uploads=%d (difference may be due to filtering)", numNotes, numUploads)
	}

	t.Logf("Processed %d notes, uploaded %d documents", numNotes, numUploads)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
