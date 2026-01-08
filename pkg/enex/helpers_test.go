package enex

import (
	"enex2paperless/internal/config"
	"testing"
)

// TestConvertDateFormat tests the convertDateFormat function
func TestConvertDateFormatHelper(t *testing.T) {
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
	}
}

// TestGetExtensionFromMimeTypeHelper tests the getExtensionFromMimeType function
func TestGetExtensionFromMimeTypeHelper(t *testing.T) {
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
			mimeType:     "invalid",
			expected:     "",
			expectsError: true,
		},
	}

	for _, tc := range testCases {
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
	}
}

// TestSanitizeFilenameHelper tests the sanitizeFilename function
func TestSanitizeFilenameHelper(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes forward slashes",
			input:    "Meeting/Notes/Q1",
			expected: "Meeting_Notes_Q1",
		},
		{
			name:     "removes backslashes",
			input:    "Path\\To\\File",
			expected: "Path_To_File",
		},
		{
			name:     "removes colons",
			input:    "Meeting: Q1 Planning",
			expected: "Meeting_ Q1 Planning",
		},
		{
			name:     "removes asterisks",
			input:    "File*Name",
			expected: "File_Name",
		},
		{
			name:     "removes question marks",
			input:    "What?Why?",
			expected: "What_Why_",
		},
		{
			name:     "removes quotes",
			input:    "File\"Name",
			expected: "File_Name",
		},
		{
			name:     "removes angle brackets",
			input:    "<File>Name",
			expected: "_File_Name",
		},
		{
			name:     "removes pipes",
			input:    "File|Name",
			expected: "File_Name",
		},
		{
			name:     "removes multiple invalid chars",
			input:    "Meeting: Q1/Q2 <Draft>",
			expected: "Meeting_ Q1_Q2 _Draft_",
		},
		{
			name:     "trims whitespace",
			input:    "  filename  ",
			expected: "filename",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "unnamed",
		},
		{
			name:     "handles only whitespace",
			input:    "   ",
			expected: "unnamed",
		},
		{
			name:     "handles only invalid chars",
			input:    "///:::***",
			expected: "_________",
		},
		{
			name:     "preserves clean filename",
			input:    "normal_file.pdf",
			expected: "normal_file.pdf",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeFilename(tc.input)
			if result != tc.expected {
				t.Errorf("sanitizeFilename(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// TestGetMimeTypeHelper tests the getMimeType function
func TestGetMimeTypeHelper(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "pdf extension",
			filename: "document.pdf",
			expected: "application/pdf",
		},
		{
			name:     "txt extension",
			filename: "notes.txt",
			expected: "text/plain",
		},
		{
			name:     "jpg extension",
			filename: "photo.jpg",
			expected: "image/jpeg",
		},
		{
			name:     "jpeg extension",
			filename: "photo.jpeg",
			expected: "image/jpeg",
		},
		{
			name:     "png extension",
			filename: "image.png",
			expected: "image/png",
		},
		{
			name:     "gif extension",
			filename: "animation.gif",
			expected: "image/gif",
		},
		{
			name:     "webp extension",
			filename: "modern.webp",
			expected: "image/webp",
		},
		{
			name:     "tiff extension",
			filename: "scan.tiff",
			expected: "image/tiff",
		},
		{
			name:     "tif extension",
			filename: "scan.tif",
			expected: "image/tiff",
		},
		{
			name:     "uppercase extension",
			filename: "DOCUMENT.PDF",
			expected: "application/pdf",
		},
		{
			name:     "mixed case extension",
			filename: "Photo.JpG",
			expected: "image/jpeg",
		},
		{
			name:     "unknown extension",
			filename: "file.xyz",
			expected: "application/octet-stream",
		},
		{
			name:     "no extension",
			filename: "filename",
			expected: "application/octet-stream",
		},
		{
			name:     "multiple dots",
			filename: "my.document.pdf",
			expected: "application/pdf",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getMimeType(tc.filename)
			if result != tc.expected {
				t.Errorf("getMimeType(%q) = %q, expected %q", tc.filename, result, tc.expected)
			}
		})
	}
}

// TestCheckFileTypeHelper tests the checkFileType method
func TestCheckFileTypeHelper(t *testing.T) {
	testCases := []struct {
		name          string
		allowedTypes  []string
		mimeType      string
		expectedOk    bool
		expectedError bool
	}{
		{
			name:          "any allows all types",
			allowedTypes:  []string{"any"},
			mimeType:      "application/pdf",
			expectedOk:    true,
			expectedError: false,
		},
		{
			name:          "any allows unknown types",
			allowedTypes:  []string{"any"},
			mimeType:      "application/unknown",
			expectedOk:    true,
			expectedError: false,
		},
		{
			name:          "pdf allowed",
			allowedTypes:  []string{"pdf"},
			mimeType:      "application/pdf",
			expectedOk:    true,
			expectedError: false,
		},
		{
			name:          "pdf not allowed",
			allowedTypes:  []string{"txt"},
			mimeType:      "application/pdf",
			expectedOk:    false,
			expectedError: false,
		},
		{
			name:          "txt mapped to plain",
			allowedTypes:  []string{"txt"},
			mimeType:      "text/plain",
			expectedOk:    true,
			expectedError: false,
		},
		{
			name:          "jpeg allowed",
			allowedTypes:  []string{"jpeg"},
			mimeType:      "image/jpeg",
			expectedOk:    true,
			expectedError: false,
		},
		{
			name:          "multiple types - match first",
			allowedTypes:  []string{"pdf", "txt", "jpeg"},
			mimeType:      "application/pdf",
			expectedOk:    true,
			expectedError: false,
		},
		{
			name:          "multiple types - match last",
			allowedTypes:  []string{"pdf", "txt", "jpeg"},
			mimeType:      "image/jpeg",
			expectedOk:    true,
			expectedError: false,
		},
		{
			name:          "multiple types - no match",
			allowedTypes:  []string{"pdf", "txt"},
			mimeType:      "image/jpeg",
			expectedOk:    false,
			expectedError: false,
		},
		{
			name:          "case insensitive - uppercase PDF",
			allowedTypes:  []string{"PDF"},
			mimeType:      "application/pdf",
			expectedOk:    true,
			expectedError: false,
		},
		{
			name:          "case insensitive - mixed case",
			allowedTypes:  []string{"PdF"},
			mimeType:      "application/pdf",
			expectedOk:    true,
			expectedError: false,
		},
		{
			name:          "invalid mime type format",
			allowedTypes:  []string{"pdf"},
			mimeType:      "invalid",
			expectedOk:    false,
			expectedError: true,
		},
		{
			name:          "empty mime type",
			allowedTypes:  []string{"pdf"},
			mimeType:      "",
			expectedOk:    false,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create an EnexFile with the specified config
			e := &EnexFile{
				config: config.Config{
					FileTypes: tc.allowedTypes,
				},
			}

			ok, err := e.checkFileType(tc.mimeType)

			if tc.expectedError && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tc.expectedError && err != nil {
				t.Errorf("did not expect error but got: %v", err)
			}

			if ok != tc.expectedOk {
				t.Errorf("checkFileType() = %v, expected %v", ok, tc.expectedOk)
			}
		})
	}
}
