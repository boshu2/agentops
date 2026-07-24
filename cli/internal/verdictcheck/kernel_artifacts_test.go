package verdictcheck

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelArtifactReadersRejectStructuralHostilityPerSurface(t *testing.T) {
	valid := validKernelArtifactPayloads(t)
	missingField := map[string]string{
		"subject-manifest.v2":          "entries",
		"scope-index.v1":               "criteria",
		"check-receipt.v1":             "command",
		"effect-receipt.v1":            "changes",
		"proof-contract-transition.v1": "candidate",
		"proof-identity":               "contract_ref",
		"verdict.v3":                   "criteria",
		"rpi-report.v2":                "status",
	}
	for contract, payload := range valid {
		t.Run(contract, func(t *testing.T) {
			if err := readKernelArtifact(contract, payload); err != nil {
				t.Fatalf("valid payload: %v", err)
			}

			var value map[string]any
			if err := json.Unmarshal(payload, &value); err != nil {
				t.Fatal(err)
			}
			delete(value, missingField[contract])
			missing, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			assertKernelReaderError(t, contract, missing, "missing required field")

			if err := json.Unmarshal(payload, &value); err != nil {
				t.Fatal(err)
			}
			value["next_action"] = "retry"
			unknown, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			assertKernelReaderError(t, contract, unknown, "unknown field")

			duplicateField := "schema_version"
			duplicateValue := `"duplicate"`
			if contract == "proof-identity" {
				duplicateField = "contract_ref"
				duplicateValue = `"duplicate"`
			}
			duplicate := bytes.Replace(
				payload,
				[]byte(`"`+duplicateField+`":`),
				[]byte(`"`+duplicateField+`":`+duplicateValue+`,"`+duplicateField+`":`),
				1,
			)
			assertKernelReaderError(t, contract, duplicate, "duplicate")
			assertKernelReaderError(t, contract, append(payload, []byte(`{}`)...), "trailing")

			value = nil
			if err := json.Unmarshal(payload, &value); err != nil {
				t.Fatal(err)
			}
			digestField := "artifact_digest"
			if contract == "subject-manifest.v2" {
				digestField = "canonical_manifest_digest"
			}
			if _, bindsContent := value[digestField]; bindsContent {
				value[digestField] = strings.Repeat("0", 64)
				mutated, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				assertKernelReaderError(t, contract, mutated, "digest")
			}
		})
	}
}

func validKernelArtifactPayloads(t *testing.T) map[string][]byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "tests", "fixtures", "rpi-kernel-v3", "corpus.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Class       string `json:"class"`
			Contract    string `json:"contract"`
			Expected    string `json:"expected"`
			PayloadText string `json:"payload_text"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(payload, &corpus); err != nil {
		t.Fatal(err)
	}
	result := map[string][]byte{}
	for _, testCase := range corpus.Cases {
		if testCase.Class == "artifact-reader" && testCase.Expected == "ACCEPT" {
			result[testCase.Contract] = []byte(testCase.PayloadText)
		}
	}
	if len(result) != 8 {
		t.Fatalf("valid artifact surface count = %d, want 8", len(result))
	}
	return result
}

func readKernelArtifact(contract string, payload []byte) error {
	digest := corpusArtifactDigest(payload)
	switch contract {
	case "subject-manifest.v2":
		_, err := ReadSubjectManifestV2(payload)
		return err
	case "scope-index.v1":
		_, err := ReadScopeIndexV1(payload, digest)
		return err
	case "check-receipt.v1":
		_, err := ReadCheckReceiptV1(payload, digest)
		return err
	case "effect-receipt.v1":
		_, err := ReadEffectReceiptV1(payload, digest)
		return err
	case "proof-contract-transition.v1":
		_, err := ReadProofContractTransitionV1(payload)
		return err
	case "proof-identity":
		_, err := ReadProofIdentity(payload)
		return err
	case "verdict.v3":
		_, err := ReadArtifact(payload, digest)
		return err
	case "rpi-report.v2":
		_, err := ReadRPIReportArtifact(payload, digest)
		return err
	default:
		return &unknownCorpusClass{class: contract}
	}
}

func assertKernelReaderError(t *testing.T, contract string, payload []byte, contains string) {
	t.Helper()
	err := readKernelArtifact(contract, payload)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(contains)) {
		t.Fatalf("%s error = %v, want substring %q", contract, err, contains)
	}
}
