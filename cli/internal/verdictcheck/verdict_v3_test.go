package verdictcheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestReadArtifactDispatchesV3AndPreservesSemantics(t *testing.T) {
	value := validVerdictV3()
	payload, digest := finalizedJSON(t, value)

	verified, err := ReadArtifact(payload, digest)
	if err != nil {
		t.Fatalf("ReadArtifact: %v", err)
	}
	if verified.SchemaVersion != "verdict.v3" || verified.V2 != nil || verified.V3 == nil {
		t.Fatalf("wrong version dispatch: %+v", verified)
	}
	if verified.V3.InvocationID != "invocation:test" ||
		verified.V3.JudgmentID != "judgment:test" ||
		verified.V3.IntentRef != ".agents/ao/intents/sha256/intent.intent" ||
		verified.V3.ProofIdentity.Epoch.String() != "1" ||
		verified.V3.SchemaDigests.CheckReceipt != strings.Repeat("f", 64) ||
		len(verified.V3.Criteria) != 1 ||
		verified.V3.Criteria[0].EvidenceReceiptDigests[0] != strings.Repeat("1", 64) {
		t.Fatalf("semantic fields were lost: %+v", verified.V3)
	}
}

func TestReadArtifactDispatchesStrictlyBySchemaVersion(t *testing.T) {
	for _, version := range []any{nil, "verdict.v1", " verdict.v3", 3} {
		value := validVerdictV3()
		value["schema_version"] = version
		payload, digest := finalizedJSON(t, value)
		if _, err := ReadArtifact(payload, digest); err == nil {
			t.Fatalf("schema_version %v unexpectedly accepted", version)
		}
	}
}

func TestVerdictV3RejectsHostileAndIncoherentArtifacts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown field", func(value map[string]any) { value["next_action"] = "retry" }},
		{"missing field", func(value map[string]any) { delete(value, "judgment_id") }},
		{"null findings", func(value map[string]any) { value["findings"] = nil }},
		{"identity collision", func(value map[string]any) { value["judgment_id"] = value["invocation_id"] }},
		{"context collision", func(value map[string]any) { value["validator_context_id"] = value["author_context_id"] }},
		{"unsupported context characters", func(value map[string]any) { value["author_context_id"] = "author context" }},
		{"absolute intent ref", func(value map[string]any) { value["intent_ref"] = "/tmp/intent" }},
		{"parent scope ref", func(value map[string]any) { value["scope_index_ref"] = "../scope.json" }},
		{"backslash effect ref", func(value map[string]any) { value["effect_receipt_ref"] = `proof\\effect.json` }},
		{"windows final ref", func(value map[string]any) { value["final_manifest_ref"] = "C:/final.json" }},
		{"negative proof epoch", func(value map[string]any) {
			value["proof_identity"].(map[string]any)["epoch"] = -1
		}},
		{"fractional proof epoch", func(value map[string]any) {
			value["proof_identity"].(map[string]any)["epoch"] = 1.5
		}},
		{"unknown proof field", func(value map[string]any) {
			value["proof_identity"].(map[string]any)["candidate"] = true
		}},
		{"invalid transition digest", func(value map[string]any) {
			value["proof_identity"].(map[string]any)["activation_transition_digest"] = "ABC"
		}},
		{"schema set drift", func(value map[string]any) {
			delete(value["schema_digests"].(map[string]any), "check_receipt")
		}},
		{"duplicate criterion id", func(value map[string]any) {
			criteria := value["criteria"].([]any)
			value["criteria"] = append(criteria, cloneMap(t, criteria[0].(map[string]any)))
		}},
		{"duplicate criterion evidence", func(value map[string]any) {
			value["criteria"].([]any)[0].(map[string]any)["evidence_receipt_digests"] =
				[]any{strings.Repeat("1", 64), strings.Repeat("1", 64)}
		}},
		{"unknown criterion field", func(value map[string]any) {
			value["criteria"].([]any)[0].(map[string]any)["evidence_ref"] = "legacy"
		}},
		{"duplicate finding id", func(value map[string]any) {
			finding := map[string]any{
				"id": "finding:test", "summary": "first",
				"evidence_receipt_digests": []any{strings.Repeat("2", 64)},
			}
			value["findings"] = []any{finding, cloneMap(t, finding)}
		}},
		{"finding without evidence", func(value map[string]any) {
			value["findings"] = []any{map[string]any{
				"id": "finding:test", "summary": "empty",
				"evidence_receipt_digests": []any{},
			}}
		}},
		{"pass unproven criterion", func(value map[string]any) {
			value["criteria"].([]any)[0].(map[string]any)["result"] = "NOT_PROVEN"
		}},
		{"pass criterion without evidence", func(value map[string]any) {
			value["criteria"].([]any)[0].(map[string]any)["evidence_receipt_digests"] = []any{}
		}},
		{"pass unchecked", func(value map[string]any) { value["not_checked"] = []any{"coverage"} }},
		{"pass missing checked", func(value map[string]any) { value["checked"] = []any{} }},
		{"invalid validated_at", func(value map[string]any) { value["validated_at"] = "2026-07-24" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := validVerdictV3()
			test.mutate(value)
			payload, digest := finalizedJSON(t, value)
			if err := VerifyArtifact(payload, digest); err == nil {
				t.Fatal("hostile verdict.v3 unexpectedly accepted")
			}
		})
	}
}

