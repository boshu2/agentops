package batch

import (
	"fmt"
	"strconv"
	"strings"
)

type Record struct {
	Name   string
	Amount int
}

// Parse accepts nonblank name,nonnegative-integer rows; blank lines are ignored.
// Any malformed row invalidates the entire batch.
func Parse(input string) ([]Record, error) {
	var records []Record
	for line, row := range strings.Split(input, "\n") {
		if strings.TrimSpace(row) == "" {
			continue
		}
		fields := strings.Split(row, ",")
		if len(fields) != 2 {
			return nil, fmt.Errorf("line %d: expected name,amount", line+1)
		}
		amount, err := strconv.Atoi(strings.TrimSpace(fields[1]))
		if strings.TrimSpace(fields[0]) == "" || err != nil || amount < 0 {
			return nil, fmt.Errorf("line %d: invalid name or amount", line+1)
		}
		records = append(records, Record{strings.TrimSpace(fields[0]), amount})
	}
	return records, nil
}
