package enex

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"enex2paperless/internal/config"
	"enex2paperless/pkg/paperless"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

func (e *EnexFile) ReadFromFile(filePath string, noteChannel chan<- Note) error {
	slog.Debug(fmt.Sprintf("opening file: %v", filePath))
	file, err := e.Fs.Open(filePath)
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
				noteChannel <- note
			}
		}
	}
	slog.Debug("completed XML decoding: closing noteChannel")
	close(noteChannel)
	return nil
}

func (e *EnexFile) PrintNoteInfo(noteChannel chan Note) {
	i := 0
	pdfs := 0

	for note := range noteChannel {

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

// uploadFileToPaperless handles the common upload logic for both regular and extracted files
func (e *EnexFile) uploadFileToPaperless(title string, fileName string, mimeType string, data []byte, note Note, url string, failedNoteChannel chan Note) error {
	// Create a new buffer and multipart writer for form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Set form fields
	err := writer.WriteField("title", title)
	if err != nil {
		failedNoteChannel <- note
		slog.Error("error setting form fields", "error", err)
		return fmt.Errorf("error setting form fields: %v", err)
	}

	formattedCreatedDate, err := paperless.ConvertDateFormat(note.Created)
	if err != nil {
		failedNoteChannel <- note
		slog.Error("error converting date format", "error", err)
		return fmt.Errorf("error converting date format: %v", err)
	}
	_ = writer.WriteField("created", formattedCreatedDate)

	// Get or create tag IDs
	var tagIDs []int
	for _, tagName := range note.Tags {
		id, err := paperless.GetTagID(tagName)
		if err != nil {
			failedNoteChannel <- note
			slog.Error("failed to check for tag", "error", err)
			return fmt.Errorf("failed to check for tag: %v", err)
		}

		if id == 0 {
			slog.Debug("creating tag", "tag", tagName)
			id, err = paperless.CreateTag(tagName)
			if err != nil {
				failedNoteChannel <- note
				slog.Error("couldn't create tag", "error", err)
				return fmt.Errorf("couldn't create tag: %v", err)
			}
		} else {
			slog.Debug(fmt.Sprintf("found tag: %s with ID: %v", tagName, id))
		}

		tagIDs = append(tagIDs, id)
	}

	// Add tag IDs to POST request
	for _, id := range tagIDs {
		err = writer.WriteField("tags", strconv.Itoa(id))
		if err != nil {
			failedNoteChannel <- note
			slog.Error("couldn't write fields", "error", err)
			return fmt.Errorf("couldn't write fields: %v", err)
		}
	}

	// Create form file header
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="document"; filename="%s"`, fileName))
	h.Set("Content-Type", mimeType)

	// Create the file field with the header and write data into it
	part, err := writer.CreatePart(h)
	if err != nil {
		failedNoteChannel <- note
		slog.Error("error creating multipart writer", "error", err)
		return fmt.Errorf("error creating multipart writer: %v", err)
	}

	_, err = io.Copy(part, bytes.NewReader(data))
	if err != nil {
		failedNoteChannel <- note
		slog.Error("error writing file data", "error", err)
		return fmt.Errorf("error writing file data: %v", err)
	}

	// Close the writer to finish the multipart content
	writer.Close()

	// Create a new HTTP request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		failedNoteChannel <- note
		slog.Error("error creating new HTTP request", "error", err)
		return fmt.Errorf("error creating new HTTP request: %v", err)
	}

	// Get settings for authentication
	settings, _ := config.GetConfig()

	// auth
	if settings.Token != "" {
		req.Header.Set("Authorization", "Token "+settings.Token)
	} else {
		req.SetBasicAuth(settings.Username, settings.Password)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request
	slog.Debug("sending POST request", "file", fileName)
	slog.Debug("request details", "method", req.Method, "url", req.URL.String(), "headers", req.Header)

	resp, err := e.client.Do(req)
	if err != nil {
		failedNoteChannel <- note
		slog.Error("error making POST request", "error", err)
		return fmt.Errorf("error making POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// print response body
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		failedNoteChannel <- note
		slog.Error("non 200 status code received", "status code", resp.StatusCode)
		slog.Error("response:", "body", buf.String())
		return fmt.Errorf("non 200 status code received (%d): %s", resp.StatusCode, buf.String())
	}

	e.Uploads.Add(1)
	return nil
}

func (e *EnexFile) UploadFromNoteChannel(noteChannel, failedNoteChannel chan Note, outputFolder string) error {
	slog.Debug("starting UploadFromNoteChannel")
	settings, _ := config.GetConfig()

	url := fmt.Sprintf("%s/api/documents/post_document/", settings.PaperlessAPI)

	for note := range noteChannel {
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
				failedNoteChannel <- note
				slog.Error("error decoding resource data", "error", err)
				break
			}

			// Handle zip files first, regardless of output folder setting
			if settings.Unzip && strings.HasSuffix(strings.ToLower(resource.ResourceAttributes.FileName), ".zip") {
				slog.Info("processing zip file", "file", resource.ResourceAttributes.FileName)

				// Create a reader from the byte slice to inspect zip contents
				zipReader, err := zip.NewReader(bytes.NewReader(decodedData), int64(len(decodedData)))
				if err != nil {
					slog.Error("failed to create zip reader", "error", err)
					continue
				}

				// Debug output for zip contents
				slog.Info("zip file contents:", "total_files", len(zipReader.File))
				for _, file := range zipReader.File {
					slog.Info("zip entry:",
						"name", file.Name,
						"size", file.UncompressedSize64,
						"compressed_size", file.CompressedSize64,
						"is_dir", file.FileInfo().IsDir())
				}

				// Create a temporary directory for extraction if output folder is not set
				extractDir := outputFolder
				if extractDir == "" {
					extractDir = os.TempDir()
				}

				extractedFiles, err := unzipFile(decodedData, extractDir, e.Fs, resource.ResourceAttributes.FileName)
				if err != nil {
					slog.Error("failed to extract zip file", "error", err)
					continue
				}

				// Track files for cleanup
				var filesToCleanup []string

				for _, file := range extractedFiles {
					slog.Info("uploading extracted file",
						"name", file.Name,
						"mime_type", file.MimeType)
					fileNameWithoutExt := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))
					zipFileNameWithoutExt := strings.TrimSuffix(file.ZipFileName, filepath.Ext(file.ZipFileName))
					err := e.uploadFileToPaperless(
						note.Title+" | "+zipFileNameWithoutExt+" | "+fileNameWithoutExt,
						file.Name,
						file.MimeType,
						file.Data,
						note,
						url,
						failedNoteChannel)
					if err != nil {
						slog.Error("failed to upload extracted file", "error", err)
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
					// Try to remove the temporary directory if it's empty
					if err := e.Fs.Remove(extractDir); err != nil {
						slog.Debug("could not remove temporary directory (may not be empty)", "dir", extractDir)
					}
				}
				continue // skip to next resource
			}

			// if outputFolder is set, output to disk and continue
			if outputFolder != "" {
				if err := e.Fs.MkdirAll(outputFolder, 0755); err != nil {
					failedNoteChannel <- note
					slog.Error(fmt.Sprintf("failed to create directory: %v", err))
					break
				}

				fileName := filepath.Join(outputFolder, resource.ResourceAttributes.FileName)

				exists, err := afero.Exists(e.Fs, fileName)
				if err != nil {
					failedNoteChannel <- note
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
						failedNoteChannel <- note
						break
					}
				}

				if err := afero.WriteFile(e.Fs, fileName, decodedData, 0644); err != nil {
					failedNoteChannel <- note
					slog.Error(fmt.Sprintf("failed to write file %v", err))
					break
				}
				e.Uploads.Add(1)
				break
			}

			// Upload the file to Paperless
			if resource.ResourceAttributes.FileName == "" {
				resource.ResourceAttributes.FileName = note.Title
			}

			err = e.uploadFileToPaperless(note.Title, resource.ResourceAttributes.FileName, resource.Mime, decodedData, note, url, failedNoteChannel)
			if err != nil {
				failedNoteChannel <- note
				slog.Error("failed to upload file", "error", err)
				break
			}
		}
	}

	return nil
}

// SaveAttachments saves all the resources in each note to a folder named after the note's title
func (e *EnexFile) SaveAttachments(noteChannel chan Note) error {
	for note := range noteChannel {
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

func FailedNoteCatcher(failedNoteChannel chan Note, failedNotes *[]Note) {
	slog.Debug("starting FailedNoteCatcher")
	for note := range failedNoteChannel {
		*failedNotes = append(*failedNotes, note)
	}
}

func RetryFeeder(failedNotes *[]Note, retryChannel chan Note) {
	slog.Debug("starting RetryFeeder")
	for _, note := range *failedNotes {
		retryChannel <- note
	}
	close(retryChannel)
}

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
	Path       string
	Name       string
	Data       []byte
	MimeType   string
	ZipFileName string
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
	if err := fs.MkdirAll(destDir, 0755); err != nil {
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
			Path:       filePath,
			Name:       file.Name,
			Data:       buf.Bytes(),
			MimeType:   getMimeType(file.Name),
			ZipFileName: zipFileName,
		})

		slog.Info("extracted file from zip", "file", file.Name)
	}

	return extractedFiles, nil
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
