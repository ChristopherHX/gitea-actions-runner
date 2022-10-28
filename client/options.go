package client

import (
	"context"
	"net/http"

	"gitea.com/gitea/act_runner/core"

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

// WithUUIDHeader add runner uuid in header
func WithUUIDHeader(uuid string) Option {
	return OptionFunc(func(cfg *config) {
		if uuid == "" {
			return
		}
		cfg.opts = append(
			cfg.opts,
			connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
				return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
					req.Header().Set(core.UUIDHeader, uuid)
					return next(ctx, req)
				}
			})),
		)
	})
}

// WithTokenHeader add runner token in header
func WithTokenHeader(token string) Option {
	return OptionFunc(func(cfg *config) {
		if token == "" {
			return
		}
		cfg.opts = append(
			cfg.opts,
			connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
				return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
					req.Header().Set(core.TokenHeader, token)
					return next(ctx, req)
				}
			})),
		)
	})
}
