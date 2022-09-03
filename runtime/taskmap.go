package runtime

import (
	"context"
	"fmt"
	"sync"
)

var globalTaskMap sync.Map

// startTask adds the task to global map
func startTask(buildID int64, ctx context.Context) error {
	_, exist := globalTaskMap.Load(buildID)
	if exist {
		return fmt.Errorf("task %d already exists", buildID)
	}

	task := NewTask(buildID)

	// set task ve to global map
	// when task is done or canceled, it will be removed from the map
	globalTaskMap.Store(buildID, task)

	go task.Run(ctx)

	return nil
}

// finishTask removes the task from global map
func finishTask(buildID int64) {
	globalTaskMap.Delete(buildID)
}
