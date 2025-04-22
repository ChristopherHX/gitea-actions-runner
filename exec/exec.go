package exec

import (
	"context"
	"fmt"

	pingv1 "code.gitea.io/actions-proto-go/ping/v1"
	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"connectrpc.com/connect"
	"github.com/ChristopherHX/gitea-actions-runner/runtime"
	structpb "google.golang.org/protobuf/types/known/structpb"
	"gopkg.in/yaml.v3"
)

type mockClient struct{}

// Address implements client.Client.
func (m *mockClient) Address() string {
	return "http://localhost:8080"
}

// Declare implements client.Client.
func (m *mockClient) Declare(context.Context, *connect.Request[runnerv1.DeclareRequest]) (*connect.Response[runnerv1.DeclareResponse], error) {
	return connect.NewResponse(&runnerv1.DeclareResponse{
		Runner: &runnerv1.Runner{
			Id: 1,
		},
	}), nil
}

// FetchTask implements client.Client.
func (m *mockClient) FetchTask(context.Context, *connect.Request[runnerv1.FetchTaskRequest]) (*connect.Response[runnerv1.FetchTaskResponse], error) {
	return connect.NewResponse(&runnerv1.FetchTaskResponse{
		Task: &runnerv1.Task{
			Id: 1,
		},
	}), nil
}

// Ping implements client.Client.
func (m *mockClient) Ping(_ context.Context, req *connect.Request[pingv1.PingRequest]) (*connect.Response[pingv1.PingResponse], error) {
	return connect.NewResponse(&pingv1.PingResponse{
		Data: req.Msg.Data,
	}), nil
}

// Register implements client.Client.
func (m *mockClient) Register(context.Context, *connect.Request[runnerv1.RegisterRequest]) (*connect.Response[runnerv1.RegisterResponse], error) {
	return connect.NewResponse(&runnerv1.RegisterResponse{
		Runner: &runnerv1.Runner{
			Id: 1,
		},
	}), nil
}

// UpdateLog implements client.Client.
func (m *mockClient) UpdateLog(_ context.Context, req *connect.Request[runnerv1.UpdateLogRequest]) (*connect.Response[runnerv1.UpdateLogResponse], error) {
	for _, row := range req.Msg.Rows {
		fmt.Println(row.Content)
	}
	return connect.NewResponse(&runnerv1.UpdateLogResponse{
		AckIndex: req.Msg.Index + int64(len(req.Msg.Rows)),
	}), nil
}

// UpdateTask implements client.Client.
func (m *mockClient) UpdateTask(_ context.Context, req *connect.Request[runnerv1.UpdateTaskRequest]) (*connect.Response[runnerv1.UpdateTaskResponse], error) {
	if req.Msg.State.Result != runnerv1.Result_RESULT_UNSPECIFIED {
		fmt.Println("Task completed with result:", req.Msg.State.Result)
	}
	return connect.NewResponse(&runnerv1.UpdateTaskResponse{
		State: req.Msg.State,
	}), nil
}

func Exec(ctx context.Context, content, contextData, varsData, secretsData string, args []string) error {
	mapData := make(map[string]any)
	yaml.Unmarshal([]byte(contextData), &mapData)
	if len(mapData) == 0 {
		mapData = map[string]any{}
	}
	if mapData["gitea_runtime_token"] == nil {
		mapData["gitea_runtime_token"] = "1234567890abcdef"
	}
	if mapData["repository"] == nil {
		mapData["repository"] = "test/test"
	}
	secrets := make(map[string]string)
	vars := make(map[string]string)
	yaml.Unmarshal([]byte(secretsData), &secrets)
	yaml.Unmarshal([]byte(varsData), &vars)

	pContext, _ := structpb.NewStruct(mapData)
	task := runtime.NewTask("gitea", 0, &mockClient{}, nil, nil)
	return task.Run(ctx, &runnerv1.Task{
		Id:              1,
		WorkflowPayload: []byte(content),
		Context:         pContext,
		Secrets:         secrets,
		Vars:            vars,
	}, args)
}
