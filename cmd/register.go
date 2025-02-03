package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	pingv1 "code.gitea.io/actions-proto-go/ping/v1"
	"gitea.com/gitea/act_runner/client"
	"gitea.com/gitea/act_runner/config"
	"gitea.com/gitea/act_runner/register"
	"gitea.com/gitea/act_runner/util"

	"github.com/bufbuild/connect-go"
	"github.com/joho/godotenv"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	_ "embed"
)

// runRegister registers a runner to the server
func runRegister(ctx context.Context, regArgs *registerArgs, envFile string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.SetReportCaller(false)
		isTerm := isatty.IsTerminal(os.Stdout.Fd())
		log.SetFormatter(&log.TextFormatter{
			DisableColors:    !isTerm,
			DisableTimestamp: true,
		})
		log.SetLevel(log.DebugLevel)

		log.Infof("Registering runner, arch=%s, os=%s, version=%s.",
			runtime.GOARCH, runtime.GOOS, version)

		if regArgs.NoInteractive {
			if err := registerNoInteractive(envFile, regArgs); err != nil {
				return err
			}
		} else {
			go func() {
				if err := registerInteractive(envFile); err != nil {
					// log.Errorln(err)
					os.Exit(2)
					return
				}
				os.Exit(0)
			}()

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			<-c
		}

		return nil
	}
}

// registerArgs represents the arguments for register command
type registerArgs struct {
	NoInteractive bool
	RunnerWorker  []string
	InstanceAddr  string
	Token         string
	RunnerName    string
	Labels        string
}

type registerStage int8

const (
	StageUnknown              registerStage = -1
	StageOverwriteLocalConfig registerStage = iota + 1
	StageInputRunnerChoice
	StageInputRunnerVersion
	StageInputRunnerWorker
	StageInputInstance
	StageInputToken
	StageInputRunnerName
	StageInputCustomLabels
	StageWaitingForRegistration
	StageExit
)

var (
	defaultLabels = []string{
		"self-hosted",
	}
)

type registerInputs struct {
	RunnerWorker []string
	InstanceAddr string
	Token        string
	RunnerName   string
	CustomLabels []string
	runnerType   int32
}

func (r *registerInputs) validate() error {
	if len(r.RunnerWorker) == 0 {
		return fmt.Errorf("Runner.Worker Path is Empty")
	}
	if r.InstanceAddr == "" {
		return fmt.Errorf("instance address is empty")
	}
	if r.Token == "" {
		return fmt.Errorf("token is empty")
	}
	if len(r.CustomLabels) > 0 {
		return validateLabels(r.CustomLabels)
	}
	return nil
}

func validateLabels(labels []string) error {
	return nil
}

//go:embed actions-runner-worker.py
var pythonWorkerScript string

//go:embed actions-runner-worker.ps1
var pwshWorkerScript string

