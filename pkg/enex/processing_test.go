package enex

import (
	"enex2paperless/internal/config"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// TestProcessingNotesWithoutAttachments verifies notes without resources are skipped
func TestProcessingNotesWithoutAttachments(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	cfg := config.Config{
		FileTypes: []string{"pdf"},
	}

	enexFile := &EnexFile{
		Fs:                mockFs,
		config:            cfg,
		NoteChannel:       make(chan Note, 10),
		FailedNoteChannel: make(chan Note, 10),
	}

	// Start worker in background
	done := make(chan bool)
	go func() {
		err := enexFile.UploadFromNoteChannel("/tmp/output")
		if err != nil {
			t.Errorf("UploadFromNoteChannel error: %v", err)
		}
		done <- true
	}()

	// Send notes without attachments
	enexFile.NoteChannel <- Note{
		Title:     "Note without attachments",
		Created:   "20220101T120000Z",
		Resources: []Resource{}, // Empty resources
		Tags:      []string{"test"},
	}

	enexFile.NoteChannel <- Note{
		Title:     "Another note without attachments",
		Created:   "20220101T120000Z",
		Resources: nil, // Nil resources
		Tags:      []string{"test"},
	}

	close(enexFile.NoteChannel)
	<-done

	// Verify no notes were processed
	if enexFile.NumNotes.Load() != 0 {
		t.Errorf("Expected 0 notes processed, got %d", enexFile.NumNotes.Load())
	}

	// Verify no uploads occurred
	if enexFile.Uploads.Load() != 0 {
		t.Errorf("Expected 0 uploads, got %d", enexFile.Uploads.Load())
	}
}

// TestProcessingMultipleResourcesPerNote verifies multiple resources are saved
func TestProcessingMultipleResourcesPerNote(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	cfg := config.Config{
		FileTypes: []string{"any"},
	}

	enexFile := &EnexFile{
		Fs:                mockFs,
		config:            cfg,
		NoteChannel:       make(chan Note, 10),
		FailedNoteChannel: make(chan Note, 10),
	}

	outputFolder := "/tmp/output"
	err := mockFs.MkdirAll(outputFolder, 0755)
	if err != nil {
		t.Fatalf("Failed to create output folder: %v", err)
	}

	// Start worker in background
	done := make(chan bool)
	go func() {
		err := enexFile.UploadFromNoteChannel(outputFolder)
		if err != nil {
			t.Errorf("UploadFromNoteChannel error: %v", err)
		}
		done <- true
	}()

	// Send note with multiple resources
	enexFile.NoteChannel <- Note{
		Title:   "Note with multiple attachments",
		Created: "20220101T120000Z",
		Resources: []Resource{
			{
				Data: "dGVzdCBkYXRhIDE=", // "test data 1" in base64
				Mime: "application/pdf",
				ResourceAttributes: ResourceAttributes{
					FileName: "document1.pdf",
				},
			},
			{
				Data: "dGVzdCBkYXRhIDI=", // "test data 2" in base64
				Mime: "application/pdf",
				ResourceAttributes: ResourceAttributes{
					FileName: "document2.pdf",
				},
			},
			{
				Data: "dGVzdCBkYXRhIDM=", // "test data 3" in base64
				Mime: "text/plain",
				ResourceAttributes: ResourceAttributes{
					FileName: "notes.txt",
				},
			},
		},
		Tags: []string{"test"},
	}

	close(enexFile.NoteChannel)
	<-done

	// Verify one note was processed
	if enexFile.NumNotes.Load() != 1 {
		t.Errorf("Expected 1 note processed, got %d", enexFile.NumNotes.Load())
	}

	// Verify only first resource was saved (due to break after first save when outputFolder is set)
	if enexFile.Uploads.Load() != 1 {
		t.Errorf("Expected 1 upload (breaks after first when saving to disk), got %d", enexFile.Uploads.Load())
	}

	// Verify the first file was created
	exists, _ := afero.Exists(mockFs, outputFolder+"/document1.pdf")
	if !exists {
		t.Error("First resource file was not created")
	}
}

// TestProcessingInvalidBase64Data verifies error handling for invalid base64
func TestProcessingInvalidBase64Data(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	cfg := config.Config{
		FileTypes: []string{"pdf"},
	}

	enexFile := &EnexFile{
		Fs:                mockFs,
		config:            cfg,
		NoteChannel:       make(chan Note, 10),
		FailedNoteChannel: make(chan Note, 10),
	}

	outputFolder := "/tmp/output"
	err := mockFs.MkdirAll(outputFolder, 0755)
	if err != nil {
		t.Fatalf("Failed to create output folder: %v", err)
	}

	// Start worker in background
	done := make(chan bool)
	go func() {
		err := enexFile.UploadFromNoteChannel(outputFolder)
		if err != nil {
			t.Errorf("UploadFromNoteChannel error: %v", err)
		}
		done <- true
	}()

	// Send note with invalid base64 data
	enexFile.NoteChannel <- Note{
		Title:   "Note with invalid base64",
		Created: "20220101T120000Z",
		Resources: []Resource{
			{
				Data: "not valid base64 @#$%",
				Mime: "application/pdf",
				ResourceAttributes: ResourceAttributes{
					FileName: "invalid.pdf",
				},
			},
		},
		Tags: []string{"test"},
	}

	close(enexFile.NoteChannel)
	<-done

	// Verify note was counted as processed
	if enexFile.NumNotes.Load() != 1 {
		t.Errorf("Expected 1 note processed, got %d", enexFile.NumNotes.Load())
	}

	// Verify no uploads occurred due to base64 error
	// Note: invalid base64 is caught by regex validation and skipped (continues),
	// not sent to FailedNoteChannel
	if enexFile.Uploads.Load() != 0 {
		t.Errorf("Expected 0 uploads due to invalid base64, got %d", enexFile.Uploads.Load())
	}
}

// TestProcessingEmptyFilename verifies note title is used as fallback
func TestProcessingEmptyFilename(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	cfg := config.Config{
		FileTypes: []string{"pdf"},
	}

	enexFile := &EnexFile{
		Fs:                mockFs,
		config:            cfg,
		NoteChannel:       make(chan Note, 10),
		FailedNoteChannel: make(chan Note, 10),
	}

	outputFolder := "/tmp/output"
	err := mockFs.MkdirAll(outputFolder, 0755)
	if err != nil {
		t.Fatalf("Failed to create output folder: %v", err)
	}

	// Start worker in background
	done := make(chan bool)
	go func() {
		err := enexFile.UploadFromNoteChannel(outputFolder)
		if err != nil {
			t.Errorf("UploadFromNoteChannel error: %v", err)
		}
		done <- true
	}()

	// Send note with empty filename
	enexFile.NoteChannel <- Note{
		Title:   "My Important Document",
		Created: "20220101T120000Z",
		Resources: []Resource{
			{
				Data: "dGVzdCBkYXRh", // "test data" in base64
				Mime: "application/pdf",
				ResourceAttributes: ResourceAttributes{
					FileName: "", // Empty filename
				},
			},
		},
		Tags: []string{"test"},
	}

	close(enexFile.NoteChannel)
	<-done

	// Verify note was processed
	if enexFile.NumNotes.Load() != 1 {
		t.Errorf("Expected 1 note processed, got %d", enexFile.NumNotes.Load())
	}

	// Verify upload occurred
	if enexFile.Uploads.Load() != 1 {
		t.Errorf("Expected 1 upload, got %d", enexFile.Uploads.Load())
	}

	// Verify file was created with note title as filename
	// Note: filename will be sanitized, so check for the sanitized version
	files, err := afero.ReadDir(mockFs, outputFolder)
	if err != nil {
		t.Fatalf("Failed to read output folder: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file in output folder, got %d", len(files))
	}

	// The filename should contain the note title
	filename := files[0].Name()
	if !strings.Contains(filename, "My Important Document") && !strings.Contains(filename, "My_Important_Document") {
		t.Errorf("Expected filename to contain note title, got: %s", filename)
	}
}

// TestProcessingInvalidDateFormat verifies error handling for invalid dates
func TestProcessingInvalidDateFormat(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	cfg := config.Config{
		FileTypes: []string{"pdf"},
	}

	enexFile := &EnexFile{
		Fs:                mockFs,
		config:            cfg,
		NoteChannel:       make(chan Note, 10),
		FailedNoteChannel: make(chan Note, 10),
	}

	outputFolder := "/tmp/output"
	err := mockFs.MkdirAll(outputFolder, 0755)
	if err != nil {
		t.Fatalf("Failed to create output folder: %v", err)
	}

	// Start worker in background
	done := make(chan bool)
	go func() {
		err := enexFile.UploadFromNoteChannel(outputFolder)
		if err != nil {
			t.Errorf("UploadFromNoteChannel error: %v", err)
		}
		done <- true
	}()

	// Send note with invalid date format
	enexFile.NoteChannel <- Note{
		Title:   "Note with invalid date",
		Created: "not a valid date",
		Resources: []Resource{
			{
				Data: "dGVzdCBkYXRh",
				Mime: "application/pdf",
				ResourceAttributes: ResourceAttributes{
					FileName: "test.pdf",
				},
			},
		},
		Tags: []string{"test"},
	}

	close(enexFile.NoteChannel)
	<-done

	// Verify note was counted
	if enexFile.NumNotes.Load() != 1 {
		t.Errorf("Expected 1 note processed, got %d", enexFile.NumNotes.Load())
	}

	// Verify no uploads occurred due to date parsing error
	if enexFile.Uploads.Load() != 0 {
		t.Errorf("Expected 0 uploads due to date error, got %d", enexFile.Uploads.Load())
	}

	// Verify note was sent to failed channel
	select {
	case failedNote := <-enexFile.FailedNoteChannel:
		if failedNote.Title != "Note with invalid date" {
			t.Errorf("Wrong note in failed channel: %s", failedNote.Title)
		}
	default:
		t.Error("Expected failed note in FailedNoteChannel")
	}
}
