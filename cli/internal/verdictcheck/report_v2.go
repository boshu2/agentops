package verdictcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// RPIReportV2 is the terminal, continuation-free report emitted by the exact
// RPI kernel. Correlation is opaque caller data; the reader bounds and
// preserves it but never interprets it.
type RPIReportV2 struct {
	SchemaVersion        string            `json:"schema_version"`
	InvocationID         string            `json:"invocation_id"`
	Correlation          map[string]string `json:"correlation"`
	Status               string            `json:"status"`
	IntentRef            *string           `json:"intent_ref"`
	IntentDigest         *string           `json:"intent_digest"`
	ProofIdentity        *ProofIdentity    `json:"proof_identity"`
	BeforeManifestDigest *string           `json:"before_manifest_digest"`
	FinalManifestDigest  *string           `json:"final_manifest_digest"`
	EffectReceiptDigest  *string           `json:"effect_receipt_digest"`
	VerdictRef           *string           `json:"verdict_ref"`
	VerdictDigest        *string           `json:"verdict_digest"`
	Checked              []string          `json:"checked"`
	NotChecked           []string          `json:"not_checked"`
	ArtifactDigest       string            `json:"artifact_digest"`
}

var rpiReportV2Fields = []string{
	"schema_version",
	"invocation_id",
	"correlation",
	"status",
	"intent_ref",
	"intent_digest",
	"proof_identity",
	"before_manifest_digest",
	"final_manifest_digest",
	"effect_receipt_digest",
	"verdict_ref",
	"verdict_digest",
	"checked",
	"not_checked",
	"artifact_digest",
}

// VerifyRPIReportArtifact verifies a stored rpi-report.v2 payload against its
// content-addressed filename digest.
func VerifyRPIReportArtifact(payload []byte, expectedDigest string) error {
	_, err := ReadRPIReportArtifact(payload, expectedDigest)
	return err
}

// ReadRPIReportArtifact strictly verifies and returns an rpi-report.v2. It
// rejects duplicate keys, trailing data, unknown or missing fields, hostile
// references, invalid proof bindings, and any continuation field.
func ReadRPIReportArtifact(payload []byte, expectedDigest string) (*RPIReportV2, error) {
	raw, err := decodeStrictJSONObject(payload, "rpi-report")
	if err != nil {
		return nil, err
	}
	if version, ok := raw["schema_version"].(string); !ok || version != "rpi-report.v2" {
		return nil, fmt.Errorf("unsupported rpi-report schema_version %q", raw["schema_version"])
	}
	if err := validateRPIReportV2Raw(raw); err != nil {
		return nil, err
	}
	var report RPIReportV2
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("invalid rpi-report.v2 shape: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, err
	}
	if err := verifyCanonicalArtifact(raw, report.ArtifactDigest, expectedDigest, "rpi-report.v2"); err != nil {
		return nil, err
	}
	return &report, nil
}

func validateRPIReportV2Raw(raw map[string]any) error {
	if err := requireExactFields(raw, rpiReportV2Fields, "rpi-report.v2"); err != nil {
		return err
	}
	if raw["schema_version"] != "rpi-report.v2" {
		return fmt.Errorf("rpi-report.v2 schema_version is invalid")
	}
	if _, err := requireID(raw["invocation_id"], "rpi-report.v2 invocation_id"); err != nil {
		return err
	}
	if err := validateCorrelation(raw["correlation"]); err != nil {
		return err
	}
	status, err := stringValue(raw["status"], "rpi-report.v2 status", true)
	if err != nil {
		return err
	}
	switch status {
	case "PASS", "FAIL", "NOT_PROVEN", "NOT_PLANNED", "NOT_BUILT":
	default:
		return fmt.Errorf("rpi-report.v2 status is invalid")
	}

	for _, field := range []string{
		"intent_digest",
		"before_manifest_digest",
		"final_manifest_digest",
		"effect_receipt_digest",
		"verdict_digest",
		"artifact_digest",
	} {
		if raw[field] != nil {
			if _, err := requireDigest(raw[field], "rpi-report.v2 "+field); err != nil {
				return err
			}
		} else if field == "artifact_digest" {
			return fmt.Errorf("rpi-report.v2 artifact_digest cannot be null")
		}
	}
	for _, field := range []string{"intent_ref", "verdict_ref"} {
		if raw[field] != nil {
			if _, err := requireRepositoryRef(raw[field], "rpi-report.v2 "+field); err != nil {
				return err
			}
		}
	}
	if raw["proof_identity"] != nil {
		if err := validateProofIdentityRaw(raw["proof_identity"], "rpi-report.v2 proof_identity"); err != nil {
			return err
		}
	}
	if _, err := validateStringArray(raw["checked"], "rpi-report.v2 checked"); err != nil {
		return err
	}
	if _, err := validateStringArray(raw["not_checked"], "rpi-report.v2 not_checked"); err != nil {
		return err
	}

	semantic := status == "PASS" || status == "FAIL" || status == "NOT_PROVEN"
	semanticFields := []string{
		"intent_ref",
		"intent_digest",
		"proof_identity",
		"before_manifest_digest",
		"final_manifest_digest",
		"effect_receipt_digest",
		"verdict_ref",
		"verdict_digest",
	}
	if semantic {
		for _, field := range semanticFields {
			if raw[field] == nil {
				return fmt.Errorf("semantic rpi-report.v2 requires complete durable bindings")
			}
		}
	} else {
		for _, field := range []string{
			"proof_identity",
			"before_manifest_digest",
			"final_manifest_digest",
			"effect_receipt_digest",
			"verdict_ref",
			"verdict_digest",
		} {
			if raw[field] != nil {
				return fmt.Errorf("report-only rpi-report.v2 cannot claim subject or verdict proof")
			}
		}
	}
	return nil
}

func validateCorrelation(value any) error {
	if value == nil {
		return nil
	}
	correlation, err := objectValue(value, "rpi-report.v2 correlation")
	if err != nil {
		return fmt.Errorf("rpi-report.v2 correlation is not a bounded object")
	}
	if len(correlation) > 8 {
		return fmt.Errorf("rpi-report.v2 correlation is not a bounded object")
	}
	for key, rawValue := range correlation {
		if !validID(key) || runeLength(key) > 64 {
			return fmt.Errorf("rpi-report.v2 correlation key is invalid")
		}
		text, ok := rawValue.(string)
		if !ok || runeLength(text) > 256 {
			return fmt.Errorf("rpi-report.v2 correlation entry exceeds bounds")
		}
	}
	canonical, err := CanonicalJSON(correlation)
	if err != nil {
		return fmt.Errorf("canonicalize rpi-report.v2 correlation: %w", err)
	}
	if len(canonical) > 2048 {
		return fmt.Errorf("rpi-report.v2 correlation exceeds byte bound")
	}
	return nil
}

func decodeStrictJSONObject(payload []byte, label string) (map[string]any, error) {
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid %s JSON: %w", label, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, err
	}
	if duplicate, err := duplicateKey(payload); err != nil {
		return nil, fmt.Errorf("invalid %s JSON: %w", label, err)
	} else if duplicate != "" {
		return nil, fmt.Errorf("%s contains duplicate key %q", label, duplicate)
	}
	return raw, nil
}
