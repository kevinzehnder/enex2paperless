package enex

import (
	"bufio"
	"encoding/base64"
	"encoding/xml"
	"enex2paperless/internal/config"
	"enex2paperless/pkg/paperless"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/afero"
)

func (e *EnexFile) ReadFromFile() error {
	slog.Debug(fmt.Sprintf("opening file: %v", e.FilePath))
	file, err := e.Fs.Open(e.FilePath)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	decoder.Strict = false

	slog.Debug("decoding XML")
	for {
		t, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Log this error but continue parsing
			slog.Error("XML parsing error", "error", err)
			break
		}
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "en-note" {
				continue
			}
			if se.Name.Local == "note" {
				var note Note
				err := decoder.DecodeElement(&note, &se)
				if err != nil {
					slog.Error("XML decoding error", "error", err)
					continue
				}
				e.NoteChannel <- note
			}
		}
	}
	slog.Debug("completed XML decoding: closing noteChannel")
	close(e.NoteChannel)
	return nil
}

func (e *EnexFile) PrintNoteInfo() {
	i := 0
	pdfs := 0

	for note := range e.NoteChannel {

		i++
		var resourceInfo []string
		for _, resource := range note.Resources {
			resourceStr := resource.ResourceAttributes.FileName + " - " + resource.Mime
			resourceInfo = append(resourceInfo, resourceStr)

			if resource.Mime == "application/pdf" {
				pdfs++
			}
		}
		resourceInfoStr := strings.Join(resourceInfo, ", ")

		slog.Info(
			note.Title,
			slog.Int("Note Index", i),
			slog.String("Created At", note.Created),
			slog.String("Updated At", note.Updated),
			slog.String("Attached Files", resourceInfoStr),
			slog.String("Tags", strings.Join(note.Tags, ",")),
		)
	}
	slog.Info(fmt.Sprint("total Notes: ", i), "totalNotes", i, "pdfs", pdfs)
}

