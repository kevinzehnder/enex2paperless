package enex

import (
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