package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nektos/act/pkg/artifacts"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const version = "0.1"

type Input struct {
	actor                 string
	workdir               string
	workflowsPath         string
	autodetectEvent       bool
	eventPath             string
	reuseContainers       bool
	bindWorkdir           bool
	secrets               []string
	envs                  []string
	platforms             []string
	dryrun                bool
	forcePull             bool
	forceRebuild          bool
	noOutput              bool
	envfile               string
	secretfile            string
	insecureSecrets       bool
	defaultBranch         string
	privileged            bool
	usernsMode            string
	containerArchitecture string
	containerDaemonSocket string
	noWorkflowRecurse     bool
	useGitIgnore          bool
	forgeInstance         string
	containerCapAdd       []string
	containerCapDrop      []string
	autoRemove            bool
	artifactServerPath    string
	artifactServerPort    string
	jsonLogger            bool
	noSkipCheckout        bool
	remoteName            string
}

func (i *Input) newPlatforms() map[string]string {
	return map[string]string{
		"ubuntu-latest": "node:16-buster-slim",
		"ubuntu-20.04":  "node:16-buster-slim",
		"ubuntu-18.04":  "node:16-buster-slim",
	}
}

func Execute(ctx context.Context) {
	input := Input{}

	rootCmd := &cobra.Command{
		Use:          "act [event name to run]\nIf no event name passed, will default to \"on: push\"",
		Short:        "Run GitHub actions locally by specifying the event name (e.g. `push`) or an action name directly.",
		Args:         cobra.MaximumNArgs(1),
		RunE:         runCommand(ctx, &input),
		Version:      version,
		SilenceUsage: true,
	}
	rootCmd.AddCommand(&cobra.Command{
		Aliases: []string{"daemon"},
		Use:     "daemon [event name to run]\nIf no event name passed, will default to \"on: push\"",
		Short:   "Run GitHub actions locally by specifying the event name (e.g. `push`) or an action name directly.",
		Args:    cobra.MaximumNArgs(1),
		RunE:    runDaemon(ctx, &input),
	})
	rootCmd.Flags().BoolP("run", "r", false, "run workflows")
	rootCmd.Flags().StringP("job", "j", "", "run job")
	rootCmd.PersistentFlags().StringVarP(&input.forgeInstance, "forge-instance", "", "github.com", "Forge instance to use.")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// getWorkflowsPath return the workflows directory, it will try .gitea first and then fallback to .github
func getWorkflowsPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, ".gitea/workflows")
	_, err = os.Stat(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		return filepath.Join(dir, ".github/workflows"), nil
	}
	return p, nil
}

func runTask(ctx context.Context, input *Input, jobID string) error {
	workflowsPath, err := getWorkflowsPath()
	if err != nil {
		return err
	}
	planner, err := model.NewWorkflowPlanner(workflowsPath, false)
	if err != nil {
		return err
	}

	var eventName string
	events := planner.GetEvents()
	if len(events) > 0 {
		// set default event type to first event
		// this way user dont have to specify the event.
		log.Debug().Msgf("Using detected workflow event: %s", events[0])
		eventName = events[0]
	} else {
		if plan := planner.PlanEvent("push"); plan != nil {
			eventName = "push"
		}
	}

	// build the plan for this run
	var plan *model.Plan
	if jobID != "" {
		log.Debug().Msgf("Planning job: %s", jobID)
		plan = planner.PlanJob(jobID)
	} else {
		log.Debug().Msgf("Planning event: %s", eventName)
		plan = planner.PlanEvent(eventName)
	}

	curDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// run the plan
	config := &runner.Config{
		Actor:           input.actor,
		EventName:       eventName,
		EventPath:       "",
		DefaultBranch:   "",
		ForcePull:       input.forcePull,
		ForceRebuild:    input.forceRebuild,
		ReuseContainers: input.reuseContainers,
		Workdir:         curDir,
		BindWorkdir:     input.bindWorkdir,
		LogOutput:       true,
		JSONLogger:      input.jsonLogger,
		// Env:                   envs,
		// Secrets:               secrets,
		InsecureSecrets:       input.insecureSecrets,
		Platforms:             input.newPlatforms(),
		Privileged:            input.privileged,
		UsernsMode:            input.usernsMode,
		ContainerArchitecture: input.containerArchitecture,
		ContainerDaemonSocket: input.containerDaemonSocket,
		UseGitIgnore:          input.useGitIgnore,
		GitHubInstance:        input.forgeInstance,
		ContainerCapAdd:       input.containerCapAdd,
		ContainerCapDrop:      input.containerCapDrop,
		AutoRemove:            input.autoRemove,
		ArtifactServerPath:    input.artifactServerPath,
		ArtifactServerPort:    input.artifactServerPort,
		NoSkipCheckout:        input.noSkipCheckout,
		// RemoteName:            input.remoteName,
	}
	r, err := runner.New(config)
	if err != nil {
		return fmt.Errorf("New config failed: %v", err)
	}

	cancel := artifacts.Serve(ctx, input.artifactServerPath, input.artifactServerPort)

	executor := r.NewPlanExecutor(plan).Finally(func(ctx context.Context) error {
		cancel()
		return nil
	})
	return executor(ctx)
}

func runCommand(ctx context.Context, input *Input) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		jobID, err := cmd.Flags().GetString("job")
		if err != nil {
			return err
		}

		return runTask(ctx, input, jobID)
	}
}
