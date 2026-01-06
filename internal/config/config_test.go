package config

import (
	"os"
	"testing"

	"github.com/knadh/koanf/providers/fs"
	"github.com/spf13/afero"
)

func TestDefaultConfiguration(t *testing.T) {

	_, err := GetConfig()
	if err != nil {
		t.Errorf("configuration error: %s", err)
	}

}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    string
		envVars        map[string]string
		envPrefix      string
		expectedConfig Config
		expectError    bool
	}{
		{
			name: "loads basic YAML configuration",
			yamlContent: `
PaperlessAPI: https://example.com/api
token: test-token-123
filetypes:
  - pdf
  - docx
`,
			envVars:   map[string]string{},
			envPrefix: "E2P_",
			expectedConfig: Config{
				PaperlessAPI: "https://example.com/api",
				Token:        "test-token-123",
				FileTypes:    []string{"pdf", "docx"},
			},
			expectError: false,
		},
		{
			name: "environment variables override YAML",
			yamlContent: `
paperlessapi: https://original.com/api
token: original-token
filetypes:
  - pdf
`,
			envVars: map[string]string{
				"E2P_PAPERLESSAPI": "https://overridden.com/api",
				"E2P_TOKEN":        "overridden-token",
			},
			envPrefix: "E2P_",
			expectedConfig: Config{
				PaperlessAPI: "https://overridden.com/api",
				Token:        "overridden-token",
				FileTypes:    []string{"pdf"},
			},
			expectError: false,
		},
		{
			name: "environment variables with underscores and space-separated arrays",
			yamlContent: `
paperlessapi: https://example.com/api
token: test-token
filetypes:
  - pdf
`,
			envVars: map[string]string{
				"E2P_FILE_TYPES":     "pdf docx xlsx txt",
				"E2P_OUTPUT_FOLDER":  "/custom/output",
				"E2P_ADDITIONALTAGS": "urgent important",
			},
			envPrefix: "E2P_",
			expectedConfig: Config{
				PaperlessAPI:   "https://example.com/api",
				Token:          "test-token",
				FileTypes:      []string{"pdf", "docx", "xlsx", "txt"},
				OutputFolder:   "/custom/output",
				AdditionalTags: []string{"urgent", "important"},
			},
			expectError: false,
		},
		{
			name: "username and password authentication",
			yamlContent: `
paperlessapi: https://example.com/api
username: testuser
password: testpass
filetypes:
  - pdf
`,
			envVars:   map[string]string{},
			envPrefix: "E2P_",
			expectedConfig: Config{
				PaperlessAPI: "https://example.com/api",
				Username:     "testuser",
				Password:     "testpass",
				FileTypes:    []string{"pdf"},
			},
			expectError: false,
		},
		{
			name: "validation error - missing required fields",
			yamlContent: `
paperlessapi: https://example.com/api
`,
			envVars:     map[string]string{},
			envPrefix:   "E2P_",
			expectError: true,
		},
		{
			name: "validation error - invalid URL",
			yamlContent: `
paperlessapi: not-a-valid-url
token: test-token
filetypes:
  - pdf
`,
			envVars:     map[string]string{},
			envPrefix:   "E2P_",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create in-memory filesystem
			memFS := afero.NewMemMapFs()

			// Write YAML content to in-memory file
			err := afero.WriteFile(memFS, "config.yaml", []byte(tt.yamlContent), 0644)
			if err != nil {
				t.Fatalf("failed to write in-memory config file: %v", err)
			}

			// Set up environment variables
			for key, value := range tt.envVars {
				err := os.Setenv(key, value)
				if err != nil {
					t.Fatalf("failed to set env var %s: %v", key, err)
				}
			}
			defer func() {
				// Clean up environment variables
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Convert afero.FS to io/fs.FS for koanf
			ioFS := afero.NewIOFS(memFS)

			// Load configuration
			cfg, err := LoadConfig(fs.Provider(ioFS, "config.yaml"), tt.envPrefix)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify configuration fields
			if cfg.PaperlessAPI != tt.expectedConfig.PaperlessAPI {
				t.Errorf("PaperlessAPI = %q, want %q", cfg.PaperlessAPI, tt.expectedConfig.PaperlessAPI)
			}
			if cfg.Token != tt.expectedConfig.Token {
				t.Errorf("Token = %q, want %q", cfg.Token, tt.expectedConfig.Token)
			}
			if cfg.Username != tt.expectedConfig.Username {
				t.Errorf("Username = %q, want %q", cfg.Username, tt.expectedConfig.Username)
			}
			if cfg.Password != tt.expectedConfig.Password {
				t.Errorf("Password = %q, want %q", cfg.Password, tt.expectedConfig.Password)
			}
			if cfg.OutputFolder != tt.expectedConfig.OutputFolder {
				t.Errorf("OutputFolder = %q, want %q", cfg.OutputFolder, tt.expectedConfig.OutputFolder)
			}

			// Verify slices
			if len(cfg.FileTypes) != len(tt.expectedConfig.FileTypes) {
				t.Errorf("FileTypes length = %d, want %d", len(cfg.FileTypes), len(tt.expectedConfig.FileTypes))
			} else {
				for i, ft := range cfg.FileTypes {
					if ft != tt.expectedConfig.FileTypes[i] {
						t.Errorf("FileTypes[%d] = %q, want %q", i, ft, tt.expectedConfig.FileTypes[i])
					}
				}
			}

			if len(cfg.AdditionalTags) != len(tt.expectedConfig.AdditionalTags) {
				t.Errorf("AdditionalTags length = %d, want %d", len(cfg.AdditionalTags), len(tt.expectedConfig.AdditionalTags))
			} else {
				for i, tag := range cfg.AdditionalTags {
					if tag != tt.expectedConfig.AdditionalTags[i] {
						t.Errorf("AdditionalTags[%d] = %q, want %q", i, tag, tt.expectedConfig.AdditionalTags[i])
					}
				}
			}
		})
	}
}
