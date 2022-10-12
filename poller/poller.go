package poller

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"gitea.com/gitea/act_runner/client"
	"gitea.com/gitea/act_runner/config"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/appleboy/com/file"
	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
)

const errorRetryCounterLimit = 3
const errorRetryTimeSleepSecs = 30

var (
	ErrDataLock   = errors.New("Data Lock Error")
	defaultLabels = []string{"self-hosted"}
)

func New(cli client.Client, dispatch func(context.Context, *runnerv1.Task) error, filter *client.Filter) *Poller {
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
	Dispatch func(context.Context, *runnerv1.Task) error

	routineGroup      *routineGroup
	errorRetryCounter int
}

type runner struct {
	id    int64
	uuid  string
	name  string
	token string
}

func (p *Poller) Register(ctx context.Context, cfg config.Runner) error {
	// check .runner config exist
	if file.IsFile(".runner") {
		return nil
	}

	// register new runner.
	resp, err := p.Client.Register(ctx, connect.NewRequest(&runnerv1.RegisterRequest{
		Name:         cfg.Name,
		Token:        cfg.Token,
		Url:          cfg.URL,
		AgentLabels:  append(defaultLabels, []string{p.Filter.OS, p.Filter.Arch}...),
		CustomLabels: p.Filter.Labels,
	}))
	if err != nil {
		log.WithError(err).Error("poller: cannot register new runner")
		return err
	}

	data := &runner{
		id:    resp.Msg.Runner.Id,
		uuid:  resp.Msg.Runner.Uuid,
		name:  resp.Msg.Runner.Name,
		token: resp.Msg.Runner.Token,
	}

	file, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		log.WithError(err).Error("poller: cannot marshal the json input")
		return err
	}

	// store runner config in .runner file
	return os.WriteFile(".runner", file, 0o644)
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

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// request a new build stage for execution from the central
	// build server.
	resp, err := p.Client.FetchTask(ctx, connect.NewRequest(&runnerv1.FetchTaskRequest{
		Os:   p.Filter.OS,
		Arch: p.Filter.Arch,
	}))
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

	return p.Dispatch(ctx, resp.Msg.Task)
}
