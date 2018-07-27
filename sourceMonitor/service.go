package sourceMonitor

import (
	"fmt"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
	"log"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"time"
)

type sourceSinkMonitor struct {
	store Store
}

func NewSourceSinkMonitor(store Store) monitor.Monitor {
	return &sourceSinkMonitor{
		store: store,
	}
}

func (s sourceSinkMonitor) GetName() string {
	return "SourceSink"
}

func (s sourceSinkMonitor) CheckResponse(ctx context.Context, input [][]string) (failure bool, failuremsg string, err error) {

	for _, row := range input {
		failed, msg := s.checkConnected(row)
		if failed {
			failure = true
		}
		if msg != "" {
			failuremsg = failuremsg + msg + "\n"
		}
	}
	for _, row := range input {
		failed, msg := s.checkMaxConnections(ctx, row)
		if failed {
			failure = true
		}
		if msg != "" {
			failuremsg = failuremsg + msg + "\n"
		}
	}
	return
}

func (s sourceSinkMonitor) checkConnected(row []string) (failure bool, failuremsg string) {
	if row[1] == "Connected" {
		return
	}

	node := strings.ToUpper(row[0])
	log.Printf("%v detected as down", node)
	for i := 0; i < 10; i++ {
		node = strings.Replace(node, strconv.FormatInt(int64(i), 10), "", -1)
	}

	for _, times := range s.store.GetNodeTimes() {
		if strings.Index(times.Nodename, node) != -1 {
			if s.checkSend(times) {
				failure = true
				failuremsg = fmt.Sprintf("Node %v has been detected as being unavalable. ", node)
				log.Println(failuremsg)
				return
			} else {
				log.Printf("Node %v found to be outside of critical window", node)
				return
			}
		}
	}
	failure = true
	failuremsg = fmt.Sprintf("Node %v has been detected as being down, however I cannot find  a record in the database that lets me know if this is critical or not, so I am treating it as critical", node)
	log.Println(failuremsg)
	return
}

func (s sourceSinkMonitor) checkMaxConnections(ctx context.Context, row []string) (failure bool, failuremsg string) {
	for _, max := range s.store.getMaxConnections() {
		if row[0] == max.Nodename {
			v := row[2]
			connections, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				log.Printf("unable to parse %v as a int for max value", v)
				return
			}
			err = s.store.saveConnectionCount(max.Nodename, int(connections))
			s.sendElastisearch(ctx, max.Nodename, int(connections))
			if err != nil {
				log.Printf("There was an error saving the connection count %v", err)
				err = nil
			}
			avg, err := s.store.getConnectionCount(max.Nodename)
			if err != nil {
				log.Printf("No conneciton found found. %v", err.Error())
				return
			}

			if float64(connections)/avg > 2 {
				failuremsg = fmt.Sprintf("Normally at this time I excpect node %v to have %v connections. Currently there are %v connections. ", max.Nodename, avg, connections)
				if connections > int64(max.Maxval) {
					failuremsg = failuremsg + fmt.Sprintf("Since there are more than %v connections, I am marking this as an error", max.Maxval)
					failure = true
				}
				log.Println(failuremsg)
			}
			return

		}
	}
	return

}

func (s sourceSinkMonitor) checkSend(node nodeHours) bool {
	log.Printf("CHecking if we should send %v, business hours %v, busienss impact %v, after hours %v, after hours impact %v",
		node.Nodename, node.BusinessHours, node.BusinessHoursImpact, node.AfterHours, node.AfterHoursImpact)
	if node.BusinessHours == "24 X 7" {
		return node.BusinessHoursImpact == "Critical"
	}
	if s.checkTime(node.BusinessHours, node.BusinessHoursImpact) {
		return true
	}

	return s.checkTime(node.AfterHours, node.AfterHoursImpact)

}

func (s sourceSinkMonitor) sendElastisearch(ctx context.Context, nodename string, count int) {

	resp, err := ctxhttp.Post(ctx, xray.Client(nil), fmt.Sprintf("%v/write?db=prognosis", os.Getenv("KAPACITOR_URL")),
		"application/text", strings.NewReader(fmt.Sprintf("connections,node=%v value=%v", nodename, count)))
	b, _ := httputil.DumpResponse(resp, true)
	log.Println(string(b))
	if err != nil {
		xray.AddError(ctx, err)
	} else {
		resp.Body.Close()
	}
}

func (s sourceSinkMonitor) checkTime(hours, impact string) bool {
	if impact != "Critical" {
		log.Printf("%v not critical", impact)
		return false
	}
	times := strings.Split(hours, "-")
	startTime := times[0]
	endTime := times[1]

	startHour, _ := strconv.Atoi(strings.Split(startTime, "H")[0])
	endHour, _ := strconv.Atoi(strings.Split(endTime, "H")[0])
	now := time.Now().Hour()

	log.Printf("checking %v, against times %v and %v", now, startHour, endHour)
	if endHour > startHour {
		return now >= startHour && now < endHour
	} else {
		return now >= startHour || now < endHour
	}
}

type elastiRequest struct {
	Timestamp   string `json:"@timestamp"`
	Node        string `json:"node"`
	Connections int    `json:"connections"`
}
