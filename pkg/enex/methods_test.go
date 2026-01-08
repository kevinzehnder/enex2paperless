package enex

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
)

// TestSaveResourceToDisk tests the SaveResourceToDisk method
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
