// Package verdictcheck structurally verifies stored verdict.v2 artifacts.
//
// It is a READER for evidence inspection (`ao status`): it checks JSON shape,
// canonical-form digest binding, and the PASS scope rules declared by
// schemas/verdict.v2.schema.json. It never writes verdicts — the Validate
// skill (skills/validate/scripts/validate.py) is the writer and the semantic
// authority. Structural validity here is not a semantic verdict, and a
// well-formed evidence_refs string is a declared reference only: this package
// does not resolve or digest-bind the referenced evidence.
//
// The cross-language golden corpus at tests/fixtures/verdict-contract/ pins
// this implementation, the Python validator, and the JSON schema to the same
// judgments; change behavior only together with the corpus.
package verdictcheck

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Verdict mirrors the verdict.v2 artifact shape.
type Verdict struct {
	SchemaVersion         string      `json:"schema_version"`
	AcceptanceDigest      string      `json:"acceptance_digest"`
	SubjectManifestDigest string      `json:"subject_manifest_digest"`
	AuthorContextID       *string     `json:"author_context_id"`
	ValidatorContextID    *string     `json:"validator_context_id"`
	FreshnessAttestation  *Freshness  `json:"freshness_attestation"`
	Verdict               string      `json:"verdict"`
	Criteria              []Criterion `json:"criteria"`
	Findings              []Finding   `json:"findings"`
	EvidenceRefs          []string    `json:"evidence_refs"`
	Checked               []string    `json:"checked"`
	NotChecked            []string    `json:"not_checked"`
	ValidatedAt           string      `json:"validated_at"`
	ArtifactDigest        string      `json:"artifact_digest"`
}

// Freshness mirrors the freshness_attestation object.
type Freshness struct {
	Source           string `json:"source"`
	AttesterIdentity string `json:"attester_identity"`
}

// Criterion mirrors one acceptance criterion result.
type Criterion struct {
	ID           string    `json:"id"`
	Result       string    `json:"result"`
	EvidenceRefs *[]string `json:"evidence_refs"`
	Reason       string    `json:"reason,omitempty"`
}

// Finding mirrors one verdict finding.
type Finding struct {
	ID           string   `json:"id"`
	Summary      string   `json:"summary"`
	EvidenceRefs []string `json:"evidence_refs"`
}

// ValidDigest reports whether value is a 64-char lowercase hex SHA-256.
func ValidDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

// VerifyArtifact checks a stored verdict payload against the digest its
// filename declares: JSON well-formedness with no trailing data, exact
// verdict.v2 field set, structural shape rules, and recomputed canonical-form
// digest binding.
func VerifyArtifact(payload []byte, expectedDigest string) error {
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("invalid verdict JSON: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	// Reject duplicate JSON keys anywhere in the tree. Go's map/struct decode is
	// last-wins, so a duplicated key (a second top-level "verdict":"PASS", or a
	// nested second "source") would silently hide the real value and bind a
	// digest the stored bytes never canonicalize to. Detect it at the token
	// level and fail closed. (Logic mirrors cli/internal/gates/checks
	// duplicateKey; copied rather than imported — those helpers are unexported,
	// and importing the gate-framework package into this leaf reader would
	// invert the dependency direction and pull a heavy graph into it.)
	if dupKey, err := duplicateKey(payload); err != nil {
		return fmt.Errorf("invalid verdict JSON: %w", err)
	} else if dupKey != "" {
		return fmt.Errorf("verdict.v2 contains duplicate key %q", dupKey)
	}
	required := []string{
		"schema_version", "acceptance_digest", "subject_manifest_digest",
		"author_context_id", "validator_context_id", "freshness_attestation",
		"verdict", "criteria", "findings", "evidence_refs", "checked",
		"not_checked", "validated_at", "artifact_digest",
	}
	for _, key := range required {
		if _, ok := raw[key]; !ok {
			return fmt.Errorf("verdict.v2 missing required field %q", key)
		}
	}

	var verdict Verdict
	strict := json.NewDecoder(bytes.NewReader(payload))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&verdict); err != nil {
		return fmt.Errorf("invalid verdict.v2 shape: %w", err)
	}
	if err := requireJSONEOF(strict); err != nil {
		return err
	}
	if err := ValidateShape(&verdict); err != nil {
		return err
	}
	if verdict.ArtifactDigest != expectedDigest {
		return fmt.Errorf("artifact_digest does not match filename")
	}

	delete(raw, "artifact_digest")
	canonical, err := CanonicalJSON(raw)
	if err != nil {
		return fmt.Errorf("canonicalize verdict: %w", err)
	}
	actual := sha256.Sum256(canonical)
	if hex.EncodeToString(actual[:]) != expectedDigest {
		return fmt.Errorf("canonical content digest does not match filename")
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid verdict JSON: trailing value")
		}
		return fmt.Errorf("invalid verdict JSON: %w", err)
	}
	return nil
}

