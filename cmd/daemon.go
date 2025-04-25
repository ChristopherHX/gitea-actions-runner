package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/ChristopherHX/gitea-actions-runner/client"
	"github.com/ChristopherHX/gitea-actions-runner/config"
	"github.com/ChristopherHX/gitea-actions-runner/poller"
	"github.com/ChristopherHX/gitea-actions-runner/runtime"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"connectrpc.com/connect"
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
			RunnerWorker:  cfg.Runner.RunnerWorker,
		}
		flags := []string{fmt.Sprintf("--max-parallel=%d", cfg.Runner.Capacity)}

		runner.RunnerWorker = append(flags, runner.RunnerWorker...)

		resp, err := cli.Declare(cmd.Context(), &connect.Request[runnerv1.DeclareRequest]{
			Msg: &runnerv1.DeclareRequest{
				Version: cmd.Root().Version,
				Labels:  runner.Labels,
			},
		})
		if err != nil && connect.CodeOf(err) == connect.CodeUnimplemented {
			// Gitea instance is older version. skip declare step.
			log.Info("Because the Gitea instance is an old version, labels can only be set during configure.")
		} else if err != nil {
			log.WithError(err).Error("fail to invoke Declare")
			return err
		} else {
			log.Infof("runner: %s, with version: %s, with labels: %v, declare successfully",
				resp.Msg.Runner.Name, resp.Msg.Runner.Version, resp.Msg.Runner.Labels)
		}

		once, _ := cmd.Flags().GetBool("once")
		if once {
			cfg.Runner.Capacity = 1
		}
		poller := poller.New(
			cli,
			runner.Run,
			cfg.Runner.Capacity,
		)
		poller.Once = once

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
