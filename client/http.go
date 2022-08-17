package client

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	pingv1 "gitea.com/gitea/proto-go/ping/v1"
	"gitea.com/gitea/proto-go/ping/v1/pingv1connect"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"

	"github.com/bufbuild/connect-go"
	"golang.org/x/net/http2"
)

// New returns a new runner client.
func New(endpoint, secret string, skipverify bool, opts ...Option) *HTTPClient {
	client := &HTTPClient{
		Endpoint:   endpoint,
		Secret:     secret,
		SkipVerify: skipverify,
	}

	// Loop through each option
	for _, opt := range opts {
		// Call the option giving the instantiated
		opt.Apply(client)
	}

	client.Client = &http.Client{
		Timeout: 5 * time.Second,
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

	if skipverify {
		client.Client = &http.Client{
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
	return client
}

// An HTTPClient manages communication with the runner API.
type HTTPClient struct {
	Client     *http.Client
	Endpoint   string
	Secret     string
	SkipVerify bool

	opts []connect.ClientOption
}

// Ping sends a ping message to the server to test connectivity.
func (p *HTTPClient) Ping(ctx context.Context, machine string) error {
	client := pingv1connect.NewPingServiceClient(
		p.Client,
		p.Endpoint,
		p.opts...,
	)
	req := connect.NewRequest(&pingv1.PingRequest{
		Data: machine,
	})

	req.Header().Set("X-Gitea-Token", p.Secret)

	_, err := client.Ping(ctx, req)
	return err
}

// Ping sends a ping message to the server to test connectivity.
func (p *HTTPClient) Request(ctx context.Context) (*runnerv1.Stage, error) {
	client := runnerv1connect.NewRunnerServiceClient(
		p.Client,
		p.Endpoint,
		p.opts...,
	)
	req := connect.NewRequest(&runnerv1.ConnectRequest{})

	req.Header().Set("X-Gitea-Token", p.Secret)

	res, err := client.Connect(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Msg.Stage, err
}
