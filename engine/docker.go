package engine

import (
	"context"

	"github.com/docker/docker/client"
)

type Docker struct {
	client   client.APIClient
	hidePull bool
}

func New(opts ...Option) (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	srv := &Docker{
		client: cli,
	}

	// Loop through each option
	for _, opt := range opts {
		// Call the option giving the instantiated
		opt.Apply(srv)
	}

	return srv, nil
}

// Ping pings the Docker daemon.
func (e *Docker) Ping(ctx context.Context) error {
	_, err := e.client.Ping(ctx)
	return err
}
