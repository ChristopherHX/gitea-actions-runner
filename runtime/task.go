package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

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
	lock    sync.Mutex
}

func (h *taskLogHook) Levels() []log.Level {
	return log.AllLevels
}

func (h *taskLogHook) Fire(entry *log.Entry) error {
	if flag, ok := entry.Data["raw_output"]; ok {
		h.lock.Lock()
		if flagVal, ok := flag.(bool); flagVal && ok {
			log.Infof("task log: %s", entry.Message)
			h.entries = append(h.entries, entry)
		}
		h.lock.Unlock()
	}
	return nil
}

func (h *taskLogHook) swapLogs() []*log.Entry {
	if len(h.entries) == 0 {
		return nil
	}
	h.lock.Lock()
	entries := h.entries
	h.entries = nil
	h.lock.Unlock()
	return entries
}

type TaskState int

const (
	// TaskStateUnknown is the default state
	TaskStateUnknown TaskState = iota
	// TaskStatePending is the pending state
	// pending means task is received, parsing actions and preparing to run
	TaskStatePending
	// TaskStateRunning is the state when the task is running
	// running means task is running
	TaskStateRunning
	// TaskStateSuccess is the state when the task is successful
	// success means task is successful without any error
	TaskStateSuccess
	// TaskStateFailure is the state when the task is failed
	// failure means task is failed with error
	TaskStateFailure
)

type Task struct {
	BuildID int64
	Input   *TaskInput

	logHook *taskLogHook
	state   TaskState
	client  client.Client
	log     *log.Entry
}

// newTask creates a new task
func NewTask(buildID int64, client client.Client) *Task {
	task := &Task{
		Input: &TaskInput{
			reuseContainers: true,
			ForgeInstance:   "gitea",
		},
		BuildID: buildID,

		state:   TaskStatePending,
		client:  client,
		log:     log.WithField("buildID", buildID),
		logHook: &taskLogHook{},
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

// reportFailure reports the failure of the task
func (t *Task) reportFailure(ctx context.Context, err error) {
	t.state = TaskStateFailure
	finishTask(t.BuildID)

	t.log.Errorf("task failed: %v", err)

	if t.client == nil {
		// TODO: fill the step request
		stepRequest := &runnerv1.UpdateStepRequest{}
		_ = t.client.UpdateStep(ctx, stepRequest)
		return
	}
}

func (t *Task) startReporting(ctx context.Context, interval int64) {
	for {
		time.Sleep(time.Duration(interval) * time.Second)
		if t.state == TaskStateSuccess || t.state == TaskStateFailure {
			t.log.Debugf("task reporting stopped")
			break
		}
		t.reportStep(ctx)
	}
}

// reportStep reports the step of the task
func (t *Task) reportStep(ctx context.Context) {
	if t.client == nil {
		return
	}
	logValues := t.logHook.swapLogs()
	if len(logValues) == 0 {
		t.log.Debugf("no log to report")
		return
	}
	t.log.Infof("reporting %d logs", len(logValues))

	// TODO: fill the step request
	stepRequest := &runnerv1.UpdateStepRequest{}
	_ = t.client.UpdateStep(ctx, stepRequest)
}

// reportSuccess reports the success of the task
func (t *Task) reportSuccess(ctx context.Context) {
	t.state = TaskStateSuccess
	finishTask(t.BuildID)

	t.log.Infof("task success")

	if t.client == nil {
		return
	}

	// TODO: fill the step request
	stepRequest := &runnerv1.UpdateStepRequest{}
	_ = t.client.UpdateStep(ctx, stepRequest)
}

func (t *Task) Run(ctx context.Context, data *runnerv1.DetailResponse) error {
	_, exist := globalTaskMap.Load(data.Build.Id)
	if exist {
		return fmt.Errorf("task %d already exists", data.Build.Id)
	}

	// set task ve to global map
	// when task is done or canceled, it will be removed from the map
	globalTaskMap.Store(data.Build.Id, t)

	workflowsPath, err := getWorkflowsPath(t.Input.repoDirectory)
	if err != nil {
		t.reportFailure(ctx, err)
		return err
	}
	t.log.Debugf("workflows path: %s", workflowsPath)

	planner, err := model.NewWorkflowPlanner(workflowsPath, false)
	if err != nil {
		t.reportFailure(ctx, err)
		return err
	}

	var eventName string
	events := planner.GetEvents()
	if len(events) > 0 {
		// set default event type to first event
		// this way user dont have to specify the event.
		t.log.Debugf("Using detected workflow event: %s", events[0])
		eventName = events[0]
	} else {
		if plan := planner.PlanEvent("push"); plan != nil {
			eventName = "push"
		}
	}

	// build the plan for this run
	var plan *model.Plan
	jobID := ""
	if t.BuildID > 0 {
		jobID = fmt.Sprintf("%d", t.BuildID)
	}
	if jobID != "" {
		t.log.Infof("Planning job: %s", jobID)
		plan = planner.PlanJob(jobID)
	} else {
		t.log.Infof("Planning event: %s", eventName)
		plan = planner.PlanEvent(eventName)
	}

	curDir, err := os.Getwd()
	if err != nil {
		t.reportFailure(ctx, err)
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
		t.reportFailure(ctx, err)
		return err
	}

	cancel := artifacts.Serve(ctx, input.artifactServerPath, input.artifactServerPort)
	t.log.Debugf("artifacts server started at %s:%s", input.artifactServerPath, input.artifactServerPort)

	executor := r.NewPlanExecutor(plan).Finally(func(ctx context.Context) error {
		cancel()
		return nil
	})

	t.log.Infof("workflow prepared")

	// add logger recorders
	ctx = common.WithLoggerHook(ctx, t.logHook)

	go t.startReporting(ctx, 1)

	if err := executor(ctx); err != nil {
		t.reportFailure(ctx, err)
		return err
	}

	t.reportSuccess(ctx)
	return nil
}
