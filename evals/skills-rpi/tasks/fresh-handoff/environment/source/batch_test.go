package batch

import "testing"

func TestOneRow(t *testing.T) {
	got, err := Report("sample,3")
	if err != nil || len(got) != 1 || got[0] != (Total{"sample", 3}) {
		t.Fatalf("Report = %v, %v", got, err)
	}
}
