package enex

import (
	"encoding/base64"
	"encoding/xml"
	"enex2paperless/pkg/paperless"
	"fmt"
	"io"
	"log/slog"
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
	err := e.Fs.MkdirAll(outputFolder, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fileName := filepath.Join(outputFolder, resource.ResourceAttributes.FileName)
	counter := 1

	for {
		// check if the file already exists
		exists, err := afero.Exists(e.Fs, fileName)
		if err != nil {
			return fmt.Errorf("failed to check if file exists: %w", err)
		}

		if !exists {
			// if the file doesn't exist, write the file
			if err := afero.WriteFile(e.Fs, fileName, decodedData, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

			slog.Info(fmt.Sprintf("file saved: %s", fileName))
			return nil
		}

		// if file exists, construct a new file name with a counter
		baseName := strings.TrimSuffix(resource.ResourceAttributes.FileName, filepath.Ext(resource.ResourceAttributes.FileName))
		extension := filepath.Ext(resource.ResourceAttributes.FileName)
		fileName = filepath.Join(outputFolder, fmt.Sprintf("%s-%d%s", baseName, counter, extension))
		counter++
	}
}

func (e *EnexFile) UploadFromNoteChannel(outputFolder string) error {
	slog.Debug("starting UploadFromNoteChannel")

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
			continue
		}

		// Combine note.Tags and additional tags into one slice to process
		allTags := append([]string{}, note.Tags...)
		if len(e.config.AdditionalTags) > 0 {
			allTags = append(allTags, e.config.AdditionalTags...)
		}

		for _, resource := range note.Resources {
			slog.Info("processing file",
				slog.String("file", resource.ResourceAttributes.FileName),
			)

			// only process wanted file types
			isWantedFileType, err := e.checkFileType(resource.Mime)
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

			// if resource.ResourceAttributes.FileName is empty, use the note title
			if resource.ResourceAttributes.FileName == "" {
				resource.ResourceAttributes.FileName = note.Title
			}

			// Handle ZIP files if the resource is a ZIP file
			fileName := strings.ToLower(resource.ResourceAttributes.FileName)
			if strings.HasSuffix(fileName, ".zip") {
				err = e.processZipFile(decodedData, resource, note, outputFolder, formattedCreatedDate, allTags)
				if err != nil {
					slog.Error("error processing zip file", "error", err)
				}
				continue // Skip to next resource after processing the ZIP file
			}

			// if outputFolder is set, output to disk and continue
			if outputFolder != "" {
				err = e.SaveResourceToDisk(decodedData, resource, outputFolder)
				if err != nil {
					e.FailedNoteChannel <- note
					slog.Error("failed to save resource to disk", "error", err)
					break
				}
				e.Uploads.Add(1)
				break
			}

			// Upload to Paperless
			paperlessFile := paperless.NewPaperlessFile(
				note.Title,
				resource.ResourceAttributes.FileName,
				resource.Mime,
				formattedCreatedDate,
				decodedData,
				allTags,
				e.config,
			)

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
