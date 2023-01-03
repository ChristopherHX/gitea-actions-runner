package runtime

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"gitea.com/gitea/act_runner/actions/server"
	"gitea.com/gitea/act_runner/client"

	"github.com/ChristopherHX/github-act-runner/protocol"
	"github.com/google/uuid"
	"github.com/nektos/act/pkg/model"
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
	// reuseContainers bool
	// bindWorkdir     bool
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
	useGitIgnore     bool
	containerCapAdd  []string
	containerCapDrop []string
	// autoRemove         bool
	artifactServerPath string
	artifactServerPort string
	jsonLogger         bool
	// noSkipCheckout     bool
	// remoteName            string

	EnvFile string

	containerNetworkMode string
}

type Task struct {
	BuildID int64
	Input   *TaskInput

	client         client.Client
	log            *log.Entry
	platformPicker func([]string) string
}

// NewTask creates a new task
func NewTask(forgeInstance string, buildID int64, client client.Client, runnerEnvs map[string]string, picker func([]string) string) *Task {
	task := &Task{
		Input: &TaskInput{
			envs:                 runnerEnvs,
			containerNetworkMode: "bridge", // TODO should be configurable
		},
		BuildID: buildID,

		client:         client,
		log:            log.WithField("buildID", buildID),
		platformPicker: picker,
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

	dataContext := task.Context.Fields

	log.Infof("task %v repo is %v %v %v", task.Id, dataContext["repository"].GetStringValue(),
		dataContext["gitea_default_actions_url"].GetStringValue(),
		t.client.Address())

	// maxLifetime := 3 * time.Hour
	// if deadline, ok := ctx.Deadline(); ok {
	// 	maxLifetime = time.Until(deadline)
	// }

	httpServer := &http.Server{Addr: "0.0.0.0:3403", Handler: &server.ActionsServer{
		Client: t.client,
		Task:   task,
	}}

	go func() {
		httpServer.ListenAndServe()
	}()

	steps := []protocol.ActionStep{}
	for _, s := range job.Steps {
		displayName := &protocol.TemplateToken{}
		displayName.FromRawObject(s.Name)
		if s.Run != "" {
			inputs := &protocol.TemplateToken{}
			inputs.FromRawObject(map[interface{}]interface{}{
				"script": s.Run,
			})
			steps = append(steps, protocol.ActionStep{
				Type: "action",
				Reference: protocol.ActionStepDefinitionReference{
					Type: "script",
				},
				Inputs:           inputs,
				Condition:        s.If.Value,
				DisplayNameToken: displayName,
				ContextName:      s.ID,
			})
		} else {
			uses := s.Uses
			nameAndPathOrRef := strings.Split(uses, "@")
			nameAndPath := strings.Split(nameAndPathOrRef[0], "/")
			var reference protocol.ActionStepDefinitionReference
			if nameAndPath[0] == "." {
				reference = protocol.ActionStepDefinitionReference{
					Type:           "repository",
					Path:           path.Join(nameAndPath[1:]...),
					Ref:            nameAndPathOrRef[1],
					RepositoryType: "self",
				}
			} else {
				reference = protocol.ActionStepDefinitionReference{
					Type:           "repository",
					Name:           path.Join(nameAndPath[0:1]...),
					Path:           path.Join(nameAndPath[2:]...),
					Ref:            nameAndPathOrRef[1],
					RepositoryType: "GitHub",
				}
			}
			rawIn := map[interface{}]interface{}{}
			for k, v := range s.With {
				rawIn[k] = v
			}
			inputs := &protocol.TemplateToken{}
			inputs.FromRawObject(rawIn)
			steps = append(steps, protocol.ActionStep{
				Type:             "action",
				Reference:        reference,
				Inputs:           inputs,
				Condition:        s.If.Value,
				DisplayNameToken: displayName,
				ContextName:      s.ID,
			})
		}
	}

	jmessage := &protocol.AgentJobRequestMessage{
		MessageType: "jobRequest",
		Plan: &protocol.TaskOrchestrationPlanReference{
			ScopeIdentifier: uuid.New().String(),
			PlanID:          uuid.New().String(),
			PlanType:        "free",
		},
		Timeline: &protocol.TimeLineReference{
			ID: uuid.New().String(),
		},
		Resources: &protocol.JobResources{
			Endpoints: []protocol.JobEndpoint{
				{
					Name: "SYSTEMVSSCONNECTION",
					Data: map[string]string{},
					URL:  "http://localhost:3403/",
					Authorization: protocol.JobAuthorization{
						Scheme: "OAuth",
						Parameters: map[string]string{
							"AccessToken": "Hello World",
						},
					},
				},
			},
		},
		JobID:          uuid.New().String(),
		JobDisplayName: "test ()",
		JobName:        "test",
		RequestID:      475,
		LockedUntil:    "0001-01-01T00:00:00",
		Steps:          steps,
		Variables:      map[string]protocol.VariableValue{},
		ContextData: map[string]protocol.PipelineContextData{
			"github": server.ToPipelineContextData(task.Context.AsMap()),
		},
	}

	src, _ := json.Marshal(jmessage)
	jobExecCtx := ctx

	worker := exec.Command("pwsh", "C:\\Users\\Christopher\\runner.server\\invokeWorkerStdIn.ps1", "C:\\Users\\Christopher\\AppData\\Local\\gharun\\runner\\2.299.1\\bin\\Runner.Worker.exe")
	in, err := worker.StdinPipe()
	if err != nil {
		return err
	}
	er, err := worker.StderrPipe()
	if err != nil {
		return err
	}
	out, err := worker.StdoutPipe()
	if err != nil {
		return err
	}
	err = worker.Start()
	if err != nil {
		return err
	}
	mid := make([]byte, 4)
	binary.BigEndian.PutUint32(mid, 1) // NewJobRequest
	in.Write(mid)
	binary.BigEndian.PutUint32(mid, uint32(len(src)))
	in.Write(mid)
	in.Write(src)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-jobExecCtx.Done():
			binary.BigEndian.PutUint32(mid, 2) // CancelRequest
			in.Write(mid)
			binary.BigEndian.PutUint32(mid, uint32(len(src)))
			in.Write(mid)
			in.Write(src)
		case <-done:
		}
	}()
	io.Copy(os.Stdout, out)
	io.Copy(os.Stdout, er)
	worker.Wait()
	if exitcode := worker.ProcessState.ExitCode(); exitcode != 0 {
		return fmt.Errorf("failed to execute worker: %v", exitcode)
	}

	return nil
}
