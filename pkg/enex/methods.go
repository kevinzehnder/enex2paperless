package enex

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"regexp"

	"enex2paperless/internal/config"
	"enex2paperless/pkg/paperless"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

// Method to safely increment NumNotes
func (e *EnexFile) incrementNumNotes() {
	e.mutex.Lock()   // Lock the mutex before updating NumNotes
	e.NumNotes++     // Increment NumNotes
	e.mutex.Unlock() // Unlock the mutex after updating
}

// Method to safely increment Uploads
func (e *EnexFile) incrementUploads() {
	e.mutex.Lock()   // Lock the mutex before updating Uploads
	e.Uploads++      // Increment Uploads
	e.mutex.Unlock() // Unlock the mutex after updating
}

func (e *EnexFile) ReadFromFile(filePath string, noteChannel chan<- Note) error {
	file, err := e.Fs.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	decoder.Strict = false

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
		if t == nil {
			// This should technically never happen if you get io.EOF, but added for completeness
			slog.Error("received nil token without EOF", "error", err)
			break
		}
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "en-note" {
				continue
			}
			if se.Name.Local == "note" {
				var note Note
				decoder.DecodeElement(&note, &se)

				noteChannel <- note
			}
		}
	}
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

func (e *EnexFile) UploadFromNoteChannel(noteChannel, failedNoteChannel chan Note) error {
	settings, _ := config.GetConfig()

	url := settings.PaperlessAPI + "/api/documents/post_document/"

	for note := range noteChannel {

		if len(note.Resources) < 1 {
			continue
		}

		e.incrementNumNotes()

	resourceLoop:
		for _, resource := range note.Resources {

			resourceType := "application/pdf"

			if resource.Mime != resourceType {
				slog.Debug("skipping unwanted file type", "filename", resource.ResourceAttributes.FileName, "filetype", resource.Mime)
				continue
			}

			slog.Info("uploading file",
				slog.String("file", resource.ResourceAttributes.FileName),
			)

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

			// Create a new buffer and multipart writer for form
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			// Set form fields
			err = writer.WriteField("title", note.Title)
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
					slog.Debug("tag not found")
				}

				if id == 0 {
					slog.Debug("creating tag", "tag", tagName)
					id, err = paperless.CreateTag(tagName)
					if err != nil {
						failedNoteChannel <- note
						slog.Error("couldn't create tag", "error", err)
						break resourceLoop
					}
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

			// Set content type and other headers
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.SetBasicAuth(settings.Username, settings.Password)

			// Send the request
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

			e.incrementUploads()
		}
	}

	return nil
}

// SaveAttachments saves all the resources in each note to a folder named after the note's title
func (e *EnexFile) SaveAttachments(noteChannel chan Note) error {
	for note := range noteChannel {

		folderName := "output"
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
	for note := range failedNoteChannel {
		*failedNotes = append(*failedNotes, note)
	}
}

func RetryFeeder(failedNotes *[]Note, retryChannel chan Note) {
	for _, note := range *failedNotes {
		retryChannel <- note
	}
	close(retryChannel)
}
