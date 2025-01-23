package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/joho/godotenv"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var version = "0.1.5"

type globalArgs struct {
	EnvFile string
}

type RunRunnerSvc struct {
	stop func()
	wait chan error
	cmd *cobra.Command
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
		err := runDaemon(ctx, "")(svc.cmd, nil)
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
		Use:          "actions_runner",
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
		Args:  cobra.MaximumNArgs(0),
		RunE:  runDaemon(ctx, gArgs.EnvFile),
	}
	daemonCmd.Flags().Bool("once", false, "Run one job and exit after completion")
	// add all command
	rootCmd.AddCommand(daemonCmd)

	// hide completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	var cmdSvc = &cobra.Command{
		Use:   "svc",
		Short: "Manage the runner as a system service",
	}
	wd, _ := os.Getwd()
	svcRun := &cobra.Command{
		Use:   "run",
		Short: "Used as service entrypoint",
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

			err = godotenv.Overload(gArgs.EnvFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load godotenv file '%s': %s", gArgs.EnvFile, err.Error())
			}

			svc, err := service.New(&RunRunnerSvc{
				cmd: cmdSvc,
			}, getSvcConfig(wd, gArgs))

			if err != nil {
				return err
			}
			return svc.Run()
		},
	}
	svcRun.Flags().StringVar(&wd, "working-directory", wd, "path to the working directory of the runner config")
	svcInstall := &cobra.Command{
		Use:   "install",
		Short: "Install the service may require admin privileges",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := service.New(&RunRunnerSvc{
				cmd: cmdSvc,
			}, getSvcConfig(wd, gArgs))

			if err != nil {
				return err
			}
			err = svc.Install()
			if err != nil {
				return err
			}
			fmt.Printf("Success\nConsider adding required env variables for your jobs like HOME or PATH to your '%s' godotenv file\nSee https://pkg.go.dev/github.com/joho/godotenv for the syntax\n", gArgs.EnvFile)
			return nil
		},
	}
	svcUninstall := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the service may require admin privileges",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := service.New(&RunRunnerSvc{
				cmd: cmdSvc,
			}, getSvcConfig(wd, gArgs))

			if err != nil {
				return err
			}
			return svc.Uninstall()
		},
	}
	svcStart := &cobra.Command{
		Use:   "start",
		Short: "Start the service may require admin privileges",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := service.New(&RunRunnerSvc{}, getSvcConfig(wd, gArgs))

			if err != nil {
				return err
			}
			return svc.Start()
		},
	}
	svcStop := &cobra.Command{
		Use:   "stop",
		Short: "Stop the service may require admin privileges",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := service.New(&RunRunnerSvc{}, getSvcConfig(wd, gArgs))

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

func getSvcConfig(wd string, gArgs globalArgs) *service.Config {
	svcConfig := &service.Config{
		Name:        "gitea-actions-runner",
		DisplayName: "Gitea Actions Runner",
		Description: "Runner Proxy to use actions/runner and github-act-runner with Gitea Actions.",
		Arguments:   []string{"svc", "run", "--working-directory", wd, "--env-file", gArgs.EnvFile},
	}
	if runtime.GOOS == "darwin" {
		svcConfig.Option = service.KeyValue{
			"KeepAlive":   true,
			"RunAtLoad":   true,
			"UserService": os.Getuid() != 0,
		}
	}
	return svcConfig
}
