// Package statusapp inventories the durable AgentOps loop-evidence stores and
// renders the `ao status` report. It owns the filesystem and clock effects that
// the status command module is forbidden to perform directly, keeping the
// command module a thin Cobra presentation seam over this application logic.
package statusapp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

// RunOptions carries the presentation choices resolved by the command module.
// The working directory and clock are resolved inside Run so the module never
// performs a direct filesystem or clock effect.
type RunOptions struct {
	// JSON selects machine-readable output when true.
	JSON bool
	// Stdout receives the rendered report. It is the command's output stream.
	Stdout io.Writer
}

// Output is the top-level status document. Its only member is the durable loop
// evidence; status deliberately reports nothing about live runtime state.
type Output struct {
	LoopEvidence *LoopEvidenceStatus `json:"loop_evidence"`
}

// LoopEvidenceStatus summarizes the content-addressed intent and verdict
// evidence AgentOps stores on disk. It reports recency only, never active
// runtime phase, elapsed time, tool activity, retries, or remaining work.
type LoopEvidenceStatus struct {
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

// Run resolves the working directory and clock, inventories the durable stores,
// and renders the report to the configured stream.
func Run(opts RunOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	return Render(opts.Stdout, opts.JSON, &Output{LoopEvidence: LoadLoopEvidence(cwd, time.Now())})
}

// LoadLoopEvidence inventories only the two immutable stores AgentOps owns.
// Recency describes durable evidence, never live process state or remaining work.
func LoadLoopEvidence(cwd string, now time.Time) *LoopEvidenceStatus {
	result := &LoopEvidenceStatus{
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
	result.LastEvidenceAge = FormatDurationBrief(age)
	return result
}

func artifactDigestFromName(name, suffix string) (string, bool) {
	if !strings.HasSuffix(name, suffix) {
		return "", false
	}
	digest := strings.TrimSuffix(name, suffix)
	return digest, verdictcheck.ValidDigest(digest)
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

// validateVerdictArtifact delegates verdict.v2 structural verification to
// internal/verdictcheck (shape, exact field set, canonical digest binding).
func validateVerdictArtifact(path, expectedDigest string) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	return verdictcheck.VerifyArtifact(payload, expectedDigest)
}

// Render writes the status document to w, as indented JSON when asJSON is set
// or the human evidence report otherwise.
func Render(w io.Writer, asJSON bool, status *Output) error {
	if asJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	}

	fmt.Fprintln(w, "AgentOps Status")
	fmt.Fprintln(w, "===============")
	printLoopEvidence(w, status.LoopEvidence)
	return nil
}

func printLoopEvidence(w io.Writer, loop *LoopEvidenceStatus) {
	fmt.Fprintln(w, "\nLoop Evidence")
	fmt.Fprintln(w, "─────────────")
	fmt.Fprintf(w, "  Artifacts: %d intents, %d verdicts\n", loop.IntentArtifacts, loop.VerdictArtifacts)
	fmt.Fprintf(w, "  State:     %s\n", loop.State)
	if loop.LastEvidenceAt != "" {
		fmt.Fprintf(w, "  Newest:    %s (%s ago)\n", loop.LastEvidenceAt, loop.LastEvidenceAge)
	}
	if len(loop.Corrupt) > 0 {
		fmt.Fprintf(w, "  Corrupt:   %s\n", strings.Join(loop.Corrupt, "; "))
	}
	if len(loop.Unavailable) > 0 {
		fmt.Fprintf(w, "  Unavailable: %s\n", strings.Join(loop.Unavailable, "; "))
	}
	fmt.Fprintf(w, "  Checked:   %s\n", strings.Join(loop.Checked, ", "))
	fmt.Fprintf(w, "  Not checked: %s\n", strings.Join(loop.NotChecked, ", "))
}

// FormatDurationBrief formats a duration as a human-friendly short string.
func FormatDurationBrief(duration time.Duration) string {
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
