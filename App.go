package main

import (
	"PluginRPC/routes"
	"PluginRPC/tasks"
	"PluginRPC/types"
	"PluginRPC/utils"
	"runtime"
	"time"

	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron"
)

func appCycle(app *types.AppState) {
	defer app.Close()
	scheduler := cron.New()

	utils.Catch(app.Init())

	routes.InstallUserAddHandler("/api/v1/management/{token}/register", app)
	routes.InstallReceiverHandler("/api/v1/presence/{key}/receiver", app)
	routes.InstallUserPlayHandler("/api/v1/presence/{key}/play/{platform}/{application}", app)

	utils.Catch(scheduler.AddFunc("@every 1m", tasks.ActivityRefresherTask(app)))
	utils.Catch(scheduler.AddFunc("@every 10m", tasks.BearerRefresherTask(app)))
	utils.Catch(scheduler.AddFunc("@every 45m", tasks.DatabaseSaverTask(app)))

	scheduler.Start()
	defer scheduler.Stop()

	utils.Catch(app.Listen())
}

func main() {
	app := types.NewAppState()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		app.Terminate()
	}()

	appCycle(&app)
	runtime.Gosched()
	log.Println("Bye!")
	time.Sleep(2 * time.Second)
}