func (r *registerInputs) assignToNext(stage registerStage, value string) registerStage {
	// must set instance address and token.
	// if empty, keep current stage.
	if stage == StageInputInstance || stage == StageInputToken || stage == StageInputRunnerChoice {
		if value == "" {
			return stage
		}
	}

	// set hostname for runner name if empty
	if stage == StageInputRunnerName && value == "" {
		value, _ = os.Hostname()
	}

	switch stage {
	case StageOverwriteLocalConfig:
		if value == "Y" || value == "y" {
			return StageInputRunnerChoice
		}
		return StageExit
	case StageInputRunnerChoice:
		if value == "0" {
			return StageInputRunnerWorker
		}
		if value == "1" {
			r.runnerType = 1
			return StageInputRunnerVersion
		}
		if value == "2" {
			r.runnerType = 2
			return StageInputRunnerVersion
		}
		log.Infoln("Invalid choice, please input again.")
		return StageInputRunnerChoice
	case StageInputRunnerWorker:
		r.RunnerWorker = strings.Split(value, ",")
		if len(r.RunnerWorker) == 0 {
			log.Infoln("Invalid choice, please input again.")
			return StageInputRunnerChoice
		}
		return StageInputInstance
	case StageInputRunnerVersion:
		d := util.DownloadRunner
		if r.runnerType == 2 {
			d = util.DownloadRunnerServer
		}
		exePath, _ := os.Executable()
		p := filepath.Join(filepath.Dir(exePath), "actions-runner-"+value)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			log.Infof("Runner %s already exists, skip downloading.", value)
		} else {
			if err := d(context.Background(), log.StandardLogger(), runtime.GOOS+"/"+runtime.GOARCH, p, value); err != nil {
				log.Infoln("Something went wrong: %s" + err.Error())
				return StageInputRunnerChoice
			}
		}

		pwshScript := filepath.Join(p, "actions-runner-worker.ps1")
		_ = os.WriteFile(pwshScript, []byte(pwshWorkerScript), 0755)
		pythonScript := filepath.Join(p, "actions-runner-worker.py")
		_ = os.WriteFile(pythonScript, []byte(pythonWorkerScript), 0755)

		var pythonPath string
		var err error
		ext := ""
		if runtime.GOOS != "windows" {
			pythonPath, err = exec.LookPath("python3")
			if err != nil {
				pythonPath, _ = exec.LookPath("python")
			}
		} else {
			ext = ".exe"
		}
		if pythonPath == "" {
			pwshPath, err := exec.LookPath("pwsh")
			if err != nil {
				log.Infoln("pwsh not found, downloading pwsh...")
				pwshVersion := "7.4.7"
				pwshPath = filepath.Join(filepath.Dir(exePath), "pwsh-"+pwshVersion)
				err = util.DownloadPwsh(context.Background(), log.StandardLogger(), runtime.GOOS+"/"+runtime.GOARCH, pwshPath, pwshVersion)
				if err != nil {
					log.Infoln("Something went wrong: %s" + err.Error())
					return StageInputRunnerChoice
				}
				pwshPath = filepath.Join(pwshPath, "pwsh"+ext)
			} else {
				log.Infoln("pwsh found, using pwsh...")
			}
			r.RunnerWorker = []string{pwshPath, pwshScript, filepath.Join(p, "bin", "Runner.Worker"+ext)}
		} else {
			log.Infoln("python found, using python...")
			r.RunnerWorker = []string{pythonPath, pythonScript, filepath.Join(p, "bin", "Runner.Worker"+ext)}
		}
		return StageInputInstance
	case StageInputInstance:
		r.InstanceAddr = value
		return StageInputToken
	case StageInputToken:
		r.Token = value
		return StageInputRunnerName
	case StageInputRunnerName:
		r.RunnerName = value
		return StageInputCustomLabels
	case StageInputCustomLabels:
		r.CustomLabels = defaultLabels
		if value != "" {
			r.CustomLabels = strings.Split(value, ",")
		}

		if validateLabels(r.CustomLabels) != nil {
			log.Infoln("Invalid labels, please input again, leave blank to use the default labels (for example, self-hosted,ubuntu-latest)")
			return StageInputCustomLabels
		}
		return StageWaitingForRegistration
	}
	return StageUnknown
}

func registerInteractive(envFile string) error {
	var (
		reader = bufio.NewReader(os.Stdin)
		stage  = StageInputRunnerChoice
		inputs = new(registerInputs)
	)

	// check if overwrite local config
	_ = godotenv.Load(envFile)
	cfg, _ := config.FromEnviron()
	if f, err := os.Stat(cfg.Runner.File); err == nil && !f.IsDir() {
		stage = StageOverwriteLocalConfig
	}

	for {
		printStageHelp(stage)

		cmdString, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		stage = inputs.assignToNext(stage, strings.TrimSpace(cmdString))

		if stage == StageWaitingForRegistration {
			log.Infof("Registering runner, name=%s, instance=%s, labels=%v.", inputs.RunnerName, inputs.InstanceAddr, inputs.CustomLabels)
			if err := doRegister(&cfg, inputs); err != nil {
				log.Errorf("Failed to register runner: %v", err)
			} else {
				log.Infof("Runner registered successfully.")
			}
			return nil
		}

		if stage == StageExit {
			return nil
		}

		if stage <= StageUnknown {
			log.Errorf("Invalid input, please re-run act command.")
			return nil
		}
	}
}

