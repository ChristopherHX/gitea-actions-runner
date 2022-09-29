package runtime

import (
	"context"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	log "github.com/sirupsen/logrus"
)

// Runner runs the pipeline.
type Runner struct {
	Machine string
	Environ map[string]string
	Client  client.Client
}

// Run runs the pipeline stage.
func (s *Runner) Run(ctx context.Context, task *runnerv1.Task) error {
	l := log.
		WithField("task.id", task.Id)
	l.Info("start running pipeline")

	return NewTask(task.Id, s.Client).Run(ctx, task)
}
