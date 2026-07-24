package verdictcheck

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
)

// SubjectManifestV2 is the exact, repository-observation manifest consumed by
// the RPI kernel. Its canonical_manifest_digest covers every other field.
type SubjectManifestV2 struct {
	SchemaVersion           string            `json:"schema_version"`
	ObservationRoots        []ObservationRoot `json:"observation_roots"`
	Exclusions              []string          `json:"exclusions"`
	Entries                 []ManifestEntry   `json:"entries"`
	CanonicalManifestDigest string            `json:"canonical_manifest_digest"`
}

type ObservationRoot struct {
	ID       string   `json:"id"`
	Includes []string `json:"includes"`
}

type ManifestEntry struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Executable bool   `json:"executable"`
	Digest     string `json:"digest"`
}

type ScopeIndexV1 struct {
	SchemaVersion      string              `json:"schema_version"`
	IntentDigest       string              `json:"intent_digest"`
	FrozenAt           string              `json:"frozen_at"`
	Criteria           []ScopeCriterion    `json:"criteria"`
	ScopeClasses       []ScopeClass        `json:"scope_classes"`
	DeclaredExclusions []DeclaredExclusion `json:"declared_exclusions"`
	ArtifactDigest     string              `json:"artifact_digest"`
}

type ScopeCriterion struct {
	ID              string `json:"id"`
	Required        bool   `json:"required"`
	StatementDigest string `json:"statement_digest"`
}

type ScopeClass struct {
	ID       string   `json:"id"`
	Patterns []string `json:"patterns"`
}

type DeclaredExclusion struct {
	ID           string   `json:"id"`
	CriterionIDs []string `json:"criterion_ids"`
	Reason       string   `json:"reason"`
}

type CheckReceiptV1 struct {
	SchemaVersion         string      `json:"schema_version"`
	ReceiptID             string      `json:"receipt_id"`
	Command               []string    `json:"command"`
	Result                string      `json:"result"`
	ExitCode              json.Number `json:"exit_code"`
	SubjectManifestDigest string      `json:"subject_manifest_digest"`
	StdoutDigest          string      `json:"stdout_digest"`
	StderrDigest          string      `json:"stderr_digest"`
	ObservedAt            string      `json:"observed_at"`
	ArtifactDigest        string      `json:"artifact_digest"`
}

type EffectReceiptV1 struct {
	SchemaVersion        string              `json:"schema_version"`
	BeforeManifestDigest string              `json:"before_manifest_digest"`
	FinalManifestDigest  string              `json:"final_manifest_digest"`
	Coverage             string              `json:"coverage"`
	Changes              []EffectChange      `json:"changes"`
	ActualChangedPaths   []string            `json:"actual_changed_paths"`
	UndeclaredPaths      []string            `json:"undeclared_paths"`
	CheckReceiptRefs     []ArtifactReference `json:"check_receipt_refs"`
	ArtifactDigest       string              `json:"artifact_digest"`
}

type EffectChange struct {
	Path         string  `json:"path"`
	ChangeKind   string  `json:"change_kind"`
	BeforeDigest *string `json:"before_digest"`
	AfterDigest  *string `json:"after_digest"`
}

type ArtifactReference struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type ProofContractTransitionV1 struct {
	SchemaVersion        string                   `json:"schema_version"`
	Prior                ProofTransitionPrior     `json:"prior"`
	Candidate            ProofTransitionCandidate `json:"candidate"`
	QualificationVerdict ArtifactReference        `json:"qualification_verdict"`
	ValidatorIdentity    string                   `json:"validator_identity"`
	ActivatedAt          string                   `json:"activated_at"`
}

type ProofTransitionPrior struct {
	Epoch                      json.Number `json:"epoch"`
	ContractRef                string      `json:"contract_ref"`
	ContractDigest             string      `json:"contract_digest"`
	ActivationTransitionDigest *string     `json:"activation_transition_digest"`
}

type ProofTransitionCandidate struct {
	Epoch                     json.Number `json:"epoch"`
	ContractRef               string      `json:"contract_ref"`
	ContractDigest            string      `json:"contract_digest"`
	SubjectManifestRef        string      `json:"subject_manifest_ref"`
	SubjectManifestDigest     string      `json:"subject_manifest_digest"`
	QualificationCorpusRef    string      `json:"qualification_corpus_ref"`
	QualificationCorpusDigest string      `json:"qualification_corpus_digest"`
}

