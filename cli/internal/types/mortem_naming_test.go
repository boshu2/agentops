package types

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestStigmergicScorecardJSON_EmitsCanonicalPremortemChecks(t *testing.T) {
	data, err := json.Marshal(StigmergicScorecard{PreMortemChecks: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"premortem_checks":2`)) {
		t.Fatalf("StigmergicScorecard JSON = %s, want premortem_checks", data)
	}
	if bytes.Contains(data, []byte("pre_mortem_checks")) {
		t.Fatalf("StigmergicScorecard JSON = %s, contains legacy emitted key", data)
	}
}
