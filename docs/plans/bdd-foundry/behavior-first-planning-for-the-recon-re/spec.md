# Spec — Recon-Recommended Work (bdd-foundry Phase 3, SPEC)

> **Phase 3 of bdd-foundry.** Derived from the FROZEN [`behaviors.md`](behaviors.md)
> and the runnable [`acceptance-tests.md`](acceptance-tests.md). This spec's ONLY
> job is to make those tests go green — nothing more. Every component below is
> traced to the scenario(s) it satisfies; nothing here exists without a test that
> demands it. A spec, not a monument.
>
> **Red-run ground truth (2026-06-14):** 37 red, 8 already-green guardrails, 0
> harness-broken. The guardrails (A0-S2..S5, B1-S3/S4, B4-S1/S2, B3-S1, B5-S4)
> are invariants the work must NOT break; this spec preserves them by construction
> and calls out the one place each could regress.
>
> **Tracker:** `br` at `_beads/` (`BEADS_DIR=$PWD/_beads br`), run from the main
> checkout. **Models:** Claude-family workers = `opus`; cross-family validation =
> `codex`/`agy`; **never** `fable` (Mythos revoked). **Standing adversarial
> dimensions** (fail-closed; no caller-forgeable trust marker; raw strings are
> data never shell/prompt; enforce at the acting sink; no harness-only proof)
> apply to every component below.

---

## 0. Component inventory (what changes, by surface)

| # | Component | Surface (file/path) | New or change | Scenarios satisfied |
|---|---|---|---|---|
| C1 | In-repo `codebase-recon` workflow | `.claude/workflows/codebase-recon.js` (NEW) | NEW vendored file | A0-S1, A1–A4, A5 (note) |
| C2 | Workflow ledger row | `docs/contracts/skill-dispositions.yaml` `workflows.codebase-recon` | NEW row + regen `registry.json` | A0-S1, A0-S2, A0-S5 |
| C3 | Governance path+identity hardening | `scripts/check-workflow-governance.sh` | CHANGE | A0-G10, A0-G14 |
| C4 | Monitor task-state guidance | `docs/memory/monitor-binds-task-state.md` (NEW) | NEW doc | A5-S1, A5-S2, A5-G12 |
| C5 | CI wiring for doc-skill-refs | `.github/workflows/validate.yml` + sweep of stale refs | CHANGE | B1-S1, B1-S2, B1-S5, B1-G9 |
| C6 | Structural allowlist exemption | `scripts/check-doc-skill-refs.sh` | CHANGE | B1-S5, B1-G11 |
| C7 | CHANGELOG v3.2.0 + tag | `CHANGELOG.md` (+ `git tag v3.2.0`) | CHANGE + git | B2-S1..S4, B2-G15 |
| C8 | Quorum doctrine ratification | `docs/.../quorum doctrine note` + two fleet memories | CHANGE (doc/memory) | B3-S2, B3-S3, B3-S4 |
| C9 | Forged-ratification sink fix | `cli/internal/liveness/admission.go` | CHANGE (Go) | B5-G3 (keeps B3-S1 green) |
| C10 | Codex dispatch trust boundary | `cli/cmd/ao/codex.go` (`performCodexDispatch` + helpers) | CHANGE (Go) | B5-S1, B5-S2, B5-G1, B5-G2 |
| C11 | Symlink/TOCTOU-safe output paths | `cli/cmd/ao/codex.go` (`resolveCodexDispatchPath`) | CHANGE (Go) | B5-G13 |
| C12 | Provenance keying decision | keyed-digest OR `docs/provenance/keying-decision.md` (NEW) | NEW doc (recommended) | B5-S3 |
| C13 | `cli/cmd/ao` decomposition (EPIC) | `cli/cmd/ao/codex/` sub-package | CHANGE (deferred) | B4-S3 (keeps B4-S1/S2 green) |

The guardrails-only scenarios (A0-S2..S5, B1-S3/S4, B4-S1/S2, B3-S1, B5-S4) need
no new component — they are preserved as side conditions of C1–C13.

---

## Stream A — codebase-recon workflow hardening

### C1 — Vendor `.claude/workflows/codebase-recon.js` (the A0 BLOCKER)

The session-ephemeral `codebase-recon` workflow (today only the `codebase-recon`
skill exists; `.claude/workflows/codebase-recon.js` is ABSENT — verified) must be
vendored as a governed repo citizen. A1–A4 grep this exact file; until it exists
every A1–A4 test errors on `[ -f "$WF" ]`. **This is the blocker dep for all of
Stream A.**

