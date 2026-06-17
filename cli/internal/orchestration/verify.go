// practices: [design-by-contract, capability-detection]
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VerifyOptions configures a verify run.
type VerifyOptions struct {
	RepoRoot string
	Profile  string
	Session  string
	RunID    string
	Runner   CommandRunner
}

// RunVerify checks live session pane map against profile with evidence tiers.
func RunVerify(ctx context.Context, opts VerifyOptions) (InstrumentResult, error) {
	if opts.Runner == nil {
		return InstrumentResult{}, fmt.Errorf("verify: runner is nil")
	}
	if opts.Session == "" {
		return InstrumentResult{}, fmt.Errorf("verify: session name required")
	}
	if opts.RunID == "" {
		opts.RunID = NewRunID()
	}
	profiles, err := LoadProfilesContract(opts.RepoRoot)
	if err != nil {
		return InstrumentResult{}, err
	}
	profile, err := profiles.ProfileByID(opts.Profile)
	if err != nil {
		return InstrumentResult{}, err
	}

	tier, matched, detail, coordDegraded := verifyPaneEvidence(ctx, opts, profile)
	checks := []CheckStatus{{
		ID:     "pane_map",
		Status: paneCheckStatus(tier, matched, len(profile.Panes)),
		Detail: detail,
	}}

	verdict := AggregateVerdictFromChecks(checks)
	if tier == EvidenceTierWeak && verdict.Status == VerdictStatusPass {
		verdict.Status = VerdictStatusWarn
		verdict.Confidence = VerdictConfidenceMedium
		coordDegraded = true
	}

	return InstrumentResult{
		SchemaVersion:        InstrumentSchemaVersionV1,
		Command:              InstrumentCommandVerify,
		Profile:              opts.Profile,
		Session:              opts.Session,
		RunID:                opts.RunID,
		Verdict:              verdict,
		EvidenceTier:         tier,
		CoordinationDegraded: coordDegraded,
		Checks:               checks,
		Panes:                profile.Panes,
	}, nil
}

func paneCheckStatus(tier string, matched, want int) string {
	if want == 0 {
		return VerdictStatusWarn
	}
	if matched >= want && tier == EvidenceTierStrong {
		return VerdictStatusPass
	}
	if matched > 0 && tier == EvidenceTierWeak {
		return VerdictStatusWarn
	}
	if matched > 0 && tier == EvidenceTierStrong {
		return VerdictStatusPass
	}
	return VerdictStatusFail
}

func verifyPaneEvidence(ctx context.Context, opts VerifyOptions, profile ProfileSpec) (tier string, matched int, detail string, coordDegraded bool) {
	// Strong: atm activity
	if out, err := opts.Runner.Run(ctx, "atm", "activity", opts.Session, "--json"); err == nil {
		m := countActivityRuntimes(out, profile)
		if m > 0 {
			return EvidenceTierStrong, m, "atm activity pane map", false
		}
	}

	// Strong: spawn json file under .agents or ntm state
	if m, path := matchSpawnJSON(opts.RepoRoot, opts.Session, profile); m > 0 {
		return EvidenceTierStrong, m, "spawn json " + path, false
	}

	// Weak: tmux titles
	if out, err := opts.Runner.Run(ctx, "tmux", "list-panes", "-t", opts.Session, "-F", "#{pane_index}:#{pane_title}"); err == nil {
		m := countTmuxTitles(out, profile)
		if m > 0 {
			return EvidenceTierWeak, m, "tmux pane titles only", true
		}
	}

	return EvidenceTierNone, 0, "no pane evidence", true
}

func countActivityRuntimes(out []byte, profile ProfileSpec) int {
	var payload struct {
		Agents []struct {
			Type  string `json:"type"`
			Agent string `json:"agent"`
			Kind  string `json:"kind"`
		} `json:"agents"`
		Panes []struct {
			AgentType string `json:"agent_type"`
			Type      string `json:"type"`
		} `json:"panes"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return countRuntimeTokens(string(out), profile)
	}
	seen := map[string]struct{}{}
	for _, a := range payload.Agents {
		t := strings.ToLower(strings.TrimSpace(a.Type))
		if t == "" {
			t = strings.ToLower(strings.TrimSpace(a.Agent))
		}
		if t != "" {
			seen[t] = struct{}{}
		}
	}
	for _, p := range payload.Panes {
		t := strings.ToLower(strings.TrimSpace(p.AgentType))
		if t == "" {
			t = strings.ToLower(strings.TrimSpace(p.Type))
		}
		if t != "" {
			seen[t] = struct{}{}
		}
	}
	return matchProfileRuntimes(seen, profile)
}

func countRuntimeTokens(blob string, profile ProfileSpec) int {
	seen := map[string]struct{}{}
	lower := strings.ToLower(blob)
	for _, p := range profile.Panes {
		rt := strings.ToLower(p.Runtime)
		if strings.Contains(lower, rt) || runtimeAliases(rt, lower) {
			seen[rt] = struct{}{}
		}
	}
	return len(seen)
}

func runtimeAliases(rt, blob string) bool {
	switch rt {
	case "claude":
		return strings.Contains(blob, "cc") || strings.Contains(blob, "opus")
	case "codex":
		return strings.Contains(blob, "cod")
	case "agy":
		return strings.Contains(blob, "agy") || strings.Contains(blob, "antigravity")
	default:
		return false
	}
}

func matchProfileRuntimes(seen map[string]struct{}, profile ProfileSpec) int {
	n := 0
	for _, p := range profile.Panes {
		rt := strings.ToLower(p.Runtime)
		if _, ok := seen[rt]; ok {
			n++
			continue
		}
		for k := range seen {
			if runtimeAliases(rt, k) {
				n++
				break
			}
		}
	}
	return n
}

func matchSpawnJSON(repoRoot, session string, profile ProfileSpec) (int, string) {
	candidates := []string{
		filepath.Join(repoRoot, ".agents", "spawn", session+".json"),
		filepath.Join(repoRoot, ".agents", session+".spawn.json"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		m := countRuntimeTokens(string(data), profile)
		if m > 0 {
			return m, path
		}
	}
	return 0, ""
}

func countTmuxTitles(out []byte, profile ProfileSpec) int {
	return countRuntimeTokens(string(out), profile)
}
