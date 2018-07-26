package main

import (
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"os"

	"fmt"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/weAutomateEverything/go2hal/database"
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"github.com/weAutomateEverything/prognosisHalBot/sourceMonitor"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/aws/aws-xray-sdk-go/xray"
	logger2 "github.com/go-openapi/runtime/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/weAutomateEverything/prognosisHalBot/sinkBin"
)

func main() {

	xray.Configure(xray.Config{
		DaemonAddr:     "127.0.0.1:2000", // default
		LogLevel:       "info",           // default
		ServiceVersion: "1.2.3",
	})

	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = level.NewFilter(logger, level.AllowAll())
	logger = log.With(logger, "ts", log.DefaultTimestamp)

	db := database.NewConnection()

	monitorStore := monitor.NewMongoStore(db)
	sourceStore := sourceMonitor.NewMontoSourceSinkStore(db)
	transport := httptransport.New(os.Getenv("HAL_ENDPOINT"), "", nil)
	transport.SetDebug(true)
	transport.SetLogger(logger2.StandardLogger{})

	monitor.NewService(monitorStore, monitor.NewResponseCode91Monitor(), monitor.NewFailureRateMonitor(),
		sourceMonitor.NewSourceSinkMonitor(sourceStore), sinkBin.NewSinkBinMonitor())

	httpLogger := log.With(logger, "component", "http")

	mux := http.NewServeMux()
	mux.Handle("/sourceMonitor/", sourceMonitor.MakeHandler(sourceStore, httpLogger))
	http.Handle("/", accessControl(mux))
	http.Handle("/api/metrics", promhttp.Handler())

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
