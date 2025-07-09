package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/ChristopherHX/gitea-actions-runner/config"
	"github.com/ChristopherHX/gitea-actions-runner/core"
	"github.com/ChristopherHX/gitea-actions-runner/exec"
	"github.com/ChristopherHX/gitea-actions-runner/util"
	"github.com/joho/godotenv"
	"github.com/kardianos/service"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var version = "local"

type globalArgs struct {
	EnvFile string
}

type RunRunnerSvc struct {
	stop func()
	wait chan error
	cmd  *cobra.Command
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
	registerCmd.Flags().Int32Var(&regArgs.RunnerType, "type", 0, "Runner type to download, 0 for manual see --worker, 1 for official, 2 for ChristopherHX/runner.server (windows container support)")
	registerCmd.Flags().StringVar(&regArgs.RunnerVersion, "version", "", "Runner version to download without v prefix")
	registerCmd.Flags().StringVar(&regArgs.InstanceAddr, "instance", "", "Gitea instance address")
	registerCmd.Flags().StringVar(&regArgs.Token, "token", "", "Runner token")
	registerCmd.Flags().StringVar(&regArgs.RunnerName, "name", "", "Runner name")
	registerCmd.Flags().StringVar(&regArgs.Labels, "labels", "", "Runner tags, comma separated")
	registerCmd.Flags().BoolVar(&regArgs.Ephemeral, "ephemeral", false, "Configure the runner to be ephemeral and only ever be able to pick a single job (stricter than --once)")
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
				log.SetOutput(os.Stdout)
				defer stdOut.Sync()
			}
			stdErr, err := os.OpenFile("gitea-actions-runner-log-error.txt", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
			if err == nil {
				os.Stderr = stdErr
				defer stdErr.Sync()
			}

			err = godotenv.Overload(gArgs.EnvFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load godotenv file '%s': %s", gArgs.EnvFile, err.Error())
			}

			svc, err := service.New(&RunRunnerSvc{
				cmd: cmd,
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
				cmd: cmd,
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
				cmd: cmd,
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
			svc, err := service.New(&RunRunnerSvc{
				cmd: cmd,
			}, getSvcConfig(wd, gArgs))

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
			svc, err := service.New(&RunRunnerSvc{
				cmd: cmd,
			}, getSvcConfig(wd, gArgs))

			if err != nil {
				return err
			}
			return svc.Stop()
		},
	}
	cmdSvc.AddCommand(svcInstall, svcStart, svcStop, svcRun, svcUninstall)
	rootCmd.AddCommand(cmdSvc)

	filePath := ""
	workerArgs := []string{}
	contextPath := ""
	varsPath := ""
	secretsPath := ""
	cmdExec := &cobra.Command{
		Use:   "exec",
		Short: "Run a command in the runner environment",
		Args:  cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, _ := os.ReadFile(filePath)
			contentData, _ := os.ReadFile(contextPath)
			varsData, _ := os.ReadFile(varsPath)
			secretsData, _ := os.ReadFile(secretsPath)
			return exec.Exec(ctx, string(content), string(contentData), string(varsData), string(secretsData), workerArgs)
		},
	}
	cmdExec.Flags().StringVar(&filePath, "file", "", "Read in a workflow file with a single job.")
	cmdExec.Flags().StringVar(&contextPath, "context", "", "Read in a context file.")
	cmdExec.Flags().StringVar(&varsPath, "vars-file", "", "Read in a context file.")
	cmdExec.Flags().StringVar(&secretsPath, "secrets-file", "", "Read in a context file.")
	cmdExec.Flags().StringSliceVar(&workerArgs, "worker", []string{}, "worker args for example pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker")
	rootCmd.AddCommand(cmdExec)

	var capacity int
	var allowCloneUpgrade bool
	cmdUpdate := &cobra.Command{
		Use:   "update",
		Short: "Update the managed runner",
		Args:  cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromEnviron()
			if err != nil {
				return err
			}
			content, err := os.ReadFile(cfg.Runner.File)
			if err != nil {
				return err
			}

			var runner core.Runner
			if err := json.Unmarshal(content, &runner); err != nil {
				return err
			}

			if capacity > 0 && runner.Capacity != capacity {
				runner.Capacity = capacity
				log.Info("update: updated capacity to ", capacity)
			}
			if len(regArgs.RunnerWorker) > 0 {
				runner.RunnerWorker = regArgs.RunnerWorker
			}
			if regArgs.RunnerType != 0 {
				worker := util.SetupRunner(regArgs.RunnerType, regArgs.RunnerVersion)
				if worker != nil {
					runner.RunnerWorker = worker
				} else {
					log.Error("update: failed to setup runner of type ", regArgs.RunnerType, " and version ", regArgs.RunnerVersion)
					return err
				}
			}
			flags := []string{}
			if allowCloneUpgrade && len(runner.RunnerWorker) == 3 && path.IsAbs(runner.RunnerWorker[0]) && path.IsAbs(runner.RunnerWorker[1]) && path.IsAbs(runner.RunnerWorker[2]) {
				wd, _ := os.Getwd()
				bindir := path.Dir(runner.RunnerWorker[2])
				rootdir := path.Dir(bindir)
				runnerDirBaseName := path.Base(rootdir)
				var version string
				m, err := fmt.Sscanf(runnerDirBaseName, "actions-runner-%s", &version)
				interpreter := strings.TrimSuffix(path.Base(runner.RunnerWorker[0]), path.Ext(runner.RunnerWorker[0]))
				if m == 1 && err == nil && path.Dir(rootdir) == wd && path.Base(bindir) == "bin" && (interpreter == "python" || interpreter == "python3") && path.Base(runner.RunnerWorker[1]) == "actions-runner-worker.py" || interpreter == "pwsh" && path.Base(runner.RunnerWorker[1]) == "actions-runner-worker.ps1" {
					flags = append(flags, "--runner-dir="+rootdir, "--allow-clone")
					log.Info("update: upgraded runner config to allow capacity > 1")
				} else {
					log.WithError(err).Info("update: automated upgrade not applicable")
				}
			}
			runner.RunnerWorker = append(flags, runner.RunnerWorker...)
			file, err := json.MarshalIndent(runner, "", "  ")
			if err != nil {
				log.WithError(err).Error("update: cannot marshal the json input")
				return err
			}

			// store runner config in .runner file
			return os.WriteFile(cfg.Runner.File, file, 0o644)
		},
	}
	cmdUpdate.Flags().IntVarP(&capacity, "capacity", "c", 0, "Runner capacity")
	cmdUpdate.Flags().StringSliceVar(&regArgs.RunnerWorker, "worker", []string{}, fmt.Sprintf("worker args for example pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker%s", suffix))
	cmdUpdate.Flags().Int32Var(&regArgs.RunnerType, "type", 0, "Runner type to download, 0 for manual see --worker, 1 for official, 2 for ChristopherHX/runner.server (windows container support)")
	cmdUpdate.Flags().StringVar(&regArgs.RunnerVersion, "version", "", "Runner version to download without v prefix")
	cmdUpdate.Flags().BoolVar(&allowCloneUpgrade, "allow-clone-upgrade", false, "tries to upgrade an old runner setup to allow capacity > 1")
	rootCmd.AddCommand(cmdUpdate)

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
