package verdictcheck

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReadRPIReportArtifactPreservesOpaqueCorrelationAndBindings(t *testing.T) {
	value := validRPIReportV2("PASS")
	payload, digest := finalizedJSON(t, value)
	report, err := ReadRPIReportArtifact(payload, digest)
	if err != nil {
		t.Fatalf("ReadRPIReportArtifact: %v", err)
	}
	if report.Status != "PASS" ||
		report.Correlation["goal_id"] != "goal:123" ||
		report.Correlation["experiment_id"] != "experiment:456" ||
		report.IntentDigest == nil || *report.IntentDigest != strings.Repeat("a", 64) ||
		report.ProofIdentity == nil || report.ProofIdentity.Epoch.String() != "1" ||
		report.VerdictDigest == nil || *report.VerdictDigest != strings.Repeat("1", 64) {
		t.Fatalf("report fields were lost or interpreted: %+v", report)
	}
}

func TestRPIReportV2AcceptsReportOnlyWithoutProofClaims(t *testing.T) {
	for _, status := range []string{"NOT_PLANNED", "NOT_BUILT"} {
		value := validRPIReportV2(status)
		for _, field := range []string{
			"proof_identity", "before_manifest_digest", "final_manifest_digest",
			"effect_receipt_digest", "verdict_ref", "verdict_digest",
		} {
			value[field] = nil
		}
		// Report-only outcomes may still identify the intent that was not
		// planned or built; they may not claim subject/verdict proof.
		payload, digest := finalizedJSON(t, value)
		if err := VerifyRPIReportArtifact(payload, digest); err != nil {
			t.Fatalf("%s report-only artifact rejected: %v", status, err)
		}
	}
}

func TestRPIReportV2RejectsHostileContinuationAndBindingClaims(t *testing.T) {
	tests := []struct {
		name   string
		status string
		mutate func(map[string]any)
	}{
		{"next action", "PASS", func(value map[string]any) { value["next_action"] = "retry" }},
		{"missing field", "PASS", func(value map[string]any) { delete(value, "correlation") }},
		{"null checked", "PASS", func(value map[string]any) { value["checked"] = nil }},
		{"absolute intent ref", "PASS", func(value map[string]any) { value["intent_ref"] = "/tmp/intent" }},
		{"parent verdict ref", "PASS", func(value map[string]any) { value["verdict_ref"] = "../verdict.json" }},
		{"windows verdict ref", "PASS", func(value map[string]any) { value["verdict_ref"] = `C:\\verdict.json` }},
		{"semantic missing intent", "PASS", func(value map[string]any) { value["intent_ref"] = nil }},
		{"semantic missing proof", "FAIL", func(value map[string]any) { value["proof_identity"] = nil }},
		{"semantic missing verdict", "NOT_PROVEN", func(value map[string]any) { value["verdict_digest"] = nil }},
		{"report-only subject claim", "NOT_PLANNED", func(value map[string]any) {
			value["proof_identity"] = nil
			value["effect_receipt_digest"] = strings.Repeat("0", 64)
		}},
		{"correlation over property bound", "PASS", func(value map[string]any) {
			correlation := map[string]any{}
			for index := 0; index < 9; index++ {
				correlation["key"+string(rune('a'+index))] = "value"
			}
			value["correlation"] = correlation
		}},
		{"correlation invalid key", "PASS", func(value map[string]any) {
			value["correlation"] = map[string]any{"goal id": "value"}
		}},
		{"correlation value over bound", "PASS", func(value map[string]any) {
			value["correlation"] = map[string]any{"goal": strings.Repeat("x", 257)}
		}},
		{"correlation byte over bound", "PASS", func(value map[string]any) {
			correlation := map[string]any{}
			for index := 0; index < 8; index++ {
				correlation["key"+string(rune('a'+index))] = strings.Repeat("界", 256)
			}
			value["correlation"] = correlation
		}},
		{"invalid proof ref", "PASS", func(value map[string]any) {
			value["proof_identity"].(map[string]any)["contract_ref"] = "proof/../contract.json"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := validRPIReportV2(test.status)
			if test.status == "NOT_PLANNED" || test.status == "NOT_BUILT" {
				for _, field := range []string{
					"proof_identity", "before_manifest_digest", "final_manifest_digest",
					"effect_receipt_digest", "verdict_ref", "verdict_digest",
				} {
					value[field] = nil
				}
			}
			test.mutate(value)
			payload, digest := finalizedJSON(t, value)
			if err := VerifyRPIReportArtifact(payload, digest); err == nil {
				t.Fatal("hostile rpi-report.v2 unexpectedly accepted")
			}
		})
	}
}

func TestRPIReportV2RejectsDuplicateTrailingAndDigestMutation(t *testing.T) {
	value := validRPIReportV2("PASS")
	payload, digest := finalizedJSON(t, value)

	duplicate := strings.Replace(string(payload), `"status":"PASS"`, `"status":"FAIL","status":"PASS"`, 1)
	if err := VerifyRPIReportArtifact([]byte(duplicate), digest); err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("duplicate key error = %v", err)
	}
	if err := VerifyRPIReportArtifact(append(payload, []byte(`{}`)...), digest); err == nil {
		t.Fatal("trailing JSON unexpectedly accepted")
	}
	var mutated map[string]any
	if err := json.Unmarshal(payload, &mutated); err != nil {
		t.Fatal(err)
	}
	mutated["checked"] = []any{"mutated"}
	mutatedPayload, err := json.Marshal(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyRPIReportArtifact(mutatedPayload, digest); err == nil || !strings.Contains(err.Error(), "canonical content digest") {
		t.Fatalf("digest mutation error = %v", err)
	}
}

func validRPIReportV2(status string) map[string]any {
	return map[string]any{
		"schema_version": "rpi-report.v2",
		"invocation_id":  "invocation:test",
		"correlation": map[string]any{
			"goal_id": "goal:123", "experiment_id": "experiment:456",
		},
		"status":        status,
		"intent_ref":    ".agents/ao/intents/sha256/intent.intent",
		"intent_digest": strings.Repeat("a", 64),
		"proof_identity": map[string]any{
			"epoch":                        1,
			"contract_ref":                 "docs/contracts/proof-contracts/epoch-1.json",
			"contract_digest":              strings.Repeat("b", 64),
			"activation_transition_digest": strings.Repeat("c", 64),
		},
		"before_manifest_digest": strings.Repeat("d", 64),
		"final_manifest_digest":  strings.Repeat("e", 64),
		"effect_receipt_digest":  strings.Repeat("f", 64),
		"verdict_ref":            ".agents/ao/verdicts/sha256/verdict.json",
		"verdict_digest":         strings.Repeat("1", 64),
		"checked":                []any{"verdict.v3"},
		"not_checked":            []any{},
	}
}
