package client

import (
	"net/http"

	"github.com/bufbuild/connect-go"
)

type config struct {
	httpClient *http.Client
	skipVerify bool
	opts       []connect.ClientOption
}

// An Option configures a mutex.
type Option interface {
	apply(*config)
}

// OptionFunc is a function that configure a value.
type OptionFunc func(*config)

// Apply calls f(option)
func (f OptionFunc) apply(cfg *config) {
	f(cfg)
}

func WithSkipVerify(c bool) Option {
	return OptionFunc(func(cfg *config) {
		cfg.skipVerify = c
	})
}

func WithClientOptions(opts ...connect.ClientOption) Option {
	return OptionFunc(func(cfg *config) {
		cfg.opts = append(cfg.opts, opts...)
	})
}

// WithGRPC configures clients to use the HTTP/2 gRPC protocol.
func WithGRPC(c bool) Option {
	return OptionFunc(func(cfg *config) {
		if !c {
			return
		}
		cfg.opts = append(cfg.opts, connect.WithGRPC())
	})
}

// WithGRPCWeb configures clients to use the gRPC-Web protocol.
func WithGRPCWeb(c bool) Option {
	return OptionFunc(func(cfg *config) {
		if !c {
			return
		}
		cfg.opts = append(cfg.opts, connect.WithGRPCWeb())
	})
}
