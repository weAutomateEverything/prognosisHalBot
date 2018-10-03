package monitor

import (
	"fmt"
	"golang.org/x/net/context"
	"log"
	"sort"
	"strconv"
)

type failureRateMonitor struct {
}

func (s failureRateMonitor) GetName() string {
	return "FailureRate"
}

func NewFailureRateMonitor() Monitor {
	return &failureRateMonitor{}
}

func (s failureRateMonitor) CheckResponse(ctx context.Context, input [][]string) (response []Response, err error) {
	result := map[string]data{}

	for _, y := range input {
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

	if row.approved == 0 {
		if row.failed > 0 {
			response = append(response, Response{
				FailureMsg: fmt.Sprintf("No successful transactions found, only failed transactions (%v) ", row.failed),
				Failure:    true,
			})
		}
		return
	}

	if row.failed/row.approved > 20/100 {
		response = append(response, Response{
			FailureMsg: fmt.Sprintf("There is a high number of failed transactions (%v) when compared to successful transactions (%v)", row.failed, row.approved),
			Failure:    true,
		})
		return
	}
	response = append(response, Response{})
	return
}

func (s *failureRateMonitor) parseRow(y []string, d *data) {
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
