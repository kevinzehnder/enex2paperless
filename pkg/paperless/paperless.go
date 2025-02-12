package paperless

import (
	"bytes"
	"encoding/json"
	"enex2paperless/internal/config"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

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

func ConvertDateFormat(dateStr string) (string, error) {
	// Parse the original date string into a time.Time
	parsedTime, err := time.Parse("20060102T150405Z", dateStr)
	if err != nil {
		return "", fmt.Errorf("error parsing time: %v", err)
	}

	// Convert time.Time to the desired string format
	return parsedTime.Format("2006-01-02 15:04:05-07:00"), nil
}
