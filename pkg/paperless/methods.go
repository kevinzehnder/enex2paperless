package paperless

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
)

// Upload uploads the file to Paperless-NGX
func (pf *PaperlessFile) Upload() error {
	url := fmt.Sprintf("%s/api/documents/post_document/", pf.config.PaperlessAPI)

	// Create a new buffer and multipart writer for form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Set form fields
	err := writer.WriteField("title", pf.Title)
	if err != nil {
		return fmt.Errorf("error setting form fields: %v", err)
	}

	err = writer.WriteField("created", pf.Created)
	if err != nil {
		return fmt.Errorf("error setting form fields: %v", err)
	}

	// Process tags
	err = pf.processTags()
	if err != nil {
		return err
	}

	// Add tag IDs to POST request
	for _, id := range pf.TagIds {
		err = writer.WriteField("tags", strconv.Itoa(id))
		if err != nil {
			return fmt.Errorf("couldn't write fields: %v", err)
		}
	}

	// Create form file header
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="document"; filename="%s"`, pf.FileName))
	h.Set("Content-Type", pf.MimeType)

	// Create the file field with the header and write data into it
	part, err := writer.CreatePart(h)
	if err != nil {
		return fmt.Errorf("error creating multipart writer: %v", err)
	}

	_, err = io.Copy(part, bytes.NewReader(pf.Data))
	if err != nil {
		return fmt.Errorf("error writing file data: %v", err)
	}

	// Close the writer to finish the multipart content
	writer.Close()

	// Create a new HTTP request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("error creating new HTTP request: %v", err)
	}

	// Get settings for authentication
	if pf.config.Token != "" {
		req.Header.Set("Authorization", "Token "+pf.config.Token)
	} else {
		req.SetBasicAuth(pf.config.Username, pf.config.Password)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request
	slog.Debug("sending POST request", "file", pf.FileName)
	slog.Debug("request details", "method", req.Method, "url", req.URL.String(), "headers", req.Header)

	resp, err := pf.client.Do(req)
	if err != nil {
		return fmt.Errorf("error making POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// print response body
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		slog.Error("non 200 status code received", "status code", resp.StatusCode)
		slog.Error("response:", "body", buf.String())
		return fmt.Errorf("non 200 status code received (%d): %s", resp.StatusCode, buf.String())
	}

	return nil
}

// processTags gets or creates all tags and populates the TagIds field
func (pf *PaperlessFile) processTags() error {
	// Process each tag
	for _, tagName := range pf.Tags {
		id, err := pf.getTagID(tagName)
		if err != nil {
			return fmt.Errorf("failed to check for tag: %v", err)
		}

		if id == 0 {
			slog.Debug("creating tag", "tag", tagName)
			id, err = pf.createTag(tagName)
			if err != nil {
				return fmt.Errorf("couldn't create tag: %v", err)
			}
		} else {
			slog.Debug(fmt.Sprintf("found tag: %s with ID: %v", tagName, id))
		}

		pf.TagIds = append(pf.TagIds, id)
	}

	return nil
}
