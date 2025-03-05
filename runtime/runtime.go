package runtime

import (
	"context"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/ChristopherHX/gitea-actions-runner/client"
)

// Runner runs the pipeline.
type Runner struct {
	Machine       string
	ForgeInstance string
	Environ       map[string]string
	Client        client.Client
	Labels        []string
	RunnerWorker  []string
}

// Run runs the pipeline stage.
func (s *Runner) Run(ctx context.Context, task *runnerv1.Task) error {
	return NewTask(s.ForgeInstance, task.Id, s.Client, s.Environ, s.platformPicker).Run(ctx, task, s.RunnerWorker)
}

func (s *Runner) platformPicker(labels []string) string {
	return ""
}
