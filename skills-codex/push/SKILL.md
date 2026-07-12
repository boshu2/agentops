---
name: push
description: "Validate, commit, and push."
---
# Push Skill

Ship a change to `main` **in this repo** with proof. AgentOps lands work by **direct push to main** — PR-per-change is retired here. The release authority is the **local cockpit gate** plus a **CONFIRMED cross-family pawl verdict** bound to the commit.

## Constraints

- **No verdict means no push.** Require a CONFIRMED commit-bound pawl verdict because green producer tests cannot independently authorize release.
- **Keep reviewed identity explicit.** Commit every repair and re-earn the verdict before landing because a verdict for an earlier feature SHA proves a different artifact; after landing, distinguish the reviewed feature from an optional canonical provenance-bind tip.
- **Contain the write surface.** Stage only bead-owned files and exclude credentials/private ledgers because direct-main landing makes accidental inclusion immediately public.
- **Consult the pawl before raising the andon.** WARN, FAIL, or REFUTED repairs and reruns automatically because ordinary rejection is useful evidence, not a breaker; only a breaker may enter HOLD or consume one helper.

## Breaker State Machine

- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** repair, recommit, and rerun gate plus pawl; plain rejection never enters HOLD and never consumes the helper lane.
- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** pause landing in HOLD and route exactly one bounded helper consultation.
- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, resume repair, and re-earn gate plus independent verdict before landing.
- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and surface the helper's escalation to the human operator.
- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** stop automation and route directly to the human operator with the helper skipped.

## The invariant — no verdict = no push

**Never push without a CONFIRMED, commit-bound pawl verdict.** This replaces the old "never push to main without permission" guardrail: direct-main IS the routine path for THIS repo, and the pawl verdict IS the permission. A push carrying no CONFIRMED verdict is refused by the pre-push hook (`scripts/check-pawl-pre-push.sh`); the ONLY waiver is a `#trivial` docs/provenance-only commit. **No verdict = not done.** (PR flow survives ONLY for external repos — see below.)

## Ship path (THIS repo)

Run in order from the bead's own worktree (worktree-mandatory under shared load — never edit the canonical checkout).

### Step 1: Pre-flight — build + the tests the diff touches

Fail fast locally before the gate. Run what the diff actually exercises:

- **Go** (`cli/` changes): `cd cli && go build ./... && go vet ./... && go test ./...` — the **whole** suite, never a `-run <feature>` subset. A filtered run stays green while cross-cutting conformance / surface-parity tests are red; they only surface at push.
- **Python:** `python -m pytest --tb=short -q` for the touched package.
- **Shell:** `shellcheck <modified .sh files>` (if installed).
- **Regenerated artifacts:** if you touched a *generating* source (a CLI command/flag, a skill, a schema), regenerate its derived file NOW and commit it WITH the change — `make regen-all` (or scoped `scripts/regen-changed-scope.sh --scope head`); for skills, `scripts/regen-codex-hashes.sh --only <name>`.

Any failure -> STOP and fix. Then commit the bead's code as HEAD — the message MUST cite the bead id (the gate and pawl resolve the bead from the commit message).

### Step 2: Local cockpit gate

```bash
ao gate check --fast --scope head
```

The smart conditional Go gate — checks only what changed; the same gate the pre-push hook runs, so running it manually fails fast. (`AGENTOPS_GATE_BASH=1` is the documented legacy fallback only.)

### Step 3: Pawl review — the cross-family verdict (CONFIRMED required)

```bash
REVIEWER=agy bash scripts/pawl-review.sh <bead> --scope head --author-family codex
```

