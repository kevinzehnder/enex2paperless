//go:build integration

package integration

import (
	"enex2paperless/internal/config"
	"fmt"
	"os"
	"testing"
)

// GetTestConfig creates a configuration for integration tests
// It reads from environment variables with fallback defaults for local Docker setup
func GetTestConfig(t *testing.T) config.Config {
	t.Helper()

	paperlessAPI := getEnvOrDefault("E2P_PAPERLESSAPI", "http://localhost:8000")
	token := getEnvOrDefault("E2P_TOKEN", "")
	username := getEnvOrDefault("E2P_USERNAME", "admin")
	password := getEnvOrDefault("E2P_PASSWORD", "admin123")
	fileTypes := []string{"pdf", "png", "jpg", "jpeg"}

	cfg := config.Config{
		PaperlessAPI: paperlessAPI,
		Token:        token,
		Username:     username,
		Password:     password,
		FileTypes:    fileTypes,
	}

	// Validate the config
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Invalid test configuration: %v", err)
	}

	return cfg
}

// GetPaperlessClient creates a Paperless client for verification
func GetPaperlessClient(t *testing.T, cfg config.Config) *PaperlessClient {
	t.Helper()
	return NewPaperlessClient(cfg.PaperlessAPI, cfg.Token, cfg.Username, cfg.Password)
}

// CleanupTestDocuments removes documents created during tests
func CleanupTestDocuments(t *testing.T, client *PaperlessClient, titlePrefix string) {
	t.Helper()

	docs, err := client.GetDocuments()
	if err != nil {
		t.Logf("Warning: failed to list documents for cleanup: %v", err)
		return
	}

	for _, doc := range docs {
		if len(doc.Title) >= len(titlePrefix) && doc.Title[:len(titlePrefix)] == titlePrefix {
			if err := client.DeleteDocument(doc.ID); err != nil {
				t.Logf("Warning: failed to delete test document %d: %v", doc.ID, err)
			} else {
				t.Logf("Cleaned up test document: %s (ID: %d)", doc.Title, doc.ID)
			}
		}
	}
}

// CleanupTestTags removes tags created during tests
func CleanupTestTags(t *testing.T, client *PaperlessClient, tagNames []string) {
	t.Helper()

	for _, tagName := range tagNames {
		tag, err := client.GetTagByName(tagName)
		if err != nil {
			continue // Tag doesn't exist, nothing to clean
		}

		if err := client.DeleteTag(tag.ID); err != nil {
			t.Logf("Warning: failed to delete test tag %s: %v", tagName, err)
		} else {
			t.Logf("Cleaned up test tag: %s (ID: %d)", tagName, tag.ID)
		}
	}
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// SkipIfPaperlessUnavailable checks if Paperless is reachable and skips the test if not
func SkipIfPaperlessUnavailable(t *testing.T, cfg config.Config) {
	t.Helper()

	client := NewPaperlessClient(cfg.PaperlessAPI, cfg.Token, cfg.Username, cfg.Password)
	_, err := client.GetDocuments()
	if err != nil {
		t.Skipf("Paperless instance not available at %s: %v", cfg.PaperlessAPI, err)
	}
}

// AssertDocumentExists verifies that a document with the given title exists
func AssertDocumentExists(t *testing.T, client *PaperlessClient, title string) *Document {
	t.Helper()

	doc, err := client.GetDocumentByTitle(title)
	if err != nil {
		t.Fatalf("Expected document '%s' to exist, but got error: %v", title, err)
	}

	if doc.Title != title {
		t.Fatalf("Expected document title to be '%s', got '%s'", title, doc.Title)
	}

	return doc
}

// AssertDocumentHasTag verifies that a document has a specific tag
func AssertDocumentHasTag(t *testing.T, client *PaperlessClient, doc *Document, tagName string) {
	t.Helper()

	tag, err := client.GetTagByName(tagName)
	if err != nil {
		t.Fatalf("Expected tag '%s' to exist, but got error: %v", tagName, err)
	}

	hasTag := false
	for _, tagID := range doc.Tags {
		if tagID == tag.ID {
			hasTag = true
			break
		}
	}

	if !hasTag {
		t.Fatalf("Document '%s' does not have tag '%s'", doc.Title, tagName)
	}
}

// GetAssetPath returns the absolute path to a test asset file
func GetAssetPath(filename string) string {
	// Assuming tests run from project root or test/integration directory
	// Try both paths
	paths := []string{
		fmt.Sprintf("../../assets/%s", filename),
		fmt.Sprintf("assets/%s", filename),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return fmt.Sprintf("../../assets/%s", filename)
}