Required source shape (each line is a grep contract from `stream-a-codebase-recon.bats`):

- **A0-S1 — identity:** `export const meta = { name: 'codebase-recon', ... }` —
  `meta.name` literal exactly `codebase-recon` (matches `name:\s*['"]codebase-recon['"]`).
- **A1-S1/S3 — no dead pin:** the source contains **zero** occurrences of the
  string `fable` UNLESS each occurrence is a rejection guard (A1-S3 allows `fable`
  only in a `reject|unsupported|unavailable|never` context). Simplest satisfying
  design: name it once, in the A1-G7 reject guard, nowhere else.
- **A1-S2 — override threaded:** read `args.model` and pass `model: <resolved>` to
  every `agent()` worker AND repair call. Resolution: `args.model || <session model>`;
  no hardcoded literal default.
- **A1-G7 — bad override rejected before fan-out:** a model-selection step that,
  given `args.model === 'fable'` (or any unavailable tier), `throw`s / returns
  `{ status: 'failed' }` with the literal `unsupported model override` BEFORE any
  worker/repair `agent()` dispatch. Must not produce an empty-but-green result.
- **A2 (HIGHEST VALUE) — fail-closed empty-output guard:** after the repair round
  and BEFORE the `Synth…` phase, a guard that:
  - counts **USABLE** reports (A2-G4): a report counts only if non-empty AND
    carries the required report marker/heading (grep contract:
    `size|length|non-empty|trim|byteLength|usable`).
  - returns `{ status: 'failed', reports_landed: 0, reason: ... }` when zero usable
    reports (A2-S1, A2-S3) — unambiguous failure, never a green summary.
  - treats a report-dir IO error (ENOENT / unreadable / readdir throw) as a hard
    `status: 'failed'` (A2-G8 grep: `ENOENT|catch|readdir|IO|unreadable|error`),
    never coerced to "zero files → green".
  - proceeds to `Synth…` only when ≥1 usable report (A2-S2).
- **A3 — escalate-on-repair:** the repair round selects a model tier **≠** the
  worker tier (grep: `escalat|repairModel|different.*model|model.*tier`); on a
  model-unavailable straggler (null/empty agent return) it does NOT re-dispatch the
  same model (grep: `unavailable|null|empty.*return|unrepairable`) — it escalates
  or records unrepairable; the cross-family escalation target is `codex`/`agy`,
  never `fable` (A3-S3).
- **A4 — first-class scope/since:**
  - `args.scope` (string) → a `scopeBlock` injected into every worker AND repair
    prompt (A4-S1).
  - a **bare string arg** is coerced to `args.scope` (A4-S3 grep:
    `typeof.*string|string.*scope|bare string|scope`).
  - `args.since` resolves `REF..HEAD` and injects `git diff --stat <ref>..HEAD`
    plus the `git log` range into every worker prompt (A4-S2 grep:
    `diff --stat|diffstat` and `\.\.HEAD|HEAD`).
  - an unresolvable since ref fails loudly with `unresolvable since` /
    `invalid since` — **no** silent full-repo fallback (A4-S4).
  - **A4-G5 (security):** the since value goes to git as **argv/data**, never
    interpolated into a shell string. The source must NOT contain a `git …${args.since}`
    shell template (negative grep `git .*\$\{?(args\.since|since)` must miss) and
    must carry a validation guard (grep:
    `invalid since|metachar|allowlist|git rev-parse|argv|sanitiz`). Use
    `git("rev-parse", "--verify", ref)` style argv calls.
  - **A4-G6 (security):** the scope value is wrapped in a fenced/quoted data block
    inside `scopeBlock` (grep: a fence/`<scope>`/`BEGIN SCOPE`/`--- scope` marker)
    so injected newlines/backticks/"IGNORE PRIOR INSTRUCTIONS" cannot terminate the
    block or open a new instruction section; the mandatory recon instructions follow
    the encoded scope intact.

> **Bound on C1:** the workflow is a port (orchestrator), not new product
> intelligence. Mirror the structure of the four existing `.claude/workflows/*.js`
> (bdd-foundry, bead-crank, operating-loop, ship-beads); reuse their `agent()` /
> phase idioms. Do NOT re-derive a recon discipline — fold in the session script's
> behavior, harden per A1–A4, stop.

### C2 — Ledger row + regen (A0-S1, A0-S2 guardrail, A0-S5 guardrail)

