package register

import (
	"context"
	"encoding/json"
	"os"

	runnerv1 "code.gitea.io/bots-proto-go/runner/v1"
	"gitea.com/gitea/act_runner/client"
	"gitea.com/gitea/act_runner/config"
	"gitea.com/gitea/act_runner/core"

	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
)

var defaultLabels = []string{"self-hosted"}

func New(cli client.Client, filter *client.Filter) *Register {
	return &Register{
		Client: cli,
		Filter: filter,
	}
}

type Register struct {
	Client client.Client
	Filter *client.Filter
}

func (p *Register) Register(ctx context.Context, cfg config.Runner) (*core.Runner, error) {
	// register new runner.
	resp, err := p.Client.Register(ctx, connect.NewRequest(&runnerv1.RegisterRequest{
		Name:         cfg.Name,
		Token:        cfg.Token,
		AgentLabels:  append(defaultLabels, []string{p.Filter.OS, p.Filter.Arch}...),
		CustomLabels: p.Filter.Labels,
	}))
	if err != nil {
		log.WithError(err).Error("poller: cannot register new runner")
		return nil, err
	}

	data := &core.Runner{
		ID:      resp.Msg.Runner.Id,
		UUID:    resp.Msg.Runner.Uuid,
		Name:    resp.Msg.Runner.Name,
		Token:   resp.Msg.Runner.Token,
		Address: p.Client.Address(),
	}

	file, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.WithError(err).Error("poller: cannot marshal the json input")
		return data, err
	}

	// store runner config in .runner file
	return data, os.WriteFile(cfg.File, file, 0o644)
}
