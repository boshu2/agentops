package batch

import (
	"reflect"
	"testing"
)

func TestEvalRejectsMalformedBatchWithoutPartialData(t *testing.T) {
	for _, row := range []string{"a,no", "a,-1", " ,4", "a,9999999999999999999999999999999999", "a,2,3"} {
		records, err := Parse("good,1\n" + row)
		if err == nil || len(records) != 0 {
			t.Errorf("Parse(%q) = %v, %v", row, records, err)
		}
		if report, err := Report("good,1\n" + row); err == nil || len(report) != 0 {
			t.Errorf("Report(%q) = %v, %v", row, report, err)
		}
	}
}

func TestEvalIntegratesBothChanges(t *testing.T) {
	got, err := Report(" beta ,2\nalpha,1\nbeta,3\nAlpha,0\n\n")
	want := []Total{{"Alpha", 0}, {"alpha", 1}, {"beta", 5}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Report = %v, %v; want %v", got, err, want)
	}
	rows := []Record{{"z", 4}, {"a", 1}, {"z", 3}}
	before := append([]Record(nil), rows...)
	Summarize(rows)
	if !reflect.DeepEqual(rows, before) {
		t.Fatal("summary mutated caller records")
	}
}
