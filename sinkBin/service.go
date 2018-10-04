package sinkBin

import (
	"fmt"
	"github.com/weAutomateEverything/anomalyDetectionHal/detector"
	"github.com/weAutomateEverything/prognosisHalBot/anomaly"
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func NewSinkBinMonitor() monitor.Monitor {

	return &sinkBinMonitor{
		anomaly: anomaly.NewService(),
	}
}

type sinkBinMonitor struct {
	client  detector.AnomalyDetectorClient
	anomaly anomaly.Service
}

func (m sinkBinMonitor) CheckResponse(ctx context.Context, req [][]string) (response []monitor.Response, err error) {

	log.Printf("processing %v records", len(req))
	jobs := make(chan data, len(req))
	results := make(chan validationResult, len(req))

	log.Println("starting workers")
	for w := 1; w <= 10; w++ {
		go m.saveAndValidate(ctx, jobs, results)
	}

	c := 0
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

		jobs <- d
		c++
	}

	log.Println("closing job")
	close(jobs)

	response = make([]monitor.Response, c)

	log.Printf("waiting for %v results", c)
	for a := 1; a <= c; a++ {
		i := <-results
		response[a-1] = monitor.Response{
			FailureMsg: i.msg,
			Key:        i.key,
			Failure:    i.failed,
		}
	}

	log.Println("Done")
	return

}

func (m sinkBinMonitor) saveAndValidate(ctx context.Context, requests <-chan data, result chan<- validationResult) {
	for d := range requests {
		s := ""
		v := validationResult{}
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
		v.key = d.BIN
		if f {
			v.failed = true
			v.msg = "Anomaly detected in the deny rate for bin " + d.BIN + " " + r + "\n"
		}

		f, r = m.validateAnomaly(ctx, d.ApprovalRate, "prognosis_approval_rate_"+d.BIN)
		if f {
			v.failed = true
			v.msg = v.msg + "Anomaly detected in the *approval rate* for bin " + d.BIN + "\n" + r + "\n\n"
		}

		resp, err := http.Post(fmt.Sprintf("%v/write?db=prognosis", os.Getenv("KAPACITOR_URL")),
			"application/text", strings.NewReader(s))

		if err != nil {
			log.Println(err)
		} else {
			resp.Body.Close()
		}

		result <- v
	}

}

func (s sinkBinMonitor) validateAnomaly(ctx context.Context, value float64, index string) (failed bool, msg string) {
	failed, msg, _ = s.anomaly.Analyse(index, value)
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

type validationResult struct {
	failed bool
	msg    string
	key    string
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
