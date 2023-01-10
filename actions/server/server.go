package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ChristopherHX/github-act-runner/protocol"
)

type ActionsServer struct {
	TraceLog         chan interface{}
	ServerUrl        string
	ActionsServerUrl string
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
	if data == nil {
		return protocol.PipelineContextData{}, nil
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
		server.TraceLog <- data
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
					{ServiceType: "ActionDownloadInfo", DisplayName: "ActionDownloadInfo", Description: "ActionDownloadInfo", Identifier: "27d7f831-88c1-4719-8ca1-6a061dad90eb", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/ActionDownloadInfo", MinVersion: "1.0", MaxVersion: "12.0"},
					{ServiceType: "TimeLineWebConsoleLog", DisplayName: "TimeLineWebConsoleLog", Description: "TimeLineWebConsoleLog", Identifier: "858983e4-19bd-4c5e-864c-507b59b58b12", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/TimeLineWebConsoleLog/{timelineId}/{recordId}", MinVersion: "1.0", MaxVersion: "12.0"},
					{ServiceType: "TimelineRecords", DisplayName: "TimelineRecords", Description: "TimelineRecords", Identifier: "8893bc5b-35b2-4be7-83cb-99e683551db4", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/Timeline/{timelineId}", MinVersion: "1.0", MaxVersion: "12.0"},
					{ServiceType: "Logfiles", DisplayName: "Logfiles", Description: "Logfiles", Identifier: "46f5667d-263a-4684-91b1-dff7fdcf64e2", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/Logfiles/{logId}", MinVersion: "1.0", MaxVersion: "12.0"},
					{ServiceType: "FinishJob", DisplayName: "FinishJob", Description: "FinishJob", Identifier: "557624af-b29e-4c20-8ab0-0399d2204f3f", ResourceVersion: 6, RelativeToSetting: "fullyQualified", ServiceOwner: "f55bccde-c830-4f78-9a68-5c0a07deae97", RelativePath: "/_apis/v1/FinishJob", MinVersion: "1.0", MaxVersion: "12.0"},
				},
			},
		}
		jsonResponse(data)
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/Timeline/") {
		recs := &protocol.TimelineRecordWrapper{}
		jsonRequest(recs)
		jsonResponse(recs)
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/FinishJob") {
		recs := &protocol.JobEvent{}
		jsonRequest(recs)
		resp.WriteHeader(200)
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/TimeLineWebConsoleLog/") {
		recs := &protocol.TimelineRecordFeedLinesWrapper{}
		jsonRequest(recs)
		resp.WriteHeader(200)
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/Logfiles") {
		if strings.Count(req.URL.Path, "/") == 7 {
			req.Body.Close()
			resp.WriteHeader(200)
		} else {
			recs := &protocol.TaskLog{}
			jsonRequest(recs)
			jsonResponse(recs)
		}
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/ActionDownloadInfo") {
		references := &protocol.ActionReferenceList{}
		jsonRequest(references)
		actions := map[string]protocol.ActionDownloadInfo{}
		for _, ref := range references.Actions {
			var auth *protocol.ActionDownloadAuthentication
			if server.ActionsServerUrl != server.ServerUrl {
				auth = &protocol.ActionDownloadAuthentication{
					ExpiresAt: "0001-01-01T00:00:00",
					Token:     "dummy-token",
				}
			}
			actions[fmt.Sprintf("%s@%s", ref.NameWithOwner, ref.Ref)] = protocol.ActionDownloadInfo{
				NameWithOwner:         ref.NameWithOwner,
				ResolvedNameWithOwner: ref.NameWithOwner,
				TarballUrl:            fmt.Sprintf("%s/%s/archive/%s.tar.gz", server.ActionsServerUrl, ref.NameWithOwner, ref.Ref),
				ZipballUrl:            fmt.Sprintf("%s/%s/archive/%s.zip", server.ActionsServerUrl, ref.NameWithOwner, ref.Ref),
				Ref:                   ref.Ref,
				ResolvedSha:           "N/A",
				Authentication:        auth,
			}
		}
		jsonResponse(&protocol.ActionDownloadInfoCollection{
			Actions: actions,
		})
	} else {
		resp.WriteHeader(404)
	}
}