Add ONE `workflows.codebase-recon` row to `docs/contracts/skill-dispositions.yaml`
carrying the identity triple the gate requires: `kind: workflow`, a Bounded
Context `domain:` (e.g. `BC5-Runtime` — recon orchestrates over the runtime), a
`hexagonal_role:` (e.g. `orchestrator`), and a `path:` pointing at the real
`.claude/workflows/codebase-recon.js` (the `path` field is what C3/A0-G10 binds).
Then `make regen-all` so `registry.json` picks up the row.

- **A0-S2 (guardrail, must STAY green):** after the row + .js exist,
  `bash scripts/check-workflow-governance.sh` exits 0 and `bash scripts/regen-all.sh --check`
  exits 0. The forward+reverse bijection already passes for a correctly-formed row.
- **A0-S5 (guardrail):** do NOT touch the four pre-existing workflow .js files or
  their ledger rows — only `codebase-recon.js` + its single row + regenerated
  `registry.json` may change.

### C3 — Governance path + identity hardening (A0-G10, A0-G14)

`scripts/check-workflow-governance.sh` today derives the id with a `grep -oE
"name:…" | head -1` (FIRST hit) and never compares the ledger `path` field to the
real tracked file. Two changes, both at the forward pass (lines ~45–85):

- **A0-G14 — parse exported `meta.name`, not the first grep hit.** The current
  `head -1` grabs a leading `// name: '…'` comment. Replace the id derivation with
  one that reads the **exported** `meta.name` value (e.g. a small node/python parse,
  or a grep scoped to a line containing `meta` / `export` and excluding comment
  lines). When the exported name has no ledger row, fail naming
  `not-zzz-comment-name-acc` or `meta.name` (the test accepts either substring).
- **A0-G10 — bind ledger `path` to the real `.js`.** In the per-workflow python
  check, additionally assert `entry.get("path")` resolves to the SAME tracked file
  the id was derived from; on mismatch, exit 1 with a message naming the workflow
  id AND the substring `path`. (Conditional note in behaviors.md: the schema DOES
  carry `path` — the insert fixtures set it — so the path-binding form applies, not
  the schemaless fallback. Record this in the A0 bead.)

> **Guardrail preserved:** A0-S2/S3/S4/S5 must still pass after C3. The fixtures in
> A0-S3 (`zzz-orphan-acc.js` with `meta.name`) and A0-S4 (phantom row) must keep
> failing as before; the new path/exported-name logic only ADDS failure modes for
> the G10/G14 fixtures, it does not relax the existing forward/reverse bijection.

### C4 — Monitor task-state guidance doc (A5-S1, A5-S2, A5-G12)

A5 is content-assertion (no runnable workflow code). Write a NEW committed
guidance file a monitor reads — `docs/memory/monitor-binds-task-state.md` (the
test's `guidance_files()` scans `docs/memory/*.md`, `.agents/playbooks`,
`.agents/planning-rules`, and the workflow note; it DELIBERATELY excludes the plan
tree, so the guidance must NOT live in this plan dir). The doc must contain, on
greppable lines:

- **A5-S1:** both `TaskGet`/`TaskOutput` AND `mtime`/`process list`/`infer` — stating
  a monitor MUST load TaskGet/TaskOutput at startup and ABORT if unavailable, and
  explicitly forbidding inferring run status from filesystem mtime or process list.
- **A5-S2:** the phrase `task-state … unavailable` (or `abort … verdict` / `MUST abort`)
  — a tool-less monitor aborts, does not emit a RUNNING/FAILED/PASSED verdict.
  Name the 2026-06-14 false-FAILED incident as the regression case.
- **A5-G12:** `malformed`/`timeout`/`partial` co-located with
  `TaskGet`/`TaskOutput`/`task-state` — the abort also fires on degraded
  (malformed JSON / timeout / partial) responses, not only on tool ABSENCE, and
  forbids mtime/process-list/marker-file/log fallback in those cases too.

---

## Stream B — recon repo action items

### C5 — Wire `check-doc-skill-refs.sh` into CI + sweep stale refs (B1-S1, B1-S2, B1-S5, B1-G9)

The detector already exists and works; it is simply not in CI (`grep -c
check-doc-skill-refs validate.yml` = 0). Two changes:

