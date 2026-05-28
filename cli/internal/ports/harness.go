// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// HarnessName identifies the runtime harness that hosts the agent.
// Values are stable strings matching the existing skills-{harness}/
// directory naming (e.g. "claude", "codex"). Adapters MAY recognize
// additional names; the in-memory adapter treats them as opaque.
type HarnessName string

const (
	HarnessClaude HarnessName = "claude"
	HarnessCodex  HarnessName = "codex"
)

// HarnessSkillSync is one (skill, harness) → checksum mapping. Path
// is the skill manifest path the harness expects to find (e.g.
// "skills-codex/evolve/SKILL.md"); ContentHash is the SHA hash the
// adapter computed; OutOfSync is true when the hash doesn't match
// the canonical source-of-truth (skills/<name>/SKILL.md by default).
type HarnessSkillSync struct {
	Harness     HarnessName
	Skill       string
	Path        string
	ContentHash string
	OutOfSync   bool
}

// HarnessPort is the BC5 Runtime surface. Callers — `make sync-hooks`,
// the codex-parity audit, the dream-loop harness-state recorder, and
// any future cross-harness sanity check — depend on this port so they
// can ask "what's the sync state of each (skill, harness) pair?"
// without coupling to a specific implementation (filesystem scan,
// registry.json snapshot, etc.).
//
// Contract:
//
//   - Status returns the full sync state across all known
//     (skill, harness) pairs.
//   - StatusForSkill returns just the entries for that skill across
//     all harnesses. Empty skill → non-nil error.
//   - The slice returned by both methods MUST be non-nil even when
//     empty.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC5 row). soc-zd7c epic
// tracks BC5 port extraction; this is the only port in that epic.
type HarnessPort interface {
	Status(ctx context.Context) ([]HarnessSkillSync, error)
	StatusForSkill(ctx context.Context, skill string) ([]HarnessSkillSync, error)
}
