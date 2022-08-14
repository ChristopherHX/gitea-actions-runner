package poller

import (
	"context"
	"errors"
	"time"

	v1 "gitea.com/gitea/proto/gen/proto/v1"
	"gitea.com/gitea/act_runner/client"

	log "github.com/sirupsen/logrus"
)

func New(cli client.Client, dispatch func(context.Context, *v1.Stage) error) *Poller {
	return &Poller{
		Client:       cli,
		Dispatch:     dispatch,
		routineGroup: newRoutineGroup(),
	}
}

type Poller struct {
	Client   client.Client
	Dispatch func(context.Context, *v1.Stage) error

	routineGroup *routineGroup
}

func (p *Poller) Poll(ctx context.Context, n int) {
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
}

func (p *Poller) poll(ctx context.Context, thread int) error {
	log.WithField("thread", thread).Info("poller: request stage from remote server")

	// TODO: fetch the job from remote server
	time.Sleep(time.Second)

	// request a new build stage for execution from the central
	// build server.
	stage, err := p.Client.Request(ctx)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		log.WithError(err).Trace("poller: no stage returned")
		return nil
	}
	if err != nil {
		log.WithError(err).Error("poller: cannot request stage")
		return err
	}

	if stage == nil || stage.BuildUuid == "" {
		return nil
	}

	return p.Dispatch(ctx, stage)
}
