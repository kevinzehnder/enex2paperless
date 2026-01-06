package config

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var (
	once     sync.Once
	settings Config
	initErr  error
	k        = koanf.New(".")
)

type Config struct {
	PaperlessAPI   string   `validate:"required,http_url"`
	Username       string   `validate:"required_with=Password"`
	Password       string   `validate:"required_with=Username"`
	Token          string   `validate:"required_without=Password"`
	FileTypes      []string `validate:"required"`
	OutputFolder   string
	AdditionalTags []string
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

// GetConfig initializes and returns the application configuration.
// It reads from a YAML file and overrides with environment variables if they exist.
// The function ensures that the configuration is loaded only once to maintain consistency
// throughout the application's lifecycle. If the configuration is invalid or cannot be
// loaded, an error will be returned.
func GetConfig() (Config, error) {
	once.Do(func() {
		// Load YAML configuration
		err := k.Load(file.Provider("config.yaml"), yaml.Parser())
		if err != nil {
			slog.Debug("couldn't read config.yaml", "error", err)
		}

		// Load Environment Variables and override YAML settings
		// TODO

		// Unmarshal into struct
		err = k.UnmarshalWithConf("", &settings, koanf.UnmarshalConf{Tag: "koanf"})
		if err != nil {
			initErr = fmt.Errorf("configuration error: %v", err)
			return
		}

		// Validate Config
		err = settings.Validate()
		if err != nil {
			initErr = err
			return
		}
	})
	return settings, initErr
}
