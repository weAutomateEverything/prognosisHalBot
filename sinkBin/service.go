package sinkBin

import (
	"fmt"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
)

func NewSinkBinMonitor() monitor.Monitor {
	return &sinkBinMonitor{}
}

type sinkBinMonitor struct {
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

		if d.ApprovalCount+d.DenyCount < 500 {
			continue
		}

		request = append(request, d)
	}

	sendKinesis(ctx, request)

	return

}

func sendKinesis(ctx context.Context, request []data) {
	s := ""
	for _, d := range request {
		s = s + fmt.Sprintf("transactions,node=%v,bin=%v approval=%v,valid_deny=%v,transaction_per_second=%v,system_malfunction=%v,issuer_timeout=%v,deny_count=%v\n",
			d.Node,
			d.BIN,
			d.ApprovalCount,
			d.ValidDenyCount,
			int(d.TransactionPerSecond),
			d.SystemMalfunction,
			d.IssuerTimeout,
			d.DenyCount,
		)
	}

	resp, err := ctxhttp.Post(ctx, http.DefaultClient, fmt.Sprintf("%v/write?db=prognosis", os.Getenv("KAPACITOR_URL")),
		"application/text", strings.NewReader(s))
	b, _ := httputil.DumpResponse(resp, true)
	log.Println(string(b))

	if err != nil {

		xray.AddError(ctx, err)
	} else {
		resp.Body.Close()
	}

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
