package register

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/ChristopherHX/gitea-actions-runner/client"
	"github.com/ChristopherHX/gitea-actions-runner/config"
	"github.com/ChristopherHX/gitea-actions-runner/core"

	"connectrpc.com/connect"
	log "github.com/sirupsen/logrus"
)

func New(cli client.Client) *Register {
	return &Register{
		Client: cli,
	}
}

type Register struct {
	Client client.Client
}

func (p *Register) Register(ctx context.Context, cfg config.Runner) (*core.Runner, error) {
	labels := make([]string, len(cfg.Labels))
	for i, v := range cfg.Labels {
		labels[i] = strings.SplitN(v, ":", 2)[0]
	}
	// register new runner.
	resp, err := p.Client.Register(ctx, connect.NewRequest(&runnerv1.RegisterRequest{
		Name:        cfg.Name,
		Token:       cfg.Token,
		AgentLabels: labels,
		Ephemeral:   cfg.Ephemeral,
	}))
	if err != nil {
		log.WithError(err).Error("poller: cannot register new runner")
		return nil, err
	}

	data := &core.Runner{
		ID:           resp.Msg.Runner.Id,
		UUID:         resp.Msg.Runner.Uuid,
		Name:         resp.Msg.Runner.Name,
		Token:        resp.Msg.Runner.Token,
		Address:      p.Client.Address(),
		RunnerWorker: cfg.RunnerWorker,
		Labels:       cfg.Labels,
		Ephemeral:    resp.Msg.Runner.Ephemeral,
	}

	file, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.WithError(err).Error("poller: cannot marshal the json input")
		return data, err
	}

	if cfg.Ephemeral != resp.Msg.Runner.Ephemeral {
		// TODO we cannot remove the configuration via runner api, if we return an error here we just fill the database
		log.Error("poller: cannot register new runner as ephemeral upgrade Gitea to gain security, run-once will be used automatically")
	}

	// store runner config in .runner file
	return data, os.WriteFile(cfg.File, file, 0o644)
}
