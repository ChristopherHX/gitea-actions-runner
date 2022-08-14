package poller

import (
	"context"
	"time"

	"gitea.com/gitea/act_runner/client"

	log "github.com/sirupsen/logrus"
)

func New(cli client.Client) *Poller {
	return &Poller{
		Client:       cli,
		routineGroup: newRoutineGroup(),
	}
}

type Poller struct {
	Client client.Client

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

	return nil
}
