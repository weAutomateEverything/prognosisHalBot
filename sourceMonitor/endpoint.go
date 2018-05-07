package sourceMonitor

import (
	"github.com/go-kit/kit/endpoint"
	"context"
	"bufio"
	"strings"
)

func makeAddNodeHours(s Store) endpoint.Endpoint{
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(string)
		scanner := bufio.NewScanner(strings.NewReader(req))
		for scanner.Scan() {
			line := scanner.Text()
			tokens := strings.Split(line, ";")
			node := strings.ToUpper(tokens[0])
			businessHours := strings.ToUpper(tokens[1])
			if businessHours == "" {
				continue
			}
			businessCritical := tokens[2]
			afterHours := strings.ToUpper(tokens[3])
			afterHoursCritical := tokens[4]

			s.setNodeTimes(node,businessHours,businessCritical,afterHours,afterHoursCritical)

		}
		return
	}
}
