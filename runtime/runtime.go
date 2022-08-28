package runtime

import (
	"context"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/sirupsen/logrus"
)

// Defines the Resource Kind and Type.
const (
	Kind = "pipeline"
	Type = "docker"
)

// Runner runs the pipeline.
type Runner struct {
	Machine string
	Environ map[string]string
	Client  client.Client
}

// Run runs the pipeline stage.
func (s *Runner) Run(ctx context.Context, runner *runnerv1.Runner) error {
	l := logrus.
		WithField("runner.UUID", runner.Uuid).
		WithField("runner.token", runner.Token)

	l.Info("request a new task")
	// TODO: get new task

	return s.run(ctx, runner)
}

func (s *Runner) run(ctx context.Context, runner *runnerv1.Runner) error {
	l := logrus.
		WithField("runner.Uuid", runner.Uuid)

	l.Info("start running pipeline")
	// TODO: docker runner with stage data
	// task.Run is blocking, so we need to use goroutine to run it in background
	// return task metadata and status to the server
	task := NewTask()
	return task.Run(ctx)
}
