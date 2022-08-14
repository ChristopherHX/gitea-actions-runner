package client

import "github.com/bufbuild/connect-go"

// An Option configures a mutex.
type Option interface {
	Apply(*HTTPClient)
}

// OptionFunc is a function that configure a value.
type OptionFunc func(*HTTPClient)

// Apply calls f(option)
func (f OptionFunc) Apply(cli *HTTPClient) {
	f(cli)
}

// WithGRPC configures clients to use the HTTP/2 gRPC protocol.
func WithGRPC(c bool) Option {
	return OptionFunc(func(cli *HTTPClient) {
		if !c {
			return
		}
		cli.opts = append(cli.opts, connect.WithGRPC())
	})
}

// WithGRPCWeb configures clients to use the gRPC-Web protocol.
func WithGRPCWeb(c bool) Option {
	return OptionFunc(func(cli *HTTPClient) {
		if !c {
			return
		}
		cli.opts = append(cli.opts, connect.WithGRPCWeb())
	})
}
