package tasks

import (
	"PluginRPC/types"
	"log"
	"time"
)

func BearerRefresherTask(appState *types.AppState) func() {
	return func() {
		taskDurationTracker := time.Now()
		defer log.Printf("Finished scheduled task: BearerRefresherTask in %s\n", time.Since(taskDurationTracker))
	}
}