func checkFileType(mimeType string) (bool, error) {
	// Get configuration and check for errors
	settings, err := config.GetConfig()
	if err != nil {
		return false, err
	}

	// if filetypes contains "any" then allow all file types
	for _, fileType := range settings.FileTypes {
		if fileType == "any" {
			return true, nil
		}
	}

	// Extract the extension from the MIME type
	extension, err := getExtensionFromMimeType(mimeType)
	if err != nil {
		return false, err
	}

	// Convert extension and allowed file types to lowercase for case-insensitive comparison
	extensionLower := strings.ToLower(extension)
	allowedFileTypes := make([]string, len(settings.FileTypes))
	for i, fileType := range settings.FileTypes {
		allowedFileTypes[i] = strings.ToLower(fileType)
		if fileType == "txt" {
			allowedFileTypes[i] = "plain"
		}
	}

	// Check if the extension matches any allowed file type
	for _, allowedType := range allowedFileTypes {
		if extensionLower == allowedType {
			return true, nil
		}
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

func (e *EnexFile) UploadFromNoteChannel(outputFolder string) error {
	slog.Debug("starting UploadFromNoteChannel")
	settings, _ := config.GetConfig()

	for note := range e.NoteChannel {
		if len(note.Resources) < 1 {
			slog.Debug(fmt.Sprintf("ignoring note without attachement: %s", note.Title))
			continue
		}

		e.NumNotes.Add(1)

		for _, resource := range note.Resources {
			slog.Info("processing file",
				slog.String("file", resource.ResourceAttributes.FileName),
			)

			// only process wanted file types
			isWantedFileType, err := checkFileType(resource.Mime)
			if err != nil {
				slog.Error("error when handling MIME type", "error", err)
				continue
			}

			if !isWantedFileType {
				slog.Debug("skipping unwanted file type", "filename", resource.ResourceAttributes.FileName, "filetype", resource.Mime)
				continue
			}

			// add padding if necessary
			data := resource.Data
			padding := len(data) % 4
			if padding > 0 {
				slog.Debug("adding padding", "padding", padding)
				data += strings.Repeat("=", 4-padding)
			}

			// Remove newlines and spaces from Resource.Data
			data = strings.ReplaceAll(resource.Data, "\n", "")
			data = strings.ReplaceAll(data, " ", "")

			// Validate that Resource.Data is valid base64
			validBase64 := regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
			if !validBase64.MatchString(data) {
				slog.Error("data is not valid base64")
				continue
			}

			// Decode the base64 Resource.Data
			decodedData, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				e.FailedNoteChannel <- note
				slog.Error("error decoding resource data", "error", err)
				break
			}

			// // Handle zip files first, regardless of output folder setting
			// if settings.Unzip && strings.HasSuffix(strings.ToLower(resource.ResourceAttributes.FileName), ".zip") {
			// 	slog.Info("processing zip file", "file", resource.ResourceAttributes.FileName)
			//
			// 	// Create a reader from the byte slice to inspect zip contents
			// 	zipReader, err := zip.NewReader(bytes.NewReader(decodedData), int64(len(decodedData)))
			// 	if err != nil {
			// 		slog.Error("failed to create zip reader", "error", err)
			// 		continue
			// 	}
			//
			// 	// Debug output for zip contents
			// 	slog.Info("zip file contents:", "total_files", len(zipReader.File))
			// 	for _, file := range zipReader.File {
			// 		slog.Info("zip entry:",
			// 			"name", file.Name,
			// 			"size", file.UncompressedSize64,
			// 			"compressed_size", file.CompressedSize64,
			// 			"is_dir", file.FileInfo().IsDir())
			// 	}
			//
			// 	// Create a temporary directory for extraction if output folder is not set
			// 	extractDir := outputFolder
			// 	if extractDir == "" {
			// 		extractDir = os.TempDir()
			// 	}
			//
			// 	extractedFiles, err := unzipFile(decodedData, extractDir, e.Fs, resource.ResourceAttributes.FileName)
			// 	if err != nil {
			// 		slog.Error("failed to extract zip file", "error", err)
			// 		continue
			// 	}
			//
			// 	// Track files for cleanup
			// 	var filesToCleanup []string
			//
			// 	for _, file := range extractedFiles {
			// 		slog.Info("uploading extracted file",
			// 			"name", file.Name,
			// 			"mime_type", file.MimeType)
			// 		fileNameWithoutExt := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))
			// 		zipFileNameWithoutExt := strings.TrimSuffix(file.ZipFileName, filepath.Ext(file.ZipFileName))
			// 		err := e.uploadFileToPaperless(
			// 			note.Title+" | "+zipFileNameWithoutExt+" | "+fileNameWithoutExt,
			// 			file.Name,
			// 			file.MimeType,
			// 			file.Data,
			// 			note)
			// 		if err != nil {
			// 			slog.Error("failed to upload extracted file", "error", err)
			// 		}
			// 		// Add file to cleanup list if it's in a temporary directory
			// 		if extractDir == os.TempDir() {
			// 			filesToCleanup = append(filesToCleanup, file.Path)
			// 		}
			// 	}
			//
			// 	// Clean up temporary files
			// 	if extractDir == os.TempDir() {
			// 		for _, filePath := range filesToCleanup {
			// 			if err := e.Fs.Remove(filePath); err != nil {
			// 				slog.Error("failed to clean up temporary file", "file", filePath, "error", err)
			// 			} else {
			// 				slog.Debug("cleaned up temporary file", "file", filePath)
			// 			}
			// 		}
			// 		// Try to remove the temporary directory if it's empty
			// 		if err := e.Fs.Remove(extractDir); err != nil {
			// 			slog.Debug("could not remove temporary directory (may not be empty)", "dir", extractDir)
			// 		}
			// 	}
			// 	continue // skip to next resource
			// }

			// if outputFolder is set, output to disk and continue
			if outputFolder != "" {
				if err := e.Fs.MkdirAll(outputFolder, 0755); err != nil {
					e.FailedNoteChannel <- note
					slog.Error(fmt.Sprintf("failed to create directory: %v", err))
					break
				}

				fileName := filepath.Join(outputFolder, resource.ResourceAttributes.FileName)

				// TODO: improve duplicate handling
				exists, err := afero.Exists(e.Fs, fileName)
				if err != nil {
					e.FailedNoteChannel <- note
					slog.Error(fmt.Sprintf("failed to check if file exists: %v", err))
					break
				} else if exists {
					slog.Warn(fmt.Sprintf("file already exists: %s", fileName))
					// Prompt user for overwrite confirmation
					reader := bufio.NewReader(os.Stdin)
					fmt.Printf("File %s already exists. Do you want to overwrite it? (y/N): ", fileName)
					response, _ := reader.ReadString('\n')
					response = strings.TrimSpace(response)

					// Handle the response
					if strings.ToLower(response) != "y" {
						slog.Warn(fmt.Sprintf("skipping file: %v", fileName))
						e.FailedNoteChannel <- note
						break
					}
				}

				if err := afero.WriteFile(e.Fs, fileName, decodedData, 0644); err != nil {
					e.FailedNoteChannel <- note
					slog.Error(fmt.Sprintf("failed to write file %v", err))
					break
				}
				e.Uploads.Add(1)
				break
			}

			formattedCreatedDate, err := ConvertDateFormat(note.Created)
			if err != nil {
				e.FailedNoteChannel <- note
				slog.Error("error converting date format", "error", err)
				break
			}

			// Combine note.Tags and additional tags into one slice to process
			allTags := append([]string{}, note.Tags...)
			if len(settings.AdditionalTags) > 0 {
				allTags = append(allTags, settings.AdditionalTags...)
			}

			// if resource.ResourceAttributes.FileName is empty, use the note title
			if resource.ResourceAttributes.FileName == "" {
				resource.ResourceAttributes.FileName = note.Title
			}

			// Create PaperlessFile
			paperlessFile := paperless.NewPaperlessFile(
				note.Title,
				resource.ResourceAttributes.FileName,
				resource.Mime,
				formattedCreatedDate,
				decodedData,
				allTags,
			)

			// Upload
			err = paperlessFile.Upload()
			if err != nil {
				e.FailedNoteChannel <- note
				slog.Error("failed to upload file", "error", err)
				break
			}

			e.Uploads.Add(1)
		}
	}

	return nil
}

// SaveAttachments saves all the resources in each note to a folder named after the note's title
func (e *EnexFile) SaveAttachments() error {
	for note := range e.NoteChannel {
		config, _ := config.GetConfig()

		folderName := config.OutputFolder
		if err := e.Fs.MkdirAll(folderName, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %v", err)
		}

		for i, resource := range note.Resources {
			decodedData, err := base64.StdEncoding.DecodeString(resource.Data)
			if err != nil {
				return fmt.Errorf("failed to decode base64 data for resource %d: %v", i, err)
			}

			fileName := filepath.Join(folderName, resource.ResourceAttributes.FileName)
			if err := afero.WriteFile(e.Fs, fileName, decodedData, 0644); err != nil {
				return fmt.Errorf("failed to write file: %v", err)
			}
		}
	}
	return nil
}

func (e *EnexFile) FailedNoteCatcher(failedNotes *[]Note) {
	slog.Debug("starting FailedNoteCatcher")
	for note := range e.FailedNoteChannel {
		*failedNotes = append(*failedNotes, note)
	}
}

func (e *EnexFile) RetryFeeder(failedNotes *[]Note) {
	slog.Debug("starting RetryFeeder")
	for _, note := range *failedNotes {
		e.NoteChannel <- note
	}
	close(e.NoteChannel)
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

func ConvertDateFormat(dateStr string) (string, error) {
	// Parse the original date string into a time.Time
	parsedTime, err := time.Parse("20060102T150405Z", dateStr)
	if err != nil {
		return "", fmt.Errorf("error parsing time: %v", err)
	}

	// Convert time.Time to the desired string format
	return parsedTime.Format("2006-01-02 15:04:05-07:00"), nil
}
