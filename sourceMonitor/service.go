package sourceMonitor

import (
	"fmt"
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"log"
	"strconv"
	"strings"
	"time"
)

type sourceSinkMonitor struct {
	nodeTimes     []nodeHours
	nodeMaxValues []nodeMax
}

func NewSourceSinkMonitor(store Store) monitor.Monitor {
	return &sourceSinkMonitor{
		nodeTimes:     store.GetNodeTimes(),
		nodeMaxValues: store.getMaxConnections(),
	}
}

func (s sourceSinkMonitor) GetName() string {
	return "SourceSink"
}

func (s sourceSinkMonitor) CheckResponse(input [][]string) (failure bool, failuremsg string, err error) {
	for _, row := range input {
		failure, failuremsg = s.checkConnected(row)
		if failure {
			return
		}

		failure, failuremsg = s.checkMaxConnections(row)
		if failure {
			return
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

	for _, times := range s.nodeTimes {
		if strings.Index(times.Nodename, node) != -1 {
			if s.checkSend(times) {
				failure = true
				failuremsg = node
			}
		}
	}
	return
}

func (s sourceSinkMonitor) checkMaxConnections(row []string) (failure bool, failuremsg string) {
	for _, max := range s.nodeMaxValues {
		if row[0] == max.Nodename {
			v := row[2]
			connections, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				log.Printf("unable to parse %v as a int for max value", v)
				return
			}
			if connections > int64(max.Maxval) {
				failure = true
				failuremsg = fmt.Sprintf("Node %v has breached the maximum threshold of %v. Current connections are %v", max.Nodename, max.Maxval, connections)
				return
			}
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
