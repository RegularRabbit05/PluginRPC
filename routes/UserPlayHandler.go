package routes

import (
	"PluginRPC/types"
	"PluginRPC/utils"
	"encoding/json"
	"fmt"

	"net/http"

	"github.com/gorilla/mux"
)

func InstallUserPlayHandler(at string, appState *types.AppState) {
	platformGenerator := map[string]func(string, string) (*types.UserActivity, int){}
	storageLoader := func(key string, user *types.User) {
		appState.Mutex.Lock()
		defer appState.Mutex.Unlock()
		appState.UserStore[key] = user
	}
	templateGenerator := func() types.UserActivity {
		appState.Mutex.RLock()
		defer appState.Mutex.RUnlock()
		return types.NewActivity(appState.Config)
	}

	playstationPlatforms := map[string]string{
		"playstation3":        "PS3",
		"playstationportable": "PSP",
		"playstationvita":     "PSVita",
	}
	playstationGenerator := func(id string, platform string) (*types.UserActivity, int) {
		actTemplate := templateGenerator()
		endpoint := appState.Config.PlatformPlaystationEndpoint
		if len(id) <= 5 {
			endpoint = appState.Config.PlatformOverlayEndpoint
		}
		requestUrl := fmt.Sprintf(endpoint, id)
		request, err := http.NewRequest("GET", requestUrl, nil)
		if err != nil {
			return nil, 503
		}

		type S struct {
			Type     *string      `json:"type,omitempty"`
			Language *interface{} `json:"language,omitempty"`
			Url      *string      `json:"url,omitempty"`
		}

		type T struct {
			Icons   *[]S   `json:"icons,omitempty"`
			TitleId string `json:"title_id"`
			Name    string `json:"name"`
		}

		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, 503
		}
		defer resp.Body.Close()

		var data T
		if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, 500
		}

		if data.Name == "" {
			return nil, 404
		}

		var ok bool
		platform, ok = playstationPlatforms[platform]
		if !ok {
			return nil, 400
		}

		actTemplate.GameAppID = id
		actTemplate.Name = data.Name
		actTemplate.Details = fmt.Sprintf("Playing on %s", platform)
		actTemplate.State = fmt.Sprintf("%s", data.TitleId)
		if data.Icons != nil && len(*data.Icons) > 0 {
			icon := (*data.Icons)[0]
			if icon.Url != nil {
				actTemplate.Assets = &types.UserActivityAssets{
					LargeImage: *icon.Url,
					LargeText:  data.Name,
				}
			}
		}

		return &actTemplate, 200
	}
	for platform := range playstationPlatforms {
		platformGenerator[platform] = playstationGenerator
	}

	handler := func(w http.ResponseWriter, r *http.Request) int {
		vars := mux.Vars(r)
		key := vars["key"]
		platform := vars["platform"]
		application := vars["application"]
		if key == "" || platform == "" || application == "" {
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

		if user.GetCurrentActivityUID() == application {
			w.WriteHeader(http.StatusOK)
			user.RefreshHeartbeat()
			return http.StatusOK
		}

		generator, ok := platformGenerator[platform]
		if !ok {
			http.Error(w, "Unsupported platform", http.StatusBadRequest)
			return http.StatusBadRequest
		}

		appPresence, code := generator(application, platform)
		if appPresence == nil {
			http.Error(w, fmt.Sprintf("Failed to fetch application data (%d)", code), http.StatusInternalServerError)
			user.DeleteActivity(appState.Config)
			user.DiscordSync(appState.Config)
			return http.StatusInternalServerError
		}

		if !user.UpdateActivity(*appPresence, appState.Config) {
			http.Error(w, "Failed to update activity", http.StatusOK)
			return http.StatusOK
		}

		w.WriteHeader(http.StatusOK)
		return http.StatusOK
	}

	func() {
		appState.Mutex.Lock()
		defer appState.Mutex.Unlock()

		appState.Mux.HandleFunc(at, utils.RequestLoggerWrapper(at, handler)).Methods(http.MethodPost, http.MethodGet)
	}()
}
