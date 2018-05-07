package monitor

import (
	"log"
	"strconv"
)

type responseCode91 struct {
}

func (s responseCode91) GetName() string {
	return "Code91"
}

func NewResponseCode91Monitor() Monitor {
	return &responseCode91{
	}
}

func (s responseCode91) CheckResponse(input [][]string) (failure bool, failuremsg string, err error) {
	var codes []string

	for _, row := range input {
		codes = append(codes, row[4])
		if row[4] == "91" {
			val, err := strconv.Atoi(row[3])
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
