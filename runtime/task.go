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
	"strings"
	"sync"
	"time"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"gitea.com/gitea/act_runner/actions/server"
	"gitea.com/gitea/act_runner/client"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/ChristopherHX/github-act-runner/protocol"
	"github.com/bufbuild/connect-go"
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

func ToTemplateToken(node yaml.Node) *protocol.TemplateToken {
	switch node.Kind {
	case yaml.ScalarNode:
		var number float64
		var str string
		var b bool
		var val interface{}
		if node.Tag == "!!null" || node.Value == "" {
			return nil
		}
		if err := node.Decode(&number); err == nil {
			val = number
		} else if err := node.Decode(&b); err == nil {
			val = b
		} else if err := node.Decode(&str); err == nil {
			val = str
		}
		token := &protocol.TemplateToken{}
		token.FromRawObject(val)
		return token
	case yaml.SequenceNode:
		content := make([]protocol.TemplateToken, len(node.Content))
		for i := 0; i < len(content); i++ {
			content[i] = *ToTemplateToken(*node.Content[i])
		}
		return &protocol.TemplateToken{
			Type: 1,
			Seq:  &content,
		}
	case yaml.MappingNode:
		cap := len(node.Content) / 2
		content := make([]protocol.MapEntry, 0, cap)
		for i := 0; i < cap; i++ {
			key := ToTemplateToken(*node.Content[i*2])
			val := ToTemplateToken(*node.Content[i*2+1])
			// skip null values of some yaml structures of act
			if key != nil && val != nil {
				content = append(content, protocol.MapEntry{Key: key, Value: val})
			}
		}
		return &protocol.TemplateToken{
			Type: 2,
			Map:  &content,
		}
	}
	return nil
}

