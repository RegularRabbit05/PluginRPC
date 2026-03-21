package types

import (
	"PluginRPC/utils"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/peterbourgon/diskv/v3"
)

type AppState struct {
	Config          AppConfig
	Mux             *mux.Router
	Server          http.Server
	UserStore       map[string]*User
	FileBackedStore *diskv.Diskv

	Mutex      sync.RWMutex
	ServerDead sync.Mutex
	Bye        sync.Mutex
}

func NewAppState() AppState {
	return AppState{
		Config: NewAppConfig(),
	}
}

func (s *AppState) Init() error {
	if err := s.Config.IsValid(); err != nil {
		return err
	}
	flatTransform := func(s string) []string { return []string{} }

	s.UserStore = make(map[string]*User)
	s.Mux = mux.NewRouter()
	s.FileBackedStore = diskv.New(diskv.Options{
		BasePath:     s.Config.StorageDir,
		Transform:    flatTransform,
		CacheSizeMax: 1024 * 1024,
	})

	return nil
}

func (s *AppState) GetUser(token string) *User {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	user, ok := s.UserStore[token]
	if !ok {
		return nil
	}

	return user
}

func (s *AppState) Listen() error {
	_ = handlers.RecoveryHandler()(s.Mux)
	s.Server = http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr:         utils.AsAddr(s.Config.AppPort),
		Handler:      handlers.ProxyHeaders(s.Mux),
	}

	log.Printf("Started server on port %d\n", s.Config.AppPort)
	if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *AppState) Terminate() {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if !s.ServerDead.TryLock() {
		return
	}

	_ = s.Server.Close()
	for _, user := range s.UserStore {
		func() {
			user.StructMutex.Lock()
			defer user.StructMutex.Unlock()
			if user.ClientReceiverConnection != nil {
				_ = user.ClientReceiverConnection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Server is shutting down"))
				_ = user.ClientReceiverConnection.Close()
			}
		}()
	}
}

func (s *AppState) Close() {
	s.Terminate()
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if !s.Bye.TryLock() {
		return
	}

	log.Println("App is going down")
}
