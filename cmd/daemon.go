package cmd

import (
	"context"
	"os"

	"gitea.com/gitea/act_runner/client"
	"gitea.com/gitea/act_runner/config"
	"gitea.com/gitea/act_runner/engine"
	"gitea.com/gitea/act_runner/poller"
	"gitea.com/gitea/act_runner/runtime"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/bufbuild/connect-go"
	"github.com/joho/godotenv"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func runDaemon(ctx context.Context, envFile string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Infoln("Starting runner daemon")

		_ = godotenv.Load(envFile)
		cfg, err := config.FromEnviron()
		if err != nil {
			log.WithError(err).
				Fatalln("invalid configuration")
		}

		initLogging(cfg)

		// try to connect to docker daemon
		// if failed, exit with error
		if err := engine.Start(ctx); err != nil {
			log.WithError(err).Fatalln("failed to connect docker daemon engine")
		}

		var g errgroup.Group

		cli := client.New(
			cfg.Client.Address,
			client.WithSkipVerify(cfg.Client.SkipVerify),
			client.WithGRPC(cfg.Client.GRPC),
			client.WithGRPCWeb(cfg.Client.GRPCWeb),
			client.WithUUIDHeader(cfg.Runner.UUID),
			client.WithTokenHeader(cfg.Runner.Token),
		)

		runner := &runtime.Runner{
			Client:        cli,
			Machine:       cfg.Runner.Name,
			ForgeInstance: cfg.Client.Address,
			Environ:       cfg.Runner.Environ,
		}

		poller := poller.New(
			cli,
			runner.Run,
		)

		g.Go(func() error {
			log.WithField("capacity", cfg.Runner.Capacity).
				WithField("endpoint", cfg.Client.Address).
				WithField("os", cfg.Platform.OS).
				WithField("arch", cfg.Platform.Arch).
				Infoln("polling the remote server")

			// update runner status to idle
			log.Infoln("update runner status to idle")
			if _, err := cli.UpdateRunner(
				context.Background(),
				connect.NewRequest(&runnerv1.UpdateRunnerRequest{
					Status: runnerv1.RunnerStatus_RUNNER_STATUS_IDLE,
				}),
			); err != nil {
				// go on, if return err, the program will be stuck
				log.WithError(err).
					Errorln("failed to update runner")
			}

			return poller.Poll(ctx, cfg.Runner.Capacity)
		})

		g.Go(func() error {
			// wait all workflows done.
			poller.Wait()
			// received the shutdown signal
			<-ctx.Done()
			log.Infoln("update runner status to offline")
			_, err := cli.UpdateRunner(
				context.Background(),
				connect.NewRequest(&runnerv1.UpdateRunnerRequest{
					Status: runnerv1.RunnerStatus_RUNNER_STATUS_OFFLINE,
				}),
			)
			return err
		})

		err = g.Wait()
		if err != nil {
			log.WithError(err).
				Errorln("shutting down the server")
		}
		return err
	}
}

// initLogging setup the global logrus logger.
func initLogging(cfg config.Config) {
	isTerm := isatty.IsTerminal(os.Stdout.Fd())
	log.SetFormatter(&log.TextFormatter{
		DisableColors: !isTerm,
		FullTimestamp: true,
	})

	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
	}
	if cfg.Trace {
		log.SetLevel(log.TraceLevel)
	}
}
