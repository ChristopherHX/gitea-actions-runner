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

const (
	errorRetryCounterLimit  = 3
	errorRetryTimeSleepSecs = 30
)

var (
	ErrDataLock   = errors.New("Data Lock Error")
	defaultLabels = []string{"self-hosted"}
)

func New(cli client.Client, dispatch func(context.Context, *runnerv1.Task) error) *Poller {
	return &Poller{
		Client:       cli,
		Dispatch:     dispatch,
		routineGroup: newRoutineGroup(),
	}
}

type Poller struct {
	Client   client.Client
	Filter   *client.Filter
	Dispatch func(context.Context, *runnerv1.Task) error

	routineGroup      *routineGroup
	errorRetryCounter int
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
							if p.errorRetryCounter > errorRetryCounterLimit {
								log.WithField("thread", i+1).Error("poller: too many errors, sleeping for 30 seconds")
								// FIXME: it makes ctrl+c hang up
								time.Sleep(time.Second * errorRetryTimeSleepSecs)
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
		p.errorRetryCounter++
		return nil
	}

	if err != nil && err == ErrDataLock {
		l.WithError(err).Info("task accepted by another runner")
		p.errorRetryCounter++
		return nil
	}

	if err != nil {
		l.WithError(err).Error("cannot accept task")
		p.errorRetryCounter++
		return err
	}

	// exit if a nil or empty stage is returned from the system
	// and allow the runner to retry.
	if resp.Msg.Task == nil || resp.Msg.Task.Id == 0 {
		return nil
	}

	p.errorRetryCounter = 0

	runCtx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	return p.Dispatch(runCtx, resp.Msg.Task)
}
