// Tests binding LedgerReaderVersion to the payload-hash fieldset shape
// (verification-surface-honesty S4): the reader capability level must bump
// whenever the fieldset feeding payload_hash changes, so the installed hook's
// version-floor probe catches reader/writer skew MECHANICALLY instead of the
// operator meeting it as a false "broken chain". The binding is enforced by
// test, not by comment.
package provenancegraph

import (
	"reflect"
	"testing"
)

// TestLedgerReaderVersion_BoundToPayloadHashFieldset freezes the exact
// edgePayload fieldset for the CURRENT LedgerReaderVersion. If this test
// fails you changed the payload-hash fieldset: bump LedgerReaderVersion (and
// the installed-hook floor with it — see the const's doc), then register the
// new fieldset under the new level here. Editing the frozen list WITHOUT
// bumping the level recreates the 2026-07-10 skew false-alarm.
func TestLedgerReaderVersion_BoundToPayloadHashFieldset(t *testing.T) {
	frozen := map[int][]string{
		1: {
			"degraded",
			"duration_s",
			"evidence_path",
			"evidence_ref",
			"from_id",
			"from_type",
			"relation",
			"reviewer_family",
			"rounds",
			"schema_version",
			"to_id",
			"to_type",
			"tokens_est",
			"trust_tier",
			"ts",
		},
	}
	want, ok := frozen[LedgerReaderVersion]
	if !ok {
		t.Fatalf("no frozen payload-hash fieldset registered for LedgerReaderVersion %d — register the new level's fieldset in this test WITH the version bump", LedgerReaderVersion)
	}
	got := payloadHashFieldset()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("payload-hash fieldset changed without a LedgerReaderVersion bump:\n got %v\nwant %v\nBump LedgerReaderVersion + the installed-hook floor together, then freeze the new fieldset here (see reader_version.go)", got, want)
	}
}
