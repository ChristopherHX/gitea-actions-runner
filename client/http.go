package client

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"gitea.com/gitea/proto-go/ping/v1/pingv1connect"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"

	"golang.org/x/net/http2"
)

// New returns a new runner client.
func New(endpoint string, opts ...Option) *HTTPClient {
	cfg := &config{}

	// Loop through each option
	for _, opt := range opts {
		// Call the option giving the instantiated
		opt.apply(cfg)
	}

	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{
			Timeout: 1 * time.Minute,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLS: func(netw, addr string, cfg *tls.Config) (net.Conn, error) {
					return net.Dial(netw, addr)
				},
			},
		}
	}

	if cfg.skipVerify {
		cfg.httpClient = &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
	}

	return &HTTPClient{
		PingServiceClient: pingv1connect.NewPingServiceClient(
			cfg.httpClient,
			endpoint,
			cfg.opts...,
		),
		RunnerServiceClient: runnerv1connect.NewRunnerServiceClient(
			cfg.httpClient,
			endpoint,
			cfg.opts...,
		),
		endpoint: endpoint,
	}
}

func (c *HTTPClient) Address() string {
	return c.endpoint
}

var _ Client = (*HTTPClient)(nil)

// An HTTPClient manages communication with the runner API.
type HTTPClient struct {
	pingv1connect.PingServiceClient
	runnerv1connect.RunnerServiceClient
	endpoint string
}
