package cmd

import (
	"bufio"
	"context"
	"os"
	"os/signal"
	"runtime"
	"strings"

	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// runRegister registers a runner to the server
func runRegister(ctx context.Context, regArgs *registerArgs) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.SetReportCaller(false)
		isTerm := isatty.IsTerminal(os.Stdout.Fd())
		log.SetFormatter(&log.TextFormatter{
			DisableColors:    !isTerm,
			DisableTimestamp: true,
		})

		log.Infof("Registering runner, arch=%s, os=%s, version=%s.",
			runtime.GOARCH, runtime.GOOS, version)

		// runner always needs root permission
		if os.Getuid() != 0 {
			// TODO: use a better way to check root permission
			log.Warnf("Runner in user-mode.")
		}

		go func() {
			if err := registerInteractive(); err != nil {
				// log.Errorln(err)
				os.Exit(2)
				return
			}
			os.Exit(0)
		}()

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c

		return nil
	}
}

// registerArgs represents the arguments for register command
type registerArgs struct {
	NoInteractive bool
	InstanceAddr  string
	Token         string
}

type registerStage int8

const (
	StageUnknown       registerStage = -1
	StageInputInstance registerStage = iota + 1
	StageInputToken
	StageInputRunnerName
	StageInputCustomLabels
	StageWaitingForRegistration
)

type registerInputs struct {
	InstanceAddr string
	Token        string
	RunnerName   string
	CustomLabels []string
}

func (r *registerInputs) assignToNext(stage registerStage, value string) registerStage {

	// must set instance address and token.
	// if empty, keep current stage.
	if stage == StageInputInstance || stage == StageInputToken {
		if value == "" {
			return stage
		}
	}

	// set hostname for runner name if empty
	if stage == StageInputRunnerName && value == "" {
		value, _ = os.Hostname()
	}

	switch stage {
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
		r.CustomLabels = strings.Split(value, ",")
		return StageWaitingForRegistration
	}
	return StageUnknown
}

func registerInteractive() error {
	var (
		reader = bufio.NewReader(os.Stdin)
		stage  = StageInputInstance
		inputs = new(registerInputs)
	)

	for {
		printStageHelp(stage)
		cmdString, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		stage = inputs.assignToNext(stage, strings.TrimSpace(cmdString))

		if stage == StageWaitingForRegistration {
			log.Infof("Registering runner, name=%s, instance=%s, labels=%v.", inputs.RunnerName, inputs.InstanceAddr, inputs.CustomLabels)
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
	case StageInputInstance:
		log.Infoln("Enter the Gitea instance URL (for example, https://gitea.com/):")
	case StageInputToken:
		log.Infoln("Enter the runner token:")
	case StageInputRunnerName:
		hostname, _ := os.Hostname()
		log.Infof("Enter the runner name (if set empty, use hostname:%s ):\n", hostname)
	case StageInputCustomLabels:
		log.Infoln("Enter the runner custom labels (comma-separated, for example, label1,label2):")
	case StageWaitingForRegistration:
		log.Infoln("Waiting for registration...")
	}
}
