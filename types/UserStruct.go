package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/fxamacker/cbor"
	"github.com/gorilla/websocket"
)

type UserActivityAssets struct {
	LargeImage string `json:"large_image"`
	LargeText  string `json:"large_text"`
}

type UserActivity struct {
	Id                string              `json:"application_id"`
	Name              string              `json:"name"`
	Type              int64               `json:"type"`
	CreatedAt         int64               `json:"created_at"`
	Platform          string              `json:"platform"`
	StatusDisplayType int64               `json:"status_display_type"`
	Details           string              `json:"details"`
	State             string              `json:"state"`
	Assets            *UserActivityAssets `json:"assets,omitempty"`
	GameAppID         string              `json:"-"`
}

type User struct {
	Bearer   string         `json:"token" cbor:"tok"`
	Activity []UserActivity `json:"activities" cbor:"-"`
	UserID   string         `json:"-" cbor:"uid"`

	IsPlaying                bool            `json:"-" cbor:"-"`
	AppNextHeartbeat         time.Time       `json:"-" cbor:"-"`
	AppNextRefresh           time.Time       `json:"-" cbor:"-"`
	BearerNextRefresh        time.Time       `json:"-" cbor:"-"`
	StructMutex              sync.Mutex      `json:"-" cbor:"-"`
	ClientReceiverConnection *websocket.Conn `json:"-" cbor:"-"`
}

func (a *UserActivity) ApplyDefaults(conf AppConfig) *UserActivity {
	a.Id = conf.AppId
	a.Type = 0
	a.Platform = "desktop"
	a.StatusDisplayType = 1

	return a
}

func NewActivity(conf AppConfig) UserActivity {
	return *(&UserActivity{
		CreatedAt: time.Now().Unix(),
		Name:      "PluginRPC",
		Details:   "Awaiting data...",
		State:     "",
		Assets:    nil,
	}).ApplyDefaults(conf)
}

func (u *User) GetCurrentActivityUID() string {
	u.StructMutex.Lock()
	defer u.StructMutex.Unlock()

	if len(u.Activity) == 0 || !u.IsPlaying {
		return ""
	}

	return u.Activity[0].GameAppID
}

func (u *User) DiscordSync(conf AppConfig) bool {
	body, err := json.Marshal(u)
	if err != nil {
		return false
	}

	// If the user is running the local client give that priority
	if u.ClientReceiverConnection != nil {
		type T struct {
			Activate bool `json:"activate"`
		}
		playState := T{Activate: u.IsPlaying}
		command, err := json.Marshal(playState)
		if err != nil {
			return false
		}

		if u.ClientReceiverConnection.WriteMessage(websocket.TextMessage, command) != nil {
			_ = u.ClientReceiverConnection.Close()
			u.ClientReceiverConnection = nil
			return false
		}

		if u.ClientReceiverConnection.WriteMessage(websocket.TextMessage, body) != nil {
			_ = u.ClientReceiverConnection.Close()
			u.ClientReceiverConnection = nil
			return false
		}
		return true
	}

	if u.IsPlaying == false {
		//Remove the session eventually
		return true
	}

	req, err := http.NewRequest("POST", conf.Endpoint+"/users/@me/headless-sessions", bytes.NewBuffer(body))
	if err != nil {
		return false
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", u.Bearer))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	return true
}

func (u *User) RefreshHeartbeat() {
	u.StructMutex.Lock()
	defer u.StructMutex.Unlock()

	u.AppNextHeartbeat = time.Now().Add(time.Minute * 6)
}

func (u *User) UpdateActivity(activity UserActivity, config AppConfig) bool {
	u.RefreshHeartbeat()
	u.StructMutex.Lock()
	defer u.StructMutex.Unlock()

	u.Activity = []UserActivity{*activity.ApplyDefaults(config)}
	u.IsPlaying = true
	u.AppNextRefresh = time.Now().Add(time.Minute * 5)
	return u.DiscordSync(config)
}

func (u *User) DeleteActivity(config AppConfig) {
	u.StructMutex.Lock()
	defer u.StructMutex.Unlock()

	u.IsPlaying = false
	u.Activity = []UserActivity{NewActivity(config)}
}

func (u *User) ApplyDefaults() *User {
	u.IsPlaying = false
	u.AppNextRefresh = time.Now().Add(time.Minute * 5)
	u.AppNextHeartbeat = time.Now().Add(time.Minute * 6)
	u.BearerNextRefresh = time.Now().Add(time.Minute * 30)
	u.StructMutex = sync.Mutex{}
	return u
}

func NewUser(id string, token string, conf AppConfig) *User {
	return (&User{
		Bearer:   token,
		UserID:   id,
		Activity: []UserActivity{NewActivity(conf)},
	}).ApplyDefaults()
}

func FindLoadUserStorageKey(caller bool, key string, appState *AppState) (*User, error) {
	t := time.Now()
	appState.Mutex.RLock()
	defer appState.Mutex.RUnlock()

	if !appState.FileBackedStore.Has(key) {
		return nil, errors.New("token does not exist")
	}

	data, err := appState.FileBackedStore.Read(key)
	if err != nil {
		return nil, err
	}

	usr := NewUser("", "", appState.Config)
	if err = cbor.Unmarshal(data, &usr); err != nil {
		return nil, err
	}

	usr.StructMutex = sync.Mutex{}

	if caller {
		log.Printf("Loaded user from storage with id: %s in %s", usr.UserID, time.Now().Sub(t))
	}
	return usr, nil
}

func FindLoadUserStorageId(userId string, appState *AppState) (*User, string, error) {
	t := time.Now()
	appState.Mutex.RLock()
	defer appState.Mutex.RUnlock()

	cancel := make(chan struct{})
	for key := range appState.FileBackedStore.Keys(cancel) {
		usr, err := FindLoadUserStorageKey(false, key, appState)
		if err != nil || usr == nil {
			continue
		}
		if usr.UserID != userId {
			continue
		}
		close(cancel)
		log.Printf("Loaded user from storage with id: %s in %s", usr.UserID, time.Now().Sub(t))
		runtime.Gosched()
		return usr, key, nil
	}

	return nil, "", errors.New("token does not exist")
}

func (u *User) SaveUserStorage(key string, appState *AppState, lock bool) error {
	data, err := cbor.Marshal(u, cbor.PreferredUnsortedEncOptions())
	if err != nil {
		return err
	}

	if lock {
		appState.Mutex.Lock()
		defer appState.Mutex.Unlock()
	}
	return appState.FileBackedStore.Write(key, data)
}

func (u *User) SyncActivity(conf AppConfig) bool {
	u.StructMutex.Lock()
	defer u.StructMutex.Unlock()

	if u.AppNextRefresh.After(time.Now()) {
		return true
	}
	u.AppNextRefresh = time.Now().Add(time.Minute * 5)

	if u.AppNextHeartbeat.Before(time.Now()) {
		u.IsPlaying = false
		u.DiscordSync(conf)
		return true
	}

	// Until discord allows me to use activities.write I cannot stop the sync on failure
	u.DiscordSync(conf)
	return true
}
