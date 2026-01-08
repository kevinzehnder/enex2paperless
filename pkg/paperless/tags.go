package paperless

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
)

var (
	tagCache      = make(map[string]int)
	tagCacheMutex sync.RWMutex
)

// getOrCreateTagID retrieves or creates a tag ID in a thread-safe manner
func (pf *PaperlessFile) getOrCreateTagID(tagName string) (int, error) {
	// First check the cache with a read lock
	tagCacheMutex.RLock()
	if id, exists := tagCache[tagName]; exists {
		tagCacheMutex.RUnlock()
		slog.Debug("tag found in cache", "tag", tagName, "id", id)
		return id, nil
	}
	tagCacheMutex.RUnlock()

	// If not in cache, acquire write lock to prevent concurrent creation
	tagCacheMutex.Lock()
	defer tagCacheMutex.Unlock()

	// Double-check the cache in case another goroutine added it
	if id, exists := tagCache[tagName]; exists {
		slog.Debug("tag found in cache after lock", "tag", tagName, "id", id)
		return id, nil
	}

	// Try to get the tag from the API
	id, err := pf.getTagID(tagName)
	if err != nil {
		return 0, fmt.Errorf("failed to check for tag: %w", err)
	}

	if id == 0 {
		// Tag doesn't exist, create it
		slog.Debug("creating tag", "tag", tagName)
		id, err = pf.createTag(tagName)
		if err != nil {
			return 0, fmt.Errorf("couldn't create tag: %w", err)
		}
	} else {
		slog.Debug("found tag", "tag", tagName, "id", id)
	}

	// Cache the result
	tagCache[tagName] = id
	return id, nil
}

func (pf *PaperlessFile) getTagID(tagName string) (int, error) {
	// Use HTTP client to send GET request
	url := fmt.Sprintf("%v/api/tags/?name__iexact=%s", pf.config.PaperlessAPI, url.QueryEscape(tagName))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// auth
	if pf.config.Token != "" {
		req.Header.Set("Authorization", "Token "+pf.config.Token)
	} else {
		req.SetBasicAuth(pf.config.Username, pf.config.Password)
	}

	// Send the request
	slog.Debug("sending GET request")

	slog.Debug("request details",
		"method", req.Method,
		"url", req.URL.String(),
		"headers", req.Header)

	client := getSharedClient()
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve tags: %w", err)
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
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if tagResponse.Count == 0 {
		slog.Debug("no tag found with name", "name", tagName)
		return 0, nil // Tag not found, but not an error
	}

	return tagResponse.Results[0].ID, nil // Return the ID of the first matching tag
}

func (pf *PaperlessFile) createTag(tagName string) (int, error) {
	url := fmt.Sprintf("%v/api/tags/", pf.config.PaperlessAPI)
	jsonData, err := json.Marshal(map[string]interface{}{
		"name": tagName,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// auth
	if pf.config.Token != "" {
		req.Header.Set("Authorization", "Token "+pf.config.Token)
	} else {
		req.SetBasicAuth(pf.config.Username, pf.config.Password)
	}
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("request details",
		"method", req.Method,
		"url", req.URL.String(),
		"headers", req.Header,
		"body", string(jsonData))

	// send request
	client := getSharedClient()
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		// If creation failed, the tag might have been created by another goroutine
		// Try to get the tag ID again
		id, err := pf.getTagID(tagName)
		if err != nil {
			return 0, fmt.Errorf("failed to create tag and couldn't verify if it exists: %w", err)
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
		return 0, fmt.Errorf("failed to read response body: %w", err)
	}
	// slog.Debug("Response Body", "body", string(bodyBytes))

	// Unmarshal the response to get the tag ID
	var tagResponse TagResponse
	err = json.Unmarshal(bodyBytes, &tagResponse)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return tagResponse.ID, nil
}

type TagResponse struct {
	ID int `json:"id"`
	// Other fields, if necessary
}
