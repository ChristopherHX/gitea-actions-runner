package config

import "github.com/kelseyhightower/envconfig"

type (
	// Config provides the system configuration.
	Config struct {
		Logging Logging
	}
)

type Logging struct {
	Debug   bool   `envconfig:"APP_LOGS_DEBUG"`
	Level   string `envconfig:"APP_LOGS_LEVEL" default:"info"`
	NoColor bool   `envconfig:"APP_LOGS_COLOR"`
	Pretty  bool   `envconfig:"APP_LOGS_PRETTY"`
	Text    bool   `envconfig:"APP_LOGS_TEXT"`
}

// Environ returns the settings from the environment.
func Environ() (Config, error) {
	cfg := Config{}
	err := envconfig.Process("", &cfg)
	return cfg, err
}
