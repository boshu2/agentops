// practices: [dora-metrics, sre]
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show durable AgentOps loop evidence",
	Long: `Display the content-addressed intent and verdict evidence stored by AgentOps.

The command validates artifact names, content identity, and verdict.v2 shape
before counting evidence. It reports recency only; it does not infer an active
runtime phase, elapsed execution time, tool activity, retries, or remaining work.

Examples:
  ao status
  ao status --json`,
	RunE: runStatus,
}

func init() {
	statusCmd.GroupID = "core"
	rootCmd.AddCommand(statusCmd)
}

type statusOutput struct {
	LoopEvidence *loopEvidenceStatus `json:"loop_evidence"`
}

type loopEvidenceStatus struct {
	IntentArtifacts  int      `json:"intent_artifacts"`
	VerdictArtifacts int      `json:"verdict_artifacts"`
	LatestKind       string   `json:"latest_kind,omitempty"`
	LastEvidenceAt   string   `json:"last_evidence_at,omitempty"`
	LastEvidenceAge  string   `json:"last_evidence_age,omitempty"`
	State            string   `json:"state"`
	Checked          []string `json:"checked"`
	Corrupt          []string `json:"corrupt,omitempty"`
	Unavailable      []string `json:"unavailable,omitempty"`
	NotChecked       []string `json:"not_checked"`
}

type evidenceSource struct {
	kind     string
	path     string
	suffix   string
	count    *int
	validate func(string, string) error
}

type storedFreshness struct {
	Source           string `json:"source"`
	AttesterIdentity string `json:"attester_identity"`
}

type storedCriterion struct {
	ID           string    `json:"id"`
	Result       string    `json:"result"`
	EvidenceRefs *[]string `json:"evidence_refs"`
	Reason       string    `json:"reason,omitempty"`
}

type storedFinding struct {
	ID           string   `json:"id"`
	Summary      string   `json:"summary"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type storedVerdict struct {
	SchemaVersion         string            `json:"schema_version"`
	AcceptanceDigest      string            `json:"acceptance_digest"`
	SubjectManifestDigest string            `json:"subject_manifest_digest"`
	AuthorContextID       *string           `json:"author_context_id"`
	ValidatorContextID    *string           `json:"validator_context_id"`
	FreshnessAttestation  *storedFreshness  `json:"freshness_attestation"`
	Verdict               string            `json:"verdict"`
	Criteria              []storedCriterion `json:"criteria"`
	Findings              []storedFinding   `json:"findings"`
	EvidenceRefs          []string          `json:"evidence_refs"`
	Checked               []string          `json:"checked"`
	NotChecked            []string          `json:"not_checked"`
	ValidatedAt           string            `json:"validated_at"`
	ArtifactDigest        string            `json:"artifact_digest"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	return outputStatus(&statusOutput{LoopEvidence: loadLoopEvidence(cwd, time.Now())})
}

// loadLoopEvidence inventories only the two immutable stores AgentOps owns.
// Recency describes durable evidence, never live process state or remaining work.
func loadLoopEvidence(cwd string, now time.Time) *loopEvidenceStatus {
	result := &loopEvidenceStatus{
		NotChecked: []string{
			"active runtime phase",
			"execution elapsed time",
			"tool-call count",
			"caller-supplied subject manifests",
			"remaining work",
		},
	}
	sources := []evidenceSource{
		{
			kind: "intent", path: filepath.Join(cwd, ".agents", "ao", "intents", "sha256"),
			suffix: ".intent", count: &result.IntentArtifacts, validate: validateIntentArtifact,
		},
		{
			kind: "verdict", path: filepath.Join(cwd, ".agents", "ao", "verdicts", "sha256"),
			suffix: ".json", count: &result.VerdictArtifacts, validate: validateVerdictArtifact,
		},
	}

	var latest time.Time
	for _, source := range sources {
		rel, err := filepath.Rel(cwd, source.path)
		if err != nil {
			rel = source.path
		}
		result.Checked = append(result.Checked, rel)
		entries, err := os.ReadDir(source.path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			result.Unavailable = append(result.Unavailable, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				result.Unavailable = append(result.Unavailable, fmt.Sprintf("%s/%s: %v", rel, entry.Name(), err))
				continue
			}
			if !info.Mode().IsRegular() {
				continue
			}
			expectedDigest, ok := artifactDigestFromName(entry.Name(), source.suffix)
			artifact := filepath.Join(rel, entry.Name())
			if !ok {
				result.Corrupt = append(result.Corrupt, artifact+": invalid content-addressed artifact name")
				continue
			}
			if err := source.validate(filepath.Join(source.path, entry.Name()), expectedDigest); err != nil {
				result.Corrupt = append(result.Corrupt, fmt.Sprintf("%s: %v", artifact, err))
				continue
			}
			(*source.count)++
			if info.ModTime().After(latest) {
				latest = info.ModTime()
				result.LatestKind = source.kind
			}
		}
	}

	total := result.IntentArtifacts + result.VerdictArtifacts
	if total == 0 {
		switch {
		case len(result.Corrupt) > 0 && len(result.Unavailable) > 0:
			result.State = "evidence_corrupt_and_unavailable"
		case len(result.Corrupt) > 0:
			result.State = "evidence_corrupt"
		case len(result.Unavailable) > 0:
			result.State = "evidence_unavailable"
		default:
			result.State = "no_evidence"
		}
		return result
	}

	result.State = result.LatestKind + "_is_latest_evidence"
	result.LastEvidenceAt = latest.UTC().Format(time.RFC3339)
	age := now.Sub(latest)
	if age < 0 {
		age = 0
	}
	result.LastEvidenceAge = formatDurationBrief(age)
	return result
}

