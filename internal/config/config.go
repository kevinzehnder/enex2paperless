package config

import (
	"fmt"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/rs/zerolog/log"
)

var (
	once     sync.Once
	settings Config
	initErr  error
	k        = koanf.New(".")
)

type Config struct {
	PaperlessAPI string `validate:"required,http_url"`
	Username     string `validate:"required"`
	Password     string `validate:"required"`
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
			log.Debug().Msg(fmt.Sprintf("couldn't read config.yaml: %s", err))
		}

		// Load Environment Variables and override YAML settings
		err = k.Load(env.Provider("", ".", func(s string) string {
			return s
		}), nil)
		if err != nil {
			initErr = fmt.Errorf("configuration error: %v", err)
		}

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
			initErr = fmt.Errorf("configuration error: %v", err)
			return
		}
	})
	return settings, initErr
}
