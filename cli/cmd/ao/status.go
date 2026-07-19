// practices: [dora-metrics, sre]
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/verdictcheck"
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

// statusContract declares status's real behavior: it accepts (and ignores)
// arbitrary positional args exactly as Cobra does today, emits text (JSON under
// -o json), reads the durable evidence stores on the filesystem and stamps
// recency from the clock, and exits 0 on success or 1 on a working-directory
// failure.
func statusContract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.status",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectFilesystem | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

func init() {
	statusCmd.GroupID = "core"
	if err := clicontract.Attach(statusCmd, statusContract()); err != nil {
		panic(err)
	}
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
