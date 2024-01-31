package config

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

var (
	once     sync.Once
	settings Config
	initErr  error
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

		vp := viper.New()

		// Load YAML
		vp.SetConfigName("config")
		vp.SetConfigType("yaml")
		vp.AddConfigPath(".")
		vp.AddConfigPath("../../.")
		if err := vp.ReadInConfig(); err != nil {
			slog.Debug("couldn't read config.yaml", "error", err)
		}

		// Load ENV variables
		// vp.SetEnvPrefix("MYAPP")
		vp.AutomaticEnv()

		// Unmarshal into struct
		err := vp.Unmarshal(&settings)
		if err != nil {
			initErr = fmt.Errorf("configuration error: %v", err)
			return
		}

		// Validate Config
		validate := validator.New()
		if err := validate.Struct(settings); err != nil {
			initErr = fmt.Errorf("configuration error: %v", err)
			return
		}
	})

	return settings, initErr
}
