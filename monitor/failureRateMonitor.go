package monitor

import (
	"net/http"
	"encoding/json"
	"log"
	"strconv"
	"sort"
)

type responseCode91Monitor struct {
	store Store
}

func NewFailureRateMonitor(store Store) Monitor {
	return &responseCode91Monitor{
		store: store,
	}
}

func (s responseCode91Monitor) checkResponse(r *http.Response) (failure bool, failuremsg string, err error) {
	var j dataObject

	err = json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		log.Println(err.Error())
		return
	}

	if j.Approval_Vs_Declines.Data == nil {
		log.Println("No results found. No Data Element")
		err = noResultsError{messsage:"Approval_Vs_Declines.Data is nul"}
		return
	}

	if len(j.Approval_Vs_Declines.Data) == 0 {
		log.Println("No results found. Data element empty")
		err = noResultsError{messsage:"Approval_Vs_Declines.Data is empty"}
		return
	}
	x := j.Approval_Vs_Declines.Data[0]
	//If there are only 2 entreies, then we have no data
	if len(x) == 2 {
		log.Println("No results found.")
		err = noResultsError{"Approval_Vs_Declines.Data has not data elements"}
		return
	}

	result := map[string]data{}

	for _, y := range x[2:] {
		d, ok := result[y[0]]
		if !ok {
			d = data{}
		}
		s.parseRow(y, &d)
		result[y[0]] = d
	}

	var keys []string

	for key := range result {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	lastKey := keys[len(keys)-1]

	row := result[lastKey]

	log.Printf("Rate Message - ID: %v, approved: %v, failed %v, declined: %v", row.id, row.approved, row.failed, row.declined)
	s.store.saveRateData(row)

	if row.approved == 0 && row.failed > 0 {
		failuremsg = "No successful transactions found, only failed transactions"
		log.Printf(failuremsg)
		failure = true
		return
	}

	if row.failed/row.approved > 20/100 {
		failuremsg = "Failed vs Successful transactions threshold breached. There is a high number of failed transactions vs successful transactions"
		log.Printf(failuremsg)
		failure = true
		return
	}

	return
}

func (s *responseCode91Monitor) parseRow(y []string, d *data) {
	d.id = y[0]
	val, _ := strconv.Atoi(y[2])

	switch y[3] {
	case "Failed":
		d.failed = val

	case "Declined":
		d.declined = val

	case "Approved":
		d.approved = val
	}
}

type data struct {
	id                         string
	approved, declined, failed int
}

type dataObject struct {
	Approval_Vs_Declines struct {
		Data [][][]string
	}
}
