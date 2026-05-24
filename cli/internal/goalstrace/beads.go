package goalstrace

import (
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
)

// scenariosLineRe matches an explicit "Scenarios: <id>[, <id>...]" line in a
// bead description or notes field (ADR-0005 §2.3). The line format mirrors the
// GOALS.md Scenarios attribute.
var scenariosLineRe = regexp.MustCompile(`(?im)^\s*Scenarios:\s*(.+)$`)

// scenarioTokenRe matches a scenario ID token anywhere in free text (the
// low-confidence heuristic discovery path).
//
// auto- IDs require at least two hyphen-separated slug segments after the
// "auto-" prefix (e.g. "auto-nightly-evolution-dry-run") so that plain English
// compound words like "auto-merge" or "auto-update" are NOT matched. A
// single-word suffix ("auto-merge") has exactly one segment and no further
// hyphen, which the required inner "-[a-z0-9]+" group rejects.
var scenarioTokenRe = regexp.MustCompile(`\b(s-\d{4}-\d{2}-\d{2}-\d{3}|auto-[a-z0-9]+-[a-z0-9][a-z0-9-]*)\b`)

// scenarioIDRe is the anchored form of the scenario-ID grammar. It matches a
// string that is *entirely* a scenario ID (the resolvable filename stem under
// spec/scenarios/<id>.json or .agents/holdout/<id>.json), with no surrounding
// prose. Unlike scenarioTokenRe (which scans free text for an embedded token),
// this is applied to each comma/semicolon-split token of an explicit
// "Scenarios:" line so that bead-description prose — frontmatter field lists,
// "and section structure", "graduation path (...)", etc. — is rejected rather
// than mis-claimed as a dangling scenario reference (soc-bhu6w).
var scenarioIDRe = regexp.MustCompile(`^(s-\d{4}-\d{2}-\d{2}-\d{3}|auto-[a-z0-9]+-[a-z0-9][a-z0-9-]*)$`)

// isScenarioID reports whether s is, in its entirety, a resolvable scenario ID
// (human "s-YYYY-MM-DD-NNN" or auto "auto-<multi-segment-slug>"). Free-form
// "## Scenarios" bullet slugs (e.g. "lesson-format-spec") and arbitrary prose
// tokens are not scenario IDs in the trace-resolution model and return false.
func isScenarioID(s string) bool {
	return scenarioIDRe.MatchString(s)
}

// directiveTokenRe matches a stable directive ID token (d-<slug>) anywhere in
// free text — used for the body-scan discovery path of directive_has_learning.
var directiveTokenRe = regexp.MustCompile(`\bd-[a-z0-9][a-z0-9-]*\b`)

// beadRecord is the subset of `bd show --json` fields the walker reads.
type beadRecord struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Notes       string `json:"notes"`
	Status      string `json:"status"`
}

// BeadQuerier abstracts the `bd` CLI so tests can inject deterministic bead
// data without depending on a real bd binary or Dolt remote.
type BeadQuerier interface {
	// Available reports whether bead querying is possible. When false the
	// walker degrades gracefully with a diagnostic and emits no bead nodes.
	Available() bool
	// Beads returns every bead record the querier can see.
	Beads() ([]beadRecord, error)
}

// execBeadQuerier queries beads via the real `bd` CLI. It is read-only:
// `bd list --json` never mutates the tracker.
type execBeadQuerier struct{}

// NewExecBeadQuerier returns a BeadQuerier backed by the `bd` binary on PATH.
func NewExecBeadQuerier() BeadQuerier {
	return execBeadQuerier{}
}

// Available reports whether the bd binary is reachable via PATH.
func (execBeadQuerier) Available() bool {
	_, err := exec.LookPath("bd")
	return err == nil
}

// Beads runs `bd list --json --all` and parses the result. Both an array and
// a {"issues": [...]} envelope are accepted since bd output has varied.
func (execBeadQuerier) Beads() ([]beadRecord, error) {
	out, err := exec.Command("bd", "list", "--json", "--all").Output()
	if err != nil {
		// Retry without --all for older bd versions.
		out, err = exec.Command("bd", "list", "--json").Output()
		if err != nil {
			return nil, err
		}
	}
	return parseBeadList(out)
}

// parseBeadList decodes `bd list --json` output, tolerating either a bare
// array or an {"issues": [...]} envelope.
func parseBeadList(data []byte) ([]beadRecord, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var arr []beadRecord
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}
	var env struct {
		Issues []beadRecord `json:"issues"`
		Beads  []beadRecord `json:"beads"`
	}
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		return nil, err
	}
	if len(env.Issues) > 0 {
		return env.Issues, nil
	}
	return env.Beads, nil
}

// staticBeadQuerier is a test/fixture-backed BeadQuerier.
type staticBeadQuerier struct {
	available bool
	beads     []beadRecord
}

// NewStaticBeadQuerier returns a BeadQuerier serving a fixed record set. When
// available is false it behaves like a missing `bd` binary.
func NewStaticBeadQuerier(available bool, beads []beadRecord) BeadQuerier {
	return staticBeadQuerier{available: available, beads: beads}
}

func (s staticBeadQuerier) Available() bool              { return s.available }
func (s staticBeadQuerier) Beads() ([]beadRecord, error) { return s.beads, nil }

// BeadInput is an exported, package-external description of a bead, used by
// callers outside this package (e.g. CLI tests) to build a deterministic
// BeadQuerier without reaching the unexported beadRecord type.
type BeadInput struct {
	ID          string
	Title       string
	Description string
	Notes       string
	Status      string
}

// NewStaticBeadQuerierFromInputs returns a BeadQuerier serving the given bead
// inputs. It is the exported seam CLI commands use to inject deterministic
// bead data into the walker without depending on a real bd binary.
func NewStaticBeadQuerierFromInputs(available bool, inputs []BeadInput) BeadQuerier {
	beads := make([]beadRecord, 0, len(inputs))
	for _, in := range inputs {
		beads = append(beads, beadRecord(in))
	}
	return staticBeadQuerier{available: available, beads: beads}
}

// claimedScenarios extracts scenario IDs a bead claims, partitioned into
// explicit (high-confidence: from a "Scenarios:" line) and heuristic
// (low-confidence: free-text token elsewhere) matches per ADR-0005 §2.3.
func claimedScenarios(b beadRecord) (explicit, heuristic []string) {
	text := b.Description + "\n" + b.Notes
	explicitSet := map[string]bool{}
	for _, m := range scenariosLineRe.FindAllStringSubmatch(text, -1) {
		for _, id := range splitIDList(m[1]) {
			// A "Scenarios:" line whose first bullet is a "<slug>: <prose>"
			// description (the canonical bead-embedded ## Scenarios form) reflows
			// into one logical line, so the comma/semicolon split yields prose
			// tokens (frontmatter field names, "and section structure", etc.).
			// Only tokens that are, in full, a resolvable scenario ID are real
			// claims; everything else is prose and must be dropped (soc-bhu6w).
			if !isScenarioID(id) {
				continue
			}
			if !explicitSet[id] {
				explicitSet[id] = true
				explicit = append(explicit, id)
			}
		}
	}
	seen := map[string]bool{}
	for _, m := range scenarioTokenRe.FindAllString(text, -1) {
		if explicitSet[m] || seen[m] {
			continue
		}
		seen[m] = true
		heuristic = append(heuristic, m)
	}
	return explicit, heuristic
}

// splitIDList splits a comma/semicolon-separated ID list, trimming whitespace.
func splitIDList(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
