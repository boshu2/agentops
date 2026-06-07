---
name: operating-loop-skill
user-invocable: false
description: |-
  Use when driving one bead end-to-end through claim, work, independent validation, closeout, and persistence.
  Triggers:
practices:
- design-by-contract
- agile-manifesto
hexagonal_role: domain
consumes:
- beads
- git
produces:
- evidence/<id>.md
- git-commit
- closed-bead
context_rel:
- kind: customer-of
  with: beads
- kind: supplier-to
  with: agy-native
- kind: supplier-to
  with: cc-loop-driver
skill_api_version: 1
user-invocable: true
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: orchestration
  dependencies: [beads-br, beads-bv, dcg, ntm]
  stability: stable
output_contract: "Per closed bead: an evidence file (evidence/<id>.md), a verified PASS verdict, a `br/bd close` carrying the evidence ref, and a git commit. Loop emits a state word: NO_READY | CONVERGED | ESCALATE."
---

# Operating Loop Skill

The portable, on-disk form of the AgentOps operating loop + bead-crank. It drives **one tracker bead** through the assured trajectory **claim → work → independent-validate → close → persist**, on any harness (Codex, Gemini, Claude), without depending on Claude plugin slash-commands.

## Why This Exists

/operating-loop and /bead-crank are Claude-plugin slash-commands — **Codex and Gemini cannot load them**, so a non-Claude (or vendor-mixed) factory tick has no way to run the assured loop. This skill encodes the *same* single-writer, evidence-gated loop as plain on-disk instructions any harness can follow, so the loop is one contract across vendors. It is the per-bead inner crank of the flywheel: each tick converts one ready bead into one independently-verified, committed increment — the smallest unit of assured forward motion. The assurance is structural (author ≠ judge; evidence or it didn't happen), not a function of which model runs it.

## Overview / When to Use

The invariant is the loop, realized on whatever fan-out the harness has:

| Role | Claude | Codex | Gemini |
|---|---|---|---|
| Orchestrator | main agent + `tick.sh` | main agent + `tick.sh` | main agent + `tick.sh` |
| Worker (fresh context) | Agent subagent | `codex exec` (fresh) or NTM pane | gemini swarm pane or NTM pane |
| Validator (fresh context) | Agent subagent | `codex exec` (fresh) or NTM pane | gemini swarm pane or NTM pane |

The **separate context** — not a separate vendor — is what makes "author ≠ judge" hold. Worker and validator must be *different* invocations.

## ⚠️ Critical Constraints

- **Single writer.** ONLY the orchestrator runs `close`. Workers and validators NEVER close, never set status, never self-grade. **Why:** if the author closes its own work, the verdict is self-reported and the loop produces unverified "looks good" closes.
- **Author ≠ judge.** The worker context and the validator context MUST be different invocations. **Why:** a context grading its own output is biased toward PASS; independence is the entire assurance.
- **Evidence or it didn't happen.** Every close cites a real proof surface (commands run + output, greps, test results). **Why:** agents are ephemeral — the proof must live in the artifact (`evidence/<id>.md`), not in chat memory.
- **Verdict must carry COMMANDS RUN.** A validator verdict (PASS or FAIL) with no cited commands is rejected as *unverified* and routed to a tie-break — it is never acted on. **Why:** an unverified FAIL is the L2 false-FAIL failure mode; an unverified PASS is a rubber stamp.
- **No `claude -p` / `claude --print` for workers.** Use in-harness subagents (Max sub), NTM panes (OAuth), or `codex exec` (Pro sub). **Why:** `claude -p` bills the API per-token instead of the subscription — banned for worker dispatch.
- **Idempotent.** Each tick reads state first (`next`) and does only the next undone increment. **Why:** re-running a tick must never double-work or re-pull a closed bead.
- **No Dolt for this loop.** `br`/`bd` (SQLite + JSONL) + git IS the entire state/persistence story. **Why:** git is the durable ledger; the close reason carries provenance.

## Workflow / Methodology

State mechanics are wrapped in `tick.sh` (subcommands: `next`, `claim`, `verdict-gate`, `council-gate`, `close`). If no `tick.sh` is present, the raw `br`/`bd` equivalents are given inline. Use `br` where the repo's `.beads/` is beads_rust; use `bd` where it is beads — check the repo's `AGENTS.md`/`CLAUDE.md` for which tracker wins.

### Phase 1: Pick + claim (orchestrator)
```
id=$(./tick.sh next)          # or: br ready --json | first unblocked, highest-priority
                              #     (no tick.sh? `bd ready` / `br ready`)
[ -z "$id" ] && echo NO_READY && exit 0
./tick.sh claim "$id"          # or: br update "$id" --claim  /  bd update "$id" --claim
```
Read the bead's ACCEPTANCE verbatim — it is the contract for both worker and validator.
**Checkpoint:** confirm `id` is non-empty and now in-progress before dispatching. If empty, stop with `NO_READY`.

### Phase 2: Work (worker — fresh context, PROPOSES only)
Dispatch a worker in a fresh context (Agent subagent / `codex exec` / NTM pane). Give it the bead id, title, ACCEPTANCE verbatim, and the AgentOps image (relevant skills). The worker:
1. Does exactly what ACCEPTANCE requires — no scope creep, no files outside the bead's concern.
2. Writes `evidence/<id>.md` with: WHAT changed (file:line per change), PROOF (commands run + output, greps, tests), ACCEPTANCE MAP (one line per acceptance point → how met).
3. Returns the evidence path + a 3-line summary. **It does not close, status-set, or self-grade.**
**Checkpoint:** confirm `evidence/<id>.md` exists and maps every acceptance point before validating.

### Phase 3: Independent validate (validator — fresh context, JUDGES only)
Dispatch a validator in a **different** fresh context (NOT the worker's). Tell it: you did not author this; author ≠ judge. It reads `evidence/<id>.md` AND the real artifacts it cites (re-runs the commands / opens the files — does not trust the claims), judges each acceptance point strictly, and returns EXACTLY:
```
VERDICT: PASS | FAIL
COMMANDS RUN: <commands executed + a snippet of each one's output>   # mandatory
REASONS: 2-4 bullets, each pointing at one of the COMMANDS RUN above
```
Default to FAIL on genuine uncertainty (fail-closed). For higher assurance (Themis council), run **two** independent validators in two different contexts.
**Checkpoint:** run `./tick.sh verdict-gate <verdict-file>` (or manually confirm a non-empty COMMANDS RUN). A verdict with no cited commands is **rejected as unverified** — route to a fresh tie-break validator; never act on it. With two validators, run `./tick.sh council-gate <v1> <v2>`.

### Phase 4: Close (orchestrator — sole writer) or reopen
- **PASS** (or council PASS = unanimous + all verified):
  ```
  ./tick.sh close "$id" "<conventional commit msg>" "evidence/<id>.md" <scoped paths...>
  # raw: br close "$id" --reason "...evidence/<id>.md" && git commit
  ```
  The close reason carries the evidence ref; the commit is the durable ledger.
- **Verified FAIL:** reopen with the validator's REASONS injected → informed fresh-worker retry. Budget **2 retries**, then **ESCALATE**.
- **Contested/unverified FAIL, or mixed council verdicts (DISAGREEMENT):** dispatch a fresh **tie-break** validator; majority of *verified* verdicts decides; fail-closed if no majority. The orchestrator records both verdicts and never self-overrules.
**Checkpoint:** after a PASS close, confirm the bead is closed and a commit landed.

### Phase 5: Persist + repeat
```
br sync     # or: bd sync   (JSONL export — git IS the provenance)
```
Then loop back to Phase 1 until `next` is empty. Emit the precise state word:
- **NO_READY** — no schedulable bead right now (blocked/in-progress may remain).
- **CONVERGED** — no open OR in-progress beads remain.
- **ESCALATE** — a bead exhausted its retry budget; hand to a human/quorum.

## Output Specification

**Format:** per-bead Markdown evidence file + tracker close + git commit. Loop emits one state word.
**Filename:** `evidence/<id>.md` (worker), `evidence/<id>.v1.md` / `evidence/<id>.v2.md` (council verdicts).
**Structure (evidence file):** `WHAT` (file:line changes) · `PROOF` (commands + output) · `ACCEPTANCE MAP` (point → how met).
**Structure (verdict):** `VERDICT` · `COMMANDS RUN` · `REASONS`.
**Loop return:** `NO_READY` | `CONVERGED` | `ESCALATE`, plus the list of bead ids closed this run.

## Quality Rubric

- [ ] Worker and validator ran in **different** contexts (author ≠ judge).
- [ ] An `evidence/<id>.md` exists and maps EVERY acceptance point to a proof surface.
- [ ] Every validator verdict carries a non-empty `COMMANDS RUN` (else rejected as unverified).
- [ ] Only the orchestrator ran `close`; no worker/validator set status.
- [ ] The close reason cites the evidence ref AND a git commit landed.
- [ ] No `claude -p` was used to dispatch a worker or validator.
- [ ] Each tick read `next` first and did exactly one increment (idempotent).
- [ ] On FAIL, retry budget (2) respected; contested/unverified FAIL got a tie-break, not a self-overrule.
- [ ] Final state word is precise: `NO_READY` ≠ `CONVERGED`.

## Examples

**Drive the top ready bead (Codex):**
```
id=$(./tick.sh next); ./tick.sh claim "$id"
# worker: codex exec "WORKER: bead $id, ACCEPTANCE <...>, write evidence/$id.md, do not close"
# validator (fresh codex exec): "VALIDATOR: judge evidence/$id.md vs ACCEPTANCE, return VERDICT/COMMANDS RUN/REASONS"
./tick.sh verdict-gate evidence/$id.v1.md && ./tick.sh close "$id" "feat: ..." "evidence/$id.md" <paths>
```

**Council (two validators, higher assurance):** dispatch two fresh validator contexts → `./tick.sh council-gate evidence/$id.v1.md evidence/$id.v2.md` → close only on unanimous + all-verified PASS.

**Drain the queue:** loop Phases 1–5 until `./tick.sh next` is empty → emit `NO_READY` (or `CONVERGED` if nothing is in-progress either).

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| Verdict acted on but turned out wrong | No `COMMANDS RUN` cited (unverified) | Always `verdict-gate`; reject + tie-break unverified verdicts |
| Loop re-works a finished bead | Tick didn't read state first | Start every tick with `next`; ensure `claim`/`close` actually persisted |
| "looks good" closes with no proof | Worker self-graded / closed | Enforce single-writer: only orchestrator closes; worker writes evidence only |
| API bill spiked during a loop | `claude -p` used for workers | Use Agent subagents / NTM panes / `codex exec` (subscription lanes) |
| Validator keeps PASS-ing weak work | Same context judged its own work | Force a different invocation for the validator (author ≠ judge) |
| `next` empty but work remains | Beads blocked or in-progress | `NO_READY` is correct; check `br ready`/`bv` for blockers; `CONVERGED` only when none open/in-progress |

## See Also / References

- `beads-br` / `beads-bv` — the `br`/`bv` tracker primitives this loop schedules over.
- `ntm` / `vibing-with-ntm` — NTM panes for non-Claude (or parallel) worker/validator fan-out.
- `dcg` — destructive-command guard (the admission layer that hard-rejects unsafe moves).
- `agentops:rpi` — the heavyweight seven-move loop for shaping a whole capability from scratch (this skill is the per-bead inner crank).
- Reference implementation (proven n=1): `~/dev/control-plane/{ARCHITECTURE,ORCHESTRATOR,WORKER,VALIDATOR}.md` + `tick.sh` (the `tick.sh` subcommands this skill calls — `next`/`claim`/`verdict-gate`/`council-gate`/`close` — are implemented there).
- **Execute** `scripts/validate.sh` to self-check this skill (frontmatter + section spine + Form-A line budget); exit 0 = pass.
