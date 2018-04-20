package monitor

import (
	"net/http"
	"encoding/json"
	"log"
)

type failureRateMonitor struct {
	cound91  int
	endpoint string
}

func NewResponseCode91Monitor(endpoint string) Monitor {
	return &failureRateMonitor{endpoint: endpoint}
}

func (s failureRateMonitor) getEndpoint() string {
	return s.endpoint
}

func (s failureRateMonitor) checkResponse(r *http.Response) (failure bool, failuremsg string, err error) {
	var d map[string]interface{}
	err = json.NewDecoder(r.Body).Decode(&d)
	if err != nil {
		return
	}

	declined := d["Analysis_of_Declines"].(map[string]interface{})
	data := declined["Data"].([]interface{})
	groups := data[0].([]interface{})

	for i := 0; i < len(groups); i++ {
		if i < 2 {
			return
		}
		row := groups[i].([]interface{})
		if row[4].(string) == "91" {
			log.Println("Code 91 found")
			s.cound91++
			if s.cound91 == 5 {
				failure = true
				failuremsg = "5 consecutive failure code 91 found"
				return
			}
			return
		}
	}
	s.cound91 = 0
	return
}
