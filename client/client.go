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

	// Request requests the next available build stage for execution.
	Request(ctx context.Context, args *runnerv1.RequestRequest) (*runnerv1.Stage, error)

	// Update updates the build stage.
	Update(ctxt context.Context, args *runnerv1.UpdateRequest) error

	// UpdateStep updates the build step.
	UpdateStep(ctx context.Context, args *runnerv1.UpdateStepRequest) error
}

type contextKey string

const clientContextKey = contextKey("gitea.rpc.client")

// FromContext returns the client from the context.
func FromContext(ctx context.Context) Client {
	val := ctx.Value(clientContextKey)
	if val != nil {
		if c, ok := val.(Client); ok {
			return c
		}
	}
	return nil
}

// WithClient returns a new context with the given client.
func WithClient(ctx context.Context, c Client) context.Context {
	return context.WithValue(ctx, clientContextKey, c)
}
