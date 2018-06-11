package main

import (
	"os"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/log"

	"os/signal"
	"syscall"
	"fmt"
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"github.com/weAutomateEverything/go2hal/database"
	"net/http"
	"github.com/weAutomateEverything/prognosisHalBot/sourceMonitor"
	"github.com/weAutomateEverything/prognosisHalBot/client"
	httptransport "github.com/go-openapi/runtime/client"

	"github.com/go-openapi/strfmt"
)

func main() {
	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = level.NewFilter(logger, level.AllowAll())
	logger = log.With(logger, "ts", log.DefaultTimestamp)

	db := database.NewConnection()

	monitorStore := monitor.NewMongoStore(db)
	sourceStore := sourceMonitor.NewMontoSourceSinkStore(db)
	transport := httptransport.New(os.Getenv("HAL_ENDPOINT"), "", nil)

	c := client.New(transport,strfmt.Default)

	monitor.NewService(c,monitorStore,monitor.NewResponseCode91Monitor(),monitor.NewFailureRateMonitor(),sourceMonitor.NewSourceSinkMonitor(sourceStore))

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
