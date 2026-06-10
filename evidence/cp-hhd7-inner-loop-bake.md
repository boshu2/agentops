# cp-hhd7 — Inner-loop disciplines baked into skills

**Branch:** fix/cp-hhd7-inner-loop-bake  
**Date:** 2026-06-10  
**Worker:** SilverLynx (Athena lane)  
**Live-deploy flag:** YES — skills/ symlinked into agentops checkout; changes deploy on merge to main.

---

## What changed and where

### Classification applied first (card 21 method, cp-hhd7 REFRAME)

Each operator card was classified POLICY or GENERAL PRACTICE before editing:

- **POLICY → mode+pointer only** (mechanism stays general; gate enforces): cross-family floor, single-writer/merged-before-close, lane-authority, gemini bench.
- **GENERAL PRACTICE → guidance** (broadly true across contexts): watch-FMs, worker raw-data contract, external-loop shape, content-push, durable identities, ACK-with-id, dedup via graph, close-with-residual, append-notes, counterfeit-judge shape, judge-empirically.

### Per-skill change summary

#### skills/validate/SKILL.md
- Added `## Validation discipline (2026-06-09, cards 6–10, cp-hhd7)` section:
  - Verdict form (bare `VERDICT:` / `COMMANDS RUN:` anchored parse contract)
  - Cross-family floor as **mode note + gate pointer** (icb6 enforces; `--mixed` supports; same-model council valid for non-assurance)
  - A7 gemini bench: CURRENT POLICY, graduation condition stated
  - Judge-empirically-on-a-fixture pattern (card 9, cp-8720)
  - Dispatch-record-first / dedup before spawning validator (card 3, cp-hhtu)
  - Judges re-measure (don't read and agree) + `judge_source` attestation

#### skills/codex-exec/SKILL.md
- Extended `## Validator dispatch rules` section (cp-2h6x partial already present):
  - Judge prompt pattern — output contract published FROM the prompt (card 10, cp-b2by)
  - Output-contract validation before acting: parse `VERDICT:` / `COMMANDS RUN:` / `judge_source:` programmatically; discard on any missing element
  - Counterfeit-judge shape named and refused

#### skills/cc-subagents/SKILL.md
- Extended Phase 3 spawn section with mandatory every-prompt inclusions: LAW-0, heavy-compile-on-bushido, FINAL REPORT contract
- Added `## Worker FINAL REPORT contract (card 16, cp-hhd7)` section: files changed (exact paths), commit SHA, test tail (verbatim), conflicts surfaced (explicit "none")

#### skills/swarm/SKILL.md
- Added `## Worker report contract + lane authority (cp-hhd7, cards 4 + 16)` section:
  - Worker result fields: `files_changed`, `commit_sha`, `test_tail`, `conflicts_surfaced`
  - Lane authority as **contextual POLICY note** (not a universal swarm mandate): in-lane decisions decided by that lane; escalate only for out-of-both-lanes or gate violations

#### skills/cc-loop-driver/SKILL.md
- Added `## External event-loop shape — the port/adapter law (card 12, cp-dfwh)`: no in-session heartbeats, arm a watch (event-driven), loop is the skill (don't re-decompose)
- Added `## Watch authoring — failure modes to guard (card 13, cp-gib3)`: FM1 false-fire on own writes, FM2 guard every poll, FM3 enumerate failure states
- Added `## Never send an editor to a running agent (card 14, cp-gib3 FM4)`

#### skills/agent-mail/SKILL.md
- Added `## Coordination disciplines (2026-06-09, cards 1–5, cp-hhd7)` section:
  - Durable lane identities (card 1, cp-9lrb)
  - Content-push not pointers (card 2, cp-9lrb)
  - Intent on the graph first — dedup (card 3, cp-hhtu)
  - ACK-with-id on routed writes (card 5, cp-fmt8)

#### skills/beads-workflow/SKILL.md
- Added `## Lifecycle disciplines (2026-06-09, cards 17–20, cp-hhd7)` section:
  - Claim-verify before dispatch (card 3, cp-hhtu)
  - Merged-before-close (card 17, cp-4gj6) as POLICY pointer to gate cp-hxp6
  - Close with residual routed (card 19, ag-67yy): `br close --reason "Residual → <new-id>"`
  - Append notes never replace (cp-7fxr): the note-eater anti-pattern named
  - Fuzzy intent → bead in same turn (card 20, cp-honb)

#### skills/council/SKILL.md
- Added `## Cross-family floor — policy note (card 21, cp-hhd7)` section:
  - Same-model council is valid for non-assurance
  - Cross-family floor is icb6's policy; `--mixed` is the supporting mode
  - A7 gemini bench: CURRENT POLICY, graduation condition stated
  - Mechanism is general; policy lives in the gate (hexagonal cut preserved)

#### skills/using-atm/SKILL.md
- Added `## Single-writer + merged-before-close (cards 17–18, cp-4gj6; POLICY → gate cp-hxp6)` section:
  - Merged-before-close: protection-off until trunk-visible
  - Read canonical not shared main: stale reads are the other half of split-brain
- Extended Anti-patterns: two new ❌ entries (close before merge, read from stale main)

---

## Validation outputs

```
validate-skill-frontmatter.sh: 166/166 ok (0 errors; warnings only on pre-existing optional fields)
go test ./cmd/ao/ -run TestSkillContract -count=1: 1032 passed
```

---

## Live-deploy file list (changes deploy on merge — symlinked via cp-zkyx)

- skills/validate/SKILL.md
- skills/codex-exec/SKILL.md
- skills/cc-subagents/SKILL.md
- skills/swarm/SKILL.md
- skills/cc-loop-driver/SKILL.md
- skills/agent-mail/SKILL.md
- skills/beads-workflow/SKILL.md
- skills/council/SKILL.md
- skills/using-atm/SKILL.md

---

## Dedup ledger — NOT re-beaded (already in cp-2h6x or memories)

- codex-exec stdin-pipe canonical: partially in cp-2h6x; extended here with judge-prompt pattern
- rust-builds-never-on-mac: fleet config / rch — not baked into general codex-exec (remains memory + rch skill)
- gemini-lane-is-agy: policy in caam/dcg, not general skill text

## Residual — none

All cards from the source map that target these skills are covered. Cards targeting
skills outside this batch (beads-br, beads, cc-cron-ticks) are out of scope and
tracked in the parent epic cp-wn8u.
