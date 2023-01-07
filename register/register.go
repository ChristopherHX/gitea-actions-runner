package register

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"gitea.com/gitea/act_runner/client"
	"gitea.com/gitea/act_runner/config"
	"gitea.com/gitea/act_runner/core"

	"github.com/bufbuild/connect-go"
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
	}

	file, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.WithError(err).Error("poller: cannot marshal the json input")
		return data, err
	}

	// store runner config in .runner file
	return data, os.WriteFile(cfg.File, file, 0o644)
}