var (
	subjectManifestV2Fields = []string{
		"schema_version", "observation_roots", "exclusions", "entries",
		"canonical_manifest_digest",
	}
	scopeIndexV1Fields = []string{
		"schema_version", "intent_digest", "frozen_at", "criteria",
		"scope_classes", "declared_exclusions", "artifact_digest",
	}
	checkReceiptV1Fields = []string{
		"schema_version", "receipt_id", "command", "result", "exit_code",
		"subject_manifest_digest", "stdout_digest", "stderr_digest",
		"observed_at", "artifact_digest",
	}
	effectReceiptV1Fields = []string{
		"schema_version", "before_manifest_digest", "final_manifest_digest",
		"coverage", "changes", "actual_changed_paths", "undeclared_paths",
		"check_receipt_refs", "artifact_digest",
	}
	proofTransitionV1Fields = []string{
		"schema_version", "prior", "candidate", "qualification_verdict",
		"validator_identity", "activated_at",
	}
)

func decodeTypedArtifact[T any](payload []byte, raw map[string]any, target *T, label string) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid %s shape: %w", label, err)
	}
	return requireJSONEOF(decoder)
}

func verifyCanonicalField(raw map[string]any, field, claimed, label string) error {
	if !ValidDigest(claimed) {
		return fmt.Errorf("%s %s is invalid", label, field)
	}
	identity := make(map[string]any, len(raw)-1)
	for key, value := range raw {
		if key != field {
			identity[key] = value
		}
	}
	canonical, err := CanonicalJSON(identity)
	if err != nil {
		return fmt.Errorf("canonicalize %s: %w", label, err)
	}
	sum := sha256.Sum256(canonical)
	if hex.EncodeToString(sum[:]) != claimed {
		return fmt.Errorf("%s %s does not bind canonical content", label, field)
	}
	return nil
}

