package close

import "testing"

func TestParseRecordsAcceptsSingleShowObject(t *testing.T) {
	records := parseRecords([]byte(`{"id":"agentops-1","status":"closed"}`))
	if len(records) != 1 || records[0].ID != "agentops-1" || records[0].Status != "closed" {
		t.Fatalf("parseRecords(single object) = %+v", records)
	}
}
