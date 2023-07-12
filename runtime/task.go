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
	"regexp"
	"strconv"
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
	"github.com/rhysd/actionlint"
	log "github.com/sirupsen/logrus"
)

var globalTaskMap sync.Map

type TaskInput struct {
	envs map[string]string
}

type Task struct {
	BuildID int64
	Input   *TaskInput

	client         client.Client
	platformPicker func([]string) string
}

// NewTask creates a new task
func NewTask(forgeInstance string, buildID int64, client client.Client, runnerEnvs map[string]string, picker func([]string) string) *Task {
	task := &Task{
		Input: &TaskInput{
			envs: runnerEnvs,
		},
		BuildID: buildID,

		client:         client,
		platformPicker: picker,
	}
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
			if number == 0 {
				return nil
			}
			val = number
		} else if err := node.Decode(&b); err == nil {
			// container.reuse causes an error
			if !b {
				return nil
			}
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

func escapeFormatString(in string) string {
	return strings.ReplaceAll(strings.ReplaceAll(in, "{", "{{"), "}", "}}")
}

func rewriteSubExpression(in string, forceFormat bool) (string, bool) {
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return in, false
	}

	strPattern := regexp.MustCompile("(?:''|[^'])*'")
	pos := 0
	exprStart := -1
	strStart := -1
	var results []string
	formatOut := ""
	for pos < len(in) {
		if strStart > -1 {
			matches := strPattern.FindStringIndex(in[pos:])
			if matches == nil {
				panic("unclosed string.")
			}

			strStart = -1
			pos += matches[1]
		} else if exprStart > -1 {
			exprEnd := strings.Index(in[pos:], "}}")
			strStart = strings.Index(in[pos:], "'")

			if exprEnd > -1 && strStart > -1 {
				if exprEnd < strStart {
					strStart = -1
				} else {
					exprEnd = -1
				}
			}

			if exprEnd > -1 {
				formatOut += fmt.Sprintf("{%d}", len(results))
				results = append(results, strings.TrimSpace(in[exprStart:pos+exprEnd]))
				pos += exprEnd + 2
				exprStart = -1
			} else if strStart > -1 {
				pos += strStart + 1
			} else {
				panic("unclosed expression.")
			}
		} else {
			exprStart = strings.Index(in[pos:], "${{")
			if exprStart != -1 {
				formatOut += escapeFormatString(in[pos : pos+exprStart])
				exprStart = pos + exprStart + 3
				pos = exprStart
			} else {
				formatOut += escapeFormatString(in[pos:])
				pos = len(in)
			}
		}
	}

	if len(results) == 1 && formatOut == "{0}" && !forceFormat {
		return results[0], true
	}

	out := fmt.Sprintf("format('%s', %s)", strings.ReplaceAll(formatOut, "'", "''"), strings.Join(results, ", "))
	return out, true
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
		TraceLog:         make(chan interface{}),
		ServerUrl:        dataContext["server_url"].GetStringValue(),
		ActionsServerUrl: dataContext["gitea_default_actions_url"].GetStringValue(),
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
	outputs := map[string]string{}
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
				}
				resp, err := t.client.UpdateTask(ctx, connect.NewRequest(&runnerv1.UpdateTaskRequest{
					State: taskState,
				}))
				// The cancel request message is hidden in the implementation depth of the act_runner
				if err == nil && resp.Msg.State != nil && resp.Msg.State.Result == runnerv1.Result_RESULT_CANCELLED {
					cancel()
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
						// Updated timestamp format to allow variable amount of fraction and time offset
						if step.StartedAt == nil && v.StartTime != "" {
							if t, err := time.Parse("2006-01-02T15:04:05.9999999Z07:00", v.StartTime); err == nil {
								step.StartedAt = timestamppb.New(t)
							}
						}
						if step.StoppedAt == nil && v.FinishTime != nil {
							if t, err := time.Parse("2006-01-02T15:04:05.9999999Z07:00", *v.FinishTime); err == nil {
								step.StoppedAt = timestamppb.New(t)
							}
						}
					}
				}
			} else if jevent, ok := obj.(*protocol.JobEvent); ok {
				if jevent.Result != "" {
					switch strings.ToLower(jevent.Result) {
					case "succeeded":
						taskState.Result = runnerv1.Result_RESULT_SUCCESS
					case "skipped":
						taskState.Result = runnerv1.Result_RESULT_SKIPPED
					default:
						taskState.Result = runnerv1.Result_RESULT_FAILURE
					}
				}
				if jevent.Outputs != nil {
					for k, v := range *jevent.Outputs {
						outputs[k] = v.Value
					}
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
			State:   taskState,
			Outputs: outputs,
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
				for _, proto := range []string{"http://", "https://"} {
					if strings.HasPrefix(nameAndPathOrRef[0], proto) {
						re := strings.Split(strings.TrimPrefix(nameAndPathOrRef[0], proto), "/")
						nameAndPath = append([]string{strings.ReplaceAll(proto+re[0]+"/"+re[1], ":", "~")}, re[2:]...)
						break
					}
				}
				if nameAndPath[0] == "." {
					reference = protocol.ActionStepDefinitionReference{
						Type:           "repository",
						Path:           path.Join(nameAndPath[1:]...),
						RepositoryType: "self",
					}
				} else {
					reference = protocol.ActionStepDefinitionReference{
						Type:           "repository",
						Name:           nameAndPath[0] + "/" + nameAndPath[1],
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
		} else {
			// Remove surrounded expression syntax
			if exprcond, ok := rewriteSubExpression(condition, false); ok {
				condition = exprcond
			}
			// Try to parse the expression and inject success if no status check function has been applied
			parser := actionlint.NewExprParser()
			exprNode, err := parser.Parse(actionlint.NewExprLexer(condition + "}}"))
			if err == nil {
				hasStatusCheckFunction := false
				actionlint.VisitExprNode(exprNode, func(node, _ actionlint.ExprNode, entering bool) {
					if funcCallNode, ok := node.(*actionlint.FuncCallNode); entering && ok {
						switch strings.ToLower(funcCallNode.Callee) {
						case "success", "always", "cancelled", "failure":
							hasStatusCheckFunction = true
						}
					}
				})
				if !hasStatusCheckFunction {
					condition = fmt.Sprintf("success() && (%s)", condition)
				}
			}
		}
		var timeoutInMinutes *protocol.TemplateToken
		if len(s.TimeoutMinutes) > 0 {
			timeoutInMinutes = &protocol.TemplateToken{}
			if timeout, err := strconv.ParseFloat(s.TimeoutMinutes, 64); err == nil {
				timeoutInMinutes.FromRawObject(timeout)
			} else {
				timeoutInMinutes.FromRawObject(s.TimeoutMinutes)
			}
		}
		var continueOnError *protocol.TemplateToken
		if len(s.RawContinueOnError) > 0 {
			continueOnError = &protocol.TemplateToken{}
			if continueOnErr, err := strconv.ParseBool(s.RawContinueOnError); err == nil {
				continueOnError.FromRawObject(continueOnErr)
			} else {
				continueOnError.FromRawObject(s.TimeoutMinutes)
			}
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
			TimeoutInMinutes: timeoutInMinutes,
			ContinueOnError:  continueOnError,
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

	matrix := map[string]interface{}{}
	for _, m := range job.GetMatrixes() {
		for k, v := range m {
			matrix[k] = v
		}
	}

	github := task.Context.AsMap()
	if api_url := dataContext["api_url"].GetStringValue(); api_url == "" {
		server_url := dataContext["server_url"].GetStringValue()
		if server_url != "" {
			if server_url[len(server_url)-1] != '/' {
				server_url += "/"
			}
			github["api_url"] = fmt.Sprintf("%sapi/v1", server_url)
		}
	}
	// Convert to raw map
	needsctx := map[string]interface{}{}
	if rawneeds := task.GetNeeds(); rawneeds != nil {
		for name, rawneed := range rawneeds {
			dep := map[string]interface{}{}
			switch rawneed.Result {
			case runnerv1.Result_RESULT_SUCCESS:
				dep["result"] = "success"
			case runnerv1.Result_RESULT_FAILURE:
				dep["result"] = "failure"
			case runnerv1.Result_RESULT_SKIPPED:
				dep["result"] = "skipped"
			case runnerv1.Result_RESULT_CANCELLED:
				dep["result"] = "cancelled"
			}
			dep["outputs"] = convertToRawMap(rawneed.Outputs)
			needsctx[name] = dep
		}
	}
	var jobOutputs *protocol.TemplateToken
	if len(job.Outputs) > 0 {
		jobOutputs = &protocol.TemplateToken{}
		jobOutputs.FromRawObject(convertToRawTemplateTokenMap(job.Outputs))
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
							"AccessToken": uuid.NewString(),
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
			"github":   server.ToPipelineContextData(github),
			"matrix":   server.ToPipelineContextData(matrix),
			"strategy": server.ToPipelineContextData(map[string]interface{}{}),
			"inputs":   server.ToPipelineContextData(map[string]interface{}{}),
			"needs":    server.ToPipelineContextData(needsctx),
			"vars":     server.ToPipelineContextData(convertToRawMap(task.GetVars())),
		},
		JobContainer:         ToTemplateToken(job.RawContainer),
		JobServiceContainers: ToTemplateToken(jobServiceContainers),
		Defaults:             defs,
		EnvironmentVariables: envs,
		JobOutputs:           jobOutputs,
	}
	jmessage.Variables["DistributedTask.NewActionMetadata"] = protocol.VariableValue{Value: "true"}
	jmessage.Variables["DistributedTask.EnableCompositeActions"] = protocol.VariableValue{Value: "true"}
	jmessage.Variables["DistributedTask.EnhancedAnnotations"] = protocol.VariableValue{Value: "true"}
	jmessage.Variables["DistributedTask.AddWarningToNode12Action"] = protocol.VariableValue{Value: "true"}
	jmessage.Variables["DistributedTask.AllowRunnerContainerHooks"] = protocol.VariableValue{Value: "true"}
	jmessage.Variables["DistributedTask.DeprecateStepOutputCommands"] = protocol.VariableValue{Value: "true"}
	jmessage.Variables["DistributedTask.ForceGithubJavascriptActionsToNode16"] = protocol.VariableValue{Value: "true"}
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

func convertToRawMap(data map[string]string) map[string]interface{} {
	outputs := map[string]interface{}{}
	for k, v := range data {
		outputs[k] = v
	}
	return outputs
}

func convertToRawTemplateTokenMap(data map[string]string) map[interface{}]interface{} {
	outputs := map[interface{}]interface{}{}
	for k, v := range data {
		outputs[k] = v
	}
	return outputs
}
