---
id: research-2026-04-27-beads-cli-ref-audit
type: research
date: 2026-04-27
goal: "Audit copied Beads CLI reference links in shared and codex skills (ag-aez prereq C)"
---

# Beads CLI Reference Link Audit

## Files audited

- `skills/beads/references/CLI_REFERENCE.md` (536 lines)
- `skills-codex/beads/references/CLI_REFERENCE.md` (536 lines, byte-identical)

## Results

| Check | Status | Notes |
|---|---|---|
| Stale references to CONFIG.md / DAEMON.md / LABELS.md | PASS | Upstream commit `09ace562` ("docs(beads): refresh CLI_REFERENCE See Also sections") already removed these |
| See Also targets exist (WORKFLOWS, BOUNDARIES, TROUBLESHOOTING, ANTI_PATTERNS, AGENTS, README) | PASS | All present in both shared and codex variants |
| Quick Navigation anchors match real headers | PASS | 6 listed anchors, all match (`basic-operations`, `issue-management`, `dependencies--labels`, `filtering--search`, `advanced-operations`, `database-management`) |
| Shared vs codex parity | PASS | `diff` returns zero output |
| `bd doctor --check=conventions` orphan check | PASS | Reports `conventions.orphans` PASS (warnings on `lint` / `stale` are unrelated database-availability issues) |
| Duplicated `bd` command snippets | INFO | 7 distinct commands appear >1x (e.g. `bd vc status` 4x, `bd ready --json` 3x). All appear contextually intentional — basic-section example reused in patterns/troubleshooting. Not flagged. |

## Doc-quality observation (not blocking)

Quick Navigation lists 6 of 14 `## ` sections. Missing: `Global Flags`, `Issue Types`,
`Priorities`, `Dependency Types`, `Output Formats`, `Common Patterns for AI Agents`.
This is a discoverability gap, not a stale-ref or orphan-ref issue. Out of scope for ag-aez.

## Verdict

**PASS.** No stale or orphan refs. No duplicated content. ag-aez acceptance met
(`bd doctor --check=conventions` orphan check PASS).