func artifactDigestFromName(name, suffix string) (string, bool) {
	if !strings.HasSuffix(name, suffix) {
		return "", false
	}
	digest := strings.TrimSuffix(name, suffix)
	return digest, validDigest(digest)
}

func validDigest(value string) bool {
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

func validateIntentArtifact(path, expectedDigest string) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	actual := sha256.Sum256(payload)
	if hex.EncodeToString(actual[:]) != expectedDigest {
		return fmt.Errorf("content digest does not match filename")
	}
	return nil
}

func validateVerdictArtifact(path, expectedDigest string) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("invalid verdict JSON: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
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

	var verdict storedVerdict
	strict := json.NewDecoder(bytes.NewReader(payload))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&verdict); err != nil {
		return fmt.Errorf("invalid verdict.v2 shape: %w", err)
	}
	if err := requireJSONEOF(strict); err != nil {
		return err
	}
	if err := validateVerdictShape(&verdict); err != nil {
		return err
	}
	if verdict.ArtifactDigest != expectedDigest {
		return fmt.Errorf("artifact_digest does not match filename")
	}

	delete(raw, "artifact_digest")
	canonical, err := canonicalJSON(raw)
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

func canonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func validateVerdictShape(verdict *storedVerdict) error {
	if verdict.SchemaVersion != "verdict.v2" {
		return fmt.Errorf("schema_version is not verdict.v2")
	}
	if !validDigest(verdict.AcceptanceDigest) || !validDigest(verdict.SubjectManifestDigest) || !validDigest(verdict.ArtifactDigest) {
		return fmt.Errorf("verdict.v2 contains an invalid digest")
	}
	if !validVerdictResult(verdict.Verdict) {
		return fmt.Errorf("verdict.v2 has invalid verdict %q", verdict.Verdict)
	}
	if len(verdict.Criteria) == 0 {
		return fmt.Errorf("verdict.v2 criteria must be nonempty")
	}
	for _, criterion := range verdict.Criteria {
		if criterion.ID == "" || !validVerdictResult(criterion.Result) || criterion.EvidenceRefs == nil || !nonemptyStrings(*criterion.EvidenceRefs) {
			return fmt.Errorf("verdict.v2 contains an invalid criterion")
		}
	}
	for _, finding := range verdict.Findings {
		if finding.ID == "" || finding.Summary == "" || len(finding.EvidenceRefs) == 0 || !nonemptyStrings(finding.EvidenceRefs) {
			return fmt.Errorf("verdict.v2 contains an invalid finding")
		}
	}
	if !nonemptyStrings(verdict.EvidenceRefs) || !nonemptyStrings(verdict.Checked) || !nonemptyStrings(verdict.NotChecked) {
		return fmt.Errorf("verdict.v2 contains an empty evidence, checked, or not_checked item")
	}
	if verdict.FreshnessAttestation != nil {
		if (verdict.FreshnessAttestation.Source != "runtime" && verdict.FreshnessAttestation.Source != "caller") || verdict.FreshnessAttestation.AttesterIdentity == "" {
			return fmt.Errorf("verdict.v2 has invalid freshness_attestation")
		}
	}
	if _, err := time.Parse(time.RFC3339, verdict.ValidatedAt); err != nil {
		return fmt.Errorf("verdict.v2 has invalid validated_at: %w", err)
	}
	if verdict.Verdict == "PASS" {
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
	}
	return nil
}

func validVerdictResult(value string) bool {
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

func outputStatus(status *statusOutput) error {
	if GetOutput() == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	}

	fmt.Println("AgentOps Status")
	fmt.Println("===============")
	printLoopEvidence(status.LoopEvidence)
	return nil
}

func printLoopEvidence(loop *loopEvidenceStatus) {
	fmt.Println("\nLoop Evidence")
	fmt.Println("─────────────")
	fmt.Printf("  Artifacts: %d intents, %d verdicts\n", loop.IntentArtifacts, loop.VerdictArtifacts)
	fmt.Printf("  State:     %s\n", loop.State)
	if loop.LastEvidenceAt != "" {
		fmt.Printf("  Newest:    %s (%s ago)\n", loop.LastEvidenceAt, loop.LastEvidenceAge)
	}
	if len(loop.Corrupt) > 0 {
		fmt.Printf("  Corrupt:   %s\n", strings.Join(loop.Corrupt, "; "))
	}
	if len(loop.Unavailable) > 0 {
		fmt.Printf("  Unavailable: %s\n", strings.Join(loop.Unavailable, "; "))
	}
	fmt.Printf("  Checked:   %s\n", strings.Join(loop.Checked, ", "))
	fmt.Printf("  Not checked: %s\n", strings.Join(loop.NotChecked, ", "))
}

// formatDurationBrief formats a duration as a human-friendly short string.
func formatDurationBrief(duration time.Duration) string {
	if duration < time.Minute {
		return "<1m"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	days := int(duration.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dw", days/7)
}
