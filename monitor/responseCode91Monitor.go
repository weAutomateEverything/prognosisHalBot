package monitor

import (
	"net/http"
	"encoding/json"
	"log"
)

type failureRateMonitor struct {
	store    Store
	cound91  int
	endpoint string
}

func NewResponseCode91Monitor(endpoint string, store Store) Monitor {
	return &failureRateMonitor{
		endpoint: endpoint,
		store:    store,
	}
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

	found := false
	var codes []string


	for i := 0; i < len(groups); i++ {
		if i < 2 {
			return
		}
		row := groups[i].([]interface{})
		codes = append(codes,row[4].(string))
		if row[4].(string) == "91" {
			log.Println("Code 91 found")
			s.cound91++
			found = true
		}
	}

	if !found {
		s.cound91 = 0
	}

	if s.cound91 > 5 {
		failure = true
		failuremsg = "5 consecutive failure code 91 found"
	}

	s.store.saveResponceCodeData(codes)
	return
}