func ReadSubjectManifestV2(payload []byte) (*SubjectManifestV2, error) {
	raw, err := decodeStrictJSONObject(payload, "subject-manifest.v2")
	if err != nil {
		return nil, err
	}
	if err := validateSubjectManifestV2Raw(raw); err != nil {
		return nil, err
	}
	var artifact SubjectManifestV2
	if err := decodeTypedArtifact(payload, raw, &artifact, "subject-manifest.v2"); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func ReadScopeIndexV1(payload []byte, expectedDigest string) (*ScopeIndexV1, error) {
	raw, err := decodeStrictJSONObject(payload, "scope-index.v1")
	if err != nil {
		return nil, err
	}
	if err := validateScopeIndexV1Raw(raw); err != nil {
		return nil, err
	}
	var artifact ScopeIndexV1
	if err := decodeTypedArtifact(payload, raw, &artifact, "scope-index.v1"); err != nil {
		return nil, err
	}
	if err := verifyCanonicalArtifact(raw, artifact.ArtifactDigest, expectedDigest, "scope-index.v1"); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func ReadCheckReceiptV1(payload []byte, expectedDigest string) (*CheckReceiptV1, error) {
	raw, err := decodeStrictJSONObject(payload, "check-receipt.v1")
	if err != nil {
		return nil, err
	}
	if err := validateCheckReceiptV1Raw(raw); err != nil {
		return nil, err
	}
	var artifact CheckReceiptV1
	if err := decodeTypedArtifact(payload, raw, &artifact, "check-receipt.v1"); err != nil {
		return nil, err
	}
	if err := verifyCanonicalArtifact(raw, artifact.ArtifactDigest, expectedDigest, "check-receipt.v1"); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func ReadEffectReceiptV1(payload []byte, expectedDigest string) (*EffectReceiptV1, error) {
	raw, err := decodeStrictJSONObject(payload, "effect-receipt.v1")
	if err != nil {
		return nil, err
	}
	if err := validateEffectReceiptV1Raw(raw); err != nil {
		return nil, err
	}
	var artifact EffectReceiptV1
	if err := decodeTypedArtifact(payload, raw, &artifact, "effect-receipt.v1"); err != nil {
		return nil, err
	}
	if err := verifyCanonicalArtifact(raw, artifact.ArtifactDigest, expectedDigest, "effect-receipt.v1"); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func ReadProofContractTransitionV1(payload []byte) (*ProofContractTransitionV1, error) {
	raw, err := decodeStrictJSONObject(payload, "proof-contract-transition.v1")
	if err != nil {
		return nil, err
	}
	if err := validateProofTransitionV1Raw(raw); err != nil {
		return nil, err
	}
	var artifact ProofContractTransitionV1
	if err := decodeTypedArtifact(payload, raw, &artifact, "proof-contract-transition.v1"); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func ReadProofIdentity(payload []byte) (*ProofIdentity, error) {
	raw, err := decodeStrictJSONObject(payload, "proof-identity")
	if err != nil {
		return nil, err
	}
	if err := validateProofIdentityRaw(raw, "proof-identity"); err != nil {
		return nil, err
	}
	var identity ProofIdentity
	if err := decodeTypedArtifact(payload, raw, &identity, "proof-identity"); err != nil {
		return nil, err
	}
	return &identity, nil
}

func validateSubjectManifestV2Raw(raw map[string]any) error {
	if err := requireExactFields(raw, subjectManifestV2Fields, "subject-manifest.v2"); err != nil {
		return err
	}
	if raw["schema_version"] != "subject-manifest.v2" {
		return fmt.Errorf("subject-manifest.v2 schema_version is invalid")
	}
	if err := validateManifestObservationRoots(raw["observation_roots"]); err != nil {
		return err
	}
	if err := validateManifestExclusions(raw["exclusions"]); err != nil {
		return err
	}
	if err := validateManifestEntries(raw["entries"]); err != nil {
		return err
	}
	claimed, err := requireDigest(raw["canonical_manifest_digest"], "subject-manifest.v2 canonical_manifest_digest")
	if err != nil {
		return err
	}
	return verifyCanonicalField(raw, "canonical_manifest_digest", claimed, "subject-manifest.v2")
}

func validateManifestObservationRoots(value any) error {
	roots, err := arrayValue(value, "subject-manifest.v2 observation_roots")
	if err != nil || len(roots) == 0 {
		return fmt.Errorf("subject-manifest.v2 observation_roots must be nonempty")
	}
	lastRoot := ""
	for index, item := range roots {
		label := fmt.Sprintf("subject-manifest.v2 observation_roots[%d]", index)
		root, err := objectValue(item, label)
		if err != nil {
			return err
		}
		if err := requireExactFields(root, []string{"id", "includes"}, label); err != nil {
			return err
		}
		id, err := requireID(root["id"], label+".id")
		if err != nil {
			return err
		}
		if index > 0 && id <= lastRoot {
			return fmt.Errorf("subject-manifest.v2 observation roots must be sorted and unique")
		}
		lastRoot = id
		includes, err := validateObservationRootArray(root["includes"], label+".includes")
		if err != nil {
			return err
		}
		if !sort.StringsAreSorted(includes) {
			return fmt.Errorf("%s must be sorted", label+".includes")
		}
	}
	return nil
}

func validateManifestExclusions(value any) error {
	exclusions, err := validateRepositoryRefArray(value, "subject-manifest.v2 exclusions", false, true)
	if err != nil {
		return err
	}
	if !sort.StringsAreSorted(exclusions) {
		return fmt.Errorf("subject-manifest.v2 exclusions must be sorted")
	}
	allowedExclusions := map[string]struct{}{
		".git": {}, ".agents/ao/intents": {}, ".agents/ao/verdicts": {}, ".agents/ao/reports": {},
	}
	for _, exclusion := range exclusions {
		if _, ok := allowedExclusions[exclusion]; !ok {
			return fmt.Errorf("subject-manifest.v2 contains unsupported runtime exclusion")
		}
	}
	return nil
}

func validateManifestEntries(value any) error {
	entries, err := arrayValue(value, "subject-manifest.v2 entries")
	if err != nil {
		return err
	}
	lastPath := ""
	for index, item := range entries {
		label := fmt.Sprintf("subject-manifest.v2 entries[%d]", index)
		entry, err := objectValue(item, label)
		if err != nil {
			return err
		}
		if err := requireExactFields(entry, []string{"path", "kind", "executable", "digest"}, label); err != nil {
			return err
		}
		path, err := requireRepositoryRef(entry["path"], label+".path")
		if err != nil {
			return err
		}
		if index > 0 && path <= lastPath {
			return fmt.Errorf("subject-manifest.v2 entries must be sorted and unique")
		}
		lastPath = path
		kind, err := stringValue(entry["kind"], label+".kind", true)
		if err != nil || (kind != "file" && kind != "symlink") {
			return fmt.Errorf("%s.kind is invalid", label)
		}
		if _, ok := entry["executable"].(bool); !ok {
			return fmt.Errorf("%s.executable must be boolean", label)
		}
		if _, err := requireDigest(entry["digest"], label+".digest"); err != nil {
			return err
		}
	}
	return nil
}

func validateScopeIndexV1Raw(raw map[string]any) error {
	if err := requireExactFields(raw, scopeIndexV1Fields, "scope-index.v1"); err != nil {
		return err
	}
	if raw["schema_version"] != "scope-index.v1" {
		return fmt.Errorf("scope-index.v1 schema_version is invalid")
	}
	if _, err := requireDigest(raw["intent_digest"], "scope-index.v1 intent_digest"); err != nil {
		return err
	}
	if err := validateRFC3339(raw["frozen_at"], "scope-index.v1 frozen_at"); err != nil {
		return err
	}
	criterionIDs, err := validateScopeCriteria(raw["criteria"])
	if err != nil {
		return err
	}
	if err := validateScopeClasses(raw["scope_classes"]); err != nil {
		return err
	}
	if err := validateDeclaredExclusions(raw["declared_exclusions"], criterionIDs); err != nil {
		return err
	}
	_, err = requireDigest(raw["artifact_digest"], "scope-index.v1 artifact_digest")
	return err
}

func validateScopeCriteria(value any) (map[string]bool, error) {
	criteria, err := arrayValue(value, "scope-index.v1 criteria")
	if err != nil || len(criteria) == 0 {
		return nil, fmt.Errorf("scope-index.v1 criteria must be nonempty")
	}
	criterionIDs := map[string]bool{}
	for index, item := range criteria {
		label := fmt.Sprintf("scope-index.v1 criteria[%d]", index)
		criterion, err := objectValue(item, label)
		if err != nil {
			return nil, err
		}
		if err := requireExactFields(criterion, []string{"id", "required", "statement_digest"}, label); err != nil {
			return nil, err
		}
		id, err := requireID(criterion["id"], label+".id")
		if err != nil {
			return nil, err
		}
		if _, duplicate := criterionIDs[id]; duplicate {
			return nil, fmt.Errorf("scope-index.v1 criterion IDs must be unique")
		}
		required, ok := criterion["required"].(bool)
		if !ok {
			return nil, fmt.Errorf("%s.required must be boolean", label)
		}
		criterionIDs[id] = required
		if _, err := requireDigest(criterion["statement_digest"], label+".statement_digest"); err != nil {
			return nil, err
		}
	}
	return criterionIDs, nil
}

func validateScopeClasses(value any) error {
	classes, err := arrayValue(value, "scope-index.v1 scope_classes")
	if err != nil || len(classes) == 0 {
		return fmt.Errorf("scope-index.v1 scope_classes must be nonempty")
	}
	classIDs := map[string]struct{}{}
	for index, item := range classes {
		label := fmt.Sprintf("scope-index.v1 scope_classes[%d]", index)
		class, err := objectValue(item, label)
		if err != nil {
			return err
		}
		if err := requireExactFields(class, []string{"id", "patterns"}, label); err != nil {
			return err
		}
		id, err := requireID(class["id"], label+".id")
		if err != nil {
			return err
		}
		if _, duplicate := classIDs[id]; duplicate {
			return fmt.Errorf("scope-index.v1 scope class IDs must be unique")
		}
		classIDs[id] = struct{}{}
		if _, err := validateRepositoryRefArray(class["patterns"], label+".patterns", true, true); err != nil {
			return err
		}
	}
	return nil
}

func validateDeclaredExclusions(value any, criterionIDs map[string]bool) error {
	exclusions, err := arrayValue(value, "scope-index.v1 declared_exclusions")
	if err != nil {
		return err
	}
	exclusionIDs := map[string]struct{}{}
	for index, item := range exclusions {
		label := fmt.Sprintf("scope-index.v1 declared_exclusions[%d]", index)
		exclusion, err := objectValue(item, label)
		if err != nil {
			return err
		}
		if err := requireExactFields(exclusion, []string{"id", "criterion_ids", "reason"}, label); err != nil {
			return err
		}
		id, err := requireID(exclusion["id"], label+".id")
		if err != nil {
			return err
		}
		if _, duplicate := exclusionIDs[id]; duplicate {
			return fmt.Errorf("scope-index.v1 exclusion IDs must be unique")
		}
		exclusionIDs[id] = struct{}{}
		ids, err := validateIDArray(exclusion["criterion_ids"], label+".criterion_ids", true)
		if err != nil {
			return err
		}
		for _, criterionID := range ids {
			required, known := criterionIDs[criterionID]
			if !known {
				return fmt.Errorf("declared exclusion references unknown criterion %q", criterionID)
			}
			if required {
				return fmt.Errorf("declared exclusions cannot absorb required criterion %q", criterionID)
			}
		}
		if _, err := stringValue(exclusion["reason"], label+".reason", true); err != nil {
			return err
		}
	}
	return nil
}

func validateCheckReceiptV1Raw(raw map[string]any) error {
	if err := requireExactFields(raw, checkReceiptV1Fields, "check-receipt.v1"); err != nil {
		return err
	}
	if raw["schema_version"] != "check-receipt.v1" {
		return fmt.Errorf("check-receipt.v1 schema_version is invalid")
	}
	if _, err := requireID(raw["receipt_id"], "check-receipt.v1 receipt_id"); err != nil {
		return err
	}
	commandValues, err := arrayValue(raw["command"], "check-receipt.v1 command")
	if err != nil || len(commandValues) == 0 {
		return fmt.Errorf("check-receipt.v1 command must be a nonempty string array")
	}
	for index, item := range commandValues {
		if _, ok := item.(string); !ok {
			return fmt.Errorf("check-receipt.v1 command[%d] must be a string", index)
		}
	}
	result, err := stringValue(raw["result"], "check-receipt.v1 result", true)
	if err != nil {
		return err
	}
	if result != "PASS" && result != "FAIL" && result != "ERROR" {
		return fmt.Errorf("check-receipt.v1 result is invalid")
	}
	exitCode, err := integerValue(raw["exit_code"], "check-receipt.v1 exit_code")
	if err != nil {
		return err
	}
	if result == "PASS" && exitCode.Sign() != 0 {
		return fmt.Errorf("check-receipt.v1 PASS requires exit_code 0")
	}
	if result == "FAIL" && exitCode.Sign() == 0 {
		return fmt.Errorf("check-receipt.v1 FAIL requires nonzero exit_code")
	}
	for _, field := range []string{"subject_manifest_digest", "stdout_digest", "stderr_digest", "artifact_digest"} {
		if _, err := requireDigest(raw[field], "check-receipt.v1 "+field); err != nil {
			return err
		}
	}
	return validateRFC3339(raw["observed_at"], "check-receipt.v1 observed_at")
}

func validateEffectReceiptV1Raw(raw map[string]any) error {
	if err := requireExactFields(raw, effectReceiptV1Fields, "effect-receipt.v1"); err != nil {
		return err
	}
	if raw["schema_version"] != "effect-receipt.v1" {
		return fmt.Errorf("effect-receipt.v1 schema_version is invalid")
	}
	for _, field := range []string{"before_manifest_digest", "final_manifest_digest", "artifact_digest"} {
		if _, err := requireDigest(raw[field], "effect-receipt.v1 "+field); err != nil {
			return err
		}
	}
	coverage, err := stringValue(raw["coverage"], "effect-receipt.v1 coverage", true)
	if err != nil || (coverage != "COMPLETE" && coverage != "INCOMPLETE") {
		return fmt.Errorf("effect-receipt.v1 coverage is invalid")
	}
	changedPaths, err := validateEffectChanges(raw["changes"])
	if err != nil {
		return err
	}
	if err := validateEffectPathSets(
		raw["actual_changed_paths"],
		raw["undeclared_paths"],
		changedPaths,
	); err != nil {
		return err
	}
	return validateEffectCheckReferences(raw["check_receipt_refs"])
}

func validateEffectChanges(value any) ([]string, error) {
	changes, err := arrayValue(value, "effect-receipt.v1 changes")
	if err != nil {
		return nil, err
	}
	changedPaths := make([]string, 0, len(changes))
	for index, item := range changes {
		label := fmt.Sprintf("effect-receipt.v1 changes[%d]", index)
		change, err := objectValue(item, label)
		if err != nil {
			return nil, err
		}
		if err := requireExactFields(change, []string{"path", "change_kind", "before_digest", "after_digest"}, label); err != nil {
			return nil, err
		}
		path, err := requireRepositoryRef(change["path"], label+".path")
		if err != nil {
			return nil, err
		}
		if index > 0 && path <= changedPaths[index-1] {
			return nil, fmt.Errorf("effect-receipt.v1 changes must be sorted and unique")
		}
		changedPaths = append(changedPaths, path)
		kind, err := stringValue(change["change_kind"], label+".change_kind", true)
		if err != nil {
			return nil, err
		}
		switch kind {
		case "ADDED", "MODIFIED", "DELETED", "MODE_CHANGED", "TYPE_CHANGED":
		default:
			return nil, fmt.Errorf("%s.change_kind is invalid", label)
		}
		for _, field := range []string{"before_digest", "after_digest"} {
			if change[field] != nil {
				if _, err := requireDigest(change[field], label+"."+field); err != nil {
					return nil, err
				}
			}
		}
		if kind == "ADDED" && (change["before_digest"] != nil || change["after_digest"] == nil) ||
			kind == "DELETED" && (change["before_digest"] == nil || change["after_digest"] != nil) ||
			(kind == "MODIFIED" || kind == "MODE_CHANGED" || kind == "TYPE_CHANGED") &&
				(change["before_digest"] == nil || change["after_digest"] == nil) {
			return nil, fmt.Errorf("%s digest nullability contradicts change_kind", label)
		}
	}
	return changedPaths, nil
}

func validateEffectPathSets(actualValue, undeclaredValue any, changedPaths []string) error {
	actual, err := validateRepositoryRefArray(actualValue, "effect-receipt.v1 actual_changed_paths", false, true)
	if err != nil || !reflect.DeepEqual(actual, changedPaths) {
		return fmt.Errorf("effect-receipt.v1 actual_changed_paths do not match changes")
	}
	undeclared, err := validateRepositoryRefArray(undeclaredValue, "effect-receipt.v1 undeclared_paths", false, true)
	if err != nil || !sort.StringsAreSorted(undeclared) {
		return fmt.Errorf("effect-receipt.v1 undeclared_paths are not canonical")
	}
	changedSet := make(map[string]struct{}, len(changedPaths))
	for _, path := range changedPaths {
		changedSet[path] = struct{}{}
	}
	for _, path := range undeclared {
		if _, ok := changedSet[path]; !ok {
			return fmt.Errorf("effect-receipt.v1 undeclared path is not an actual change")
		}
	}
	return nil
}

func validateEffectCheckReferences(value any) error {
	references, err := arrayValue(value, "effect-receipt.v1 check_receipt_refs")
	if err != nil {
		return err
	}
	lastRef := ""
	for index, item := range references {
		label := fmt.Sprintf("effect-receipt.v1 check_receipt_refs[%d]", index)
		reference, err := objectValue(item, label)
		if err != nil {
			return err
		}
		if err := requireExactFields(reference, []string{"ref", "digest"}, label); err != nil {
			return err
		}
		ref, err := requireRepositoryRef(reference["ref"], label+".ref")
		if err != nil {
			return err
		}
		if index > 0 && ref <= lastRef {
			return fmt.Errorf("effect-receipt.v1 check_receipt_refs must be sorted and unique")
		}
		lastRef = ref
		if _, err := requireDigest(reference["digest"], label+".digest"); err != nil {
			return err
		}
	}
	return nil
}

func validateProofTransitionV1Raw(raw map[string]any) error {
	if err := requireExactFields(raw, proofTransitionV1Fields, "proof-contract-transition.v1"); err != nil {
		return err
	}
	if raw["schema_version"] != "proof-contract-transition.v1" {
		return fmt.Errorf("proof-contract-transition.v1 schema_version is invalid")
	}
	prior, err := objectValue(raw["prior"], "proof transition prior")
	if err != nil {
		return err
	}
	candidate, err := objectValue(raw["candidate"], "proof transition candidate")
	if err != nil {
		return err
	}
	if err := requireExactFields(prior, []string{"epoch", "contract_ref", "contract_digest", "activation_transition_digest"}, "proof transition prior"); err != nil {
		return err
	}
	if err := requireExactFields(candidate, []string{"epoch", "contract_ref", "contract_digest", "subject_manifest_ref", "subject_manifest_digest", "qualification_corpus_ref", "qualification_corpus_digest"}, "proof transition candidate"); err != nil {
		return err
	}
	priorEpoch, err := nonnegativeInteger(prior["epoch"], "proof transition prior.epoch")
	if err != nil {
		return err
	}
	candidateEpoch, err := nonnegativeInteger(candidate["epoch"], "proof transition candidate.epoch")
	if err != nil {
		return err
	}
	if candidateEpoch.Cmp(new(big.Int).Add(priorEpoch, big.NewInt(1))) != 0 {
		return fmt.Errorf("proof transition candidate epoch must follow prior epoch")
	}
	for label, binding := range map[string]map[string]any{"prior": prior, "candidate": candidate} {
		refFields := []string{"contract_ref"}
		digestFields := []string{"contract_digest"}
		if label == "candidate" {
			refFields = append(refFields, "subject_manifest_ref", "qualification_corpus_ref")
			digestFields = append(digestFields, "subject_manifest_digest", "qualification_corpus_digest")
		}
		for _, field := range refFields {
			if _, err := requireRepositoryRef(binding[field], "proof transition "+label+"."+field); err != nil {
				return err
			}
		}
		for _, field := range digestFields {
			if _, err := requireDigest(binding[field], "proof transition "+label+"."+field); err != nil {
				return err
			}
		}
	}
	if prior["activation_transition_digest"] != nil {
		if _, err := requireDigest(prior["activation_transition_digest"], "proof transition prior.activation_transition_digest"); err != nil {
			return err
		}
	}
	if prior["contract_ref"] == candidate["contract_ref"] ||
		prior["contract_digest"] == candidate["contract_digest"] {
		return fmt.Errorf("proof transition candidate must differ from prior")
	}
	verdict, err := objectValue(raw["qualification_verdict"], "proof transition qualification_verdict")
	if err != nil {
		return err
	}
	if err := requireExactFields(verdict, []string{"ref", "digest"}, "proof transition qualification_verdict"); err != nil {
		return err
	}
	if _, err := requireRepositoryRef(verdict["ref"], "proof transition qualification_verdict.ref"); err != nil {
		return err
	}
	if _, err := requireDigest(verdict["digest"], "proof transition qualification_verdict.digest"); err != nil {
		return err
	}
	if _, err := requireID(raw["validator_identity"], "proof transition validator_identity"); err != nil {
		return err
	}
	return validateRFC3339(raw["activated_at"], "proof transition activated_at")
}

func nonnegativeInteger(value any, label string) (*big.Int, error) {
	integer, err := integerValue(value, label)
	if err != nil || integer.Sign() < 0 {
		return nil, fmt.Errorf("%s must be a nonnegative integer", label)
	}
	return integer, nil
}

func validateRepositoryRefArray(value any, label string, nonempty, unique bool) ([]string, error) {
	items, err := arrayValue(value, label)
	if err != nil {
		return nil, err
	}
	if nonempty && len(items) == 0 {
		return nil, fmt.Errorf("%s must be nonempty", label)
	}
	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for index, item := range items {
		ref, err := requireRepositoryRef(item, fmt.Sprintf("%s[%d]", label, index))
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[ref]; unique && duplicate {
			return nil, fmt.Errorf("%s contains duplicates", label)
		}
		seen[ref] = struct{}{}
		result = append(result, ref)
	}
	return result, nil
}

func validateObservationRootArray(value any, label string) ([]string, error) {
	items, err := arrayValue(value, label)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%s must be nonempty", label)
	}
	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for index, item := range items {
		ref, err := requireObservationRootRef(
			item,
			fmt.Sprintf("%s[%d]", label, index),
		)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[ref]; duplicate {
			return nil, fmt.Errorf("%s contains duplicates", label)
		}
		seen[ref] = struct{}{}
		result = append(result, ref)
	}
	return result, nil
}

func validateIDArray(value any, label string, nonempty bool) ([]string, error) {
	items, err := arrayValue(value, label)
	if err != nil {
		return nil, err
	}
	if nonempty && len(items) == 0 {
		return nil, fmt.Errorf("%s must be nonempty", label)
	}
	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for index, item := range items {
		id, err := requireID(item, fmt.Sprintf("%s[%d]", label, index))
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("%s contains duplicates", label)
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}