// CanonicalJSON renders a value in the contract's canonical form: sorted keys,
// compact separators, raw UTF-8 (no HTML escaping), no trailing newline.
// Matches the Python writer's canonical_bytes.
//
// Go's encoding/json ALWAYS escapes U+2028 (LINE SEPARATOR) and U+2029
// (PARAGRAPH SEPARATOR) to  /  for JSONP safety, regardless of
// SetEscapeHTML(false). Python's json.dumps(ensure_ascii=False) — the writer
// this reader must byte-match — emits them raw. Left alone, that forks the
// canonical bytes and thus the digest, so ao status would false-reject legit
// Python-written verdicts. unescapeLineSeparators undoes exactly that escape.
func CanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSuffix(buffer.Bytes(), []byte("\n"))
	return unescapeLineSeparators(trimmed), nil
}

// unescapeLineSeparators rewrites Go's   /   escapes back to their raw
// UTF-8 bytes so the canonical form matches Python's ensure_ascii=False output.
//
// It is backslash-parity aware: a REAL escape is a ` ` introduced by an odd
// run of backslashes, while the LITERAL 6-char text backslash-u-2028 (a string
// value that actually contains those characters) is encoded by Go as `\\u2028`
// (an even run) and MUST NOT be mutated. Because backslashes inside a JSON
// string only ever appear as part of an escape, consuming each `\<char>` escape
// as a unit — including `\\` as one escaped backslash — naturally respects that
// parity: after a `\\` pair the following `u2028` is plain text, not an escape.
func unescapeLineSeparators(in []byte) []byte {
	// Fast path: no separator ESCAPE present, return the input unchanged. We
	// scan for the ASCII escape sequences Go emits (backslash-u-2028 /
	// backslash-u-2029), never the raw runes (which encoded JSON never contains).
	esc2028 := []byte{'\\', 'u', '2', '0', '2', '8'}
	esc2029 := []byte{'\\', 'u', '2', '0', '2', '9'}
	if !bytes.Contains(in, esc2028) && !bytes.Contains(in, esc2029) {
		return in
	}
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); {
		if in[i] != '\\' {
			out = append(out, in[i])
			i++
			continue
		}
		// A backslash always begins an escape sequence in encoded JSON. If it is
		// ` ` / ` `, emit the raw rune; otherwise copy the escape (its
		// two leading bytes at minimum) verbatim so `\\` is consumed as a unit
		// and cannot be misread as an escape introducer for following text.
		if i+5 < len(in) && in[i+1] == 'u' {
			switch string(in[i+2 : i+6]) {
			case "2028":
				out = append(out, " "...)
				i += 6
				continue
			case "2029":
				out = append(out, " "...)
				i += 6
				continue
			}
		}
		if i+1 < len(in) {
			out = append(out, in[i], in[i+1])
			i += 2
		} else {
			out = append(out, in[i])
			i++
		}
	}
	return out
}

// duplicateKey scans the ENTIRE JSON tree and returns the first object key that
// repeats within the same object (or "" if none). Go's last-wins decode would
// otherwise let a duplicate hide the real value at any depth — a second
// top-level "verdict" dropping the real value, or a nested second "source"
// downgrading a freshness attestation. Recursion closes the whole class, not
// just the top level. Returns a non-nil error only on a malformed token stream
// (which the strict decode also rejects).
//
// Copied from cli/internal/gates/checks/constraints.go rather than imported:
// those helpers are unexported, and importing the gate-framework package into
// this leaf reader would invert the dependency direction.
func duplicateKey(raw []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	return scanDuplicateKey(dec)
}

// scanDuplicateKey consumes exactly one JSON value from dec, recursing into
// objects/arrays, and returns the first within-object duplicate key it finds.
func scanDuplicateKey(dec *json.Decoder) (string, error) {
	t, err := dec.Token()
	if err != nil {
		return "", err
	}
	d, ok := t.(json.Delim)
	if !ok {
		return "", nil // scalar value
	}
	switch d {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return "", err
			}
			key, ok := keyTok.(string)
			if !ok {
				return "", fmt.Errorf("malformed object key")
			}
			if seen[key] {
				return key, nil
			}
			seen[key] = true
			if dup, err := scanDuplicateKey(dec); err != nil || dup != "" {
				return dup, err
			}
		}
	case '[':
		for dec.More() {
			if dup, err := scanDuplicateKey(dec); err != nil || dup != "" {
				return dup, err
			}
		}
	}
	// consume the matching close delimiter
	if _, err := dec.Token(); err != nil {
		return "", err
	}
	return "", nil
}

