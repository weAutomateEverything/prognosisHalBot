package sinkBin

import (
	"bytes"
	"encoding/json"
	"github.com/weAutomateEverything/prognosisHalBot/anomaly"
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"golang.org/x/net/context"
	"net/http"
	"os"
)

func NewSinkBinMonitor() monitor.Monitor {

	return &sinkBinMonitor{
		anomaly: anomaly.NewService(),
	}
}

type sinkBinMonitor struct {
	anomaly anomaly.Service
}

func (m sinkBinMonitor) CheckResponse(ctx context.Context, req [][]string) (response []monitor.Response, err error) {

	b, err := json.Marshal(req)
	if err != nil {
		return
	}
	resp, err := http.Post(os.Getenv("SINKBIN_URL"), "application/text", bytes.NewReader(b))
	if err != nil {
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	return

}
func (sinkBinMonitor) GetName() string {
	return "SinkBin"
}