func printStageHelp(stage registerStage) {
	switch stage {
	case StageOverwriteLocalConfig:
		log.Infoln("Runner is already registered, overwrite local config? [y/N]")
	case StageInputRunnerChoice:
		log.Infoln("Choose between custom worker / official actions runner / runner.server actions runner (windows container support)? [0/1/2]")
	case StageInputRunnerWorker:
		suffix := ""
		if runtime.GOOS == "windows" {
			suffix = ".exe"
		}
		log.Infof("Enter the worker args for example pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker%s:\n", suffix)
	case StageInputRunnerVersion:
		log.Infoln("Specify the version of the runner? (for example, 3.12.1):")
	case StageInputInstance:
		log.Infoln("Enter the Gitea instance URL (for example, https://gitea.com/):")
	case StageInputToken:
		log.Infoln("Enter the runner token:")
	case StageInputRunnerName:
		hostname, _ := os.Hostname()
		log.Infof("Enter the runner name (if set empty, use hostname:%s ):\n", hostname)
	case StageInputCustomLabels:
		log.Infoln("Enter the runner labels, leave blank to use the default labels (comma-separated, for example, self-hosted,ubuntu-latest):")
	case StageWaitingForRegistration:
		log.Infoln("Waiting for registration...")
	}
}

func registerNoInteractive(envFile string, regArgs *registerArgs) error {
	_ = godotenv.Load(envFile)
	cfg, _ := config.FromEnviron()
	inputs := &registerInputs{
		RunnerWorker: regArgs.RunnerWorker,
		InstanceAddr: regArgs.InstanceAddr,
		Token:        regArgs.Token,
		RunnerName:   regArgs.RunnerName,
		CustomLabels: defaultLabels,
	}
	regArgs.Labels = strings.TrimSpace(regArgs.Labels)
	if regArgs.Labels != "" {
		inputs.CustomLabels = strings.Split(regArgs.Labels, ",")
	}
	if inputs.RunnerName == "" {
		inputs.RunnerName, _ = os.Hostname()
		log.Infof("Runner name is empty, use hostname '%s'.", inputs.RunnerName)
	}
	if err := inputs.validate(); err != nil {
		log.WithError(err).Errorf("Invalid input, please re-run act command.")
		return nil
	}
	if err := doRegister(&cfg, inputs); err != nil {
		log.Errorf("Failed to register runner: %v", err)
		return nil
	}
	log.Infof("Runner registered successfully.")
	return nil
}

func doRegister(cfg *config.Config, inputs *registerInputs) error {
	ctx := context.Background()

	// initial http client
	cli := client.New(
		inputs.InstanceAddr,
		"", "",
	)

	for {
		_, err := cli.Ping(ctx, connect.NewRequest(&pingv1.PingRequest{
			Data: inputs.RunnerName,
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
				Errorln("Cannot ping the Gitea instance server")
			// TODO: if ping failed, retry or exit
			time.Sleep(time.Second)
		} else {
			log.Debugln("Successfully pinged the Gitea instance server")
			break
		}
	}

	cfg.Runner.Name = inputs.RunnerName
	cfg.Runner.Token = inputs.Token
	cfg.Runner.Labels = inputs.CustomLabels
	cfg.Runner.RunnerWorker = inputs.RunnerWorker
	_, err := register.New(cli).Register(ctx, cfg.Runner)
	return err
}