- **B1-S1 / B1-G9:** add a step to `.github/workflows/validate.yml` that runs
  `bash scripts/check-doc-skill-refs.sh --strict` in a **T0 or T1** job that is
  REQUIRED — not `continue-on-error: true`, not `if: false`, reachable by a normal
  push diff (the test asserts no `if: false` anywhere and that the detector line
  exists; the summary/gate job must not pass while this job is skipped/advisory).
- **B1-S2:** the detector must exit 0 `--strict` at HEAD — i.e. **sweep** every
  currently-stale `/skillname` ref in `CLAUDE.md`,
  `docs/architecture/operating-loop.md`, `skills/SKILL-TIERS.md` (point each at an
  existing skill, mark the line retired, or allowlist it). Run the detector now to
  enumerate findings before scheduling.
- **B1-S5:** known archival refs (docs/releases/, docs/comparisons/ — note these
  are NOT in the detector's scanned DOCS set today; if a sweep pulls them in, each
  kept-stale ref is removed OR allowlisted with an inline reason). No history
  rewrite.

> **Guardrails preserved:** B1-S3 (a non-exempt `/bug-hunt` is caught) and B1-S4 (a
> `retired` line is exempt) already pass; C6's exemption tightening must keep them
> green.

### C6 — Structural allowlist exemption (B1-S5, B1-G11)

`check-doc-skill-refs.sh` currently exempts any line matching
`retired|folded|legacy|historical` as a bare substring (line 45/106). Two changes:

- **B1-G11:** the exemption must be **structural**, not "any line containing the
  word retired". A line like `` `/zzz-phantom` is not retired; … `` must still be
  caught (exit 1, names `zzz-phantom`). Replace the loose substring exemption with
  a structured marker — e.g. a true retirement-note form (the cited skill is the
  SUBJECT of the retirement) or an explicit inline allowlist token
  (`<!-- doc-skill-refs:allow zzz reason -->`). Implement so B1-S4's
  `` `/vibe` was retired and folded… `` (the skill IS the retirement subject) stays
  exempt while B1-G11's incidental usage does not.
- **B1-S5:** the detector source must contain a recognizable `allowlist`/`ALLOWLIST`
  token (grep contract) — i.e. ship the structured allowlist mechanism, not just a
  regex tweak.

### C7 — CHANGELOG v3.2.0 + tag (B2-S1..S4, B2-G15)

Doc + git work, run AFTER C8 (B3) so the quorum wording matches ratified doctrine.

- **B2-S1:** a `v3.2.0` section in `CHANGELOG.md` covering `v3.1.0..HEAD`, listing:
  the ~104-skill prune (`ao skills retire`), provenance ledger, the quorum
  context-floor rewrite, converge loop, codex dispatch, bd/Dolt→br tracker, BC6
  added. (Greps: `skills retire|prune|~?104`, `provenance`, `quorum`, `br|tracker`,
  `BC6`.)
- **B2-S2 / B2-S4:** the quorum entry is marked `BREAKING` and states the new
  default consistently with B3 — `fresh context` + `cross-family … opt-in` (greps:
  `BREAKING`, `fresh context|cross-family.*opt-in`).
- **B2-S3:** `git tag v3.2.0` at the release-window HEAD (local lightweight tag is
  enough for this assertion).
- **B2-G15:** the tag must exist on `origin` at HEAD (`git ls-remote --tags origin
  refs/tags/v3.2.0` == `git rev-parse HEAD`). **The push is
  conductor/operator-performed — this lane does NOT push.** The assertion is the
  gate; the implementing lane prepares the tag and flags the operator to push.

### C8 — Quorum doctrine ratification (B3-S2, B3-S3, B3-S4)

> **NON-GOAL: never revert the default.** Doc/memory only; the code default stays OFF.

- **B3-S2:** write/extend a quorum doctrine note stating "the context, not the
  model, makes a judge independent; cross-family is an opt-in upgrade for
  multi-model setups" and documenting `RequireCrossFamily` as the opt-in
  strengthener.
- **B3-S3:** edit the two fleet memories `cost-law-quorum-at-gates` and
  `quorum-gate-exists` so NEITHER asserts "≥2 model families at one-way doors" as
  the default; both reflect ≥2-fresh-contexts / cross-family-opt-in.
- **B3-S4:** grep olympusd + fleet consumers for an assumed family-floor
  dependency; any real-safety-property consumer is given `RequireCrossFamily:true`
  EXPLICITLY (not a silent default flip).
- **B3-S1 (guardrail, must STAY green):** the liveness test asserts
  `SignificantActionRequest.RequireCrossFamily` defaults false AND no caller in
  `quorum.go`/`admission.go`/`guards.go` sets it true. C8 must NOT add such an
  assignment (that is the forbidden revert); C9 must also avoid it.

