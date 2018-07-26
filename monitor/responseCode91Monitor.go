package monitor

import (
	"fmt"
	"golang.org/x/net/context"
	"log"
	"strconv"
)

type responseCode91 struct {
}

func (s responseCode91) GetName() string {
	return "Code91"
}

func NewResponseCode91Monitor() Monitor {
	return &responseCode91{}
}

func (s responseCode91) CheckResponse(ctx context.Context, input [][]string) (failure bool, failuremsg string, err error) {
	var codes []string

	for _, row := range input {
		codes = append(codes, row[4])
		switch row[4] {
		case "91", "68":
			val, err := strconv.Atoi(row[3])
			if err != nil {
				continue
			}
			if val > 5 {
				log.Printf("Code %v found", row[4])
				failuremsg = fmt.Sprintf("%v instances of Code %v found", val, row[4])
				return true, failuremsg, nil
			}
		}
	}
	return

}
