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

	"github.com/spf13/afero"
)

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

func (e *EnexFile) SaveResourceToDisk(decodedData []byte, resource Resource, outputFolder string) error {
	// Create the output folder if it doesn't exist
	if err := e.Fs.MkdirAll(outputFolder, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	fileName := filepath.Join(outputFolder, resource.ResourceAttributes.FileName)

	// Check if the file already exists
	exists, err := afero.Exists(e.Fs, fileName)
	if err != nil {
		return fmt.Errorf("failed to check if file exists: %v", err)
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
			return fmt.Errorf("file already exists and overwrite not confirmed")
		}
	}

	// Write the file to disk
	if err := afero.WriteFile(e.Fs, fileName, decodedData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
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
		
		// Convert date format early to fail fast if there's an issue
		formattedCreatedDate, err := convertDateFormat(note.Created)
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

			 // if outputFolder is set, output to disk and continue
			if outputFolder != "" {
				if err := e.SaveResourceToDisk(decodedData, resource, outputFolder); err != nil {
					e.FailedNoteChannel <- note
					slog.Error(fmt.Sprintf("failed to save resource to disk: %v", err))
					break
				}
				e.Uploads.Add(1)
				break
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
