package tasks

import (
	"PluginRPC/types"
	"log"
	"sync/atomic"
	"time"

	"github.com/RegularRabbit05/go-parallel/parallel"
)

func DatabaseSaverTask(appState *types.AppState) func() {
	userStorageCloner := func() map[string]*types.User {
		appState.Mutex.RLock()
		defer appState.Mutex.RUnlock()

		userStorageCopy := make(map[string]*types.User, len(appState.UserStore))
		for k, v := range appState.UserStore {
			userStorageCopy[k] = v
		}

		return userStorageCopy
	}

	return func() {
		taskDurationTracker := time.Now()
		userCloneList := userStorageCloner()
		var failCount atomic.Uint32
		var totalCount = len(userCloneList)
		defer log.Printf("Finished scheduled task: DatabaseSaverTask in %s with total: %d and failed: %d\n", time.Since(taskDurationTracker), totalCount, failCount.Load())
		appState.Mutex.RLock()
		defer appState.Mutex.RUnlock()

		keys := make([]string, 0, totalCount)
		for k := range userCloneList {
			keys = append(keys, k)
		}

		saveWorker := func(idx int, _ int) {
			key := keys[idx]
			user := userCloneList[key]
			func() {
				user.StructMutex.Lock()
				defer user.StructMutex.Unlock()
				if user.SaveUserStorage(key, appState, false) != nil {
					failCount.Add(1)
				}
			}()
		}
		parallel.For(len(keys), saveWorker)
	}
}
