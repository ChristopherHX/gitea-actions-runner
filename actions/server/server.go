package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"gitea.com/gitea/act_runner/client"
	"github.com/ChristopherHX/github-act-runner/protocol"
	"github.com/bufbuild/connect-go"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ActionsServer struct {
	Client client.Client
	Task   *runnerv1.Task
}

func ToPipelineContextDataWithError(data interface{}) (protocol.PipelineContextData, error) {
	if b, ok := data.(bool); ok {
		var typ int32 = 3
		return protocol.PipelineContextData{
			Type:      &typ,
			BoolValue: &b,
		}, nil
	} else if n, ok := data.(float64); ok {
		var typ int32 = 4
		return protocol.PipelineContextData{
			Type:        &typ,
			NumberValue: &n,
		}, nil
	} else if s, ok := data.(string); ok {
		var typ int32
		return protocol.PipelineContextData{
			Type:        &typ,
			StringValue: &s,
		}, nil
	} else if a, ok := data.([]interface{}); ok {
		arr := []protocol.PipelineContextData{}
		for _, v := range a {
			e, err := ToPipelineContextDataWithError(v)
			if err != nil {
				return protocol.PipelineContextData{}, err
			}
			arr = append(arr, e)
		}
		var typ int32 = 1
		return protocol.PipelineContextData{
			Type:       &typ,
			ArrayValue: &arr,
		}, nil
	} else if o, ok := data.(map[string]interface{}); ok {
		obj := []protocol.DictionaryContextDataPair{}
		for k, v := range o {
			e, err := ToPipelineContextDataWithError(v)
			if err != nil {
				return protocol.PipelineContextData{}, err
			}
			obj = append(obj, protocol.DictionaryContextDataPair{Key: k, Value: e})
		}
		var typ int32 = 2
		return protocol.PipelineContextData{
			Type:            &typ,
			DictionaryValue: &obj,
		}, nil
	}
	return protocol.PipelineContextData{}, fmt.Errorf("unknown type")
}

func ToPipelineContextData(data interface{}) protocol.PipelineContextData {
	ret, _ := ToPipelineContextDataWithError(data)
	return ret
}

