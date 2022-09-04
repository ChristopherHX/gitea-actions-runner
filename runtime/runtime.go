package runtime

import (
	"context"
	"errors"
	"fmt"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

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
func (s *Runner) Run(ctx context.Context, stage *runnerv1.Stage) error {
	l := log.
		WithField("stage.id", stage.Id).
		WithField("stage.name", stage.Name)

	l.Info("start running pipeline")

	// update machine in stage
	stage.Machine = s.Machine
	data, err := s.Client.Detail(ctx, &runnerv1.DetailRequest{
		Stage: stage,
	})
	if err != nil && err == ErrDataLock {
		l.Info("stage accepted by another runner")
		return nil
	}
	if err != nil {
		l.WithError(err).Error("cannot accept stage")
		return err
	}

	l = log.WithField("repo.id", data.Repo.Id).
		WithField("repo.name", data.Repo.Name).
		WithField("build.id", data.Build.Id).
		WithField("build.name", data.Build.Name)

	l.Info("stage details fetched")

	return s.run(ctx, data)
}

func (s *Runner) run(ctx context.Context, data *runnerv1.DetailResponse) error {
	_, exist := globalTaskMap.Load(data.Build.Id)
	if exist {
		return fmt.Errorf("task %d already exists", data.Build.Id)
	}

	task := NewTask(data.Build.Id, s.Client)

	// set task ve to global map
	// when task is done or canceled, it will be removed from the map
	globalTaskMap.Store(data.Build.Id, task)

	go task.Run(ctx)

	return nil
}
