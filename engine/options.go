package engine

import "github.com/docker/docker/client"

// An Option configures a mutex.
type Option interface {
	Apply(*Docker)
}

// OptionFunc is a function that configure a value.
type OptionFunc func(*Docker)

// Apply calls f(option)
func (f OptionFunc) Apply(docker *Docker) {
	f(docker)
}

// WithClient set custom client
func WithClient(c client.APIClient) Option {
	return OptionFunc(func(q *Docker) {
		q.client = c
	})
}

// WithHidePull hide pull event.
func WithHidePull(v bool) Option {
	return OptionFunc(func(q *Docker) {
		q.hidePull = v
	})
}
