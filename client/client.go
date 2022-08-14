package client

import (
	"context"

	v1 "gitea.com/gitea/proto/gen/proto/v1"
)

// A Client manages communication with the runner.
type Client interface {
	// Ping sends a ping message to the server to test connectivity.
	Ping(ctx context.Context, machine string) error

	// Request requests the next available build stage for execution.
	Request(ctx context.Context) (*v1.Stage, error)
}
