package batch

import "sort"

type Total struct {
	Name   string
	Amount int
}

// Summarize adds amounts by exact, case-sensitive name and sorts by name.
func Summarize(records []Record) []Total {
	values := make(map[string]int)
	for _, record := range records {
		values[record.Name] = record.Amount
	}
	result := make([]Total, 0, len(values))
	for name, amount := range values {
		result = append(result, Total{name, amount})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func Report(input string) ([]Total, error) {
	records, err := Parse(input)
	if err != nil {
		return nil, err
	}
	return Summarize(records), nil
}
