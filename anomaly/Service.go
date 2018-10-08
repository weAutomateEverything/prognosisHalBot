package anomaly

import (
	"encoding/json"
	"fmt"
	"github.com/weAutomateEverything/anomalyDetectionHal/detector"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func NewService() Service {
	return &service{}
}

type Service interface {
	Analyse(key string, value float64) (anomlay bool, msg string, err error)
}

type service struct {
}

func (s service) Analyse(key string, value float64) (anomlay bool, msg string, err error) {
	resp, err := http.Post(os.Getenv("DETECTOR_ENDPOINT")+"/api/anomaly/"+key, "application/text",
		strings.NewReader(fmt.Sprintf("%v", value)))

	if err != nil {
		log.Println(err)
		return
	}

	defer resp.Body.Close()

	var v detector.AnomalyAddDataResponse
	err = json.NewDecoder(resp.Body).Decode(&v)

	if err != nil {
		log.Println(err)
		return
	}

	if v.Average < getAverageThreshold() {
		return
	}

	if v.AnomalyScore > getThreshold() {
		anomlay = true
		msg = v.Explination
	}

	return
}

func getThreshold() float64 {
	v := os.Getenv("ANOMALY_THRESHOLD")
	if v == "" {
		return 3
	}

	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 3
	}
	return f

}

func getAverageThreshold() float64 {
	v := os.Getenv("AVERAGE_THRESHOLD")
	if v == "" {
		return 5
	}

	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 5
	}
	return f

}
