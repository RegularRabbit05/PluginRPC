package routes

import (
	"PluginRPC/types"
	"PluginRPC/utils"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

func InstallReceiverHandler(at string, appState *types.AppState) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool {
			return true
		},
	}
	storageLoader := func(key string, user *types.User) {
		appState.Mutex.Lock()
		defer appState.Mutex.Unlock()
		appState.UserStore[key] = user
	}

	handler := func(w http.ResponseWriter, r *http.Request) int {
		vars := mux.Vars(r)
		key := vars["key"]
		if key == "" {
			http.Error(w, "Bad parameters", http.StatusBadRequest)
			return http.StatusBadRequest
		}

		user := appState.GetUser(key)
		if user == nil {
			var err error
			user, err = types.FindLoadUserStorageKey(true, key, appState)
			if err != nil || user == nil {
				http.Error(w, "User not found", http.StatusNotFound)
				return http.StatusNotFound
			}
			storageLoader(key, user)
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return http.StatusInternalServerError
		}

		func() {
			user.StructMutex.Lock()
			defer user.StructMutex.Unlock()
			if user.ClientReceiverConnection != nil {
				_ = user.ClientReceiverConnection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Double login"))
				_ = user.ClientReceiverConnection.Close()
			}
			user.ClientReceiverConnection = conn

			user.DiscordSync(appState.Config)
		}()

		defer func() {
			user.StructMutex.Lock()
			defer user.StructMutex.Unlock()
			if user.ClientReceiverConnection == conn {
				user.ClientReceiverConnection = nil
			}
			_ = conn.Close()
		}()

		for {
			_, _, wErr := conn.ReadMessage()
			if wErr != nil {
				break
			}

			wErr = conn.WriteMessage(websocket.PongMessage, nil)
			if wErr != nil {
				break
			}
		}

		return http.StatusOK
	}

	func() {
		cors := func(w http.ResponseWriter, r *http.Request) int {
			w.WriteHeader(http.StatusOK)
			return http.StatusOK
		}

		appState.Mutex.Lock()
		defer appState.Mutex.Unlock()

		appState.Mux.HandleFunc(at, utils.RequestLoggerWrapper(at, handler)).Methods(http.MethodGet)
		appState.Mux.HandleFunc(at, utils.RequestLoggerWrapper(at, cors)).Methods(http.MethodOptions)
	}()
}
