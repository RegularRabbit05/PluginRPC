package tasks

import (
	"PluginRPC/types"
	"sync"
	"sync/atomic"
	"time"

	"log"

	"github.com/RegularRabbit05/go-parallel/parallel"
)

func ActivityRefresherTask(appState *types.AppState) func() {
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
		totalCount := len(userCloneList)
		var discardCount atomic.Uint32
		defer log.Printf("Finished scheduled task: ActivityRefresherTask in %s with total: %d and discarded: %d\n", time.Since(taskDurationTracker), totalCount, discardCount.Load())
		var userFinalList sync.Map

		keys := make([]string, 0, totalCount)
		for k := range userCloneList {
			keys = append(keys, k)
		}

		refresherWorker := func(idx int, _ int) {
			key := keys[idx]
			user := userCloneList[key]
			if !user.SyncActivity(appState.Config) {
				log.Printf("Unable to update user an activity for: %s", user.UserID)
				discardCount.Add(1)
				return
			}
			userFinalList.Store(key, user)
		}
		parallel.For(len(keys), refresherWorker)

		appState.Mutex.Lock()
		defer appState.Mutex.Unlock()
		appState.UserStore = make(map[string]*types.User, len(keys))
		userFinalList.Range(func(k, v any) bool {
			key := k.(string)
			user := v.(*types.User)
			appState.UserStore[key] = user
			return true
		})
	}
}
