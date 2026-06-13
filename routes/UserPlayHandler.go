package routes

import (
	"PluginRPC/types"
	"PluginRPC/utils"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"net/http"

	"github.com/gorilla/mux"
)

func InstallUserPlayHandler(at string, appState *types.AppState) {
	platformGenerator := map[string]func(string, string, *http.Request) (*types.UserActivity, int){}
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
	playstationGenerator := func(id string, platform string, _ *http.Request) (*types.UserActivity, int) {
		var ok bool
		platform, ok = playstationPlatforms[platform]
		if !ok {
			return nil, http.StatusBadRequest
		}

		endpoint := appState.Config.PlatformPlaystationEndpoint
		if platform != "PS3" {
			endpoint = appState.Config.PlatformPlaystationMobileEndpoint
		}

		if len(id) <= 5 {
			endpoint = appState.Config.PlatformOverlayEndpoint
		}
		requestUrl := fmt.Sprintf(endpoint, id)
		request, err := http.NewRequest("GET", requestUrl, nil)
		if err != nil {
			return nil, http.StatusServiceUnavailable
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
			return nil, http.StatusServiceUnavailable
		}
		defer resp.Body.Close()

		var data T
		if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, http.StatusInternalServerError
		}

		if data.Name == "" {
			return nil, http.StatusNotFound
		}

		actTemplate := templateGenerator()

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

		return &actTemplate, http.StatusOK
	}
	for platform := range playstationPlatforms {
		platformGenerator[platform] = playstationGenerator
	}

	customGenerator := func(id string, platform string, r *http.Request) (*types.UserActivity, int) {
		cStringToGoString := func(b []byte) string {
			idx := bytes.IndexByte(b, 0)
			if idx == -1 {
				return string(b)
			}
			return string(b[:idx])
		}

		if r.Method != http.MethodPost {
			return nil, http.StatusMethodNotAllowed
		}

		const apiVersion = 1
		const v1BodySize = 836

		if r.ContentLength > v1BodySize {
			return nil, http.StatusBadRequest
		}
		body := make([]byte, r.ContentLength)
		n, err := io.ReadFull(r.Body, body)
		if err != nil || n != v1BodySize {
			return nil, http.StatusLengthRequired
		}

		type ActivityApiV1 struct {
			ApiVersion  uint32
			Name        [64]byte
			Description [128]byte
			State       [128]byte
			Image       [512]byte
		}
		var payload ActivityApiV1

		err = binary.Read(bytes.NewReader(body), binary.LittleEndian, &payload)
		if err != nil {
			return nil, http.StatusExpectationFailed
		}

		if payload.ApiVersion != apiVersion {
			return nil, http.StatusFailedDependency
		}

		img := cStringToGoString(payload.Image[:])
		name := cStringToGoString(payload.Name[:])
		details := cStringToGoString(payload.Description[:])
		state := cStringToGoString(payload.State[:])

		actTemplate := templateGenerator()

		actTemplate.GameAppID = fmt.Sprint(time.Now().Unix())
		if len(name) > 0 {
			actTemplate.Name = name
		}
		if len(details) > 0 {
			actTemplate.Details = details
		}
		if len(state) > 0 {
			actTemplate.State = state
		}
		if len(img) > 0 {
			actTemplate.Assets = &types.UserActivityAssets{
				LargeImage: img,
				LargeText:  "PluginRPC",
			}
		}

		return &actTemplate, http.StatusOK
	}
	platformGenerator["custom"] = customGenerator

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

		appPresence, code := generator(application, platform, r)
		if appPresence == nil {
			http.Error(w, fmt.Sprintf("Failed to fetch application data (%d)", code), code)
			user.DeleteActivity(appState.Config)
			user.DiscordSync(appState.Config)
			return code
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
