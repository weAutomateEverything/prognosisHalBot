package sourceMonitor

import (
	"github.com/weAutomateEverything/prognosisHalBot/monitor"
	"strings"
	"fmt"
	"strconv"
	"time"
	"log"
)

type sourceSinkMonitor struct {
	nodeTimes []nodeHours
}

func (s sourceSinkMonitor) GetName() string {
	return "SourceSink"
}

func (s sourceSinkMonitor) CheckResponse(input [][]string) (failure bool, failuremsg string, err error) {
	rowloop:
	for _, row := range input {
		if row[1] == "Connected" {
			continue
		}

		node := strings.ToUpper(row[0])
		log.Printf("%v detected as down", node)

		for _, times := range s.nodeTimes {
			if strings.Index(times.Nodename, node) != -1 {
				if s.checkSend(times) {
					failure = true
					failuremsg = fmt.Sprintf("Link down found. %v", node)
					return
				}
				continue rowloop
			}
		}
		log.Printf("No node mapping found for %v",node)
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

func NewSourceSinkMonitor(store Store) monitor.Monitor {
	return &sourceSinkMonitor{
		nodeTimes: store.GetNodeTimes(),
	}
}