func TestVerdictV3RejectsDuplicateKeysTrailingDataAndDigestMutation(t *testing.T) {
	value := validVerdictV3()
	payload, digest := finalizedJSON(t, value)

	duplicate := strings.Replace(string(payload), `"verdict":"PASS"`, `"verdict":"FAIL","verdict":"PASS"`, 1)
	if err := VerifyArtifact([]byte(duplicate), digest); err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("duplicate key error = %v", err)
	}
	if err := VerifyArtifact(append(payload, []byte(`{"extra":true}`)...), digest); err == nil {
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
	if err := VerifyArtifact(mutatedPayload, digest); err == nil || !strings.Contains(err.Error(), "canonical content digest") {
		t.Fatalf("digest mutation error = %v", err)
	}
}

func validVerdictV3() map[string]any {
	return map[string]any{
		"schema_version": "verdict.v3",
		"invocation_id":  "invocation:test",
		"judgment_id":    "judgment:test",
		"intent_ref":     ".agents/ao/intents/sha256/intent.intent",
		"intent_digest":  strings.Repeat("a", 64),
		"proof_identity": map[string]any{
			"epoch":                        1,
			"contract_ref":                 "docs/contracts/proof-contracts/epoch-1.json",
			"contract_digest":              strings.Repeat("b", 64),
			"activation_transition_digest": strings.Repeat("c", 64),
		},
		"schema_digests": map[string]any{
			"verdict":          strings.Repeat("a", 64),
			"rpi_report":       strings.Repeat("b", 64),
			"subject_manifest": strings.Repeat("c", 64),
			"scope_index":      strings.Repeat("d", 64),
			"effect_receipt":   strings.Repeat("e", 64),
			"check_receipt":    strings.Repeat("f", 64),
		},
		"before_manifest_ref":    "proof/before.json",
		"before_manifest_digest": strings.Repeat("d", 64),
		"final_manifest_ref":     "proof/final.json",
		"final_manifest_digest":  strings.Repeat("e", 64),
		"scope_index_ref":        "proof/scope.json",
		"scope_index_digest":     strings.Repeat("f", 64),
		"effect_receipt_ref":     "proof/effect.json",
		"effect_receipt_digest":  strings.Repeat("0", 64),
		"author_context_id":      "author:test",
		"validator_context_id":   "validator:test",
		"freshness_attestation": map[string]any{
			"source": "runtime", "attester_identity": "runtime:test",
		},
		"verdict": "PASS",
		"criteria": []any{map[string]any{
			"id": "criterion:test", "result": "PASS",
			"evidence_receipt_digests": []any{strings.Repeat("1", 64)},
			"reason":                   "checked",
		}},
		"findings":     []any{},
		"checked":      []any{"go test ./..."},
		"not_checked":  []any{},
		"validated_at": "2026-07-24T12:00:00Z",
	}
}

func finalizedJSON(t *testing.T, value map[string]any) ([]byte, string) {
	t.Helper()
	delete(value, "artifact_digest")
	canonical, err := CanonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(canonical)
	digest := hex.EncodeToString(sum[:])
	value["artifact_digest"] = digest
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload, digest
}

func cloneMap(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
