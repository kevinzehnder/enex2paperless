package enex

import (
	"archive/zip"
	"bytes"
	"enex2paperless/pkg/paperless"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// isSystemFile checks if a file or directory is a system file that should be excluded
func isSystemFile(name string) bool {
	// Convert to lowercase for case-insensitive comparison
	name = strings.ToLower(name)

	// Check for common system files and directories
	systemFiles := []string{
		".ds_store",
		"thumbs.db",
		"desktop.ini",
		"__macosx",
		"._",
	}

	for _, systemFile := range systemFiles {
		if strings.Contains(name, systemFile) {
			return true
		}
	}

	return false
}

// ExtractedFile represents a file extracted from a zip archive
type ExtractedFile struct {
	Path        string
	Name        string
	Data        []byte
	MimeType    string
	ZipFileName string
}

// processZipFile handles a zip file, extracts its contents and processes each file
// based on the current settings (either saving to disk or uploading to Paperless)
func (e *EnexFile) processZipFile(decodedData []byte, resource Resource, note Note, outputFolder string, formattedCreatedDate string, allTags []string) error {
	slog.Info("processing zip file", "file", resource.ResourceAttributes.FileName)

	// Create a temporary directory for extraction if output folder is not set
	extractDir := outputFolder
	if extractDir == "" {
		extractDir = os.TempDir()
	}

	// Extract the ZIP file
	extractedFiles, err := unzipFile(decodedData, extractDir, e.Fs, resource.ResourceAttributes.FileName)
	if err != nil {
		return fmt.Errorf("failed to extract zip file: %v", err)
	}

	// Track files for cleanup
	var filesToCleanup []string

	// Process each extracted file
	for _, file := range extractedFiles {
		slog.Info("processing extracted file",
			"name", file.Name,
			"mime_type", file.MimeType,
		)

		// Check if the extracted file type is allowed
		fileExt, err := getExtensionFromMimeType(file.MimeType)
		if err != nil {
			slog.Error("error getting extension from mime type", "error", err)
			continue
		}

		fileTypeAllowed := false
		for _, fileType := range e.config.FileTypes {
			if strings.ToLower(fileType) == strings.ToLower(fileExt) ||
				fileType == "any" {
				fileTypeAllowed = true
				break
			}
		}

		if !fileTypeAllowed {
			slog.Debug("skipping unwanted file type from zip",
				"filename", file.Name,
				"filetype", file.MimeType)
			continue
		}

		// Handle output to disk if specified
		if outputFolder != "" {
			zipFileNameWithoutExt := strings.TrimSuffix(file.ZipFileName, filepath.Ext(file.ZipFileName))
			fileNameWithoutExt := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))
			outputName := fmt.Sprintf("%s_%s_%s%s",
				note.Title,
				zipFileNameWithoutExt,
				fileNameWithoutExt,
				filepath.Ext(file.Name))

			// Clean up filename (replace invalid characters)
			outputName = strings.ReplaceAll(outputName, "/", "_")
			outputName = strings.ReplaceAll(outputName, "\\", "_")

			extractedResource := Resource{
				Mime: file.MimeType,
				ResourceAttributes: ResourceAttributes{
					FileName: outputName,
				},
			}

			err = e.SaveResourceToDisk(file.Data, extractedResource, outputFolder)
			if err != nil {
				slog.Error("failed to save extracted file to disk", "error", err)
			} else {
				e.Uploads.Add(1)
			}
		} else {
			// Upload to Paperless
			zipFileNameWithoutExt := strings.TrimSuffix(file.ZipFileName, filepath.Ext(file.ZipFileName))
			fileNameWithoutExt := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))
			combinedTitle := fmt.Sprintf("%s | %s | %s",
				note.Title,
				zipFileNameWithoutExt,
				fileNameWithoutExt)

			paperlessFile := paperless.NewPaperlessFile(
				combinedTitle,
				file.Name,
				file.MimeType,
				formattedCreatedDate,
				file.Data,
				allTags,
				e.config,
			)

			err = paperlessFile.Upload()
			if err != nil {
				slog.Error("failed to upload extracted file", "error", err)
			} else {
				e.Uploads.Add(1)
			}
		}

		// Add file to cleanup list if it's in a temporary directory
		if extractDir == os.TempDir() {
			filesToCleanup = append(filesToCleanup, file.Path)
		}
	}

	// Clean up temporary files
	if extractDir == os.TempDir() {
		for _, filePath := range filesToCleanup {
			if err := e.Fs.Remove(filePath); err != nil {
				slog.Error("failed to clean up temporary file", "file", filePath, "error", err)
			} else {
				slog.Debug("cleaned up temporary file", "file", filePath)
			}
		}
	}

	return nil
}

// unzipFile takes a byte slice of a zip file and extracts its contents to the specified directory
// It returns a slice of extracted files with their paths and data
func unzipFile(data []byte, destDir string, fs afero.Fs, zipFileName string) ([]ExtractedFile, error) {
	var extractedFiles []ExtractedFile

	// Create a reader from the byte slice
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %v", err)
	}

	// Create destination directory if it doesn't exist
	err = fs.MkdirAll(destDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %v", err)
	}

	// Extract each file
	for _, file := range zipReader.File {
		// Skip directories and system files
		if file.FileInfo().IsDir() || isSystemFile(file.Name) {
			slog.Debug("skipping system file or directory", "file", file.Name)
			continue
		}

		// Open the file in the zip
		rc, err := file.Open()
		if err != nil {
			return extractedFiles, fmt.Errorf("failed to open file in zip: %v", err)
		}

		// Create the file path
		filePath := filepath.Join(destDir, file.Name)

		// Read the file contents
		var buf bytes.Buffer
		_, err = io.Copy(&buf, rc)
		rc.Close()
		if err != nil {
			return extractedFiles, fmt.Errorf("failed to read file contents: %v", err)
		}

		// Create the file
		f, err := fs.Create(filePath)
		if err != nil {
			return extractedFiles, fmt.Errorf("failed to create file: %v", err)
		}

		// Write the contents
		_, err = f.Write(buf.Bytes())
		f.Close()
		if err != nil {
			return extractedFiles, fmt.Errorf("failed to write file contents: %v", err)
		}

		// Add file to extracted files list
		extractedFiles = append(extractedFiles, ExtractedFile{
			Path:        filePath,
			Name:        file.Name,
			Data:        buf.Bytes(),
			MimeType:    getMimeType(file.Name),
			ZipFileName: zipFileName,
		})

		slog.Info("extracted file from zip", "file", file.Name)
	}

	return extractedFiles, nil
}
