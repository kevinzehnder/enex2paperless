package paperless

import (
	"bytes"
	"encoding/json"
	"enex2paperless/internal/config"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func GetTagID(tagName string) (int, error) {
	settings, _ := config.GetConfig()

	// Use HTTP client to send GET request
	url := fmt.Sprintf("%v/api/tags/?name__iexact=%s", settings.PaperlessAPI, url.QueryEscape(tagName))

	resp, err := makeAuthenticatedRequest("GET", url, settings.Username, settings.Password, nil)
	if err != nil {
		return 0, fmt.Errorf("Failed to retrieve tags: %v", err)
	}
	defer resp.Body.Close()

	var tagResponse struct {
		Count   int `json:"count"`
		Results []struct {
			ID int `json:"id"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tagResponse); err != nil {
		return 0, fmt.Errorf("Failed to decode response: %v", err)
	}

	if tagResponse.Count == 0 {
		return 0, fmt.Errorf("Tag not found") // Tag not found, return error or zero value
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
		return 0, fmt.Errorf("Failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("Failed to create request: %v", err)
	}

	// Basic Auth and Content-Type setting
	req.SetBasicAuth(settings.Username, settings.Password)
	req.Header.Set("Content-Type", "application/json")

	// Debug print request

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Read and debug print the response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("Failed to read response body: %v", err)
	}
	// slog.Debug("Response Body", "body", string(bodyBytes))

	// Unmarshal the response to get the tag ID
	var tagResponse TagResponse
	err = json.Unmarshal(bodyBytes, &tagResponse)
	if err != nil {
		return 0, fmt.Errorf("Failed to unmarshal response: %v", err)
	}

	return tagResponse.ID, nil
}

type TagResponse struct {
	ID int `json:"id"`
	// Other fields, if necessary
}

func makeAuthenticatedRequest(method, url, username, password string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(username, password)

	client := &http.Client{
		Timeout: time.Second * 10,
	}
	return client.Do(req)
}

func ConvertDateFormat(dateStr string) (string, error) {
	// Parse the original date string into a time.Time
	parsedTime, err := time.Parse("20060102T150405Z", dateStr)
	if err != nil {
		return "", fmt.Errorf("Error parsing time: %v", err)
	}

	// Convert time.Time to the desired string format
	return parsedTime.Format("2006-01-02 15:04:05-07:00"), nil
}
