package poller

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"gitea.com/gitea/act_runner/client"

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
		ready:        make(chan struct{}, 1),
	}
}

type Poller struct {
	Client   client.Client
	Dispatch func(context.Context, *runnerv1.Task) error

	sync.Mutex
	routineGroup *routineGroup
	metric       *metric
	ready        chan struct{}
	workerNum    int
	tasksVersion atomic.Int64 // tasksVersion used to store the version of the last task fetched from the Gitea.
	Once         bool
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

func (p *Poller) Wait() {
	p.routineGroup.Wait()
}

func (p *Poller) Poll(rootctx context.Context) error {
	ctx, cancel := context.WithCancel(rootctx)
	defer cancel()

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

				p.metric.IncBusyWorker()
				p.routineGroup.Run(func() {
					defer p.schedule()
					defer p.metric.DecBusyWorker()
					if p.Once {
						defer cancel()
					}
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

	// Load the version value that was in the cache when the request was sent.
	v := p.tasksVersion.Load()
	resp, err := p.Client.FetchTask(reqCtx, connect.NewRequest(&runnerv1.FetchTaskRequest{
		TasksVersion: v,
	}))
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
	if resp == nil || resp.Msg == nil {
		return nil, nil
	}

	if resp.Msg.TasksVersion > v {
		p.tasksVersion.CompareAndSwap(v, resp.Msg.TasksVersion)
	}

	if resp.Msg.Task == nil || resp.Msg.Task.Id == 0 {
		return nil, nil
	}

	// got a task, set `tasksVersion` to zero to force query db in next request.
	p.tasksVersion.CompareAndSwap(resp.Msg.TasksVersion, 0)

	return resp.Msg.Task, nil
}

func (p *Poller) dispatchTask(ctx context.Context, task *runnerv1.Task) error {
	l := log.WithField("func", "dispatchTask")
	defer func() {
		e := recover()
		if e != nil {
			l.Errorf("panic error: %v", e)
		}
	}()

	runCtx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	return p.Dispatch(runCtx, task)
}
