package cmd

import "github.com/kelseyhightower/envconfig"

type (
	// Config provides the system configuration.
	Config struct {
		Debug bool `envconfig:"GITEA_DEBUG"`
		Trace bool `envconfig:"GITEA_TRACE"`
	}
)

// fromEnviron returns the settings from the environment.
func fromEnviron() (Config, error) {
	cfg := Config{}
	err := envconfig.Process("", &cfg)
	return cfg, err
}
