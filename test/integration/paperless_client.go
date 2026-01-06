//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PaperlessClient is a minimal client for verifying documents in Paperless
type PaperlessClient struct {
	baseURL    string
	token      string
	username   string
	password   string
	httpClient *http.Client
}

// NewPaperlessClient creates a new client for Paperless API verification
func NewPaperlessClient(baseURL, token, username, password string) *PaperlessClient {
	return &PaperlessClient{
		baseURL:  baseURL,
		token:    token,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Document represents a Paperless document
type Document struct {
	ID           int      `json:"id"`
	Title        string   `json:"title"`
	Created      string   `json:"created"`
	Added        string   `json:"added"`
	OriginalName string   `json:"original_file_name"`
	Tags         []int    `json:"tags"`
}

// DocumentsResponse represents the API response for documents list
type DocumentsResponse struct {
	Count   int        `json:"count"`
	Results []Document `json:"results"`
}

// Tag represents a Paperless tag
type Tag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TagsResponse represents the API response for tags list
type TagsResponse struct {
	Count   int   `json:"count"`
	Results []Tag `json:"results"`
}

// doRequest performs an authenticated HTTP request
func (c *PaperlessClient) doRequest(method, path string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	if c.token != "" {
		req.Header.Set("Authorization", "Token "+c.token)
	} else if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// GetDocuments retrieves all documents from Paperless
func (c *PaperlessClient) GetDocuments() ([]Document, error) {
	resp, err := c.doRequest("GET", "/api/documents/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var docsResp DocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&docsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return docsResp.Results, nil
}

// GetTrashedDocuments retrieves all trashed documents from Paperless
func (c *PaperlessClient) GetTrashedDocuments() ([]Document, error) {
	resp, err := c.doRequest("GET", "/api/documents/?is_trashed=true")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var docsResp DocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&docsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return docsResp.Results, nil
}

// GetDocumentByTitle finds a document by its title
func (c *PaperlessClient) GetDocumentByTitle(title string) (*Document, error) {
	path := fmt.Sprintf("/api/documents/?title__icontains=%s", title)
	resp, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var docsResp DocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&docsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(docsResp.Results) == 0 {
		return nil, fmt.Errorf("no document found with title: %s", title)
	}

	return &docsResp.Results[0], nil
}

// GetTags retrieves all tags from Paperless
func (c *PaperlessClient) GetTags() ([]Tag, error) {
	resp, err := c.doRequest("GET", "/api/tags/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var tagsResp TagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return tagsResp.Results, nil
}

// GetTagByName finds a tag by its name
func (c *PaperlessClient) GetTagByName(name string) (*Tag, error) {
	tags, err := c.GetTags()
	if err != nil {
		return nil, err
	}

	for _, tag := range tags {
		if tag.Name == name {
			return &tag, nil
		}
	}

	return nil, fmt.Errorf("tag not found: %s", name)
}

// DeleteDocument deletes a document by ID (moves to trash)
func (c *PaperlessClient) DeleteDocument(id int) error {
	path := fmt.Sprintf("/api/documents/%d/", id)
	resp, err := c.doRequest("DELETE", path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete document %d: status %d: %s", id, resp.StatusCode, string(body))
	}

	return nil
}

// PermanentlyDeleteDocument permanently deletes a document (bypassing trash)
func (c *PaperlessClient) PermanentlyDeleteDocument(id int) error {
	// First delete (moves to trash)
	if err := c.DeleteDocument(id); err != nil {
		// If already in trash or doesn't exist, continue
		// We'll try to delete from trash anyway
	}

	// Then delete from trash (permanent deletion)
	path := fmt.Sprintf("/api/documents/%d/", id)
	resp, err := c.doRequest("DELETE", path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Accept 404 as success (already deleted)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to permanently delete document %d: status %d: %s", id, resp.StatusCode, string(body))
	}

	return nil
}

// DeleteTag deletes a tag by ID
func (c *PaperlessClient) DeleteTag(id int) error {
	path := fmt.Sprintf("/api/tags/%d/", id)
	resp, err := c.doRequest("DELETE", path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete tag %d: status %d: %s", id, resp.StatusCode, string(body))
	}

	return nil
}

// WaitForDocument polls until a document with the given title appears or timeout occurs
func (c *PaperlessClient) WaitForDocument(title string, timeout time.Duration) (*Document, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			doc, err := c.GetDocumentByTitle(title)
			if err == nil {
				return doc, nil
			}

			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for document: %s", title)
			}
		}
	}
}