func (server *ActionsServer) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	jsonRequest := func(data interface{}) {
		dec := json.NewDecoder(req.Body)
		_ = dec.Decode(data)
	}
	jsonResponse := func(data interface{}) {
		resp.Header().Add("content-type", "application/json")
		resp.WriteHeader(200)
		json, _ := json.Marshal(data)
		resp.Write(json)
	}
	if strings.HasPrefix(req.URL.Path, "/_apis/connectionData") {
		data := &protocol.ConnectionData{
			LocationServiceData: protocol.LocationServiceData{
				ServiceDefinitions: []protocol.ServiceDefinition{
					// {ServiceType: "AgentRequest", DisplayName: "AgentRequest", Description: "AgentRequest", Identifier: "fc825784-c92a-4299-9221-998a02d1b54f", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/AgentRequest/{poolId}/{requestId}", MinVersion: "1.0", MaxVersion: "12.0"},
					{ServiceType: "ActionDownloadInfo", DisplayName: "ActionDownloadInfo", Description: "ActionDownloadInfo", Identifier: "27d7f831-88c1-4719-8ca1-6a061dad90eb", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/ActionDownloadInfo/{scopeIdentifier}/{hubName}/{planId}", MinVersion: "1.0", MaxVersion: "12.0"},
					{ServiceType: "TimeLineWebConsoleLog", DisplayName: "TimeLineWebConsoleLog", Description: "TimeLineWebConsoleLog", Identifier: "858983e4-19bd-4c5e-864c-507b59b58b12", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/TimeLineWebConsoleLog/{scopeIdentifier}/{hubName}/{planId}/{timelineId}/{recordId}", MinVersion: "1.0", MaxVersion: "12.0"},
					{ServiceType: "TimelineRecords", DisplayName: "TimelineRecords", Description: "TimelineRecords", Identifier: "8893bc5b-35b2-4be7-83cb-99e683551db4", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/Timeline/{scopeIdentifier}/{hubName}/{planId}/{timelineId}", MinVersion: "1.0", MaxVersion: "12.0"},
					{ServiceType: "Logfiles", DisplayName: "Logfiles", Description: "Logfiles", Identifier: "46f5667d-263a-4684-91b1-dff7fdcf64e2", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/Logfiles/{scopeIdentifier}/{hubName}/{planId}/{logId}", MinVersion: "1.0", MaxVersion: "12.0"},
					{ServiceType: "FinishJob", DisplayName: "FinishJob", Description: "FinishJob", Identifier: "557624af-b29e-4c20-8ab0-0399d2204f3f", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/FinishJob/{scopeIdentifier}/{hubName}/{planId}", MinVersion: "1.0", MaxVersion: "12.0"},
				},
			},
		}
		jsonResponse(data)
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/Timeline/") {
		recs := &protocol.TimelineRecordWrapper{}
		jsonRequest(recs)

		steps := []*runnerv1.StepState{}
		for _, v := range recs.Value {
			step := &runnerv1.StepState{
				Id: int64(v.Order),
			}
			if v.Result != nil {
				switch strings.ToLower(*v.Result) {
				case "succeeded":
					step.Result = runnerv1.Result_RESULT_SUCCESS
				case "skipped":
					step.Result = runnerv1.Result_RESULT_SKIPPED
				default:
					step.Result = runnerv1.Result_RESULT_FAILURE
				}
			}
			if v.StartTime != "" {
				t, _ := time.Parse("2006-01-02T15:04:05", v.StartTime)
				step.StartedAt = timestamppb.New(t)
			}
			if v.FinishTime != nil {
				t, _ := time.Parse("2006-01-02T15:04:05", *v.FinishTime)
				step.StoppedAt = timestamppb.New(t)
			}
			steps = append(steps, step)
		}
		server.Client.UpdateTask(req.Context(), connect.NewRequest(&runnerv1.UpdateTaskRequest{
			State: &runnerv1.TaskState{
				Id:    server.Task.GetId(),
				Steps: steps,
			},
		}))
		jsonResponse(recs)
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/FinishJob/") {
		recs := &protocol.JobEvent{}
		jsonRequest(recs)
		resp.WriteHeader(200)
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/TimeLineWebConsoleLog/") {
		now := timestamppb.New(time.Now())
		recs := &protocol.TimelineRecordFeedLinesWrapper{}
		jsonRequest(recs)

		rows := []*runnerv1.LogRow{}
		for _, row := range recs.Value {
			rows = append(rows, &runnerv1.LogRow{
				Time:    now,
				Content: row,
			})
		}
		server.Client.UpdateLog(req.Context(), connect.NewRequest(&runnerv1.UpdateLogRequest{
			TaskId: server.Task.GetId(),
			Index:  *recs.StartLine,
			Rows:   rows,
		}))
		resp.WriteHeader(200)
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/Logfiles/") {
		if strings.Count(req.URL.Path, "/") == 7 {
			req.Body.Close()
			resp.WriteHeader(200)
		} else {
			recs := &protocol.TaskLog{}
			jsonRequest(recs)
			jsonResponse(recs)
		}
	} else {
		// resp.WriteHeader(404)
		inputs := &protocol.TemplateToken{}
		inputs.FromRawObject(map[interface{}]interface{}{
			"script": "echo 'Hello World'",
		})
		jsonResponse(&protocol.AgentJobRequestMessage{
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
			Steps: []protocol.ActionStep{
				{
					Type: "action",
					Reference: protocol.ActionStepDefinitionReference{
						Type: "script",
					},
					Inputs:      inputs,
					Condition:   "success()",
					ContextName: "__initial",
				},
			},
			Variables: map[string]protocol.VariableValue{},
			ContextData: map[string]protocol.PipelineContextData{
				"github": ToPipelineContextData(map[string]interface{}{
					"test": "val",
				}),
			},
		})
	}
}
