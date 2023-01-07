package config

import (
	"encoding/json"
	"io"
	"os"
	"runtime"

	"gitea.com/gitea/act_runner/core"

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
		Address string `ignored:"true"`
	}

	Runner struct {
		UUID         string            `ignored:"true"`
		Name         string            `envconfig:"GITEA_RUNNER_NAME"`
		Token        string            `ignored:"true"`
		RunnerWorker []string          `envconfig:"GITEA_RUNNER_WORKER"`
		Capacity     int               `envconfig:"GITEA_RUNNER_CAPACITY" default:"1"`
		File         string            `envconfig:"GITEA_RUNNER_FILE" default:".runner"`
		Environ      map[string]string `envconfig:"GITEA_RUNNER_ENVIRON"`
		EnvFile      string            `envconfig:"GITEA_RUNNER_ENV_FILE"`
		Labels       []string          `envconfig:"GITEA_RUNNER_LABELS"`
	}

	Platform struct {
		OS   string `envconfig:"GITEA_PLATFORM_OS"`
		Arch string `envconfig:"GITEA_PLATFORM_ARCH"`
	}
)

// FromEnviron returns the settings from the environment.
func FromEnviron() (Config, error) {
	cfg := Config{}
	if err := envconfig.Process("", &cfg); err != nil {
		return cfg, err
	}

	// check runner config exist
	if f, err := os.Stat(cfg.Runner.File); err == nil && !f.IsDir() {
		jsonFile, _ := os.Open(cfg.Runner.File)
		defer jsonFile.Close()
		byteValue, _ := io.ReadAll(jsonFile)
		var runner core.Runner
		if err := json.Unmarshal(byteValue, &runner); err != nil {
			return cfg, err
		}
		if runner.UUID != "" {
			cfg.Runner.UUID = runner.UUID
		}
		if runner.Token != "" {
			cfg.Runner.Token = runner.Token
		}
		if len(runner.RunnerWorker) != 0 {
			cfg.Runner.RunnerWorker = runner.RunnerWorker
		}
		if len(runner.Labels) != 0 {
			cfg.Runner.Labels = runner.Labels
		}
		if runner.Address != "" {
			cfg.Client.Address = runner.Address
		}
	}

	// runner config
	if cfg.Runner.Environ == nil {
		cfg.Runner.Environ = map[string]string{
			"GITHUB_API_URL":    cfg.Client.Address + "/api/v1",
			"GITHUB_SERVER_URL": cfg.Client.Address,
		}
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

	if file := cfg.Runner.EnvFile; file != "" {
		envs, err := godotenv.Read(file)
		if err != nil {
			return cfg, err
		}
		for k, v := range envs {
			cfg.Runner.Environ[k] = v
		}
	}

	return cfg, nil
}
