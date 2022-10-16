package cmd

import (
	"context"
	"os"
	"strconv"

	"gitea.com/gitea/act_runner/config"
	"gitea.com/gitea/act_runner/engine"
	"gitea.com/gitea/act_runner/runtime"

	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const version = "0.1"

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

func Execute(ctx context.Context) {
	task := runtime.NewTask("gitea", 0, nil)

	// ./act_runner
	rootCmd := &cobra.Command{
		Use:          "act [event name to run]\nIf no event name passed, will default to \"on: push\"",
		Short:        "Run GitHub actions locally by specifying the event name (e.g. `push`) or an action name directly.",
		Args:         cobra.MaximumNArgs(1),
		RunE:         runRoot(ctx, task),
		Version:      version,
		SilenceUsage: true,
	}
	rootCmd.Flags().BoolP("run", "r", false, "run workflows")
	rootCmd.Flags().StringP("job", "j", "", "run job")
	rootCmd.PersistentFlags().StringVarP(&task.Input.ForgeInstance, "forge-instance", "", "github.com", "Forge instance to use.")
	rootCmd.PersistentFlags().StringVarP(&task.Input.EnvFile, "env-file", "", ".env", "Read in a file of environment variables.")

	// ./act_runner daemon
	daemonCmd := &cobra.Command{
		Aliases: []string{"daemon"},
		Use:     "execute runner daemon",
		Args:    cobra.MaximumNArgs(1),
		RunE:    runDaemon(ctx, task.Input.EnvFile),
	}
	// add all command
	rootCmd.AddCommand(daemonCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRoot(ctx context.Context, task *runtime.Task) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		jobID, err := cmd.Flags().GetString("job")
		if err != nil {
			return err
		}

		// try to connect to docker daemon
		// if failed, exit with error
		if err := engine.Start(ctx); err != nil {
			log.WithError(err).Fatalln("failed to connect docker daemon engine")
		}

		task.BuildID, _ = strconv.ParseInt(jobID, 10, 64)
		_ = task.Run(ctx, nil)
		return nil
	}
}
