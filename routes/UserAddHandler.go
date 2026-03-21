package routes

import (
	"PluginRPC/types"
	"PluginRPC/utils"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func InstallUserAddHandler(at string, appState *types.AppState) {
	findExisting := func(Id string) (*types.User, string) {
		appState.Mutex.RLock()
		defer appState.Mutex.RUnlock()
		for key, user := range appState.UserStore {
			if key == "" {
				continue
			}
			if user.UserID == Id {
				user.StructMutex.Lock()
				return user, key
			}
		}

		return nil, ""
	}

	writeResponse := func(w http.ResponseWriter, key string) {
		type T struct {
			Key string `json:"key"`
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(T{Key: key})
	}

	handler := func(w http.ResponseWriter, r *http.Request) int {
		vars := mux.Vars(r)
		token := vars["token"]
		if token == "" {
			http.Error(w, "Bad parameters", http.StatusBadRequest)
			return http.StatusBadRequest
		}

		userId := utils.TokenTId(token)
		if userId == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return http.StatusUnauthorized
		}

		if usr, key := findExisting(userId); usr != nil {
			// !! DO NOT LOCK, IT GETS LOCKED WHEN FETCHED !!
			// !! THE UNLOCK HERE IS FINE !!
			defer usr.StructMutex.Unlock()
			usr.Bearer = token
			writeResponse(w, key)
			return http.StatusOK
		}

		if usr, key, err := types.FindLoadUserStorageId(userId, appState); err == nil && usr != nil && key != "" {
			appState.Mutex.Lock()
			defer appState.Mutex.Unlock()
			usr.StructMutex.Lock()
			defer usr.StructMutex.Unlock()
			usr.Bearer = token
			appState.UserStore[key] = usr
			writeResponse(w, key)
			return http.StatusOK
		}

		newUser := types.NewUser(userId, token, appState.Config)
		key := utils.GenerateKey(token)
		if err := newUser.SaveUserStorage(key, appState, true); err != nil {
			log.Println("Failed to save user storage for new user", userId+":", err.Error())
			http.Error(w, "Failed to save user storage", http.StatusInternalServerError)
			return http.StatusInternalServerError
		}
		defer log.Println("Created new user with id:", userId)

		appState.Mutex.Lock()
		defer appState.Mutex.Unlock()
		appState.UserStore[key] = newUser
		writeResponse(w, key)
		return http.StatusOK
	}

	func() {
		appState.Mutex.Lock()
		defer appState.Mutex.Unlock()

		appState.Mux.HandleFunc(at, utils.RequestLoggerWrapper(at, handler)).
			Methods(http.MethodPost).
			Headers("Authorization", appState.Config.ManagementToken)
	}()
}
