package cmd

import (
	"context"
	"os"

	"gitea.com/gitea/act_runner/client"
	"gitea.com/gitea/act_runner/config"
	"gitea.com/gitea/act_runner/poller"
	"gitea.com/gitea/act_runner/runtime"

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

		var g errgroup.Group

		cli := client.New(
			cfg.Client.Address,
			cfg.Runner.UUID,
			cfg.Runner.Token,
		)

		runner := &runtime.Runner{
			Client:        cli,
			Machine:       cfg.Runner.Name,
			ForgeInstance: cfg.Client.Address,
			Environ:       cfg.Runner.Environ,
			Labels:        cfg.Runner.Labels,
		}

		poller := poller.New(
			cli,
			runner.Run,
			cfg.Runner.Capacity,
		)

		g.Go(func() error {
			l := log.WithField("capacity", cfg.Runner.Capacity).
				WithField("endpoint", cfg.Client.Address).
				WithField("os", cfg.Platform.OS).
				WithField("arch", cfg.Platform.Arch)
			l.Infoln("polling the remote server")

			if err := poller.Poll(ctx); err != nil {
				l.Errorf("poller error: %v", err)
			}
			poller.Wait()
			return nil
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
