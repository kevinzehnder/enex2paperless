package enex

import (
	"archive/zip"
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// TestUnzipFile tests the functionality of the unzipFile function
func TestUnzipFile(t *testing.T) {
	// Create a test filesystem
	mockFs := afero.NewMemMapFs()

	// Create test data - a simple in-memory zip file
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Add a few test files to the zip
	testFiles := []struct {
		Name    string
		Content string
	}{
		{"test1.txt", "This is test file 1"},
		{"test2.pdf", "PDF content here"},
		{"subfolder/test3.jpg", "JPEG content here"},
	}

	for _, file := range testFiles {
		// Create subdirectories if needed
		if strings.Contains(file.Name, "/") {
			dir := filepath.Dir(file.Name)
			zipWriter.Create(dir + "/")
		}

		// Add file to zip
		f, err := zipWriter.Create(file.Name)
		if err != nil {
			t.Fatalf("Failed to create file in zip: %v", err)
		}
		_, err = f.Write([]byte(file.Content))
		if err != nil {
			t.Fatalf("Failed to write content to zip file: %v", err)
		}
	}

	// Add a system file that should be skipped
	f, _ := zipWriter.Create("__MACOSX/._hidden")
	f.Write([]byte("This should be skipped"))

	// Close the zip writer
	zipWriter.Close()

	// Extract zip contents
	zipData := buf.Bytes()
	extractDir := "/tmp/extract"
	zipFileName := "test.zip"

	extractedFiles, err := unzipFile(zipData, extractDir, mockFs, zipFileName)
	if err != nil {
		t.Fatalf("unzipFile failed: %v", err)
	}

	// Verify number of extracted files (should skip system files)
	if len(extractedFiles) != 3 {
		t.Errorf("Expected 3 extracted files, got %d", len(extractedFiles))
	}

	// Verify each file was extracted correctly
	for _, file := range testFiles {
		// Check if file exists in filesystem
		extracted := false
		for _, ef := range extractedFiles {
			if ef.Name == file.Name {
				extracted = true

				// Verify file data
				if string(ef.Data) != file.Content {
					t.Errorf("File content mismatch for %s. Expected: %s, Got: %s",
						file.Name, file.Content, string(ef.Data))
				}

				// Verify MIME type
				expectedMime := getMimeType(file.Name)
				if ef.MimeType != expectedMime {
					t.Errorf("MIME type mismatch for %s. Expected: %s, Got: %s",
						file.Name, expectedMime, ef.MimeType)
				}

				// Verify ZipFileName
				if ef.ZipFileName != zipFileName {
					t.Errorf("ZipFileName mismatch. Expected: %s, Got: %s",
						zipFileName, ef.ZipFileName)
				}

				break
			}
		}
		if !extracted {
			t.Errorf("File %s not found in extracted files", file.Name)
		}
	}
}

// TestIsSystemFile tests the isSystemFile function
func TestIsSystemFile(t *testing.T) {
	testCases := []struct {
		FileName string
		Expected bool
	}{
		{".DS_Store", true},
		{"test.txt", false},
		{"__MACOSX/test.txt", true},
		{"._test.txt", true},
		{"thumbs.db", true},
		{"THUMBS.DB", true}, // Testing case insensitivity
		{"desktop.ini", true},
		{"important.pdf", false},
	}

	for _, tc := range testCases {
		result := isSystemFile(tc.FileName)
		if result != tc.Expected {
			t.Errorf("isSystemFile(%s) = %v, expected %v", tc.FileName, result, tc.Expected)
		}
	}
}
