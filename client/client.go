package client

import (
	"context"

	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
)

type Filter struct {
	Kind     string `json:"kind"`
	Type     string `json:"type"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Capacity int    `json:"capacity"`
}

// A Client manages communication with the runner.
type Client interface {
	// Ping sends a ping message to the server to test connectivity.
	Ping(ctx context.Context, machine string) error

	// Register for new runner.
	Register(ctx context.Context, args *runnerv1.RegisterRequest) (*runnerv1.Runner, error)
}
