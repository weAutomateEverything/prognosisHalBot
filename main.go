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

	authService := bankldapService.NewService(bankLdapStore)

	telegramService := telegram.NewService(telegramStore, authService)
	alertService := alert.NewService(telegramService, alertStore)
	firstcall := firstCall.NewDefaultFirstcallService()
	alexa := halaws.NewService(alertService)

	failureRateRDC := monitor.NewFailureRateMonitor("/Prognosis/DashboardView/2f0f44ba-a6bd-4795-8de7-b4c140703912",monitorStore)
	failureRateSDC := monitor.NewFailureRateMonitor("/Prognosis/DashboardView/dc571d93-8d1a-45f6-bed3-d44e46367d3a",monitorStore)
	responseCode91RDC := monitor.NewResponseCode91Monitor("/Prognosis/DashboardView/1537cb18-a6ee-4ece-bc49-506eeab67428",monitorStore)
	responseCode91SDC := monitor.NewResponseCode91Monitor("/Prognosis/DashboardView/1759f135-353b-4760-970e-a3794b9729ba",monitorStore)

	calloutService := callout.NewService(alertService, firstcall, nil, nil, alexa)

	monitor.NewService(calloutService, alertService, failureRateRDC, failureRateSDC, responseCode91RDC, responseCode91SDC)

	telegramService.RegisterCommand(alert.NewSetGroupCommand(telegramService, alertStore))
	telegramService.RegisterCommand(alert.NewSetNonTechnicalGroupCommand(telegramService, alertStore))
	telegramService.RegisterCommand(alert.NewSetHeartbeatGroupCommand(telegramService, alertStore))
	telegramService.RegisterCommand(telegram.NewHelpCommand(telegramService))

	telegramService.RegisterCommand(bankldapService.NewRegisterCommand(telegramService, bankLdapStore))
	telegramService.RegisterCommand(bankldapService.NewTokenCommand(telegramService, bankLdapStore))

	logger.Log("All Systems GO!")
	errs := make(chan error, 2)

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()
	logger.Log("terminated", <-errs)

}
