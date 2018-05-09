package main

import (
	"os"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/log"

	"github.com/weAutomateEverything/go2hal/callout"
	"github.com/weAutomateEverything/go2hal/alert"
	"github.com/weAutomateEverything/go2hal/telegram"
	"github.com/weAutomateEverything/go2hal/firstCall"
	"github.com/weAutomateEverything/go2hal/halaws"
	"os/signal"
	"syscall"
	"fmt"
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"github.com/weAutomateEverything/go2hal/database"
	"github.com/weAutomateEverything/bankldapService"
	"net/http"
	"github.com/weAutomateEverything/prognosisHalBot/sourceMonitor"
	"golang.org/x/net/context"
)

func main() {
	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = level.NewFilter(logger, level.AllowAll())
	logger = log.With(logger, "ts", log.DefaultTimestamp)

	db := database.NewConnection()
	alertStore := alert.NewStore(db)
	telegramStore := telegram.NewMongoStore(db)
	bankLdapStore := bankldapService.NewMongoStore(db)
	monitorStore := monitor.NewMongoStore(db)
	sourceStore := sourceMonitor.NewMontoSourceSinkStore(db)

	authService := bankldapService.NewService(bankLdapStore)

	telegramService := telegram.NewService(telegramStore, authService)
	alertService := alert.NewService(telegramService, alertStore)
	firstcall := firstCall.NewDefaultFirstcallService()
	alexa := halaws.NewService(alertService)


	calloutService := callout.NewService(alertService, firstcall, nil, nil, alexa)

	variables := map[string]string{}
	variables["Link"] = "ATMGRS01"

	calloutService.InvokeCallout(context.TODO(),"Test","test",variables)

	monitor.NewService(calloutService, alertService,monitorStore,monitor.NewResponseCode91Monitor(),monitor.NewFailureRateMonitor(),sourceMonitor.NewSourceSinkMonitor(sourceStore))

	telegramService.RegisterCommand(alert.NewSetGroupCommand(telegramService, alertStore))
	telegramService.RegisterCommand(alert.NewSetNonTechnicalGroupCommand(telegramService, alertStore))
	telegramService.RegisterCommand(alert.NewSetHeartbeatGroupCommand(telegramService, alertStore))
	telegramService.RegisterCommand(telegram.NewHelpCommand(telegramService))

	telegramService.RegisterCommand(bankldapService.NewRegisterCommand(telegramService, bankLdapStore))
	telegramService.RegisterCommand(bankldapService.NewTokenCommand(telegramService, bankLdapStore))
	httpLogger := log.With(logger, "component", "http")

	mux := http.NewServeMux()
	mux.Handle("/sourceMonitor/", sourceMonitor.MakeHandler(sourceStore, httpLogger))
	http.Handle("/", accessControl(mux))

	logger.Log("All Systems GO!")
	errs := make(chan error, 2)

	go func() {
		logger.Log("transport", "http", "address", ":8001", "msg", "listening")
		errs <- http.ListenAndServe(":8001", nil)
	}()

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()
	logger.Log("terminated", <-errs)

}

func accessControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type")

		if r.Method == "OPTIONS" {
			return
		}

		h.ServeHTTP(w, r)
	})
}
