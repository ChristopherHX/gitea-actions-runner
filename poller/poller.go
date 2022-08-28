package poller

import (
	"context"
	"time"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	log "github.com/sirupsen/logrus"
)

func New(cli client.Client, dispatch func(context.Context, *runnerv1.Runner) error, filter *client.Filter) *Poller {
	return &Poller{
		Client:       cli,
		Filter:       filter,
		Dispatch:     dispatch,
		routineGroup: newRoutineGroup(),
	}
}

type Poller struct {
	Client   client.Client
	Filter   *client.Filter
	Dispatch func(context.Context, *runnerv1.Runner) error

	routineGroup *routineGroup
}

func (p *Poller) Poll(ctx context.Context, n int) error {
	// register new runner.
	runner, err := p.Client.Register(ctx, &runnerv1.RegisterRequest{
		Os:       p.Filter.OS,
		Arch:     p.Filter.Arch,
		Capacity: int64(p.Filter.Capacity),
	})
	if err != nil {
		log.WithError(err).Error("poller: cannot register new runner")
		return err
	}

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
						if err := p.poll(ctx, runner, i+1); err != nil {
							log.WithError(err).Error("poll error")
						}
					}
				}
			})
		}(i)
	}
	p.routineGroup.Wait()
	return nil
}

func (p *Poller) poll(ctx context.Context, runner *runnerv1.Runner, thread int) error {
	log.WithField("thread", thread).Info("poller: request stage from remote server")

	// TODO: fetch the job from remote server
	time.Sleep(time.Second)

	return p.Dispatch(ctx, runner)
}
