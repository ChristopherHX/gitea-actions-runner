package runtime

import (
	"context"
	"errors"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
)

var ErrDataLock = errors.New("Data Lock Error")

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
func (s *Runner) Run(ctx context.Context, task *runnerv1.Task) error {
	l := log.
		WithField("task.id", task.Id)

	l.Info("start running pipeline")

	// update machine in stage
	task.Machine = s.Machine
	data, err := s.Client.Detail(ctx, connect.NewRequest(&runnerv1.DetailRequest{
		Task: task,
	}))
	if err != nil && err == ErrDataLock {
		l.Info("task accepted by another runner")
		return nil
	}
	if err != nil {
		l.WithError(err).Error("cannot accept task")
		return err
	}

	l.Info("task details fetched")

	return s.run(ctx, data.Msg.Task)
}

func (s *Runner) run(ctx context.Context, task *runnerv1.Task) error {
	return NewTask(task.Id, s.Client).Run(ctx, task)
}