// ValidateShape enforces the structural rules of verdict.v2, including the
// PASS tightening (distinct nonempty identities, freshness, nonempty checked
// scope, empty not_checked, every criterion PASS with evidence).
func ValidateShape(verdict *Verdict) error {
	if verdict.SchemaVersion != "verdict.v2" {
		return fmt.Errorf("schema_version is not verdict.v2")
	}
	if !ValidDigest(verdict.AcceptanceDigest) || !ValidDigest(verdict.SubjectManifestDigest) || !ValidDigest(verdict.ArtifactDigest) {
		return fmt.Errorf("verdict.v2 contains an invalid digest")
	}
	// Context identities are null (unattributed) or nonempty; an empty string
	// is junk identity data on any verdict, not only PASS. Matches the Python
	// validator and the schema's minLength.
	if err := validContextID(verdict.AuthorContextID, "author_context_id"); err != nil {
		return err
	}
	if err := validContextID(verdict.ValidatorContextID, "validator_context_id"); err != nil {
		return err
	}
	if !validResult(verdict.Verdict) {
		return fmt.Errorf("verdict.v2 has invalid verdict %q", verdict.Verdict)
	}
	if len(verdict.Criteria) == 0 {
		return fmt.Errorf("verdict.v2 criteria must be nonempty")
	}
	if err := validateCriteria(verdict.Criteria); err != nil {
		return err
	}
	if err := validateFindings(verdict.Findings); err != nil {
		return err
	}
	if !nonemptyStrings(verdict.EvidenceRefs) || !nonemptyStrings(verdict.Checked) || !nonemptyStrings(verdict.NotChecked) {
		return fmt.Errorf("verdict.v2 contains an empty evidence, checked, or not_checked item")
	}
	if err := validateFreshness(verdict.FreshnessAttestation); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, verdict.ValidatedAt); err != nil {
		return fmt.Errorf("verdict.v2 has invalid validated_at: %w", err)
	}
	if verdict.Verdict == "PASS" {
		return validatePass(verdict)
	}
	return nil
}

func validContextID(value *string, field string) error {
	if value != nil && *value == "" {
		return fmt.Errorf("verdict.v2 %s must be null or nonempty", field)
	}
	return nil
}

func validateCriteria(criteria []Criterion) error {
	for _, criterion := range criteria {
		if criterion.ID == "" || !validResult(criterion.Result) || criterion.EvidenceRefs == nil || !nonemptyStrings(*criterion.EvidenceRefs) {
			return fmt.Errorf("verdict.v2 contains an invalid criterion")
		}
	}
	return nil
}

func validateFindings(findings []Finding) error {
	for _, finding := range findings {
		if finding.ID == "" || finding.Summary == "" || len(finding.EvidenceRefs) == 0 || !nonemptyStrings(finding.EvidenceRefs) {
			return fmt.Errorf("verdict.v2 contains an invalid finding")
		}
	}
	return nil
}

func validateFreshness(attestation *Freshness) error {
	if attestation == nil {
		return nil
	}
	if (attestation.Source != "runtime" && attestation.Source != "caller") || attestation.AttesterIdentity == "" {
		return fmt.Errorf("verdict.v2 has invalid freshness_attestation")
	}
	return nil
}

func validatePass(verdict *Verdict) error {
	if verdict.AuthorContextID == nil || *verdict.AuthorContextID == "" || verdict.ValidatorContextID == nil || *verdict.ValidatorContextID == "" || *verdict.AuthorContextID == *verdict.ValidatorContextID {
		return fmt.Errorf("PASS verdict requires distinct nonempty context identities")
	}
	if verdict.FreshnessAttestation == nil || len(verdict.EvidenceRefs) == 0 || len(verdict.Checked) == 0 || len(verdict.NotChecked) != 0 {
		return fmt.Errorf("PASS verdict does not satisfy evidence and freshness requirements")
	}
	for _, criterion := range verdict.Criteria {
		if criterion.Result != "PASS" || criterion.EvidenceRefs == nil || len(*criterion.EvidenceRefs) == 0 {
			return fmt.Errorf("PASS verdict contains an unproven criterion")
		}
	}
	return nil
}

func validResult(value string) bool {
	return value == "PASS" || value == "FAIL" || value == "NOT_PROVEN"
}

func nonemptyStrings(values []string) bool {
	for _, value := range values {
		if value == "" {
			return false
		}
	}
	return true
}
