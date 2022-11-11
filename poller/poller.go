package poller

import (
	"context"
	"errors"
	"time"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
)

var ErrDataLock = errors.New("Data Lock Error")

func New(cli client.Client, dispatch func(context.Context, *runnerv1.Task) error) *Poller {
	return &Poller{
		Client:       cli,
		Dispatch:     dispatch,
		routineGroup: newRoutineGroup(),
		metric:       &metric{},
	}
}

type Poller struct {
	Client   client.Client
	Filter   *client.Filter
	Dispatch func(context.Context, *runnerv1.Task) error

	routineGroup *routineGroup
	metric       *metric
}

func (p *Poller) Wait() {
	p.routineGroup.Wait()
}

func (p *Poller) Poll(ctx context.Context, n int) error {
	for i := 0; i < n; i++ {
		func(i int) {
			p.routineGroup.Run(func() {
				for {
					select {
					case <-ctx.Done():
						log.Infof("stopped the runner: %d", i+1)
						return
					default:
						if ctx.Err() != nil {
							log.Infof("stopping the runner: %d", i+1)
							return
						}
						if err := p.poll(ctx, i+1); err != nil {
							log.WithField("thread", i+1).
								WithError(err).Error("poll error")
							select {
							case <-ctx.Done():
								return
							case <-time.After(5 * time.Second):
							}
						}
					}
				}
			})
		}(i)
	}
	p.routineGroup.Wait()
	return nil
}

func (p *Poller) poll(ctx context.Context, thread int) error {
	l := log.WithField("thread", thread)
	l.Info("poller: request stage from remote server")

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// request a new build stage for execution from the central
	// build server.
	resp, err := p.Client.FetchTask(reqCtx, connect.NewRequest(&runnerv1.FetchTaskRequest{}))
	if err == context.Canceled || err == context.DeadlineExceeded {
		l.WithError(err).Trace("poller: no stage returned")
		return nil
	}

	if err != nil && err == ErrDataLock {
		l.WithError(err).Info("task accepted by another runner")
		return nil
	}

	if err != nil {
		l.WithError(err).Error("cannot accept task")
		return err
	}

	// exit if a nil or empty stage is returned from the system
	// and allow the runner to retry.
	if resp.Msg.Task == nil || resp.Msg.Task.Id == 0 {
		return nil
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	// update runner status
	// running: idle -> active
	// stopped: active -> idle
	if val := p.metric.IncBusyWorker(); val == 1 {
		if _, err := p.Client.UpdateRunner(
			ctx,
			connect.NewRequest(&runnerv1.UpdateRunnerRequest{
				Status: runnerv1.RunnerStatus_RUNNER_STATUS_ACTIVE,
			}),
		); err != nil {
			return err
		}
		l.Info("update runner status to active")
	}

	defer func() {
		if val := p.metric.DecBusyWorker(); val != 0 {
			return
		}

		defer func() {
			if _, err := p.Client.UpdateRunner(
				ctx,
				connect.NewRequest(&runnerv1.UpdateRunnerRequest{
					Status: runnerv1.RunnerStatus_RUNNER_STATUS_IDLE,
				}),
			); err != nil {
				log.Errorln("update status error:", err.Error())
			}
			l.Info("update runner status to idle")
		}()
	}()

	return p.Dispatch(runCtx, resp.Msg.Task)
}
