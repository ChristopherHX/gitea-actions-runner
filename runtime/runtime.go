package runtime

import (
	"context"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
)

// Runner runs the pipeline.
type Runner struct {
	Machine       string
	ForgeInstance string
	Environ       map[string]string
	Client        client.Client
}

// Run runs the pipeline stage.
func (s *Runner) Run(ctx context.Context, task *runnerv1.Task) error {
	return NewTask(s.ForgeInstance, task.Id, s.Client, s.Environ).Run(ctx, task)
}
