package verdictcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// VerdictV3 mirrors the complete verdict.v3 artifact. Unlike the legacy v2
// type, every runtime identity is mandatory and evidence is bound by typed
// receipt digests.
type VerdictV3 struct {
	SchemaVersion        string        `json:"schema_version"`
	InvocationID         string        `json:"invocation_id"`
	JudgmentID           string        `json:"judgment_id"`
	IntentRef            string        `json:"intent_ref"`
	IntentDigest         string        `json:"intent_digest"`
	ProofIdentity        ProofIdentity `json:"proof_identity"`
	SchemaDigests        SchemaDigests `json:"schema_digests"`
	BeforeManifestRef    string        `json:"before_manifest_ref"`
	BeforeManifestDigest string        `json:"before_manifest_digest"`
	FinalManifestRef     string        `json:"final_manifest_ref"`
	FinalManifestDigest  string        `json:"final_manifest_digest"`
	ScopeIndexRef        string        `json:"scope_index_ref"`
	ScopeIndexDigest     string        `json:"scope_index_digest"`
	EffectReceiptRef     string        `json:"effect_receipt_ref"`
	EffectReceiptDigest  string        `json:"effect_receipt_digest"`
	AuthorContextID      string        `json:"author_context_id"`
	ValidatorContextID   string        `json:"validator_context_id"`
	FreshnessAttestation Freshness     `json:"freshness_attestation"`
	Verdict              string        `json:"verdict"`
	Criteria             []CriterionV3 `json:"criteria"`
	Findings             []FindingV3   `json:"findings"`
	Checked              []string      `json:"checked"`
	NotChecked           []string      `json:"not_checked"`
	ValidatedAt          string        `json:"validated_at"`
	ArtifactDigest       string        `json:"artifact_digest"`
}

// ProofIdentity binds a judgment to the activated proof epoch.
type ProofIdentity struct {
	Epoch                      json.Number `json:"epoch"`
	ContractRef                string      `json:"contract_ref"`
	ContractDigest             string      `json:"contract_digest"`
	ActivationTransitionDigest *string     `json:"activation_transition_digest"`
}

// SchemaDigests binds all schemas used by a verdict.v3 writer.
type SchemaDigests struct {
	Verdict         string `json:"verdict"`
	RPIReport       string `json:"rpi_report"`
	SubjectManifest string `json:"subject_manifest"`
	ScopeIndex      string `json:"scope_index"`
	EffectReceipt   string `json:"effect_receipt"`
	CheckReceipt    string `json:"check_receipt"`
}

// CriterionV3 is one stable criterion judgment.
type CriterionV3 struct {
	ID                     string   `json:"id"`
	Result                 string   `json:"result"`
	EvidenceReceiptDigests []string `json:"evidence_receipt_digests"`
	Reason                 string   `json:"reason"`
}

// FindingV3 is one evidence-backed finding.
type FindingV3 struct {
	ID                     string   `json:"id"`
	Summary                string   `json:"summary"`
	EvidenceReceiptDigests []string `json:"evidence_receipt_digests"`
}

var verdictV3Fields = []string{
	"schema_version",
	"invocation_id",
	"judgment_id",
	"intent_ref",
	"intent_digest",
	"proof_identity",
	"schema_digests",
	"before_manifest_ref",
	"before_manifest_digest",
	"final_manifest_ref",
	"final_manifest_digest",
	"scope_index_ref",
	"scope_index_digest",
	"effect_receipt_ref",
	"effect_receipt_digest",
	"author_context_id",
	"validator_context_id",
	"freshness_attestation",
	"verdict",
	"criteria",
	"findings",
	"checked",
	"not_checked",
	"validated_at",
	"artifact_digest",
}

