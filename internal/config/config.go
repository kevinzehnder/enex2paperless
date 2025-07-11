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

		// // Load Environment Variables and override YAML settings
		// err = k.Load(env.Provider("E2P_", ".", func(s string) string {
		// 	// Remove prefix, convert to lowercase, replace underscores with dots
		// 	s = strings.TrimPrefix(s, "E2P_")
		// 	s = strings.ToLower(s)
		// 	s = strings.ReplaceAll(s, "_", ".")
		// 	return s
		// }), nil)
		// if err != nil {
		// 	initErr = fmt.Errorf("configuration error: %v", err)
		// 	return
		// }
		//
		// slog.Debug("Configuration loaded", "config", k.All())

		// Unmarshal into struct
		err = k.UnmarshalWithConf("", &settings, koanf.UnmarshalConf{Tag: "koanf"})
		if err != nil {
			initErr = fmt.Errorf("configuration error: %v", err)
			return
		}

		// Validate Config
		validate := validator.New()

		err = validate.Struct(settings)
		if err != nil {

			var validateErrs validator.ValidationErrors
			if errors.As(err, &validateErrs) {
				for _, e := range validateErrs {
					switch e.StructField() {
					case "Token":
						initErr = fmt.Errorf("bad auth config: need either token or username/password")
					case "Username":
						initErr = fmt.Errorf("if using password, username is required too")
					case "Password":
						initErr = fmt.Errorf("if using username, password is required too")
					default:
						initErr = fmt.Errorf("field %s: %s validation failed", e.Field(), e.Tag())
					}
					return
				}
			}
			initErr = fmt.Errorf("configuration error: %v", err)
			return
		}
	})
	return settings, initErr
}

func SetOutputFolder(path string) error {
	settings.OutputFolder = path
	return nil
}

func SetAdditionalTags(tags []string) error {
	settings.AdditionalTags = tags
	return nil
}