### C9 — Forged-ratification sink fix (B5-G3, keeps B3-S1 green)

**Confirmed-real hole.** `admission.go` line ~106: a `quorum`-source `directive`
with `QuorumRatified=true` and NO `SignificantAction`/ACK records returns
`{Allowed, Execute, "ratified quorum directive"}` — trusting the
caller-forgeable boolean. The acceptance test (`B5-G3`) supplies exactly that
(attacker sender, `QuorumRatified=true`, no `SignificantActionRequest`) and
requires `CanExecute()==false` with decision `NeedsAdmission`/`Denied` and a reason
that ratification provenance was not verified.

**Change:** in `AdmitInboundWorkMessage`, the `InboundSourceQuorum` branch must
re-derive quorum from ACK-bearing records, never from `QuorumRatified` alone. When
`QuorumRatified` is set but no `SignificantActionRequest` with verifiable ACKs is
supplied, return `inboundNeedsAdmission(...)` (or Denied) with a reason like
`"ratification provenance not verified"`. The legacy lines 106-108 and 112-114
(`if msg.QuorumRatified { return Allowed/Execute }`) are the exact branches to
fix. **Do this WITHOUT setting `RequireCrossFamily:true` anywhere** (B3-S1
guardrail) — the fix is "require verified ACK provenance", NOT "force the family
floor". Add/keep the regression that a properly ACK-bearing
`CheckSignificantAction(req)==Allowed` path still executes.

### C10 — Codex dispatch trust boundary (B5-S1, B5-S2, B5-G1, B5-G2)

The `sh -c` packet surfaces must enforce an operator-trusted-local-artifact
precondition at the **sink**, and the duplicate-sandbox escape must be closed.
Three changes in `cli/cmd/ao/codex.go`:

