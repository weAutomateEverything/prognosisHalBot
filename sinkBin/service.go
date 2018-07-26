package sinkBin

import (
	"crypto/tls"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"os"
	"strconv"
)

func NewSinkBinMonitor(store Store) monitor.Monitor {
	return &sinkBinMonitor{
		store,
	}
}

type sinkBinMonitor struct {
	Store
}

func (monitor sinkBinMonitor) CheckResponse(ctx context.Context, req [][]string) (failure bool, failuremsg string, err error) {

	request := make([]*kinesis.PutRecordsRequestEntry, len(req))
	for i, s := range req {
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

		b, err := json.Marshal(d)
		if err != nil {
			log.Printf("Count not marshal %v into a byte stream", d)
			continue
		}
		shard, err := monitor.Store.getShardId(d.BIN)
		if err != nil {
			log.Println(err.Error())
			xray.AddError(ctx, err)
			continue
		}

		request[i] = &kinesis.PutRecordsRequestEntry{
			Data:            b,
			PartitionKey:    aws.String(d.BIN),
			ExplicitHashKey: aws.String(strconv.FormatInt(int64(shard), 10)),
		}

	}

	sendKinesis(ctx, request)

	return

}

func sendKinesis(ctx context.Context, request []*kinesis.PutRecordsRequestEntry) {
	c := credentials.NewEnvCredentials()

	client := http.DefaultClient
	transport := http.DefaultTransport
	transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client.Transport = transport
	config := aws.Config{Credentials: c, Region: aws.String(os.Getenv("AWS_REGION")), HTTPClient: client}
	sess, _ := session.NewSession(&config)
	k := kinesis.New(sess, &config)
	xray.AWS(k.Client)

	for i := 0; i < len(request); i += 500 {
		end := i + 500

		if end > len(request) {
			end = len(request)
		}
		i := kinesis.PutRecordsInput{
			StreamName: aws.String("prognosis-bin"),
			Records:    request[i:end],
		}
		_, err := k.PutRecordsWithContext(ctx, &i)
		if err != nil {
			log.Printf("Error putting details to amazon kinesis. Error %v", err.Error())
		}
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
