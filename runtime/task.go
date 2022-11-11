package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/nektos/act/pkg/artifacts"
	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
	log "github.com/sirupsen/logrus"
)

var globalTaskMap sync.Map

type TaskInput struct {
	repoDirectory string
	// actor         string
	// workdir string
	// workflowsPath         string
	// autodetectEvent       bool
	// eventPath       string
	reuseContainers bool
	bindWorkdir     bool
	// secrets         []string
	envs map[string]string
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

type Task struct {
	BuildID int64
	Input   *TaskInput

	client client.Client
	log    *log.Entry
}

// NewTask creates a new task
func NewTask(forgeInstance string, buildID int64, client client.Client, runnerEnvs map[string]string) *Task {
	task := &Task{
		Input: &TaskInput{
			reuseContainers: false,
			ForgeInstance:   forgeInstance,
			envs:            runnerEnvs,
			noSkipCheckout:  true,
		},
		BuildID: buildID,

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

func getToken(task *runnerv1.Task) string {
	token := task.Secrets["GITHUB_TOKEN"]
	if task.Secrets["GITEA_TOKEN"] != "" {
		token = task.Secrets["GITEA_TOKEN"]
	}
	if task.Context.Fields["token"].GetStringValue() != "" {
		token = task.Context.Fields["token"].GetStringValue()
	}
	return token
}

func (t *Task) Run(ctx context.Context, task *runnerv1.Task) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	_, exist := globalTaskMap.Load(task.Id)
	if exist {
		return fmt.Errorf("task %d already exists", task.Id)
	}

	// set task ve to global map
	// when task is done or canceled, it will be removed from the map
	globalTaskMap.Store(task.Id, t)
	defer globalTaskMap.Delete(task.Id)

	lastWords := ""
	reporter := NewReporter(ctx, cancel, t.client, task)
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
	jobIDs := workflow.GetJobIDs()
	if len(jobIDs) != 1 {
		err := fmt.Errorf("multiple jobs found: %v", jobIDs)
		lastWords = err.Error()
		return err
	}
	jobID := jobIDs[0]
	plan = model.CombineWorkflowPlanner(workflow).PlanJob(jobID)
	job := workflow.GetJob(jobID)
	reporter.ResetSteps(len(job.Steps))

	log.Infof("plan: %+v", plan.Stages[0].Runs)

	token := getToken(task)
	dataContext := task.Context.Fields

	log.Infof("task %v token is %v %v", task.Id, token, dataContext["repository"].GetStringValue())

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
		Token:           token,
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
		Workdir:               "/root",
		BindWorkdir:           input.bindWorkdir,
		ReuseContainers:       input.reuseContainers,
		ForcePull:             input.forcePull,
		ForceRebuild:          input.forceRebuild,
		LogOutput:             true,
		JSONLogger:            input.jsonLogger,
		Env:                   input.envs,
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
		ContainerNamePrefix:   fmt.Sprintf("gitea-task-%d", task.Id),
	}
	r, err := runner.New(config)
	if err != nil {
		lastWords = err.Error()
		return err
	}

	artifactCancel := artifacts.Serve(ctx, input.artifactServerPath, input.artifactServerPort)
	t.log.Debugf("artifacts server started at %s:%s", input.artifactServerPath, input.artifactServerPort)

	executor := r.NewPlanExecutor(plan).Finally(func(ctx context.Context) error {
		artifactCancel()
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
