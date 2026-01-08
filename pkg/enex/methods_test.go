package enex

import (
	"testing"

	"github.com/spf13/afero"
)

// TestSaveResourceToDisk tests the SaveResourceToDisk method
func TestSaveResourceToDisk(t *testing.T) {
	testCases := []struct {
		name         string
		setupFiles   map[string][]byte // Files to create before test
		resource     Resource
		data         []byte
		expectedFile string
		expectedData []byte
		expectError  bool
	}{
		{
			name:       "basic save - no conflicts",
			setupFiles: map[string][]byte{},
			resource: Resource{
				ResourceAttributes: ResourceAttributes{
					FileName: "test.txt",
				},
			},
			data:         []byte("test data"),
			expectedFile: "/test/output/test.txt",
			expectedData: []byte("test data"),
			expectError:  false,
		},
		{
			name: "naming conflict - adds counter -1",
			setupFiles: map[string][]byte{
				"/test/output/test.txt": []byte("existing data"),
			},
			resource: Resource{
				ResourceAttributes: ResourceAttributes{
					FileName: "test.txt",
				},
			},
			data:         []byte("new data"),
			expectedFile: "/test/output/test-1.txt",
			expectedData: []byte("new data"),
			expectError:  false,
		},
		{
			name: "multiple naming conflicts - adds counter -2",
			setupFiles: map[string][]byte{
				"/test/output/document.pdf":   []byte("first"),
				"/test/output/document-1.pdf": []byte("second"),
			},
			resource: Resource{
				ResourceAttributes: ResourceAttributes{
					FileName: "document.pdf",
				},
			},
			data:         []byte("third"),
			expectedFile: "/test/output/document-2.pdf",
			expectedData: []byte("third"),
			expectError:  false,
		},
		{
			name:       "file with no extension",
			setupFiles: map[string][]byte{},
			resource: Resource{
				ResourceAttributes: ResourceAttributes{
					FileName: "README",
				},
			},
			data:         []byte("readme content"),
			expectedFile: "/test/output/README",
			expectedData: []byte("readme content"),
			expectError:  false,
		},
		{
			name: "conflict with no extension - adds counter",
			setupFiles: map[string][]byte{
				"/test/output/README": []byte("existing"),
			},
			resource: Resource{
				ResourceAttributes: ResourceAttributes{
					FileName: "README",
				},
			},
			data:         []byte("new readme"),
			expectedFile: "/test/output/README-1",
			expectedData: []byte("new readme"),
			expectError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock filesystem
			mockFs := afero.NewMemMapFs()

			// Create output folder
			outputFolder := "/test/output"
			err := mockFs.MkdirAll(outputFolder, 0755)
			if err != nil {
				t.Fatalf("Failed to create output folder: %v", err)
			}

			// Setup existing files
			for path, data := range tc.setupFiles {
				err := afero.WriteFile(mockFs, path, data, 0644)
				if err != nil {
					t.Fatalf("Failed to setup file %s: %v", path, err)
				}
			}

			// Create test EnexFile
			enexFile := &EnexFile{
				Fs: mockFs,
			}

			// Call the function we're testing
			err = enexFile.SaveResourceToDisk(tc.data, tc.resource, outputFolder)

			// Check for errors
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tc.expectError {
				return
			}

			// Verify the file was created at expected location
			exists, err := afero.Exists(mockFs, tc.expectedFile)
			if err != nil {
				t.Fatalf("Error checking file existence: %v", err)
			}
			if !exists {
				t.Errorf("File was not created at expected location: %s", tc.expectedFile)
			}

			// Verify the file contents
			content, err := afero.ReadFile(mockFs, tc.expectedFile)
			if err != nil {
				t.Errorf("Error reading file: %v", err)
			}

			if string(content) != string(tc.expectedData) {
				t.Errorf("File content mismatch. Expected '%s', got '%s'", tc.expectedData, content)
			}

			// Verify original files were not modified
			for path, originalData := range tc.setupFiles {
				content, err := afero.ReadFile(mockFs, path)
				if err != nil {
					t.Errorf("Error reading original file %s: %v", path, err)
				}
				if string(content) != string(originalData) {
					t.Errorf("Original file %s was modified", path)
				}
			}
		})
	}
}
