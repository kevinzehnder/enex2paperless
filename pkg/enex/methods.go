package enex

import (
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

	resourceLoop:
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

			// Create a new buffer and multipart writer for form
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			// Set form fields with conditional title
			var documentTitle string
			if len(note.Resources) > 1 {
				// Get filename without extension
				filename := resource.ResourceAttributes.FileName
				extension := filepath.Ext(filename)
				filenameWithoutExt := strings.TrimSuffix(filename, extension)
				documentTitle = fmt.Sprintf("%s | %s", note.Title, filenameWithoutExt)
			} else {
				documentTitle = note.Title
			}
			err = writer.WriteField("title", documentTitle)
			if err != nil {
				failedNoteChannel <- note
				slog.Error("error setting form fields", "error", err)
				break
			}

			formattedCreatedDate, err := paperless.ConvertDateFormat(note.Created)
			if err != nil {
				failedNoteChannel <- note
				slog.Error("error converting date format", "error", err)
				break
			}
			_ = writer.WriteField("created", formattedCreatedDate)

			// Get or create tag IDs
			var tagIDs []int
			for _, tagName := range note.Tags {
				id, err := paperless.GetTagID(tagName)
				if err != nil {
					failedNoteChannel <- note
					slog.Error("failed to check for tag", "error", err)
					break resourceLoop
				}

				if id == 0 {
					slog.Debug("creating tag", "tag", tagName)
					id, err = paperless.CreateTag(tagName)
					if err != nil {
						failedNoteChannel <- note
						slog.Error("couldn't create tag", "error", err.Error())
						break resourceLoop
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
					break
				}
			}

			if resource.ResourceAttributes.FileName == "" {
				resource.ResourceAttributes.FileName = note.Title
			}

			// Create form file header
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="document"; filename="%s"`, resource.ResourceAttributes.FileName))
			h.Set("Content-Type", resource.Mime)

			// Create the file field with the header and write decoded data into it
			part, err := writer.CreatePart(h)
			if err != nil {
				failedNoteChannel <- note
				slog.Error("error creating multipart writer", "error", err)
				break
			}

			_, err = io.Copy(part, bytes.NewReader(decodedData))
			if err != nil {
				failedNoteChannel <- note
				slog.Error("error writing file data", "error", err)
				break
			}

			// Close the writer to finish the multipart content
			writer.Close()

			// Create a new HTTP request
			req, err := http.NewRequest("POST", url, body)
			if err != nil {
				failedNoteChannel <- note
				slog.Error("error creating new HTTP request", "error", err)
				break
			}

			// auth
			if settings.Token != "" {
				req.Header.Set("Authorization", "Token "+settings.Token)
			} else {
				req.SetBasicAuth(settings.Username, settings.Password)
			}

			req.Header.Set("Content-Type", writer.FormDataContentType())

			// Send the request
			slog.Debug("sending POST request")

			slog.Debug("request details",
				"method", req.Method,
				"url", req.URL.String(),
				"headers", req.Header)

			resp, err := e.client.Do(req)
			if err != nil {
				failedNoteChannel <- note
				slog.Error("error making POST request", "error", err)
				break
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				failedNoteChannel <- note
				slog.Error("non 200 status code received", "status code", resp.StatusCode)

				// print response body
				buf := new(bytes.Buffer)
				buf.ReadFrom(resp.Body)
				slog.Error("response:", "body", buf.String())
				break
			}

			e.Uploads.Add(1)
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
