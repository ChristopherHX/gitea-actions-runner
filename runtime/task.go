package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

	state  TaskState
	client client.Client
	log    *log.Entry
}

// NewTask creates a new task
func NewTask(buildID int64, client client.Client) *Task {
	task := &Task{
		Input: &TaskInput{
			reuseContainers: true,
			ForgeInstance:   "gitea",
		},
		BuildID: buildID,

		state:  TaskStatePending,
		client: client,
		log:    log.WithField("buildID", buildID),
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

func (t *Task) Run(ctx context.Context, task *runnerv1.Task) error {
	_, exist := globalTaskMap.Load(task.Id)
	if exist {
		return fmt.Errorf("task %d already exists", task.Id)
	}

	// set task ve to global map
	// when task is done or canceled, it will be removed from the map
	globalTaskMap.Store(task.Id, t)
	defer globalTaskMap.Delete(task.Id)

	lastWords := ""
	reporter := NewReporter(ctx, t.client, task.Id)
	defer func() {
		_ = reporter.Close(lastWords)
	}()
	reporter.RunDaemon()

	reporter.Logf("received task %v of job %v", task.Id, task.Context.Fields["job"].GetStringValue())

	workflowsPath, err := getWorkflowsPath(t.Input.repoDirectory)
	if err != nil {
		lastWords = err.Error()
		return err
	}
	t.log.Debugf("workflows path: %s", workflowsPath)

	workflow, err := model.ReadWorkflow(bytes.NewReader(task.WorkflowPayload))
	if err != nil {
		lastWords = err.Error()
		return err
	}

	var plan *model.Plan
	if jobIDs := workflow.GetJobIDs(); len(jobIDs) != 1 {
		err := fmt.Errorf("multiple jobs fould: %v", jobIDs)
		lastWords = err.Error()
		return err
	} else {
		jobID := jobIDs[0]
		plan = model.CombineWorkflowPlanner(workflow).PlanJob(jobID)

		job := workflow.GetJob(jobID)
		reporter.ResetSteps(len(job.Steps))
	}

	log.Infof("plan: %+v", plan.Stages[0].Runs)

	curDir, err := os.Getwd()
	if err != nil {
		lastWords = err.Error()
		return err
	}

	dataContext := task.Context.Fields
	preset := &model.GithubContext{
		Event:           dataContext["event"].GetStructValue().AsMap(),
		RunID:           dataContext["run_id"].GetStringValue(),
		RunNumber:       dataContext["run_number"].GetStringValue(),
		Actor:           dataContext["actor"].GetStringValue(),
		Repository:      dataContext["repository"].GetStringValue(),
		EventName:       dataContext["event_name"].GetStringValue(),
		Sha:             dataContext["sha"].GetStringValue(),
		Ref:             dataContext["ref"].GetStringValue(),
		RefName:         dataContext["ref_name"].GetStringValue(),
		RefType:         dataContext["ref_type"].GetStringValue(),
		HeadRef:         dataContext["head_ref"].GetStringValue(),
		BaseRef:         dataContext["base_ref"].GetStringValue(),
		Token:           dataContext["token"].GetStringValue(),
		RepositoryOwner: dataContext["repository_owner"].GetStringValue(),
		RetentionDays:   dataContext["retention_days"].GetStringValue(),
	}
	eventJSON, err := json.Marshal(preset.Event)
	if err != nil {
		lastWords = err.Error()
		return err
	}

	input := t.Input
	config := &runner.Config{
		Workdir:               curDir, // TODO: temp dir?
		BindWorkdir:           input.bindWorkdir,
		ReuseContainers:       input.reuseContainers,
		ForcePull:             input.forcePull,
		ForceRebuild:          input.forceRebuild,
		LogOutput:             true,
		JSONLogger:            input.jsonLogger,
		Secrets:               task.Secrets,
		InsecureSecrets:       input.insecureSecrets,
		Platforms:             demoPlatforms(), // TODO: supported platforms
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
		PresetGitHubContext:   preset,
		EventJSON:             string(eventJSON),
	}
	r, err := runner.New(config)
	if err != nil {
		lastWords = err.Error()
		return err
	}

	cancel := artifacts.Serve(ctx, input.artifactServerPath, input.artifactServerPort)
	t.log.Debugf("artifacts server started at %s:%s", input.artifactServerPath, input.artifactServerPort)

	executor := r.NewPlanExecutor(plan).Finally(func(ctx context.Context) error {
		cancel()
		return nil
	})

	t.log.Infof("workflow prepared")
	reporter.Logf("workflow prepared")

	// add logger recorders
	ctx = common.WithLoggerHook(ctx, reporter)

	if err := executor(ctx); err != nil {
		lastWords = err.Error()
		return err
	}

	return nil
}
