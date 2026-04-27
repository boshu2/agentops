# Handoff: Land Eval Environment — Wave 2 Start

**Date:** 2026-04-27
**Previous handoff:** `.agents/handoff/2026-04-27-land-eval-environment-wave-1.md`
**Status:** Wave 1 complete (3/3 prereqs closed). Wave 2 = PR-1 cherry-pick (foundation 7 commits) on a fresh branch off main.

---

## What This Session Accomplished (Wave 1)

### 1. State recovery on resume

- Found 4 dirty files at start (handoff said 2; 2 extras were prior-session discovery artifacts that never committed: `.agents/findings/registry.jsonl` with 3 new findings, `.agents/rpi/execution-packet.json` = the wave plan).
- Found local main was 11 commits behind `origin/main`.
- One incoming commit (`9b08fb93`) modified `.agents/rpi/execution-packet.json` — clean conflict zone vs. our wave plan.
- Resolution: `git stash` → `git pull --ff-only` → `git stash pop` → resolved single conflict on execution-packet.json by keeping ours (the wave plan).
- All 4 dirty files preserved as before (intentionally not committed; they belong to prior sessions or pending work).

### 2. ag-l0w (closure-integrity parser fix) — CLOSED

- **Commit:** `5893c525` `fix(post-mortem): closure-integrity audit reads close_reason via issue_audit_text helper`
- **Approach:** Wholesale port of `skills/post-mortem/scripts/closure-integrity-audit.sh` from `origin/codex/eval-env-discovery` (verified safe — main had zero commits on this file since merge-base; branch had exactly one parser-fix commit `0cbd7d74`).
- **Verified:** `.agents/research/2026-04-27-closure-integrity-helper-smoke.sh` smoke test — 3 of 3 closed beads (ag-0af, ag-x4g, ag-8v8) surface `close_reason` content via the new helper. End-to-end `extract_scoped_files` also tested.
- **Caveat:** Literal acceptance criterion (run script with bead-id, look for "extracted from close_reason" string) was unsatisfiable as worded — script audits epic CHILDREN, no closed epic on main has formal `bd children`, and the script doesn't emit that exact string. Substituted equivalent semantic test on the underlying helper.

### 3. ag-5p8 (security-toolchain governance eval) — CLOSED

- **Commit:** `972c674b` `feat(evals): add security-toolchain governance canary suite (ag-5p8 prereq)`
- **Approach:** Ported 3 files byte-identical from branch commit `d5c49b2b`:
  - `evals/agentops-core/security-toolchain-governance.json` (suite, 7 cases)
  - `evals/agentops-core/fixtures/security-toolchain-governance-smoke.sh` (300+ line fixture)
  - `.agents/evals/baselines/agentops-core.security-toolchain-governance.baseline.json` (baseline)
- **Verified:** All JSON valid (`jq -e`); fixture parses (`bash -n`); sha256 alignment between suite (`4844c96a...`) and baseline reference; 3 of 6 fixture cases PASS on macOS.
- **Three known issues documented for PR-1 to resolve:**
  1. **Schema-suite domain mismatch** — branch's `eval-suite.v1.schema.json` `domain` enum is `{skill,hook,cli,rpi,runtime,retrieval,scenario,mixed}` but the suite uses `"domain": "security"`. PR-1 must add `"security"` to the schema enum or change the suite's domain.
  2. **Fixture PATH override** — `PATH=$repo/mockbin:/usr/bin:/bin` defeats homebrew bash 5+ on macOS, falls through to system bash 3.2 which lacks `declare -A`. Three of six cases trip this. Linux CI is unaffected.
  3. **`ao eval run` absent on main** — full runtime acceptance defers to post-PR-1.

### 4. ag-aez (beads CLI ref audit) — CLOSED

- **Commit:** `f342d053` `docs(audit): record beads CLI reference link audit (ag-aez prereq)`
- **Audit log:** `.agents/research/2026-04-27-beads-cli-ref-audit.md`
- **Findings:** No stale refs (upstream `09ace562` already removed CONFIG.md/DAEMON.md/LABELS.md before this session). Shared and codex variants byte-identical. All See Also targets exist. All Quick Navigation anchors valid. `bd doctor --check=conventions` reports `conventions.orphans` PASS.
- **Doc-quality observation (out of scope, NOT fixed):** Quick Navigation lists 6 of 14 `## ` sections — discoverability gap, separate concern from stale refs.

---

## Where We Paused

