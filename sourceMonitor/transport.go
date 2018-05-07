package sourceMonitor

import (
	kitlog "github.com/go-kit/kit/log"
	kithttp "github.com/go-kit/kit/transport/http"

	"net/http"
	"github.com/weAutomateEverything/go2hal/gokit"
	"github.com/gorilla/mux"
)

func MakeHandler(store Store, logger kitlog.Logger) http.Handler {
	opts := gokit.GetServerOpts(logger, nil)

	alertHandler := kithttp.NewServer(makeAddNodeHours(store), gokit.DecodeString, gokit.EncodeResponse, opts...)

	r := mux.NewRouter()

	r.Handle("/sourceMonitor/times", alertHandler).Methods("POST")

	return r
}
