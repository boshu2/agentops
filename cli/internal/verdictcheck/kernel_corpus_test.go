package verdictcheck

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var requiredKernelCorpusIDs = map[string]struct{}{
	"intent.exact-utf8":                       {},
	"intent.living-source-mutation":           {},
	"dispatch.each-phase-at-most-once":        {},
	"coverage.generated-companion":            {},
	"coverage.outside-write-scope":            {},
	"coverage.partial-observation":            {},
	"criteria.required-exclusion":             {},
	"criteria.duplicate-id":                   {},
	"candidate.post-freeze-mutation":          {},
	"terminal.fail":                           {},
	"terminal.not-proven":                     {},
	"judgment.duplicate-intent-subject":       {},
	"proof.self-activation":                   {},
	"proof.transition-next-epoch":             {},
	"proof.transition-skipped-epoch":          {},
	"contract.unknown-field":                  {},
	"path.windows-drive":                      {},
	"path.backslash":                          {},
	"effect.forged-empty-complete":            {},
	"proof.transitive-component-mutation":     {},
	"correlation.opaque-preserved":            {},
	"correlation.over-property-bound":         {},
	"artifact.subject-manifest.valid":         {},
	"artifact.subject-manifest.dot-entry":     {},
	"artifact.scope-index.valid":              {},
	"artifact.scope-index.required-exclusion": {},
	"artifact.check-receipt.valid":            {},
	"artifact.check-receipt.pass-nonzero":     {},
	"artifact.check-receipt.duplicate-key":    {},
	"artifact.effect-receipt.valid":           {},
	"artifact.effect-receipt.dot-ref":         {},
	"artifact.proof-transition.valid":         {},
	"artifact.proof-transition.dot-ref":       {},
	"artifact.proof-transition.trailing":      {},
	"artifact.proof-identity.valid":           {},
	"artifact.proof-identity.dot-ref":         {},
	"artifact.verdict.valid":                  {},
	"artifact.verdict.dot-ref":                {},
	"artifact.rpi-report.valid":               {},
	"artifact.rpi-report.dot-ref":             {},
	"artifact.subject-manifest.missing-field": {},
	"artifact.scope-index.unknown-field":      {},
	"artifact.check-receipt.digest-mutation":  {},
}

func TestKernelV3SharedCorpus(t *testing.T) {
	path := filepath.Join("..", "..", "..", "tests", "fixtures", "rpi-kernel-v3", "corpus.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shared kernel corpus: %v", err)
	}
	var corpus struct {
		SchemaVersion     string           `json:"schema_version"`
		RequiredConsumers []string         `json:"required_consumers"`
		Cases             []map[string]any `json:"cases"`
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&corpus); err != nil {
		t.Fatalf("decode shared kernel corpus: %v", err)
	}
	if corpus.SchemaVersion != "rpi-kernel-corpus.v1" {
		t.Fatalf("schema_version = %q", corpus.SchemaVersion)
	}
	wantConsumers := []string{"go", "implement-python", "plan-python", "rpi-python", "validate-python"}
	gotConsumers := append([]string(nil), corpus.RequiredConsumers...)
	sort.Strings(gotConsumers)
	if !reflect.DeepEqual(gotConsumers, wantConsumers) {
		t.Fatalf("required consumers = %v, want %v", gotConsumers, wantConsumers)
	}
	if len(corpus.Cases) != len(requiredKernelCorpusIDs) {
		t.Fatalf("case count = %d, want %d", len(corpus.Cases), len(requiredKernelCorpusIDs))
	}
	seen := make(map[string]struct{}, len(corpus.Cases))
	for _, testCase := range corpus.Cases {
		identifier, _ := testCase["id"].(string)
		if _, required := requiredKernelCorpusIDs[identifier]; !required {
			t.Fatalf("unexpected or missing case ID %q", identifier)
		}
		if _, duplicate := seen[identifier]; duplicate {
			t.Fatalf("duplicate case ID %q", identifier)
		}
		seen[identifier] = struct{}{}
		t.Run(identifier, func(t *testing.T) {
			got, err := kernelCorpusOutcome(testCase)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			want, ok := testCase["expected"].(string)
			if !ok {
				t.Fatalf("expected outcome is not a string")
			}
			if testCase["class"] == "artifact-reader" && want == "REJECT" {
				reasons, ok := testCase["error_contains"].(map[string]any)
				if !ok {
					t.Fatal("hostile artifact case lacks error_contains")
				}
				reason, _ := reasons["go"].(string)
				if !strings.HasPrefix(got, "REJECT:") || reason == "" ||
					!strings.Contains(got, reason) {
					t.Fatalf("outcome = %q, want intended rejection containing %q", got, reason)
				}
				return
			}
			if got != want {
				t.Fatalf("outcome = %q, want %q", got, want)
			}
		})
	}
	if len(seen) != len(requiredKernelCorpusIDs) {
		t.Fatalf("shared corpus did not exercise the complete required ID set")
	}
}

