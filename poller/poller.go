package poller

import (
	"context"
	"time"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	log "github.com/sirupsen/logrus"
)

func New(cli client.Client, dispatch func(context.Context, *runnerv1.Stage) error, filter *client.Filter) *Poller {
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
	Dispatch func(context.Context, *runnerv1.Stage) error

	routineGroup *routineGroup
}

func (p *Poller) Poll(ctx context.Context, n int) error {
	// register new runner.
	_, err := p.Client.Register(ctx, &runnerv1.RegisterRequest{
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
						if err := p.poll(ctx, i+1); err != nil {
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

func (p *Poller) poll(ctx context.Context, thread int) error {
	log.WithField("thread", thread).Info("poller: request stage from remote server")

	// TODO: fetch the job from remote server
	time.Sleep(time.Second)

	// request a new build stage for execution from the central
	// build server.
	stage, err := p.Client.Request(ctx, &runnerv1.RequestRequest{
		Kind: p.Filter.Kind,
		Os:   p.Filter.OS,
		Arch: p.Filter.Arch,
		Type: p.Filter.Type,
	})
	if err == context.Canceled || err == context.DeadlineExceeded {
		log.WithError(err).Trace("poller: no stage returned")
		return nil
	}
	if err != nil {
		log.WithError(err).Error("poller: cannot request stage")
		return err
	}

	// exit if a nil or empty stage is returned from the system
	// and allow the runner to retry.
	if stage == nil || stage.Id == 0 {
		return nil
	}

	// FIXME: for testing task
	// stage.Id = 111
	// stage.BuildId = 1222

	// set client to context, so that the stage can use it to
	ctx = client.WithClient(ctx, p.Client)

	return p.Dispatch(ctx, stage)
}