func readVerdictV3(payload []byte, raw map[string]any, expectedDigest string) (*VerdictV3, error) {
	if err := validateVerdictV3Raw(raw); err != nil {
		return nil, err
	}
	var artifact VerdictV3
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(&artifact); err != nil {
		return nil, fmt.Errorf("invalid verdict.v3 shape: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, err
	}
	if err := verifyCanonicalArtifact(raw, artifact.ArtifactDigest, expectedDigest, "verdict.v3"); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func validateVerdictV3Raw(raw map[string]any) error {
	if err := requireExactFields(raw, verdictV3Fields, "verdict.v3"); err != nil {
		return err
	}
	if raw["schema_version"] != "verdict.v3" {
		return fmt.Errorf("verdict.v3 schema_version is invalid")
	}
	if err := validateVerdictV3BindingsRaw(raw); err != nil {
		return err
	}
	result, err := validateVerdictResult(raw["verdict"])
	if err != nil {
		return err
	}
	criteria, err := validateCriteriaV3Raw(raw["criteria"])
	if err != nil {
		return err
	}
	if err := validateFindingsV3Raw(raw["findings"]); err != nil {
		return err
	}
	checked, err := validateStringArray(raw["checked"], "verdict.v3 checked")
	if err != nil {
		return err
	}
	notChecked, err := validateStringArray(raw["not_checked"], "verdict.v3 not_checked")
	if err != nil {
		return err
	}
	if err := validateRFC3339(raw["validated_at"], "verdict.v3 validated_at"); err != nil {
		return err
	}
	return validateVerdictPassCoherence(result, criteria, checked, notChecked)
}

func validateVerdictV3BindingsRaw(raw map[string]any) error {
	invocationID, err := requireID(raw["invocation_id"], "verdict.v3 invocation_id")
	if err != nil {
		return err
	}
	judgmentID, err := requireID(raw["judgment_id"], "verdict.v3 judgment_id")
	if err != nil {
		return err
	}
	if invocationID == judgmentID {
		return fmt.Errorf("invocation and judgment identities must be distinct")
	}
	for _, field := range []string{
		"intent_ref",
		"before_manifest_ref",
		"final_manifest_ref",
		"scope_index_ref",
		"effect_receipt_ref",
	} {
		if _, err := requireRepositoryRef(raw[field], "verdict.v3 "+field); err != nil {
			return err
		}
	}
	for _, field := range []string{
		"intent_digest",
		"before_manifest_digest",
		"final_manifest_digest",
		"scope_index_digest",
		"effect_receipt_digest",
		"artifact_digest",
	} {
		if _, err := requireDigest(raw[field], "verdict.v3 "+field); err != nil {
			return err
		}
	}
	if err := validateProofIdentityRaw(raw["proof_identity"], "verdict.v3 proof_identity"); err != nil {
		return err
	}
	if err := validateSchemaDigestsRaw(raw["schema_digests"]); err != nil {
		return err
	}
	author, err := requireID(raw["author_context_id"], "verdict.v3 author_context_id")
	if err != nil {
		return err
	}
	validator, err := requireID(raw["validator_context_id"], "verdict.v3 validator_context_id")
	if err != nil {
		return err
	}
	if author == validator {
		return fmt.Errorf("verdict.v3 author and validator contexts collide")
	}
	if err := validateFreshnessRaw(raw["freshness_attestation"]); err != nil {
		return err
	}
	return nil
}

func validateVerdictResult(value any) (string, error) {
	result, err := stringValue(value, "verdict.v3 verdict", true)
	if err != nil {
		return "", err
	}
	if result != "PASS" && result != "FAIL" && result != "NOT_PROVEN" {
		return "", fmt.Errorf("verdict.v3 verdict is invalid")
	}
	return result, nil
}

func validateVerdictPassCoherence(result string, criteria []rawCriterionV3, checked, notChecked []string) error {
	if result != "PASS" {
		return nil
	}
	for _, criterion := range criteria {
		if criterion.result == "EXCLUDED" {
			continue
		}
		if criterion.result != "PASS" || len(criterion.evidence) == 0 {
			return fmt.Errorf("verdict.v3 PASS requires evidence-backed PASS for every non-excluded criterion")
		}
	}
	if len(notChecked) != 0 {
		return fmt.Errorf("verdict.v3 PASS cannot contain not_checked items")
	}
	if len(checked) == 0 {
		return fmt.Errorf("verdict.v3 PASS requires nonempty checked evidence")
	}
	return nil
}

type rawCriterionV3 struct {
	result   string
	evidence []string
}

func validateCriteriaV3Raw(value any) ([]rawCriterionV3, error) {
	items, err := arrayValue(value, "verdict.v3 criteria")
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("verdict.v3 criteria must be nonempty")
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]rawCriterionV3, 0, len(items))
	for index, item := range items {
		label := fmt.Sprintf("verdict.v3 criteria[%d]", index)
		criterion, err := objectValue(item, label)
		if err != nil {
			return nil, err
		}
		if err := requireExactFields(criterion, []string{"id", "result", "evidence_receipt_digests", "reason"}, label); err != nil {
			return nil, err
		}
		identifier, err := requireID(criterion["id"], label+".id")
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[identifier]; duplicate {
			return nil, fmt.Errorf("verdict.v3 criterion IDs must be unique")
		}
		seen[identifier] = struct{}{}
		criterionResult, err := stringValue(criterion["result"], label+".result", true)
		if err != nil {
			return nil, err
		}
		switch criterionResult {
		case "PASS", "FAIL", "NOT_PROVEN", "EXCLUDED":
		default:
			return nil, fmt.Errorf("%s.result is invalid", label)
		}
		evidence, err := validateDigestArray(criterion["evidence_receipt_digests"], label+".evidence_receipt_digests", false)
		if err != nil {
			return nil, err
		}
		if _, err := stringValue(criterion["reason"], label+".reason", false); err != nil {
			return nil, err
		}
		result = append(result, rawCriterionV3{result: criterionResult, evidence: evidence})
	}
	return result, nil
}

