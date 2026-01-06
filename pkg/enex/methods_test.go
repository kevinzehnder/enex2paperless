package enex

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
)

// TestSaveResourceToDisk tests the SaveResourceToDisk function directly
func TestSaveResourceToDisk(t *testing.T) {
	// Create a mock filesystem
	mockFs := afero.NewMemMapFs()

	// Create a test EnexFile with the mock filesystem
	enexFile := &EnexFile{
		Fs: mockFs,
	}

	// Test data
	outputFolder := "/test/output"
	mockFs.MkdirAll(outputFolder, 0755)

	// Simple test data
	testData := []byte("test data")

	// Create a test resource
	resource := Resource{
		ResourceAttributes: ResourceAttributes{
			FileName: "test.txt",
		},
	}

	// Call the function we're testing
	err := enexFile.SaveResourceToDisk(testData, resource, outputFolder)

	// Check for errors
	if err != nil {
		t.Errorf("SaveResourceToDisk returned an error: %v", err)
	}

	// Verify the file was created
	exists, _ := afero.Exists(mockFs, fmt.Sprintf("%s/%s", outputFolder, resource.ResourceAttributes.FileName))
	if !exists {
		t.Errorf("File was not created at expected location")
	}

	// Verify the file contents
	content, err := afero.ReadFile(mockFs, fmt.Sprintf("%s/%s", outputFolder, resource.ResourceAttributes.FileName))
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	if string(content) != string(testData) {
		t.Errorf("File content mismatch. Expected '%s', got '%s'", testData, content)
	}
}

// TestGetExtensionFromMimeType tests the getExtensionFromMimeType function
func TestGetExtensionFromMimeType(t *testing.T) {
	testCases := []struct {
		mimeType     string
		expected     string
		expectsError bool
	}{
		{
			mimeType:     "application/pdf",
			expected:     "pdf",
			expectsError: false,
		},
		{
			mimeType:     "text/plain",
			expected:     "plain",
			expectsError: false,
		},
		{
			mimeType:     "image/jpeg",
			expected:     "jpeg",
			expectsError: false,
		},
		{
			mimeType:     "invalid",
			expected:     "",
			expectsError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("MIME type: %s", tc.mimeType), func(t *testing.T) {
			extension, err := getExtensionFromMimeType(tc.mimeType)

			if tc.expectsError && err == nil {
				t.Errorf("Expected error for MIME type '%s', but got none", tc.mimeType)
			}

			if !tc.expectsError && err != nil {
				t.Errorf("Did not expect error for MIME type '%s', but got: %v", tc.mimeType, err)
			}

			if extension != tc.expected {
				t.Errorf("Extension mismatch for MIME type '%s'. Expected '%s', got '%s'",
					tc.mimeType, tc.expected, extension)
			}
		})
	}
}

// TestGetMimeType tests the getMimeType function
func TestGetMimeType(t *testing.T) {
	testCases := []struct {
		filename string
		expected string
	}{
		{
			filename: "document.pdf",
			expected: "application/pdf",
		},
		{
			filename: "notes.txt",
			expected: "text/plain",
		},
		{
			filename: "image.jpg",
			expected: "image/jpeg",
		},
		{
			filename: "image.jpeg",
			expected: "image/jpeg",
		},
		{
			filename: "image.png",
			expected: "image/png",
		},
		{
			filename: "image.gif",
			expected: "image/gif",
		},
		{
			filename: "image.webp",
			expected: "image/webp",
		},
		{
			filename: "image.tiff",
			expected: "image/tiff",
		},
		{
			filename: "image.tif",
			expected: "image/tiff",
		},
		{
			filename: "unknown.xyz",
			expected: "application/octet-stream",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("File: %s", tc.filename), func(t *testing.T) {
			mimeType := getMimeType(tc.filename)

			if mimeType != tc.expected {
				t.Errorf("MIME type mismatch for filename '%s'. Expected '%s', got '%s'",
					tc.filename, tc.expected, mimeType)
			}
		})
	}
}

// TestConvertDateFormat tests the convertDateFormat function
func TestConvertDateFormat(t *testing.T) {
	testCases := []struct {
		dateStr      string
		expected     string
		expectsError bool
	}{
		{
			dateStr:      "20220101T120000Z",
			expected:     "2022-01-01 12:00:00+00:00",
			expectsError: false,
		},
		{
			dateStr:      "20220430T235959Z",
			expected:     "2022-04-30 23:59:59+00:00",
			expectsError: false,
		},
		{
			dateStr:      "invaliddate",
			expected:     "",
			expectsError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Date: %s", tc.dateStr), func(t *testing.T) {
			formatted, err := convertDateFormat(tc.dateStr)

			if tc.expectsError && err == nil {
				t.Errorf("Expected error for date string '%s', but got none", tc.dateStr)
			}

			if !tc.expectsError && err != nil {
				t.Errorf("Did not expect error for date string '%s', but got: %v", tc.dateStr, err)
			}

			if formatted != tc.expected {
				t.Errorf("Formatted date mismatch for date string '%s'. Expected '%s', got '%s'",
					tc.dateStr, tc.expected, formatted)
			}
		})
	}
}
