package engine

import (
	"context"

	"github.com/docker/docker/client"
)

// Opts configures the Docker engine.
type Opts struct {
	HidePull bool
}

type Docker struct {
	client   client.APIClient
	hidePull bool
}

func New(client client.APIClient, opts Opts) *Docker {
	return &Docker{
		client:   client,
		hidePull: opts.HidePull,
	}
}

// NewEnv returns a new Engine from the environment.
func NewEnv(opts Opts) (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return New(cli, opts), nil
}

// Ping pings the Docker daemon.
func (e *Docker) Ping(ctx context.Context) error {
	_, err := e.client.Ping(ctx)
	return err
}