**Last action:** ag-aez closed, audit log committed. Three local commits ahead of `origin/main`, no push performed (per user's explicit instruction "Do NOT push any branch to origin without explicit approval").

**Local commits awaiting push:**

```
f342d053 docs(audit): record beads CLI reference link audit (ag-aez prereq)
972c674b feat(evals): add security-toolchain governance canary suite (ag-5p8 prereq)
5893c525 fix(post-mortem): closure-integrity audit reads close_reason via issue_audit_text helper
1e488270 [origin/main] Merge pull request #167 from boshu2/codex/fix-agents-write-surface-smoke
```

**Working tree state:** 4 dirty files preserved from prior session (pre-existing, intentionally not committed):

```
 M .agents/findings/registry.jsonl                              # 3 findings from discovery session
 M .agents/learnings/2026-04-19-orchestrator-compression-anti-pattern.md  # pre-handoff dirty
 M .agents/patterns/pre-tag-ci-validation.md                     # pre-handoff dirty
 M .agents/rpi/execution-packet.json                             # the wave plan (resolved as ours during pull)
```

**Next action (Wave 2, fresh session):**

Per the user's wave-boundary rule, Wave 2 is PR-1 cherry-pick — git surgery — and goes to a fresh session.

User said in the original handoff: "**Phase 2 starts on a separate confirmation since it's git surgery.**" Get explicit approval from the user before executing Wave 2 cherry-picks.

`bd ready` now shows `ag-664` (PR-1 cherry-pick) and the parent epic `ag-3lx` as ready.

---

## Decisions Made This Session (Do Not Relitigate)

- **Stash + ff-pull + pop** is the recovery pattern when local commits are 0 and dirty files conflict with incoming. Took ours on `.agents/rpi/execution-packet.json` because the wave plan IS the active execution doc.
- **Wholesale file replacement** (vs. patch-style port) is acceptable when main has zero commits on the target file since merge-base AND the branch's only commits to that file are the desired fix. Verified for `closure-integrity-audit.sh`. Not safe to assume for files that both branches modified.
- **Acceptance criteria as worded can be unsatisfiable.** When that happens, identify the underlying intent ("does the parser fix work?") and write a semantic test that proves it. Document the substitution clearly in the close reason.
- **Pre-existing limitations of the branch** (schema-suite domain mismatch, fixture macOS bash trap) are NOT prereq blockers. Document in the closure reason and let PR-1 resolve.

---

## Files to Read (Wave 2)

```
# Priority — read first
.agents/handoff/2026-04-27-land-eval-environment-wave-2.md      # this file
.agents/rpi/execution-packet.json                                # wave structure (Wave 2 = PR-1)
.agents/plans/2026-04-27-land-eval-environment.md                # PR-1 section, hand-merge files

# PR-1 implementation references
git log --oneline cb1fc5d9..f3af16da                             # the 7 foundation commits
git show origin/codex/eval-env-discovery:.github/workflows/validate.yml
git show origin/codex/eval-env-discovery:scripts/pre-push-gate.sh
git show origin/codex/eval-env-discovery:tests/scripts/pre-push-gate.bats

# Hand-merge file currents (on main)
.github/workflows/validate.yml
scripts/pre-push-gate.sh
tests/scripts/pre-push-gate.bats
AGENTS.md
docs/contracts/index.md
docs/CI-CD.md
docs/documentation-index.md
```

---

## Open Questions for Wave 2

1. **Push the 3 Wave-1 commits to origin first, or roll into PR-1?** They're on `main` locally; PR-1 will be a separate branch. The Wave-1 commits should stay on `main` (fixes + new evals are not part of the PR-1 cherry-pick) — but they should be pushed before PR-1 starts so the PR-1 branch starts from a known-published state. Ask user before pushing.
2. **Schema-suite domain mismatch (ag-5p8 finding 1):** PR-1 must resolve. Add "security" to the schema enum, OR change the suite's `"domain": "security"` to `"domain": "mixed"`. Recommend the schema fix — security IS a primary domain and should be first-class.
3. **Fixture PATH macOS trap (ag-5p8 finding 2):** Probably out of PR-1 scope (it's a fixture-internal limitation, not a foundation issue). Leave for a later cleanup.

---

## Constraints From This Session (Carry Forward)

- **All Wave-1 prereqs closed** — ag-l0w, ag-5p8, ag-aez. Their results unblock PR-3 (`ag-xsy`).
- **Three local commits on main, not pushed.** User explicitly required approval before pushing.
- **`scripts/toolchain-validate.sh` requires bash 4+** for `declare -A`. Branch CI runs on Linux so no problem there; macOS dev-loop with the new fixture has caveats.
- **Wave-boundary rule:** Each wave starts in a fresh Claude session per user preference. Wave 2 (PR-1 cherry-pick) needs fresh session + explicit user approval before git surgery starts.