Dispatches the refuter (fresh-context, read-only, verdict-only) against the HEAD commit. **You are a Codex-runtime author: pass BOTH halves.** `--author-family codex` declares your family (the script's default is `claude`; omitting it would silently bind a same-family codex verdict). `REVIEWER=agy` routes the review to a different family — required because the DEFAULT reviewer is codex and the script refuses a same-family bind (without the override it exits 2, "no cross-family reviewer available"). Any non-codex reviewer family works. On **CONFIRMED** (exit 0) it writes the commit-bound verdict at `.agents/pawl-verdicts/<bead>.json` the pre-push gate requires. On **REFUTED** (exit 3) it prints the defects + saves evidence — fix, re-commit, re-run; a REFUTED is final for that commit. LAW 0: the refuter is codex, never `claude -p`. Use `--scope staged` for a review-only pass before committing. `$pawl-review` owns immutable reviewer execution and evidence handoff; `ao pawl` owns diversity, commit binding, and admission.

### Step 4: Land — deterministic single-shot push

**Checkpoint:** confirm HEAD cites the bead, the fast gate passed on that exact HEAD, and the CONFIRMED verdict is commit-bound before invoking the land command. After landing, run the landed verifier so a canonical provenance-bind tip is separated from its reviewed feature parent without trusting either a marker or path alone.

```bash
bash scripts/pawl-land.sh <bead>
```

Fetch + rebase onto current `origin/main`, restamp the CONFIRMED verdict onto the post-rebase feat, single-shot `push origin HEAD:main`. It enforces its own preconditions: HEAD cites the bead and a CONFIRMED verdict exists. On a rebase conflict it **aborts without pushing** — resolve locally, re-run pawl-review if the tree changed, re-land. **Do NOT force-push.**

### Step 5: Report

Run `bash skills-codex/push/scripts/verify-landed.sh <bead>`, then report files changed, suites run, verdict disposition, reviewed SHA, and landed tip. The verifier fetches `main`, proves local/remote tip identity, recognizes `HEAD^` only for the exact canonical auto-bind subject plus its non-empty provenance-ledger-only diff, requires the reviewed commit to cite the bead, and checks the real pawl verdict against that reviewed SHA.

## Output Specification

- **Path:** remote `origin` ref `refs/heads/main`, with proof at `.agents/pawl-verdicts/<bead>.json`.
- **Filename convention:** `<bead>.json` for the pawl verdict bound to the reviewed commit contained by the pushed ref.
- **Serialization/schema format:** landed-tip and reviewed-commit Git SHAs plus the JSON pawl-verdict schema bound to the reviewed SHA.
- **Validator command:** set `BEAD="<bead-id>"`, then run `bash skills-codex/push/scripts/verify-landed.sh "$BEAD"`; do not replace this with inline `HEAD`/`HEAD^` guessing.
- **Downstream handoff:** send both verifier-reported `tip=<sha>` and `reviewed=<sha>`, verdict path, suites run, and remote-ref proof to closeout; close the tracker only after this helper succeeds.

## External repos (PR flow only)

The PR-per-change flow survives **only for external repos** (upstream forks) where you cannot push `main`. For those, prepare the PR with `$pr-prep` instead of `pawl-land.sh`.

## Guardrails

- **Never push without a CONFIRMED commit-bound verdict** (no verdict = not done). The pawl verdict is the authority; direct-main is routine for THIS repo.
- NEVER stage files matching: `.env*`, `*credentials*`, `*secret*`, `*.key`, `*.pem`.
- Stage only files relevant to the work; no `git add -A` unless explicitly requested. Never `git add _beads` (private nested ledger).
- On a rebase conflict, do NOT force-push — `pawl-land.sh` aborts; resolve locally and re-gate.
- Do NOT run `ao codex stop` after the remote push. If session closeout is needed, finish it through `$validate`, `$post-mortem`, or `$handoff` before entering `$push`.

## Quality Checklist

- The commit subject cites the bead, the diff contains only owned files, and sensitive/private paths are absent from the index.
- Deterministic tests and `ao gate check --fast --scope head` pass on the exact commit reviewed by the pawl.
- The verdict is CONFIRMED, independent, cross-family where required, and bound to the verifier-resolved reviewed SHA contained by the landed tip.
- The landed verifier proves `HEAD == origin/main` and reports both tip and reviewed SHAs; ordinary rejection stays in AUTO-REDO and only the breaker state machine reaches HOLD or HUMAN.

## Local Resources

### scripts/

- `scripts/validate.sh`
