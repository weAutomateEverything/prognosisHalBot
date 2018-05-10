package monitor

import (
	"log"
	"strconv"
	"fmt"
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
		switch  row[4] {
		case "91", "68":
			val, err := strconv.Atoi(row[3])
			if err != nil {
				continue
			}
			if val > 5 {
				log.Printf("Code %v found",row[4])
				failure = true
				failuremsg = fmt.Sprintf("Multiple Code %v found",row[4])
				return
			}
		}
	}
	return

}
