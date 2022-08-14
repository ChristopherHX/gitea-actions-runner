package cmd

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type (
	// Config provides the system configuration.
	Config struct {
		Debug bool `envconfig:"GITEA_DEBUG"`
		Trace bool `envconfig:"GITEA_TRACE"`

		Client struct {
			Address    string `ignored:"true"`
			Proto      string `envconfig:"GITEA_RPC_PROTO"  default:"http"`
			Host       string `envconfig:"GITEA_RPC_HOST"   required:"true"`
			Secret     string `envconfig:"GITEA_RPC_SECRET" required:"true"`
			SkipVerify bool   `envconfig:"GITEA_RPC_SKIP_VERIFY"`
		}

		Runner struct {
			Name     string            `envconfig:"GITEA_RUNNER_NAME"`
			Capacity int               `envconfig:"GITEA_RUNNER_CAPACITY" default:"2"`
			Procs    int64             `envconfig:"GITEA_RUNNER_MAX_PROCS"`
			Environ  map[string]string `envconfig:"GITEA_RUNNER_ENVIRON"`
			EnvFile  string            `envconfig:"GITEA_RUNNER_ENV_FILE"`
		}

		Platform struct {
			OS      string `envconfig:"GITEA_PLATFORM_OS"    default:"linux"`
			Arch    string `envconfig:"GITEA_PLATFORM_ARCH"  default:"amd64"`
			Kernel  string `envconfig:"GITEA_PLATFORM_KERNEL"`
			Variant string `envconfig:"GITEA_PLATFORM_VARIANT"`
		}
	}
)

// fromEnviron returns the settings from the environment.
func fromEnviron() (Config, error) {
	cfg := Config{}
	err := envconfig.Process("", &cfg)

	// runner config
	if cfg.Runner.Environ == nil {
		cfg.Runner.Environ = map[string]string{}
	}
	if cfg.Runner.Name == "" {
		cfg.Runner.Name, _ = os.Hostname()
	}

	cfg.Client.Address = fmt.Sprintf(
		"%s://%s",
		cfg.Client.Proto,
		cfg.Client.Host,
	)

	if file := cfg.Runner.EnvFile; file != "" {
		envs, err := godotenv.Read(file)
		if err != nil {
			return cfg, err
		}
		for k, v := range envs {
			cfg.Runner.Environ[k] = v
		}
	}

	return cfg, err
}
