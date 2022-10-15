package cmd

import (
	"context"
	"time"

	"gitea.com/gitea/act_runner/client"
	"gitea.com/gitea/act_runner/config"
	"gitea.com/gitea/act_runner/poller"
	"gitea.com/gitea/act_runner/runtime"
	pingv1 "gitea.com/gitea/proto-go/ping/v1"

	"github.com/bufbuild/connect-go"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func runRegister(ctx context.Context, task *runtime.Task) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Infoln("Starting runner daemon")

		_ = godotenv.Load(task.Input.EnvFile)
		cfg, err := config.FromEnviron()
		if err != nil {
			log.WithError(err).
				Fatalln("invalid configuration")
		}

		initLogging(cfg)

		cli := client.New(
			cfg.Client.Address,
			cfg.Client.Secret,
			client.WithSkipVerify(cfg.Client.SkipVerify),
			client.WithGRPC(cfg.Client.GRPC),
			client.WithGRPCWeb(cfg.Client.GRPCWeb),
		)

		for {
			_, err := cli.Ping(ctx, connect.NewRequest(&pingv1.PingRequest{
				Data: cfg.Runner.Name,
			}))
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
				// TODO: if ping failed, retry or exit
				time.Sleep(time.Second)
			} else {
				log.Infoln("successfully connected the remote server")
				break
			}
		}

		runner := &runtime.Runner{
			Client:  cli,
			Machine: cfg.Runner.Name,
			Environ: cfg.Runner.Environ,
		}

		poller := poller.New(
			cli,
			runner.Run,
			&client.Filter{
				OS:     cfg.Platform.OS,
				Arch:   cfg.Platform.Arch,
				Labels: cfg.Runner.Labels,
			},
		)

		// register new runner
		if err := poller.Register(ctx, cfg.Runner); err != nil {
			return err
		}

		log.Infoln("successfully registered new runner")

		return nil
	}
}