func (t *Task) Run(ctx context.Context, task *runnerv1.Task, runnerWorker []string) error {
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

	workflow, err := model.ReadWorkflow(bytes.NewReader(task.WorkflowPayload))
	if err != nil {
		return err
	}

	var plan *model.Plan
	jobIDs := workflow.GetJobIDs()
	if len(jobIDs) != 1 {
		err := fmt.Errorf("multiple jobs found: %v", jobIDs)
		return err
	}
	jobID := jobIDs[0]
	plan = model.CombineWorkflowPlanner(workflow).PlanJob(jobID)
	job := workflow.GetJob(jobID)

	log.Infof("plan: %+v", plan.Stages[0].Runs)

	dataContext := task.Context.Fields

	log.Infof("task %v repo is %v %v %v", task.Id, dataContext["repository"].GetStringValue(),
		dataContext["gitea_default_actions_url"].GetStringValue(),
		t.client.Address())

	aserver := &server.ActionsServer{
		TraceLog:  make(chan interface{}),
		ServerUrl: dataContext["gitea_default_actions_url"].GetStringValue(),
	}
	defer func() {
		close(aserver.TraceLog)
	}()
	steps := []protocol.ActionStep{}
	type StepMeta struct {
		LogIndex  int64
		LogLength int64
		StepIndex int64
		Record    protocol.TimelineRecord
	}
	stepMeta := make(map[string]*StepMeta)
	var stepIndex int64 = -1
	taskState := &runnerv1.TaskState{Id: task.GetId(), Steps: make([]*runnerv1.StepState, len(job.Steps)), StartedAt: timestamppb.Now()}
	for i := 0; i < len(taskState.Steps); i++ {
		taskState.Steps[i] = &runnerv1.StepState{
			Id: int64(i),
		}
	}
	var logline int64 = 0

	go func() {
		for {
			obj, ok := <-aserver.TraceLog
			if !ok {
				break
			}

			j, _ := json.MarshalIndent(obj, "", "    ")
			fmt.Printf("MESSAGE: %s\n", j)

			if feed, ok := obj.(*protocol.TimelineRecordFeedLinesWrapper); ok {
				loglineStart := logline
				logline += feed.Count
				step, ok := stepMeta[feed.StepID]
				if ok {
					step.LogLength += feed.Count
				} else {
					step = &StepMeta{}
					stepMeta[feed.StepID] = step
					step.StepIndex = -1
					step.LogIndex = -1
					step.LogLength = feed.Count
					for i, s := range steps {
						if s.Id == feed.StepID {
							step.StepIndex = int64(i)
							break
						}
					}
				}
				if step.LogIndex == -1 {
					step.LogIndex = loglineStart
				}
				now := timestamppb.Now()
				rows := []*runnerv1.LogRow{}
				for _, row := range feed.Value {
					rows = append(rows, &runnerv1.LogRow{
						Time:    now,
						Content: row,
					})
				}
				res, err := t.client.UpdateLog(ctx, connect.NewRequest(&runnerv1.UpdateLogRequest{
					TaskId: task.GetId(),
					Index:  loglineStart,
					Rows:   rows,
				}))
				if err == nil {
					logline = res.Msg.GetAckIndex()
				}
				if step.StepIndex != -1 {
					stepIndex = step.StepIndex
					taskState.Steps[stepIndex].LogIndex = step.LogIndex
					taskState.Steps[stepIndex].LogLength = step.LogLength
					t.client.UpdateTask(ctx, connect.NewRequest(&runnerv1.UpdateTaskRequest{
						State: taskState,
					}))
				}
			} else if timeline, ok := obj.(*protocol.TimelineRecordWrapper); ok {
				for _, rec := range timeline.Value {
					step, ok := stepMeta[rec.ID]
					if ok {
						step.Record = *rec
					} else {
						step = &StepMeta{
							Record:    *rec,
							LogIndex:  -1,
							LogLength: 0,
							StepIndex: -1,
						}
						stepMeta[rec.ID] = step
						for i, s := range steps {
							if s.Id == rec.ID {
								step.StepIndex = int64(i)
								break
							}
						}
					}
					if step.StepIndex >= 0 {
						v := rec
						step := taskState.Steps[step.StepIndex]
						if v.Result != nil && step.Result == runnerv1.Result_RESULT_UNSPECIFIED {
							switch strings.ToLower(*v.Result) {
							case "succeeded":
								step.Result = runnerv1.Result_RESULT_SUCCESS
							case "skipped":
								step.Result = runnerv1.Result_RESULT_SKIPPED
							default:
								step.Result = runnerv1.Result_RESULT_FAILURE
							}
						}
						if step.StartedAt == nil && v.StartTime != "" {
							t, _ := time.Parse("2006-01-02T15:04:05.0000000Z", v.StartTime)
							step.StartedAt = timestamppb.New(t)
						}
						if step.StoppedAt == nil && v.FinishTime != nil {
							t, _ := time.Parse("2006-01-02T15:04:05.0000000Z", *v.FinishTime)
							step.StoppedAt = timestamppb.New(t)
						}
					}
				}
			} else if jevent, ok := obj.(*protocol.JobEvent); ok && jevent.Result != "" {
				switch strings.ToLower(jevent.Result) {
				case "succeeded":
					taskState.Result = runnerv1.Result_RESULT_SUCCESS
				case "skipped":
					taskState.Result = runnerv1.Result_RESULT_SKIPPED
				default:
					taskState.Result = runnerv1.Result_RESULT_FAILURE
				}
			}
		}
	}()

	httpServer := &http.Server{Addr: "0.0.0.0:3403", Handler: aserver}

	defer func() {
		httpServer.Shutdown(context.Background())
		t.client.UpdateLog(ctx, connect.NewRequest(&runnerv1.UpdateLogRequest{
			TaskId: task.GetId(),
			Index:  logline,
			Rows: []*runnerv1.LogRow{
				{
					Time:    timestamppb.New(time.Now()),
					Content: "Finished",
				},
			},
			NoMore: true,
		}))

		if taskState.Result == runnerv1.Result_RESULT_UNSPECIFIED {
			taskState.Result = runnerv1.Result_RESULT_FAILURE
		}
		taskState.StoppedAt = timestamppb.Now()
		t.client.UpdateTask(ctx, connect.NewRequest(&runnerv1.UpdateTaskRequest{
			State: taskState,
		}))
	}()

	go func() {
		httpServer.ListenAndServe()
	}()

	for _, s := range job.Steps {
		displayName := &protocol.TemplateToken{}
		displayName.FromRawObject(s.Name)
		rawIn := map[interface{}]interface{}{}
		var reference protocol.ActionStepDefinitionReference

		if s.Run != "" {
			reference = protocol.ActionStepDefinitionReference{
				Type: "script",
			}
			rawIn = map[interface{}]interface{}{
				"script": s.Run,
			}
			if s.Shell != "" {
				rawIn["shell"] = s.Shell
			}
			if s.WorkingDirectory != "" {
				rawIn["workingDirectory"] = s.WorkingDirectory
			}
		} else {
			uses := s.Uses
			if strings.HasPrefix(uses, "docker://") {
				reference = protocol.ActionStepDefinitionReference{
					Type:  "containerRegistry",
					Image: strings.TrimPrefix(uses, "docker://"),
				}
			} else {
				nameAndPathOrRef := strings.Split(uses, "@")
				nameAndPath := strings.Split(nameAndPathOrRef[0], "/")
				if nameAndPath[0] == "." {
					reference = protocol.ActionStepDefinitionReference{
						Type:           "repository",
						Path:           path.Join(nameAndPath[1:]...),
						RepositoryType: "self",
					}
				} else {
					reference = protocol.ActionStepDefinitionReference{
						Type:           "repository",
						Name:           path.Join(nameAndPath[0:2]...),
						Path:           path.Join(nameAndPath[2:]...),
						Ref:            nameAndPathOrRef[1],
						RepositoryType: "GitHub",
					}
				}
			}
			for k, v := range s.With {
				rawIn[k] = v
			}
		}

		var environment *protocol.TemplateToken
		if s.Env.Kind == yaml.ScalarNode {
			var expr string
			_ = s.Env.Decode(&expr)
			if expr != "" {
				environment = &protocol.TemplateToken{}
				environment.FromRawObject(expr)
			}
		} else if s.Env.Kind == yaml.MappingNode {
			rawEnv := map[interface{}]interface{}{}
			_ = s.Env.Decode(&rawEnv)
			environment = &protocol.TemplateToken{}
			environment.FromRawObject(rawEnv)
		}

		inputs := &protocol.TemplateToken{}
		inputs.FromRawObject(rawIn)
		condition := s.If.Value
		if condition == "" {
			condition = "success()"
		}
		steps = append(steps, protocol.ActionStep{
			Type:             "action",
			Reference:        reference,
			Inputs:           inputs,
			Condition:        condition,
			DisplayNameToken: displayName,
			ContextName:      s.ID,
			Id:               uuid.New().String(),
			Environment:      environment,
		})
	}

	jobServiceContainers := yaml.Node{}
	jobServiceContainers.Encode(job.Services)

	envs := []protocol.TemplateToken{}
	defs := []protocol.TemplateToken{}
	def := yaml.Node{}

	def.Encode(workflow.Defaults)
	if d := ToTemplateToken(def); d != nil {
		defs = append(defs, *d)
	}

	def.Encode(job.Defaults)
	if d := ToTemplateToken(def); d != nil {
		defs = append(defs, *d)
	}

	def.Encode(workflow.Env)
	if d := ToTemplateToken(def); d != nil && !def.IsZero() {
		envs = append(envs, *d)
	}

	if d := ToTemplateToken(job.Env); d != nil && !job.Env.IsZero() {
		envs = append(envs, *d)
	}

	jmessage := &protocol.AgentJobRequestMessage{
		MessageType: "jobRequest",
		Plan: &protocol.TaskOrchestrationPlanReference{
			ScopeIdentifier: uuid.New().String(),
			PlanID:          uuid.New().String(),
			PlanType:        "free",
			Version:         12,
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
			"github":   server.ToPipelineContextData(task.Context.AsMap()),
			"matrix":   server.ToPipelineContextData(map[string]interface{}{}),
			"strategy": server.ToPipelineContextData(map[string]interface{}{}),
			"inputs":   server.ToPipelineContextData(map[string]interface{}{}),
			"vars":     server.ToPipelineContextData(map[string]interface{}{}),
		},
		JobContainer:         ToTemplateToken(job.RawContainer),
		JobServiceContainers: ToTemplateToken(jobServiceContainers),
		Defaults:             defs,
		EnvironmentVariables: envs,
	}
	jmessage.Variables["DistributedTask.NewActionMetadata"] = protocol.VariableValue{Value: "true"}
	jmessage.Variables["DistributedTask.EnableCompositeActions"] = protocol.VariableValue{Value: "true"}
	jmessage.Variables["DistributedTask.EnhancedAnnotations"] = protocol.VariableValue{Value: "true"}
	for k, v := range task.Secrets {
		jmessage.Variables[k] = protocol.VariableValue{Value: v, IsSecret: true}
	}

	src, _ := json.Marshal(jmessage)
	jobExecCtx := ctx

	worker := exec.Command(runnerWorker[0], runnerWorker[1:]...)
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
