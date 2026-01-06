package enex

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func (e *EnexFile) checkFileType(mimeType string) (bool, error) {
	// if filetypes contains "any" then allow all file types
	if slices.Contains(e.config.FileTypes, "any") {
		return true, nil
	}

	// Extract the extension from the MIME type
	extension, err := getExtensionFromMimeType(mimeType)
	if err != nil {
		return false, err
	}

	// Convert extension and allowed file types to lowercase for case-insensitive comparison
	extensionLower := strings.ToLower(extension)
	allowedFileTypes := make([]string, len(e.config.FileTypes))
	for i, fileType := range e.config.FileTypes {
		allowedFileTypes[i] = strings.ToLower(fileType)
		if fileType == "txt" {
			allowedFileTypes[i] = "plain"
		}
	}

	// Check if the extension matches any allowed file type
	if slices.Contains(allowedFileTypes, extensionLower) {
		return true, nil
	}

	return false, nil
}

// Extract the file extension from the MIME type (assuming valid format)
func getExtensionFromMimeType(mimeType string) (string, error) {
	parts := strings.Split(mimeType, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid MIME type format: %s", mimeType)
	}
	return parts[1], nil
}

// getMimeType returns the MIME type based on file extension
func getMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".tiff", ".tif":
		return "image/tiff"
	default:
		return "application/octet-stream"
	}
}

func convertDateFormat(dateStr string) (string, error) {
	// Parse the original date string into a time.Time
	parsedTime, err := time.Parse("20060102T150405Z", dateStr)
	if err != nil {
		return "", fmt.Errorf("error parsing time: %v", err)
	}

	// Convert time.Time to the desired string format
	return parsedTime.Format("2006-01-02 15:04:05-07:00"), nil
}
