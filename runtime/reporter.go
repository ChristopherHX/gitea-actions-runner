package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gitea.com/gitea/act_runner/client"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/avast/retry-go/v4"
	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Reporter struct {
	ctx    context.Context
	cancel context.CancelFunc

	closed  bool
	client  client.Client
	clientM sync.Mutex

	logOffset int
	logRows   []*runnerv1.LogRow
	state     *runnerv1.TaskState
	stateM    sync.RWMutex
}

func NewReporter(ctx context.Context, cancel context.CancelFunc, client client.Client, taskID int64) *Reporter {
	return &Reporter{
		ctx:    ctx,
		cancel: cancel,
		client: client,
		state: &runnerv1.TaskState{
			Id: taskID,
		},
	}
}

func (r *Reporter) ResetSteps(l int) {
	r.stateM.Lock()
	defer r.stateM.Unlock()
	for i := 0; i < l; i++ {
		r.state.Steps = append(r.state.Steps, &runnerv1.StepState{
			Id: int64(i),
		})
	}
}

func (r *Reporter) Levels() []log.Level {
	return log.AllLevels
}

func (r *Reporter) Fire(entry *log.Entry) error {
	r.stateM.Lock()
	defer r.stateM.Unlock()

	timestamp := entry.Time
	if r.state.StartedAt == nil {
		r.state.StartedAt = timestamppb.New(timestamp)
	}

	var step *runnerv1.StepState
	if v, ok := entry.Data["stepNumber"]; ok {
		if v, ok := v.(int); ok {
			step = r.state.Steps[v]
		}
	}

	if step == nil {
		if v, ok := entry.Data["jobResult"]; ok {
			if jobResult, ok := r.parseResult(v); ok {
				r.state.Result = jobResult
				r.state.StoppedAt = timestamppb.New(timestamp)
				for _, s := range r.state.Steps {
					if s.Result == runnerv1.Result_RESULT_UNSPECIFIED {
						s.Result = runnerv1.Result_RESULT_CANCELLED
					}
				}
			}
		}
		if !r.duringSteps() {
			r.logRows = append(r.logRows, r.parseLogRow(entry))
		}
		return nil
	}

	if step.StartedAt == nil {
		step.StartedAt = timestamppb.New(timestamp)
	}

	if v, ok := entry.Data["raw_output"]; ok {
		if rawOutput, ok := v.(bool); ok && rawOutput {
			if step.LogLength == 0 {
				step.LogIndex = int64(r.logOffset + len(r.logRows))
			}
			step.LogLength++
			r.logRows = append(r.logRows, r.parseLogRow(entry))
			return nil
		}
	}

	if v, ok := entry.Data["stepResult"]; ok {
		if stepResult, ok := r.parseResult(v); ok {
			if step.LogLength == 0 {
				step.LogIndex = int64(r.logOffset + len(r.logRows))
			}
			step.Result = stepResult
			step.StoppedAt = timestamppb.New(timestamp)
		}
	}

	return nil
}

func (r *Reporter) RunDaemon() {
	if r.closed {
		return
	}
	if r.ctx.Err() != nil {
		return
	}

	_ = r.ReportLog(false)
	_ = r.ReportState()

	time.AfterFunc(time.Second, r.RunDaemon)
}

func (r *Reporter) Logf(format string, a ...interface{}) {
	r.stateM.Lock()
	defer r.stateM.Unlock()

	if !r.duringSteps() {
		r.logRows = append(r.logRows, &runnerv1.LogRow{
			Time:    timestamppb.Now(),
			Content: fmt.Sprintf(format, a...),
		})
	}
}

func (r *Reporter) Close(lastWords string) error {
	r.closed = true

	r.stateM.Lock()
	if r.state.Result == runnerv1.Result_RESULT_UNSPECIFIED {
		if lastWords == "" {
			lastWords = "Early termination"
		}
		for _, v := range r.state.Steps {
			if v.Result == runnerv1.Result_RESULT_UNSPECIFIED {
				v.Result = runnerv1.Result_RESULT_CANCELLED
			}
		}
		r.logRows = append(r.logRows, &runnerv1.LogRow{
			Time:    timestamppb.Now(),
			Content: lastWords,
		})
		return nil
	} else if lastWords != "" {
		r.logRows = append(r.logRows, &runnerv1.LogRow{
			Time:    timestamppb.Now(),
			Content: lastWords,
		})
	}
	r.stateM.Unlock()

	return retry.Do(func() error {
		if err := r.ReportLog(true); err != nil {
			return err
		}
		return r.ReportState()
	}, retry.Context(r.ctx))
}

func (r *Reporter) ReportLog(noMore bool) error {
	r.clientM.Lock()
	defer r.clientM.Unlock()

	r.stateM.RLock()
	rows := r.logRows
	r.stateM.RUnlock()

	resp, err := r.client.UpdateLog(r.ctx, connect.NewRequest(&runnerv1.UpdateLogRequest{
		TaskId: r.state.Id,
		Index:  int64(r.logOffset),
		Rows:   rows,
		NoMore: noMore,
	}))
	if err != nil {
		return err
	}

	ack := int(resp.Msg.AckIndex)
	if ack < r.logOffset {
		return fmt.Errorf("submitted logs are lost")
	}

	r.stateM.Lock()
	r.logRows = r.logRows[ack-r.logOffset:]
	r.logOffset = ack
	r.stateM.Unlock()

	if noMore && ack < r.logOffset+len(rows) {
		return fmt.Errorf("not all logs are submitted")
	}

	return nil
}

func (r *Reporter) ReportState() error {
	r.clientM.Lock()
	defer r.clientM.Unlock()

	r.stateM.RLock()
	state := proto.Clone(r.state).(*runnerv1.TaskState)
	r.stateM.RUnlock()

	resp, err := r.client.UpdateTask(r.ctx, connect.NewRequest(&runnerv1.UpdateTaskRequest{
		State: state,
	}))

	if resp.Msg.State.Result == runnerv1.Result_RESULT_CANCELLED {
		r.cancel()
	}

	return err
}

func (r *Reporter) duringSteps() bool {
	if steps := r.state.Steps; len(steps) == 0 {
		return false
	} else if first := steps[0]; first.Result == runnerv1.Result_RESULT_UNSPECIFIED && first.LogLength == 0 {
		return false
	} else if last := steps[len(steps)-1]; last.Result != runnerv1.Result_RESULT_UNSPECIFIED {
		return false
	}
	return true
}

var stringToResult = map[string]runnerv1.Result{
	"success":   runnerv1.Result_RESULT_SUCCESS,
	"failure":   runnerv1.Result_RESULT_FAILURE,
	"skipped":   runnerv1.Result_RESULT_SKIPPED,
	"cancelled": runnerv1.Result_RESULT_CANCELLED,
}

func (r *Reporter) parseResult(result interface{}) (runnerv1.Result, bool) {
	str := ""
	if v, ok := result.(string); ok { // for jobResult
		str = v
	} else if v, ok := result.(fmt.Stringer); ok { // for stepResult
		str = v.String()
	}

	ret, ok := stringToResult[str]
	return ret, ok
}

func (r *Reporter) parseLogRow(entry *log.Entry) *runnerv1.LogRow {
	return &runnerv1.LogRow{
		Time:    timestamppb.New(entry.Time),
		Content: strings.TrimSuffix(entry.Message, "\r\n"),
	}
}
