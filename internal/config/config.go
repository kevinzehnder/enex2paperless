package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var (
	// Global state used only by GetConfig for singleton pattern
	once         sync.Once
	globalConfig Config
	initErr      error
)

type Config struct {
	PaperlessAPI   string   `koanf:"paperlessapi" validate:"required,http_url"`
	Username       string   `koanf:"username" validate:"required_with=Password"`
	Password       string   `koanf:"password" validate:"required_with=Username"`
	Token          string   `koanf:"token" validate:"required_without=Password"`
	FileTypes      []string `koanf:"filetypes" validate:"required"`
	OutputFolder   string   `koanf:"outputfolder"`
	AdditionalTags []string `koanf:"additionaltags"`
}

// Validate validates the configuration using struct tags
func (c Config) Validate() error {
	validate := validator.New()

	err := validate.Struct(c)
	if err != nil {
		var validateErrs validator.ValidationErrors
		if errors.As(err, &validateErrs) {
			for _, e := range validateErrs {
				switch e.StructField() {
				case "Token":
					return fmt.Errorf("bad auth config: need either token or username/password")
				case "Username":
					return fmt.Errorf("if using password, username is required too")
				case "Password":
					return fmt.Errorf("if using username, password is required too")
				default:
					return fmt.Errorf("field %s: %s validation failed", e.Field(), e.Tag())
				}
			}
		}
		return fmt.Errorf("configuration error: %v", err)
	}
	return nil
}

// LoadConfig loads configuration from a YAML file and environment variables.
// It creates a fresh koanf instance, loads from the provided file provider,
// overrides with environment variables (using the provided prefix), and returns a validated Config.
// This function is stateless and can be called multiple times (though typically called once at startup).
func LoadConfig(fileProvider koanf.Provider, envPrefix string) (Config, error) {
	var cfg Config
	k := koanf.New(".")

	// Load YAML configuration
	err := k.Load(fileProvider, yaml.Parser())
	if err != nil {
		slog.Debug("couldn't read config file", "error", err)
	}

	// Load Environment Variables and override YAML settings
	err = k.Load(env.Provider(".", env.Opt{
		Prefix: envPrefix,
		TransformFunc: func(key, value string) (string, any) {
			// Transform {PREFIX}_PAPERLESSAPI -> paperlessapi
			// Transform {PREFIX}_FILE_TYPES -> filetypes (remove underscores)
			key = strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(key, envPrefix), "_", ""))

			// Handle space-separated values for slices (e.g., FileTypes, AdditionalTags)
			if strings.Contains(value, " ") {
				return key, strings.Split(value, " ")
			}

			return key, value
		},
	}), nil)
	if err != nil {
		slog.Debug("error loading environment variables", "error", err)
	}

	// Unmarshal into struct
	err = k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf"})
	if err != nil {
		return Config{}, fmt.Errorf("configuration error: %w", err)
	}

	// Validate Config
	err = cfg.Validate()
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// GetConfig loads configuration using the singleton pattern with sync.Once.
// It uses the default config.yaml file and E2P_ environment variable prefix.
// The configuration is loaded only once and cached for the lifetime of the application.
// For better testability and dependency injection, prefer using LoadConfig directly.
func GetConfig() (Config, error) {
	once.Do(func() {
		globalConfig, initErr = LoadConfig(file.Provider("config.yaml"), "E2P_")
	})
	return globalConfig, initErr
}