- **B5-S1 / B5-S2 — trust boundary.** Before executing any packet command via
  `sh -c`, assert the packet originates from an operator-trusted local location
  (the repo's operator task dir), not an arbitrary world-writable path. A packet
  loaded from outside that boundary (the test relocates it to `t.TempDir()`) must
  be refused with an error whose message contains one of:
  `trust boundary` / `operator-trusted` / `untrusted packet` (the test's
  `trustBoundaryError()` keys on exactly these). The refusal must happen before the
  fake codex marker is written (codex never invoked). Implement in
  `runCodexDispatch`/`performCodexDispatch` (or `loadCodexTaskPacket`), keyed off
  the packet path supplied to `--packet`.
- **B5-G2 — gate the SECOND sh -c surface.** `runCodexRequiredCommands` (line 1674,
  `exec.CommandContext(ctx, "sh", "-c", command)`) is a second sh -c surface. The
  same trust-boundary refusal must gate it: an untrusted-location packet whose
  `evidence.required_commands` smuggles `echo ok; touch <pwned>` must be refused
  before any required command runs (marker absent). Because the boundary is checked
  at dispatch entry (B5-S1/S2), this surface is covered as long as the check
  precedes `runCodexRequiredCommands`.
- **B5-G1 — duplicate-sandbox rejection.** `codexDispatchSandboxArg` (line 1202)
  returns on the FIRST `--sandbox`; the CLI may honor the LAST. Add a distinct
  check that rejects argv containing **more than one** `--sandbox`/`--sandbox=`
  occurrence with an error containing the literal `duplicate sandbox` (distinct
  from the existing sandbox-MISMATCH error). Must fire before exec; codex never
  invoked, no receipt written. Implement as a new validator called from
  `performCodexDispatch` alongside the existing sandbox check.

> **Guardrail preserved:** B5-S4 — a normal trusted-local packet (the standard
> `writeCodexDispatchPacket` path inside the repo) must still dispatch and produce a
> PASS receipt. The trust check must accept the in-repo operator path the existing
> tests use; define the boundary so `newCodexDispatchRepo`'s packets pass.

### C11 — Symlink/TOCTOU-safe output paths (B5-G13)

`resolveCodexDispatchPath` (line 1774) uses `filepath.Clean` + string-prefix
matching only — a `final.md` symlink inside an allowed dir pointing outside the
repo passes the prefix check, so dispatch would write through the escape. The test
creates `allowed/final.md -> /tmp/ao-dispatch-escape` and requires dispatch to fail
before writing, leaving the escape target absent.

**Change:** make the path resolver resolve symlinks (e.g. `filepath.EvalSymlinks`
on the candidate and its parent, or `O_NOFOLLOW`-style write, plus a re-check of
the resolved real path against the allowed roots) so a link escaping the
allowed-paths/cwd boundary is rejected with a path-boundary error. Guard the
TOCTOU window (resolve-then-write atomically or open-no-follow). Applies to the
final-message, JSONL, and receipt write paths routed through
`validateCodexDispatchPathBounds`/`writeCodexDispatchOutputFiles`.

### C12 — Provenance keying decision (B5-S3)

The test passes if EITHER a keyed-digest symbol exists in `cli/cmd/ao` source
(`provenanceHMAC` / `KeyedDigest` / `provenanceKeyedDigest`) OR a documented
rationale file exists with the right marker:
`docs/provenance/keying-decision.md` containing
`git history is the real tamper-evidence anchor`, OR
`docs/adr/provenance-keying.md` containing `tamper-evidence`.

**Recommended (lowest-cost, matches reality):** write
`docs/provenance/keying-decision.md` documenting the unkeyed-SHA-256 +
git-as-anchor design with the explicit rationale string `git history is the real
tamper-evidence anchor`. (The ledger is append-only JSONL anchored by git; an HMAC
key would add custody burden without a threat model that demands it. If a keyed
digest is later wanted, implement `provenanceKeyedDigest` with a wrong-key
tamper-detection test instead — but the doc satisfies B5-S3 today.)

### C13 — `cli/cmd/ao` decomposition (B4-S3) — DEFERRED EPIC, lowest priority

Behaviors B4 is explicitly deferred/epic. DONE-criteria only:

- **B4-S3:** a `cli/cmd/ao/codex/` sub-package exists and the top-level
  `cli/cmd/ao/*.go` file count drops below 633 (codex.go responsibilities move out).
- **B4-S1 (guardrail):** `go build ./... && go vet ./... && go test ./...` stay
  green.
- **B4-S2 (guardrail):** `scripts/regen-all.sh --check` clean — zero
  command-surface drift (`docs/cli-surface.json`/`.md` unchanged).

> Do NOT schedule C13 in the same wave as C9/C10/C11 — it is a large
> caller-migration refactor on the same package and will collide. Size it as an
> epic when picked up; this spec lists it only so its DONE-criteria are defined.

---

## Dependency / sequencing graph (for the Phase 4 bead DAG)

```
C1 (codebase-recon.js vendor)  ── BLOCKER ──>  A1 A2 A3 A4 hardening (all in C1)
   └─> C2 (ledger row + regen)  ── enables ──>  A0 governance green (C3 rides on real row)
C3 (governance path/identity) ── independent of C1 content, but lands with C2 ──
C4 (monitor guidance doc)     ── independent
C6 (allowlist exemption)      ── BEFORE/with ──>  C5 (CI wiring) so --strict is clean at HEAD
C5 (CI wiring + sweep)        ── after C6
C8 (B3 doctrine)              ── BEFORE ──>  C7 (B2 changelog/tag)   [B2-S4 ordering dep]
C9 (forged-ratification fix)  ── independent Go; MUST NOT set RequireCrossFamily:true (B3-S1)
C10 (trust boundary) C11 (symlink) C12 (provenance) ── same package (cmd/ao); sequence to avoid collisions
C13 (decomposition EPIC)      ── DEFERRED; never co-wave with C9/C10/C11
```

**Coherent-arc beads (one PR each, atomic-revert):** {C1+C2 (recon vendor+governed)},
{C3 (governance hardening)}, {C4 (monitor doc)}, {C6+C5 (doc-skill-refs structural
exemption + CI wiring + sweep)}, {C8 (quorum doctrine+memories)}, {C7 (changelog+tag,
after C8)}, {C9 (admission fix)}, {C10 (codex trust boundary+dup-sandbox)},
{C11 (symlink-safe paths)}, {C12 (provenance keying doc)}, {C13 (decomposition, deferred epic)}.

## Validation surface (how Phase 5 confirms green)

Whole suite:
`bash docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/run-acceptance.sh`
exits 0 only when every component above lands. Per-stream: `bats <file>`; per-scenario:
`bats <file> --filter "<id>"`. The 8 guardrails (A0-S2..S5, B1-S3/S4, B4-S1/S2,
B3-S1, B5-S4) must remain green throughout — any red guardrail means a component
broke an invariant it was required to preserve.
