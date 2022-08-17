package runtime

import (
	"context"

	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
	"gitea.com/gitea/act_runner/client"

	"github.com/sirupsen/logrus"
)

// Runner runs the pipeline.
type Runner struct {
	Machine string
	Environ map[string]string
	Client  client.Client
}

// Run runs the pipeline stage.
func (s *Runner) Run(ctx context.Context, stage *runnerv1.Stage) error {
	l := logrus.
		WithField("stage.build_uuid", stage.BuildUuid).
		WithField("stage.runner_uuid", stage.RunnerUuid)

	l.Info("stage received")
	// TODO: Update stage structure

	return s.run(ctx, stage)
}

func (s *Runner) run(ctx context.Context, stage *runnerv1.Stage) error {
	l := logrus.
		WithField("stage.build_uuid", stage.BuildUuid).
		WithField("stage.runner_uuid", stage.RunnerUuid)

	l.Info("start running pipeline")
	// TODO: docker runner with stage data

	return nil
}
