package monitor

import (
	"net/http"
	"encoding/json"
	"log"
	"strconv"
)

type failureRateMonitor struct {
	store Store
}

func NewResponseCode91Monitor(store Store) Monitor {
	return &failureRateMonitor{
		store: store,
	}
}

func (s failureRateMonitor) checkResponse(r *http.Response) (failure bool, failuremsg string, err error) {
	var d map[string]interface{}
	err = json.NewDecoder(r.Body).Decode(&d)
	if err != nil {
		log.Println("Error unmarshalling response")
		return
	}

	tmp, ok := d["Analysis_of_Declines"]
	if !ok {
		log.Println("No Response Code 91 body found")
		err = noResultsError{messsage:"Analysis_of_Declines not found"}
		return
	}

	declined := tmp.(map[string]interface{})
	data := declined["Data"].([]interface{})
	if len(data) == 0 {
		log.Println("No Data found in response code 91")
		err = noResultsError{messsage:"Data Length == 0"}
		return
	}
	groups := data[0].([]interface{})

	var codes []string

	if len(groups) < 2 {
		err = noResultsError{messsage:"Data Length == 2"}
		return
	}

	for _, x := range groups[2:] {
		row := x.([]interface{})
		codes = append(codes, row[4].(string))
		if row[4].(string) == "91" {
			val, err := strconv.Atoi(row[3].(string))
			if err != nil {
				continue
			}
			if val > 5 {
				log.Println("Code 91 found")
				failure = true
				failuremsg = "Code 91 Found"
			}
		}
	}
	return
}