func validateFindingsV3Raw(value any) error {
	items, err := arrayValue(value, "verdict.v3 findings")
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		label := fmt.Sprintf("verdict.v3 findings[%d]", index)
		finding, err := objectValue(item, label)
		if err != nil {
			return err
		}
		if err := requireExactFields(finding, []string{"id", "summary", "evidence_receipt_digests"}, label); err != nil {
			return err
		}
		identifier, err := requireID(finding["id"], label+".id")
		if err != nil {
			return err
		}
		if _, duplicate := seen[identifier]; duplicate {
			return fmt.Errorf("verdict.v3 finding IDs must be unique")
		}
		seen[identifier] = struct{}{}
		if _, err := stringValue(finding["summary"], label+".summary", true); err != nil {
			return err
		}
		if _, err := validateDigestArray(finding["evidence_receipt_digests"], label+".evidence_receipt_digests", true); err != nil {
			return err
		}
	}
	return nil
}

func validateProofIdentityRaw(value any, label string) error {
	proof, err := objectValue(value, label)
	if err != nil {
		return err
	}
	if err := requireExactFields(proof, []string{"epoch", "contract_ref", "contract_digest", "activation_transition_digest"}, label); err != nil {
		return err
	}
	epoch, err := integerValue(proof["epoch"], label+".epoch")
	if err != nil || epoch.Sign() < 0 {
		return fmt.Errorf("%s.epoch must be a nonnegative integer", label)
	}
	if _, err := requireRepositoryRef(proof["contract_ref"], label+".contract_ref"); err != nil {
		return err
	}
	if _, err := requireDigest(proof["contract_digest"], label+".contract_digest"); err != nil {
		return err
	}
	if proof["activation_transition_digest"] != nil {
		if _, err := requireDigest(proof["activation_transition_digest"], label+".activation_transition_digest"); err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaDigestsRaw(value any) error {
	schemas, err := objectValue(value, "verdict.v3 schema_digests")
	if err != nil {
		return err
	}
	fields := []string{"verdict", "rpi_report", "subject_manifest", "scope_index", "effect_receipt", "check_receipt"}
	if err := requireExactFields(schemas, fields, "verdict.v3 schema_digests"); err != nil {
		return err
	}
	for _, field := range fields {
		if _, err := requireDigest(schemas[field], "verdict.v3 schema_digests."+field); err != nil {
			return err
		}
	}
	return nil
}

func validateFreshnessRaw(value any) error {
	freshness, err := objectValue(value, "verdict.v3 freshness_attestation")
	if err != nil {
		return err
	}
	if err := requireExactFields(freshness, []string{"source", "attester_identity"}, "verdict.v3 freshness_attestation"); err != nil {
		return err
	}
	source, err := stringValue(freshness["source"], "verdict.v3 freshness_attestation.source", true)
	if err != nil {
		return err
	}
	if source != "runtime" && source != "caller" {
		return fmt.Errorf("verdict.v3 freshness source is invalid")
	}
	_, err = requireID(freshness["attester_identity"], "verdict.v3 freshness attester_identity")
	return err
}
