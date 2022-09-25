package client

import (
	"gitea.com/gitea/proto-go/ping/v1/pingv1connect"
	"gitea.com/gitea/proto-go/runner/v1/runnerv1connect"
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
	pingv1connect.PingServiceClient
	runnerv1connect.RunnerServiceClient
}
