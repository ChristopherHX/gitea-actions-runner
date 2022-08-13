package client

import (
	"context"
)

// A Client manages communication with the runner.
type Client interface {
	// Ping sends a ping message to the server to test connectivity.
	Ping(ctx context.Context, machine string) error
}
