package runtime

import (
	"sync"
)

var globalTaskMap sync.Map

// finishTask removes the task from global map
func finishTask(buildID int64) {
	globalTaskMap.Delete(buildID)
}
