package runtime

import (
	"context"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
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
	l := log.
		WithField("task.id", task.Id)
	l.Info("start running pipeline")

	// update runner status
	// running: idle -> active
	// stopped: active -> idle
	if _, err := s.Client.UpdateRunner(
		ctx,
		connect.NewRequest(&runnerv1.UpdateRunnerRequest{
			Status: runnerv1.RunnerStatus_RUNNER_STATUS_ACTIVE,
		}),
	); err != nil {
		return err
	}

	l.Info("update runner status to active")
	defer func() {
		if _, err := s.Client.UpdateRunner(
			ctx,
			connect.NewRequest(&runnerv1.UpdateRunnerRequest{
				Status: runnerv1.RunnerStatus_RUNNER_STATUS_IDLE,
			}),
		); err != nil {
			log.Errorln("update status error:", err.Error())
		}
		l.Info("update runner status to idle")
	}()

	return NewTask(s.ForgeInstance, task.Id, s.Client, s.Environ).Run(ctx, task)
}
