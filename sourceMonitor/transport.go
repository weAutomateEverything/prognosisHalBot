package sourceMonitor

import (
	kitlog "github.com/go-kit/kit/log"
	kithttp "github.com/go-kit/kit/transport/http"

	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/weAutomateEverything/go2hal/gokit"
	"io/ioutil"
	"net/http"
)

func MakeHandler(store Store, logger kitlog.Logger) http.Handler {
	opts := gokit.GetServerOpts(logger, nil)

	nodeHours := kithttp.NewServer(makeAddNodeHours(store), gokit.DecodeString, gokit.EncodeResponse, opts...)
	nodeMax := kithttp.NewServer(makeSetNodeMax(store), decodeMaxNodes, gokit.EncodeResponse, opts...)
	r := mux.NewRouter()

	r.Handle("/sourceMonitor/times", nodeHours).Methods("POST")
	r.Handle("/sourceMonitor/max", nodeMax).Methods("POST")

	return r
}

func decodeMaxNodes(_ context.Context, r *http.Request) (resp interface{}, err error) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	var v []nodeMax
	err = json.Unmarshal(b, &v)
	return v, err
}
