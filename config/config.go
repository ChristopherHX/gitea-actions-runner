package config

import (
	"fmt"
	"net/url"
	"os"
	"runtime"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type (
	// Config provides the system configuration.
	Config struct {
		Debug    bool `envconfig:"GITEA_DEBUG"`
		Trace    bool `envconfig:"GITEA_TRACE"`
		Client   Client
		Runner   Runner
		Platform Platform
	}

	Client struct {
		Address    string `ignored:"true"`
		Proto      string `envconfig:"GITEA_RPC_PROTO"  default:"http"`
		Host       string `envconfig:"GITEA_RPC_HOST"`
		Secret     string `envconfig:"GITEA_RPC_SECRET"`
		SkipVerify bool   `envconfig:"GITEA_RPC_SKIP_VERIFY"`
		GRPC       bool   `envconfig:"GITEA_RPC_GRPC" default:"true"`
		GRPCWeb    bool   `envconfig:"GITEA_RPC_GRPC_WEB"`
	}

	Runner struct {
		Name     string            `envconfig:"GITEA_RUNNER_NAME"`
		URL      string            `envconfig:"GITEA_RUNNER_URL" required:"true"`
		Token    string            `envconfig:"GITEA_RUNNER_TOKEN" required:"true"`
		Capacity int               `envconfig:"GITEA_RUNNER_CAPACITY" default:"1"`
		Environ  map[string]string `envconfig:"GITEA_RUNNER_ENVIRON"`
		EnvFile  string            `envconfig:"GITEA_RUNNER_ENV_FILE"`
		Labels   []string          `envconfig:"GITEA_RUNNER_LABELS"`
	}

	Platform struct {
		OS      string `envconfig:"GITEA_PLATFORM_OS"`
		Arch    string `envconfig:"GITEA_PLATFORM_ARCH"`
		Kernel  string `envconfig:"GITEA_PLATFORM_KERNEL"`
		Variant string `envconfig:"GITEA_PLATFORM_VARIANT"`
	}
)

// FromEnviron returns the settings from the environment.
func FromEnviron() (Config, error) {
	cfg := Config{}
	if err := envconfig.Process("", &cfg); err != nil {
		return cfg, err
	}

	// check runner remote url
	u, err := url.Parse(cfg.Runner.URL)
	if err != nil {
		return cfg, err
	}

	cfg.Client.Proto = u.Scheme
	cfg.Client.Host = u.Host
	cfg.Client.Secret = cfg.Runner.Token

	// runner config
	if cfg.Runner.Environ == nil {
		cfg.Runner.Environ = map[string]string{}
	}
	if cfg.Runner.Name == "" {
		cfg.Runner.Name, _ = os.Hostname()
	}

	// platform config
	if cfg.Platform.OS == "" {
		cfg.Platform.OS = runtime.GOOS
	}
	if cfg.Platform.Arch == "" {
		cfg.Platform.Arch = runtime.GOARCH
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
