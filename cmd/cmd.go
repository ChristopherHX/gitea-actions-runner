package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

const version = "0.1.5"

type globalArgs struct {
	EnvFile string
}

type RunRunnerSvc struct {
	stop func()
	wait chan error
}

// Start implements service.Interface.
func (svc *RunRunnerSvc) Start(s service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	svc.stop = func() {
		cancel()
	}
	svc.wait = make(chan error)
	go func() {
		defer cancel()
		defer close(svc.wait)
		err := runDaemon(ctx, "")(nil, nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		svc.wait <- err
		s.Stop()
	}()
	return nil
}

// Stop implements service.Interface.
func (svc *RunRunnerSvc) Stop(s service.Service) error {
	svc.stop()
	if err, ok := <-svc.wait; ok && err != nil {
		return err
	}
	return nil
}

func Execute(ctx context.Context) {
	// task := runtime.NewTask("gitea", 0, nil, nil)

	var gArgs globalArgs

	// ./act_runner
	rootCmd := &cobra.Command{
		Use:          "act [event name to run]\nIf no event name passed, will default to \"on: push\"",
		Short:        "Run GitHub actions locally by specifying the event name (e.g. `push`) or an action name directly.",
		Args:         cobra.MaximumNArgs(1),
		Version:      version,
		SilenceUsage: true,
	}
	rootCmd.PersistentFlags().StringVarP(&gArgs.EnvFile, "env-file", "", ".env", "Read in a file of environment variables.")

	// ./act_runner register
	var regArgs registerArgs
	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register a runner to the server",
		Args:  cobra.MaximumNArgs(0),
		RunE:  runRegister(ctx, &regArgs, gArgs.EnvFile), // must use a pointer to regArgs
	}
	registerCmd.Flags().BoolVar(&regArgs.NoInteractive, "no-interactive", false, "Disable interactive mode")
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	registerCmd.Flags().StringSliceVar(&regArgs.RunnerWorker, "worker", []string{}, fmt.Sprintf("worker args for example pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker%s", suffix))
	registerCmd.Flags().StringVar(&regArgs.InstanceAddr, "instance", "", "Gitea instance address")
	registerCmd.Flags().StringVar(&regArgs.Token, "token", "", "Runner token")
	registerCmd.Flags().StringVar(&regArgs.RunnerName, "name", "", "Runner name")
	registerCmd.Flags().StringVar(&regArgs.Labels, "labels", "", "Runner tags, comma separated")
	rootCmd.AddCommand(registerCmd)

	// ./act_runner daemon
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run as a runner daemon",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDaemon(ctx, gArgs.EnvFile),
	}
	// add all command
	rootCmd.AddCommand(daemonCmd)

	// hide completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	var cmdSvc = &cobra.Command{
		Use: "svc",
	}
	wd, _ := os.Getwd()
	svcConfig := &service.Config{
		Name:        "gitea-actions-runner",
		DisplayName: "Gitea Actions Runner",
		Description: "Runner Proxy to use actions/runner and github-act-runner with Gitea Actions.",
		Arguments:   []string{"svc", "run", "--working-directory", wd},
	}
	if runtime.GOOS == "darwin" {
		if path, ok := os.LookupEnv("PATH"); ok {
			svcConfig.EnvVars = map[string]string{
				"PATH": path,
			}
		}
		svcConfig.Option = service.KeyValue{
			"KeepAlive":   true,
			"RunAtLoad":   true,
			"UserService": os.Getuid() != 0,
		}
	}
	svcRun := &cobra.Command{
		Use: "run",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := os.Chdir(wd)
			if err != nil {
				return err
			}
			stdOut, err := os.OpenFile("gitea-actions-runner-log.txt", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
			if err == nil {
				os.Stdout = stdOut
				defer os.Stdout.Close()
			}
			stdErr, err := os.OpenFile("gitea-actions-runner-log-error.txt", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
			if err == nil {
				os.Stderr = stdErr
				defer os.Stderr.Close()
			}

			svc, err := service.New(&RunRunnerSvc{}, svcConfig)

			if err != nil {
				return err
			}
			return svc.Run()
		},
	}
	svcRun.Flags().StringVar(&wd, "working-directory", wd, "path to the working directory of the runner config")
	svcInstall := &cobra.Command{
		Use: "install",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := service.New(&RunRunnerSvc{}, svcConfig)

			if err != nil {
				return err
			}
			return svc.Install()
		},
	}
	svcUninstall := &cobra.Command{
		Use: "uninstall",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := service.New(&RunRunnerSvc{}, svcConfig)

			if err != nil {
				return err
			}
			return svc.Uninstall()
		},
	}
	svcStart := &cobra.Command{
		Use: "start",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := service.New(&RunRunnerSvc{}, svcConfig)

			if err != nil {
				return err
			}
			return svc.Start()
		},
	}
	svcStop := &cobra.Command{
		Use: "stop",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := service.New(&RunRunnerSvc{}, svcConfig)

			if err != nil {
				return err
			}
			return svc.Stop()
		},
	}
	cmdSvc.AddCommand(svcInstall, svcStart, svcStop, svcRun, svcUninstall)
	rootCmd.AddCommand(cmdSvc)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
