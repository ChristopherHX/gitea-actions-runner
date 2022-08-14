package cmd

import (
	"context"
	"time"

	"gitea.com/gitea/act_runner/client"
	"gitea.com/gitea/act_runner/engine"
	"gitea.com/gitea/act_runner/poller"
	"gitea.com/gitea/act_runner/runtime"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func runDaemon(ctx context.Context, input *Input) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Infoln("Starting runner daemon")

		_ = godotenv.Load(input.envFile)
		cfg, err := fromEnviron()
		if err != nil {
			log.WithError(err).
				Fatalln("invalid configuration")
		}

		initLogging(cfg)

		engine, err := engine.New()
		if err != nil {
			log.WithError(err).
				Fatalln("cannot load the docker engine")
		}

		count := 0
		for {
			err := engine.Ping(ctx)
			if err == context.Canceled {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if err != nil {
				log.WithError(err).
					Errorln("cannot ping the docker daemon")
				count++
				if count == 5 {
					log.WithError(err).
						Fatalf("retry count reached: %d", count)
				}
				time.Sleep(time.Second)
			} else {
				log.Infoln("successfully pinged the docker daemon")
				break
			}
		}

		cli := client.New(
			cfg.Client.Address,
			cfg.Client.Secret,
			cfg.Client.SkipVerify,
			client.WithGRPC(cfg.Client.GRPC),
			client.WithGRPCWeb(cfg.Client.GRPCWeb),
		)

		for {
			err := cli.Ping(ctx, cfg.Runner.Name)
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			if ctx.Err() != nil {
				break
			}
			if err != nil {
				log.WithError(err).
					Errorln("cannot ping the remote server")
				time.Sleep(time.Second)
			} else {
				log.Infoln("successfully pinged the remote server")
				break
			}
		}

		var g errgroup.Group

		runner := &runtime.Runner{
			Client:  cli,
			Machine: cfg.Runner.Name,
			Environ: cfg.Runner.Environ,
		}

		poller := poller.New(cli, runner.Run)

		g.Go(func() error {
			log.WithField("capacity", cfg.Runner.Capacity).
				WithField("endpoint", cfg.Client.Address).
				WithField("os", cfg.Platform.OS).
				WithField("arch", cfg.Platform.Arch).
				Infoln("polling the remote server")

			poller.Poll(ctx, cfg.Runner.Capacity)
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
