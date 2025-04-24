package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ChristopherHX/github-act-runner/protocol"
)

type ActionsServer struct {
	TraceLog         chan interface{}
	ServerUrl        string
	ActionsServerUrl string
	AuthData         map[string]*protocol.ActionDownloadAuthentication
	JobRequest       *protocol.AgentJobRequestMessage
	CancelCtx        context.Context
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
		logPath := "/_apis/v1/Logfiles/"
		if strings.HasPrefix(req.URL.Path, logPath) && len(logPath) < len(req.URL.Path) {
			io.Copy(io.Discard, req.Body)
			resp.WriteHeader(200)
		} else {
			p := "logs\\0.log"
			recs := &protocol.TaskLog{
				TaskLogReference: protocol.TaskLogReference{
					ID: 1,
				},
				CreatedOn:     "2022-01-01T00:00:00",
				LastChangedOn: "2022-01-01T00:00:00",
				Path:          &p,
			}
			jsonRequest(recs)
			jsonResponse(recs)
		}
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/ActionDownloadInfo") {
		references := &protocol.ActionReferenceList{}
		jsonRequest(references)
		actions := map[string]protocol.ActionDownloadInfo{}
		for _, ref := range references.Actions {
			resolved := protocol.ActionDownloadInfo{
				NameWithOwner:         ref.NameWithOwner,
				ResolvedNameWithOwner: ref.NameWithOwner,
				Ref:                   ref.Ref,
				ResolvedSha:           "N/A",
			}
			noAuth := false
			absolute := false
			for _, proto := range []string{"http~//", "https~//"} {
				if strings.HasPrefix(ref.NameWithOwner, proto) {
					absolute = true
					noAuth = true
					originalNameOwner := strings.ReplaceAll(ref.NameWithOwner, "~", ":")
					if authData, ok := server.AuthData[originalNameOwner]; ok {
						resolved.Authentication = authData
						noAuth = false
					}
					pURL, _ := url.Parse(originalNameOwner)
					p := pURL.Path
					pURL.Path = ""
					host := pURL.String()
					if host == "https://github.com" || noAuth {
						resolved.TarballUrl = fmt.Sprintf("%s/archive/%s.tar.gz", originalNameOwner, ref.Ref)
						resolved.ZipballUrl = fmt.Sprintf("%s/archive/%s.zip", originalNameOwner, ref.Ref)
					} else {
						// Gitea does not support auth on the web route
						resolved.TarballUrl = fmt.Sprintf("%s/api/v1/repos/%s/archive/%s.tar.gz", host, p, ref.Ref)
						resolved.ZipballUrl = fmt.Sprintf("%s/api/v1/repos/%s/archive/%s.zip", host, p, ref.Ref)
					}
					break
				}
			}
			if !absolute {
				var urls []string
				if server.ServerUrl != server.ActionsServerUrl {
					urls = []string{server.ServerUrl, server.ActionsServerUrl}
				} else {
					urls = []string{server.ActionsServerUrl}
				}
				for _, url := range urls {
					noAuth = url != server.ServerUrl
					if noAuth {
						resolved.TarballUrl = fmt.Sprintf("%s/%s/archive/%s.tar.gz", strings.TrimRight(url, "/"), ref.NameWithOwner, ref.Ref)
						resolved.ZipballUrl = fmt.Sprintf("%s/%s/archive/%s.zip", strings.TrimRight(url, "/"), ref.NameWithOwner, ref.Ref)
					} else {
						resolved.TarballUrl = fmt.Sprintf("%s/api/v1/repos/%s/archive/%s.tar.gz", strings.TrimRight(url, "/"), ref.NameWithOwner, ref.Ref)
						resolved.ZipballUrl = fmt.Sprintf("%s/api/v1/repos/%s/archive/%s.zip", strings.TrimRight(url, "/"), ref.NameWithOwner, ref.Ref)
					}
					if resp, err := http.Head(resolved.TarballUrl); err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
						break
					}
				}
			}
			if noAuth {
				// Using a dummy token has worked in 2022, but now it's broken
				// resolved.Authentication = &protocol.ActionDownloadAuthentication{
				// 	ExpiresAt: "0001-01-01T00:00:00",
				// 	Token:     "dummy-token",
				// }
				dst := *req.URL
				dst.Scheme = "http"
				dst.Host = req.Host
				dst.Path = "/_apis/v1/ActionDownload"
				q := dst.Query()
				q.Set("url", resolved.TarballUrl)
				dst.RawQuery = q.Encode()
				resolved.TarballUrl = dst.String()
				q.Set("url", resolved.ZipballUrl)
				dst.RawQuery = q.Encode()
				resolved.ZipballUrl = dst.String()
			}
			actions[fmt.Sprintf("%s@%s", ref.NameWithOwner, ref.Ref)] = resolved
		}
		jsonResponse(&protocol.ActionDownloadInfoCollection{
			Actions: actions,
		})
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/ActionDownload") {
		req, err := http.NewRequestWithContext(req.Context(), "GET", req.URL.Query().Get("url"), nil)
		if err != nil {
			resp.WriteHeader(404)
			return
		}
		req.Header.Add("User-Agent", "github-act-runner/1.0.0")
		req.Header.Add("Accept", "*/*")
		rsp, err := http.DefaultClient.Do(req)
		if err != nil {
			resp.WriteHeader(404)
			return
		}
		defer rsp.Body.Close()
		resp.WriteHeader(rsp.StatusCode)
		io.Copy(resp, rsp.Body)
	} else if strings.HasPrefix(req.URL.Path, "/_apis/pipelines/workflows/") {
		surl, _ := url.Parse(server.ServerUrl)
		url := *req.URL
		url.Scheme = surl.Scheme
		url.Host = surl.Host
		url.Path = "/api/actions_pipeline" + url.Path
		defer req.Body.Close()
		myreq, err := http.NewRequestWithContext(req.Context(), req.Method, url.String(), req.Body)
		if err != nil {
			resp.WriteHeader(404)
			return
		}
		for k, vs := range req.Header {
			myreq.Header[k] = vs
		}
		rsp, err := http.DefaultClient.Do(myreq)
		if err != nil {
			resp.WriteHeader(404)
			return
		}
		defer rsp.Body.Close()

		for k, vs := range rsp.Header {
			resp.Header()[k] = vs
		}
		resp.WriteHeader(rsp.StatusCode)
		io.Copy(resp, rsp.Body)
	} else if strings.HasPrefix(req.URL.Path, "/_apis/v1/ActionDownload") {
		resp.WriteHeader(404)
	} else if strings.HasPrefix(req.URL.Path, "/JobRequest") {
		SYSTEMVSSCONNECTION := req.URL.Query().Get("SYSTEMVSSCONNECTION")
		if SYSTEMVSSCONNECTION != "" {
			for i, endpoint := range server.JobRequest.Resources.Endpoints {
				if endpoint.Name == "SYSTEMVSSCONNECTION" {
					server.JobRequest.Resources.Endpoints[i].URL = SYSTEMVSSCONNECTION
					break
				}
			}
		}
		resp.WriteHeader(200)
		resp.Header().Add("content-type", "application/json")
		resp.Header().Add("accept", "application/json")
		src, _ := json.Marshal(server.JobRequest)
		resp.Write(src)
	} else if strings.HasPrefix(req.URL.Path, "/WaitForCancellation") {
		resp.Header().Add("content-type", "application/json")
		resp.Header().Add("accept", "application/json")
		resp.WriteHeader(200)
		resp.(http.Flusher).Flush()
		for {
			select {
			case <-server.CancelCtx.Done():
				resp.Write([]byte("cancelled\n\n"))
				resp.(http.Flusher).Flush()
				return
			case <-req.Context().Done():
				resp.Write([]byte("stopped\n\n"))
				resp.(http.Flusher).Flush()
				return
			case <-time.After(10 * time.Second):
				resp.Write([]byte("ping\n\n"))
				resp.(http.Flusher).Flush()
			}
		}
	} else {
		resp.WriteHeader(404)
	}
}
