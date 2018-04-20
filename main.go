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

	authService := bankldapService.NewService(bankLdapStore)

	telegram := telegram.NewService(telegramStore, authService)
	alert := alert.NewService(telegram, alertStore)
	firstcall := firstCall.NewDefaultFirstcallService()
	alexa := halaws.NewService(alert)

	failureRateRDC := monitor.NewFailureRateMonitor("/Prognosis/DashboardView/2f0f44ba-a6bd-4795-8de7-b4c140703912")
	failureRateSDC := monitor.NewFailureRateMonitor("/Prognosis/DashboardView/dc571d93-8d1a-45f6-bed3-d44e46367d3a")
	responseCode91RDC := monitor.NewResponseCode91Monitor("/Prognosis/DashboardView/1537cb18-a6ee-4ece-bc49-506eeab67428")
	responseCode91SDC := monitor.NewResponseCode91Monitor("/Prognosis/DashboardView/1759f135-353b-4760-970e-a3794b9729ba")

	calloutService := callout.NewService(alert, firstcall, nil, nil, alexa)

	monitor.NewService(calloutService, alert, failureRateRDC, failureRateSDC, responseCode91RDC, responseCode91SDC)

	logger.Log("All Systems GO!")
	errs := make(chan error, 2)

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()
	logger.Log("terminated", <-errs)

}
