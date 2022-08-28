package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nektos/act/pkg/artifacts"
	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
	log "github.com/sirupsen/logrus"
)

type TaskInput struct {
	repoDirectory string
	actor         string
	// workdir string
	// workflowsPath         string
	// autodetectEvent       bool
	// eventPath       string
	reuseContainers bool
	bindWorkdir     bool
	// secrets         []string
	// envs []string
	// platforms       []string
	// dryrun       bool
	forcePull    bool
	forceRebuild bool
	// noOutput     bool
	// envfile         string
	// secretfile            string
	insecureSecrets bool
	// defaultBranch         string
	privileged            bool
	usernsMode            string
	containerArchitecture string
	containerDaemonSocket string
	// noWorkflowRecurse     bool
	useGitIgnore       bool
	containerCapAdd    []string
	containerCapDrop   []string
	autoRemove         bool
	artifactServerPath string
	artifactServerPort string
	jsonLogger         bool
	noSkipCheckout     bool
	// remoteName            string

	ForgeInstance string
	EnvFile       string
}

type taskLogHook struct {
	entries []*log.Entry
}

func (h *taskLogHook) Levels() []log.Level {
	return log.AllLevels
}

func (h *taskLogHook) Fire(entry *log.Entry) error {
	if flag, ok := entry.Data["raw_output"]; ok {
		if flagVal, ok := flag.(bool); flagVal && ok {
			log.Infof("task log: %s", entry.Message)
			h.entries = append(h.entries, entry)
		}
	}
	return nil
}

type Task struct {
	JobID   string
	Input   *TaskInput
	LogHook *taskLogHook
}

func NewTask() *Task {
	task := &Task{
		Input: &TaskInput{
			reuseContainers: true,
			ForgeInstance:   "gitea",
		},
		LogHook: &taskLogHook{},
	}
	task.Input.repoDirectory, _ = os.Getwd()
	return task
}

// getWorkflowsPath return the workflows directory, it will try .gitea first and then fallback to .github
func getWorkflowsPath(dir string) (string, error) {
	p := filepath.Join(dir, ".gitea/workflows")
	_, err := os.Stat(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		return filepath.Join(dir, ".github/workflows"), nil
	}
	return p, nil
}

func demoPlatforms() map[string]string {
	return map[string]string{
		"ubuntu-latest": "node:16-buster-slim",
		"ubuntu-20.04":  "node:16-buster-slim",
		"ubuntu-18.04":  "node:16-buster-slim",
	}
}

func (t *Task) Run(ctx context.Context) error {
	workflowsPath, err := getWorkflowsPath(t.Input.repoDirectory)
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
		log.Debugf("Using detected workflow event: %s", events[0])
		eventName = events[0]
	} else {
		if plan := planner.PlanEvent("push"); plan != nil {
			eventName = "push"
		}
	}

	// build the plan for this run
	var plan *model.Plan
	jobID := t.JobID
	if jobID != "" {
		log.Debugf("Planning job: %s", jobID)
		plan = planner.PlanJob(jobID)
	} else {
		log.Debugf("Planning event: %s", eventName)
		plan = planner.PlanEvent(eventName)
	}

	curDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// run the plan
	input := t.Input
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
		Platforms:             demoPlatforms(),
		Privileged:            input.privileged,
		UsernsMode:            input.usernsMode,
		ContainerArchitecture: input.containerArchitecture,
		ContainerDaemonSocket: input.containerDaemonSocket,
		UseGitIgnore:          input.useGitIgnore,
		GitHubInstance:        input.ForgeInstance,
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
		return fmt.Errorf("new config failed: %v", err)
	}

	cancel := artifacts.Serve(ctx, input.artifactServerPath, input.artifactServerPort)

	executor := r.NewPlanExecutor(plan).Finally(func(ctx context.Context) error {
		cancel()
		return nil
	})

	ctx = common.WithLoggerHook(ctx, t.LogHook)
	if err := executor(ctx); err != nil {
		log.Warnf("workflow execution failed:%v, logs: %d", err, len(t.LogHook.entries))
		return err
	}
	log.Infof("workflow completed, logs: %d", len(t.LogHook.entries))
	return nil
}
