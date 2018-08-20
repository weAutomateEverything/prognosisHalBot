package sinkBin

import (
	"fmt"
	"github.com/weAutomateEverything/anomalyDetectionHal/detector"
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func NewSinkBinMonitor() monitor.Monitor {

	conn, err := grpc.Dial(os.Getenv("DETECTOR_ENDPOINT"), grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	c := detector.NewAnomalyDetectorClient(conn)

	return &sinkBinMonitor{
		client: c,
	}
}

type sinkBinMonitor struct {
	client detector.AnomalyDetectorClient
}

func (monitor sinkBinMonitor) CheckResponse(ctx context.Context, req [][]string) (failure bool, failuremsg string, err error) {

	request := make([]data, 0, len(req))
	for _, s := range req {
		var d data
		switch len(s) {
		case 11:
			d = data{
				Node:              s[0],
				BIN:               s[1],
				Product:           s[2],
				ApprovalCount:     getInt(s[3]),
				ValidDenyCount:    getInt(s[4]),
				DenyCount:         getInt(s[5]),
				IssuerTimeout:     getInt(s[6]),
				SystemMalfunction: getInt(s[7]),
				ApprovalRate:      getFloat(s[8]),
			}
		case 10:
			d = data{
				Node:                 s[0],
				ApprovalCount:        getInt(s[1]),
				ValidDenyCount:       getInt(s[2]),
				DenyCount:            getInt(s[3]),
				IssuerTimeout:        getInt(s[4]),
				SystemMalfunction:    getInt(s[5]),
				TransactionPerSecond: getFloat(s[6]),
				ApprovalRate:         getFloat(s[7]),
			}
		default:
			continue

		}

		if d.Node == "" {
			continue
		}

		request = append(request, d)
	}

	failure, failuremsg = monitor.saveAndValidate(ctx, request)

	return

}

func (m sinkBinMonitor) saveAndValidate(ctx context.Context, request []data) (failed bool, msg string) {
	s := ""
	for _, d := range request {
		s = s + fmt.Sprintf("transactions,node=%v,bin=%v approval=%v,valid_deny=%v,transaction_per_second=%v,system_malfunction=%v,issuer_timeout=%v,deny_count=%v,approval_rate=%v\n",
			d.Node,
			d.BIN,
			d.ApprovalCount,
			d.ValidDenyCount,
			int(d.TransactionPerSecond),
			d.SystemMalfunction,
			d.IssuerTimeout,
			d.DenyCount,
			int(d.ApprovalRate),
		)

		if d.BIN == "" {
			continue
		}

		f, r := m.validateAnomaly(ctx, float64(d.DenyCount), "prognosis_deny_"+d.BIN)
		if f {
			failed = true
			msg = msg + "Anomaly detected in the deny rate for bin " + d.BIN + " " + r + "\n"
		}

		f, r = m.validateAnomaly(ctx, d.ApprovalRate, "prognosis_approval_rate_"+d.BIN)
		if f {
			failed = true
			msg = msg + "Anomaly detected in the approval rate for bin " + d.BIN + " " + r + "\n"
		}

	}
	resp, err := http.Post(fmt.Sprintf("%v/write?db=prognosis", os.Getenv("KAPACITOR_URL")),
		"application/text", strings.NewReader(s))

	if err != nil {
		log.Println(err)
	} else {
		resp.Body.Close()
	}
	return

}

func (s sinkBinMonitor) validateAnomaly(ctx context.Context, value float64, index string) (failed bool, msg string) {
	i := detector.InputData{
		Key:   index,
		Value: value,
	}

	resp, err := s.client.AnalyseData(ctx, &i)
	if err != nil {
		log.Println(err)
		return
	}

	if resp.Score > 3 {
		failed = true
		msg = resp.Explanation
	}
	return
}

func (sinkBinMonitor) GetName() string {
	return "SinkBin"
}

func getInt(s string) (x int64) {
	x, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		log.Printf("Could not conver string %v to a int, returning 0", s)
		return 0
	}
	return
}

func getFloat(s string) (x float64) {
	x, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("Could not conver string %v to a float, returning 0", s)
		return 0

	}
	return
}

type data struct {
	Node                 string
	BIN                  string
	Product              string
	ApprovalCount        int64
	ValidDenyCount       int64
	DenyCount            int64
	IssuerTimeout        int64
	SystemMalfunction    int64
	TransactionPerSecond float64
	ApprovalRate         float64
}
