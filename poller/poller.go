package poller

import (
	"context"
	"errors"
	"sync"
	"time"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
)

var ErrDataLock = errors.New("Data Lock Error")

func New(cli client.Client, dispatch func(context.Context, *runnerv1.Task) error, workerNum int) *Poller {
	return &Poller{
		Client:       cli,
		Dispatch:     dispatch,
		routineGroup: newRoutineGroup(),
		metric:       &metric{},
		workerNum:    workerNum,
	}
}

type Poller struct {
	Client   client.Client
	Filter   *client.Filter
	Dispatch func(context.Context, *runnerv1.Task) error

	sync.Mutex
	routineGroup *routineGroup
	metric       *metric
	ready        chan struct{}
	workerNum    int
}

func (p *Poller) Wait() {
	p.routineGroup.Wait()
}

func (p *Poller) schedule() {
	p.Lock()
	defer p.Unlock()
	if int(p.metric.BusyWorkers()) >= p.workerNum {
		return
	}

	select {
	case p.ready <- struct{}{}:
	default:
	}
}

func (p *Poller) Poll(ctx context.Context) error {
	l := log.WithField("func", "Poll")

	for {
		// check worker number
		p.schedule()

		select {
		// wait worker ready
		case <-p.ready:
		case <-ctx.Done():
			return nil
		}
	LOOP:
		for {
			select {
			case <-ctx.Done():
				break LOOP
			default:
				task, err := p.pollTask(ctx)
				if task == nil || err != nil {
					if err != nil {
						l.Errorf("can't find the task: %v", err.Error())
					}
					time.Sleep(5 * time.Second)
					break
				}

				// update runner status
				// running: idle -> active
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
				p.routineGroup.Run(func() {
					if err := p.dispatchTask(ctx, task); err != nil {
						l.Errorf("execute task: %v", err.Error())
					}
				})
				break LOOP
			}
		}
	}
}

func (p *Poller) pollTask(ctx context.Context) (*runnerv1.Task, error) {
	l := log.WithField("func", "pollTask")
	l.Info("poller: request stage from remote server")

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// request a new build stage for execution from the central
	// build server.
	resp, err := p.Client.FetchTask(reqCtx, connect.NewRequest(&runnerv1.FetchTaskRequest{}))
	if err == context.Canceled || err == context.DeadlineExceeded {
		l.WithError(err).Trace("poller: no stage returned")
		return nil, nil
	}

	if err != nil && err == ErrDataLock {
		l.WithError(err).Info("task accepted by another runner")
		return nil, nil
	}

	if err != nil {
		l.WithError(err).Error("cannot accept task")
		return nil, err
	}

	// exit if a nil or empty stage is returned from the system
	// and allow the runner to retry.
	if resp.Msg.Task == nil || resp.Msg.Task.Id == 0 {
		return nil, nil
	}

	return resp.Msg.Task, nil
}

func (p *Poller) dispatchTask(ctx context.Context, task *runnerv1.Task) error {
	l := log.WithField("func", "dispatchTask")
	defer func() {
		val := p.metric.DecBusyWorker()
		e := recover()
		if e != nil {
			l.Errorf("panic error: %v", e)
		}
		p.schedule()

		if val != 0 {
			return
		}
		if _, err := p.Client.UpdateRunner(
			ctx,
			connect.NewRequest(&runnerv1.UpdateRunnerRequest{
				Status: runnerv1.RunnerStatus_RUNNER_STATUS_IDLE,
			}),
		); err != nil {
			l.Errorln("update status error:", err.Error())
		}
		l.Info("update runner status to idle")
	}()

	runCtx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	return p.Dispatch(runCtx, task)
}