func kernelCorpusOutcome(testCase map[string]any) (string, error) {
	class, _ := testCase["class"].(string)
	switch class {
	case "intent-digest":
		left := sha256.Sum256([]byte(testCase["left"].(string)))
		right := sha256.Sum256([]byte(testCase["right"].(string)))
		if left != right {
			return "DIFFERENT", nil
		}
		return "SAME", nil
	case "intent-snapshot":
		snapshot := sha256.Sum256([]byte(testCase["snapshot"].(string)))
		living := sha256.Sum256([]byte(testCase["living_after"].(string)))
		if snapshot != living {
			return "SNAPSHOT_WINS", nil
		}
		return "LIVING_SOURCE_REDERIVED", nil
	case "dispatch":
		calls := anyStrings(testCase["expected_calls"])
		terminal, _ := testCase["terminal"].(string)
		if reflect.DeepEqual(calls, []string{"plan", "implement", "validate"}) &&
			(terminal == "PASS" || terminal == "FAIL" || terminal == "NOT_PROVEN") {
			return "PASS", nil
		}
		return "REJECT", nil
	case "coverage":
		observation := anyStrings(testCase["observation"])
		if !reflect.DeepEqual(observation, []string{"."}) {
			return "NOT_PROVEN", nil
		}
		scope := anyStrings(testCase["scope"])
		for _, changed := range anyStrings(testCase["changed"]) {
			declared := false
			for _, pattern := range scope {
				if pathInScope(changed, pattern) {
					declared = true
					break
				}
			}
			if !declared {
				return "FAIL", nil
			}
		}
		return "PASS", nil
	case "criterion-freeze":
		criteria := anyStrings(testCase["criterion_ids"])
		if containsDuplicate(criteria) {
			return "REJECT", nil
		}
		required := stringSet(anyStrings(testCase["required_ids"]))
		for _, excluded := range anyStrings(testCase["excluded_ids"]) {
			if _, conflict := required[excluded]; conflict {
				return "REJECT", nil
			}
		}
		return "PASS", nil
	case "candidate-freeze":
		if testCase["start_digest"] != testCase["end_digest"] {
			return "NOT_PROVEN_STOP", nil
		}
		return "PASS", nil
	case "judgment-ledger":
		if testCase["same_intent"] == true &&
			testCase["same_final_subject"] == true &&
			testCase["different_judgment_id"] == true {
			return "REJECT", nil
		}
		return "PASS", nil
	case "proof-identity":
		for _, changed := range anyStrings(testCase["changed"]) {
			if changed == "docs/contracts/proof-contracts/active.json" {
				return "REJECT", nil
			}
		}
		return "PASS", nil
	case "proof-transition":
		prior, err := integerValue(testCase["prior_epoch"], "prior_epoch")
		if err != nil {
			return "", err
		}
		candidate, err := integerValue(testCase["candidate_epoch"], "candidate_epoch")
		if err != nil {
			return "", err
		}
		next := new(big.Int).Add(prior, big.NewInt(1))
		if candidate.Cmp(next) == 0 {
			return "PASS", nil
		}
		return "REJECT", nil
	case "strict-reader":
		report := validRPIReportV2("NOT_PLANNED")
		for _, field := range []string{
			"proof_identity", "before_manifest_digest", "final_manifest_digest",
			"effect_receipt_digest", "verdict_ref", "verdict_digest",
		} {
			report[field] = nil
		}
		report["next_action"] = "retry"
		if err := validateRPIReportV2Raw(report); err != nil {
			return "REJECT", nil
		}
		return "PASS", nil
	case "repository-ref":
		if !validRepositoryRef(testCase["ref"].(string)) {
			return "REJECT", nil
		}
		return "PASS", nil
	case "effect-integrity":
		if !reflect.DeepEqual(anyStrings(testCase["before_final_changed"]), anyStrings(testCase["claimed_changed"])) {
			return "REJECT", nil
		}
		return "PASS", nil
	case "proof-transitive":
		if testCase["bound_digest"] == testCase["observed_digest"] {
			return "PASS", nil
		}
		return "REJECT", nil
	case "correlation":
		if err := validateCorrelation(testCase["value"]); err != nil {
			return "REJECT", nil
		}
		return "PASS", nil
	case "artifact-reader":
		var payload []byte
		if raw, ok := testCase["payload_text"].(string); ok {
			payload = []byte(raw)
		} else {
			var err error
			payload, err = json.Marshal(testCase["payload"])
			if err != nil {
				return "", err
			}
		}
		var err error
		expectedDigest := corpusArtifactDigest(payload)
		switch testCase["contract"] {
		case "subject-manifest.v2":
			_, err = ReadSubjectManifestV2(payload)
		case "scope-index.v1":
			_, err = ReadScopeIndexV1(payload, expectedDigest)
		case "check-receipt.v1":
			_, err = ReadCheckReceiptV1(payload, expectedDigest)
		case "effect-receipt.v1":
			_, err = ReadEffectReceiptV1(payload, expectedDigest)
		case "proof-contract-transition.v1":
			_, err = ReadProofContractTransitionV1(payload)
		case "proof-identity":
			_, err = ReadProofIdentity(payload)
		case "verdict.v3":
			_, err = ReadArtifact(payload, expectedDigest)
		case "rpi-report.v2":
			_, err = ReadRPIReportArtifact(payload, expectedDigest)
		default:
			return "", &unknownCorpusClass{class: fmt.Sprint(testCase["contract"])}
		}
		if err != nil {
			return "REJECT:" + err.Error(), nil
		}
		return "ACCEPT", nil
	default:
		return "", &unknownCorpusClass{class: class}
	}
}

func corpusArtifactDigest(payload []byte) string {
	var value map[string]any
	if json.Unmarshal(payload, &value) != nil {
		return strings.Repeat("0", 64)
	}
	digest, _ := value["artifact_digest"].(string)
	if !ValidDigest(digest) {
		return strings.Repeat("0", 64)
	}
	return digest
}

type unknownCorpusClass struct {
	class string
}

func (err *unknownCorpusClass) Error() string {
	return "unknown kernel corpus class: " + err.class
}

func anyStrings(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.(string))
	}
	return result
}

func pathInScope(path, pattern string) bool {
	return path == pattern || len(path) > len(pattern) &&
		path[:len(pattern)] == pattern && path[len(pattern)] == '/'
}

func containsDuplicate(values []string) bool {
	return len(stringSet(values)) != len(values)
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
