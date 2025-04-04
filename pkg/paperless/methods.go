package paperless

import (
	"bytes"
	"encoding/json"
	"enex2paperless/internal/config"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
)

// Upload uploads the file to Paperless-NGX
func (pf *PaperlessFile) Upload() error {
	settings, _ := config.GetConfig()
	url := fmt.Sprintf("%s/api/documents/post_document/", settings.PaperlessAPI)

	// Create a new buffer and multipart writer for form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Set form fields
	err := writer.WriteField("title", pf.Title)
	if err != nil {
		return fmt.Errorf("error setting form fields: %v", err)
	}

	// pull CreatedDate from STRUCT
	formattedCreatedDate, err := ConvertDateFormat(pf.Created)
	if err != nil {
		return fmt.Errorf("error converting date format: %v", err)
	}
	_ = writer.WriteField("created", formattedCreatedDate)
	// pull CreatedDate from STRUCT

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
	if settings.Token != "" {
		req.Header.Set("Authorization", "Token "+settings.Token)
	} else {
		req.SetBasicAuth(settings.Username, settings.Password)
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
		id, err := GetTagID(tagName)
		if err != nil {
			return fmt.Errorf("failed to check for tag: %v", err)
		}

		if id == 0 {
			slog.Debug("creating tag", "tag", tagName)
			id, err = CreateTag(tagName)
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

func GetTagID(tagName string) (int, error) {
	settings, _ := config.GetConfig()

	// Use HTTP client to send GET request
	url := fmt.Sprintf("%v/api/tags/?name__iexact=%s", settings.PaperlessAPI, url.QueryEscape(tagName))

	req, err := http.NewRequest("GET", url, nil)

	// auth
	if settings.Token != "" {
		req.Header.Set("Authorization", "Token "+settings.Token)
	} else {
		req.SetBasicAuth(settings.Username, settings.Password)
	}

	// Send the request
	slog.Debug("sending GET request")

	slog.Debug("request details",
		"method", req.Method,
		"url", req.URL.String(),
		"headers", req.Header)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve tags: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// print response body
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)

		slog.Error("non 200 status code received", "status code", resp.StatusCode, "body", buf.String())

		return 0, fmt.Errorf("non 200 status code")
	}

	var tagResponse struct {
		Count   int `json:"count"`
		Results []struct {
			ID int `json:"id"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tagResponse); err != nil {
		return 0, fmt.Errorf("failed to decode response: %v", err)
	}

	if tagResponse.Count == 0 {
		slog.Debug("no tag found with name", "name", tagName)
		return 0, nil // Tag not found, but not an error
	}

	return tagResponse.Results[0].ID, nil // Return the ID of the first matching tag
}

func CreateTag(tagName string) (int, error) {
	settings, _ := config.GetConfig()

	url := fmt.Sprintf("%v/api/tags/", settings.PaperlessAPI)
	jsonData, err := json.Marshal(map[string]interface{}{
		"name": tagName,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %v", err)
	}

	// auth
	if settings.Token != "" {
		req.Header.Set("Authorization", "Token "+settings.Token)
	} else {
		req.SetBasicAuth(settings.Username, settings.Password)
	}
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("request details",
		"method", req.Method,
		"url", req.URL.String(),
		"headers", req.Header,
		"body", string(jsonData))

	// send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		// If creation failed, the tag might have been created by another goroutine
		// Try to get the tag ID again
		id, err := GetTagID(tagName)
		if err != nil {
			return 0, fmt.Errorf("failed to create tag and couldn't verify if it exists: %v", err)
		}
		if id != 0 {
			// Tag exists now, probably created by another goroutine
			slog.Debug("tag was created by another process", "tag", tagName, "id", id)
			return id, nil
		}

		// If we still can't find the tag, then there's a real error
		slog.Error("non 201 status code received", "status code", resp.StatusCode)

		// print response body
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		slog.Error("response:", "body", buf.String())

		return 0, fmt.Errorf("failed to create tag")
	}

	// read response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %v", err)
	}
	// slog.Debug("Response Body", "body", string(bodyBytes))

	// Unmarshal the response to get the tag ID
	var tagResponse TagResponse
	err = json.Unmarshal(bodyBytes, &tagResponse)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %v", err)
	}
	return tagResponse.ID, nil
}

type TagResponse struct {
	ID int `json:"id"`
	// Other fields, if necessary
}
