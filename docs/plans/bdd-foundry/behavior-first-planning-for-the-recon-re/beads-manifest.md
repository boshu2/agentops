# Proposed Bead Set — Recon-Remediation (bdd-foundry)

> **Status: REPAIRED — needs operator re-validation before tracker write.** Drift-guard +
> coverage repairs applied (see "Repair pass" below); NOT yet re-graded, NOT yet in the
> tracker. The conductor/operator re-runs validation, then writes these to `br`
> (from the main checkout, prefix `ag`, `BEADS_DIR=$PWD/_beads`) — this lane only proposes.
> Tracker invocation is `br`; never from a worktree.
>
> **Repair pass (this revision):**
> - **Drift-guard (one-invocation-per-test):** every ACCEPTANCE command now selects EXACTLY
>   ONE test. The `&&`-chained multi-bats commands (a0-vendor-govern, a0-govern-harden,
>   a1-model-pin, a2-empty-guard, a3-escalate-repair, a4-scope-since, a5-monitor-guidance,
>   b1-doc-skill-refs, b2-changelog-tag, b4-decompose-cmd-ao) were narrowed to the bead's
>   primary `scenario_ref` test; sibling scenarios remain separate coverage entries. The three
>   B5 Go beads' `-run` regexes were anchored to a single test func each.
> - **Vacuous-filter fix (the gating defect):** `b3-quorum-doctrine` and `b5-forged-ratification`
>   both used `--filter "B3-S1 + B5-G3:"`, which VACUOUSLY PASSED (`+` is ERE; the literal
>   " + " never matches → `1..0`, exit 0 → a bead could close having run no test).
>   `b5-forged-ratification` now keys on the metacharacter-free substring
>   `"forged-ratification cannot execute"` (same test, regex-safe).
> - **Wrong-surface fix + coverage:** `b3-quorum-doctrine`'s frozen behavior is B3-S2/S3/S4
>   (doctrine note + two memory edits + consumer reconcile), which had ZERO acceptance coverage.
>   Authored content-assertion tests B3-S2/S3/S4 in `acceptance-tests/stream-b-recon-actions.bats`
>   (all RED by construction) and repointed the bead's acceptance at the primary B3-S2 test.
>
> Source bead bodies: `beads.json` (this directory). Acceptance specs: `acceptance-tests.md`,
> `spec.md`, `behaviors.md`. Runnable acceptance harnesses live under `acceptance-tests/`.

## Conductor mechanical findings (verbatim)

- **coverage holes** — `[A0-S2,A0-S3,A0-S4,A0-S5,A0-G10,A1-S2,A1-S3,A1-G7,A2-S2,A2-S3,A2-G4,A2-G8,A3-S2,A3-S3,A4-S2,A4-S3,A4-S4,A4-G5,A4-G6,A5-S2,A5-G12,B1-S2,B1-S3,B1-S4,B1-S5,B1-G9,B1-G11,B2-S2,B2-S3,B2-S4,B3-S1,B3-S3,B3-S4,B5-S2,B5-S4,B5-G1,B5-G2,B4-S1,B4-S2]`
- **cycle_free** — `true`
- **rejected** — `[]`

---

## Beads (gate-PASSED)

### A0 — Stream A foundation

#### `a0-vendor-govern` — Vendor codebase-recon.js + ledger row + regen (governed repo citizen)

- **why:** BLOCKER for all of Stream A: A1-A4 grep the in-repo `.claude/workflows/codebase-recon.js`, which is ABSENT today. Vendor it (C1 base shape: `meta.name='codebase-recon'`, `args.model` threading hooks, phase structure mirrored from the 4 existing workflows) plus the single `workflows.codebase-recon` ledger row in `skill-dispositions.yaml` (kind: workflow, BC domain, hexagonal_role, path) and regen `registry.json` (C2). Preserves guardrails A0-S2..S5 (bijection/drift clean, four existing workflows byte-identical).
- **scenario_ref:** A0-S1
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-a-codebase-recon.bats --filter "A0-S1:"
  ```
- **deps:** _(none)_

#### `a0-govern-harden` — Governance path-binding + exported-meta.name identity hardening (C3)

- **why:** `check-workflow-governance.sh` derives id via `grep|head -1` (first hit = a leading comment) and never binds the ledger path to the real `.js`. A0-G14: parse exported `meta.name` not the first grep hit. A0-G10: assert ledger path resolves to the same tracked `.js` (schema carries path). Must keep A0-S3/S4 (forward/reverse bijection) green. Depends on `a0-vendor-govern` so a real row+file exist to harden against.
- **scenario_ref:** A0-G14
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-a-codebase-recon.bats --filter "A0-G14:"
  ```
- **deps:** `a0-vendor-govern`

---

### A1–A5 — Stream A codebase-recon hardening

#### `a1-model-pin` — Drop dead fable pin; thread session model; reject unsupported override (C1/A1)

- **why:** Remove any `'fable'` `WORKER_MODEL` default so a single-model outage can't wipe the fan-out (A1-S1/S3); thread `args.model` to every worker AND repair `agent()` call (A1-S2); reject `args.model='fable'`/unavailable tier BEFORE fan-out with `'unsupported model override'`, no green/empty conflation (A1-G7). Edits `codebase-recon.js` model-selection; depends on the vendored file.
- **scenario_ref:** A1-S1
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-a-codebase-recon.bats --filter "A1-S1:"
  ```
- **deps:** `a0-vendor-govern`

#### `a2-empty-guard` — Fail-closed empty-output guard before synth (C1/A2, HIGHEST VALUE)

- **why:** Before the Synth phase, count USABLE reports (non-empty + required marker, not bare file presence) and return `{status:'failed', reports_landed:0, reason}` when zero usable (A2-S1/S3/G4); proceed to synth only when >=1 usable (A2-S2); treat report-dir IO error (ENOENT/unreadable/readdir throw) as hard `status:'failed'`, never coerced to zero-files-green (A2-G8). Edits `codebase-recon.js` synth gate.
- **scenario_ref:** A2-S1
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-a-codebase-recon.bats --filter "A2-S1:"
  ```
- **deps:** `a0-vendor-govern`

#### `a3-escalate-repair` — Escalate-on-repair to a different model tier (C1/A3)

- **why:** Repair round selects a model tier != worker tier (A3-S1); a model-unavailable straggler (null/empty agent return) does NOT re-dispatch the same model — escalate or record unrepairable (A3-S2); cross-family escalation target is codex/agy, never fable (A3-S3). Edits `codebase-recon.js` repair round.
- **scenario_ref:** A3-S1
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-a-codebase-recon.bats --filter "A3-S1:"
  ```
- **deps:** `a0-vendor-govern`

#### `a4-scope-since` — First-class scope/since args with injection-safe handling (C1/A4)

- **why:** `args.scope` -> `scopeBlock` in every worker AND repair prompt (A4-S1); `args.since` resolves `REF..HEAD` with `git diff --stat` + log range injected (A4-S2); bare string arg coerced to `args.scope` (A4-S3); unresolvable since fails loudly, no silent full-repo fallback (A4-S4); since passed to git as argv/data not a shell string, metachar rejected with no side effects (A4-G5); scope wrapped in a fenced/quoted data block so injected instructions cannot escape (A4-G6). Edits `codebase-recon.js` arg parsing + prompt builders.
- **scenario_ref:** A4-S1
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-a-codebase-recon.bats --filter "A4-S1:"
  ```
- **deps:** `a0-vendor-govern`

#### `a5-monitor-guidance` — Monitor task-state guidance doc (C4/A5, content-assertion)

- **why:** Write `docs/memory/monitor-binds-task-state.md` (a surface a monitor reads; the test excludes the plan tree). Must hard-require loading TaskGet/TaskOutput at startup + ABORT if unavailable, forbid mtime/process-list inference (A5-S1); state tool-less monitor aborts with `'task-state unavailable'` not a verdict, naming the 2026-06-14 false-FAILED incident (A5-S2); abort on degraded responses (malformed JSON/timeout/partial) too, forbidding mtime/process/marker/log fallback (A5-G12). Independent of the workflow.
- **scenario_ref:** A5-S1
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-a-codebase-recon.bats --filter "A5-S1:"
  ```
- **deps:** _(none)_

---

### B1 — Stream B doc/skill-ref hygiene

#### `b1-doc-skill-refs` — Structural allowlist exemption + CI wiring + HEAD sweep for check-doc-skill-refs (C6+C5)

- **why:** Coherent arc per spec {C6+C5}. C6 first: replace the loose `retired|folded|legacy|historical` substring exemption with a STRUCTURAL marker (true retirement-note form or inline allowlist token), keeping B1-S3 (non-exempt `/bug-hunt` caught) and B1-S4 (true retirement-note exempt) green, catching B1-G11 incidental usage, shipping a recognizable allowlist token. C5: add a REQUIRED, non-continue-on-error, not-`if:false`, path-reaching CI step running `check-doc-skill-refs.sh --strict` (B1-S1/G9); sweep every stale ref so `--strict` exits 0 at HEAD (B1-S2); archival refs removed or inline-allowlisted, no history rewrite (B1-S5).
- **scenario_ref:** B1-S1
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-b-recon-actions.bats --filter "B1-S1:"
  ```
- **deps:** _(none)_

---

### B3 / B5 — Quorum doctrine + admission forged-ratification (Go, coupled bats line)

#### `b3-quorum-doctrine` — Ratify + document quorum context-floor doctrine + reconcile memories/consumers (C8)

- **why:** Doc/memory only; NON-GOAL to revert the default. Write the quorum doctrine note stating `'the context, not the model, makes a judge independent; cross-family is an opt-in upgrade'` and documenting `RequireCrossFamily` as the opt-in strengthener (B3-S2); edit the two fleet memories `cost-law-quorum-at-gates` + `quorum-gate-exists` so neither asserts a family floor as default (B3-S3); grep olympusd + fleet consumers and give any real-safety-property consumer `RequireCrossFamily:true` EXPLICITLY (B3-S4). Must NOT touch the code default (B3-S1 guardrail stays green). Runs BEFORE C7 changelog so wording matches.
- **scenario_ref:** B3-S2
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-b-recon-actions.bats --filter "B3-S2:"
  ```
  > FIX (was `--filter "B3-S1 + B5-G3:"`): that command tested the WRONG surface (the B3-S1
  > code-default guardrail belongs to `b5-forged-ratification`, not this doc/memory bead) AND
  > vacuously passed (`+` is ERE; `1..0`, exit 0). This bead's frozen behavior is B3-S2/S3/S4
  > (doctrine note + two memory edits + consumer reconcile). New content-assertion tests
  > B3-S2/S3/S4 were authored in `stream-b-recon-actions.bats`; acceptance now points at the
  > primary B3-S2 doctrine-note test (B3-S3 memory + B3-S4 consumer-reconcile are sibling
  > coverage tests in the same suite).
- **deps:** _(none)_

#### `b5-forged-ratification` — Forged-ratification sink fix in admission.go (C9, B5-G3)

- **why:** `admission.go` ~line106: an `InboundSourceQuorum` directive with `QuorumRatified=true` and NO SignificantAction/ACK records returns Allowed/Execute, trusting a caller-forgeable boolean. Re-derive quorum from ACK-bearing records; when `QuorumRatified` is set but no verifiable ACKs supplied, return `inboundNeedsAdmission/Denied` with reason `'ratification provenance not verified'` (B5-G3). MUST NOT set `RequireCrossFamily:true` anywhere (keeps B3-S1 green). Keep the properly-ACK-bearing path executing. Lives in liveness; depends on `b3-quorum-doctrine` so the same bats line (B3-S1+B5-G3) goes green together.
- **scenario_ref:** B5-G3
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-b-go.bats --filter "forged-ratification cannot execute"
  ```
  > NOTE: the bats test name is literally `B3-S1 + B5-G3: ...`; `--filter` is ERE, so a
  > filter containing the literal `+` (`"B3-S1 + B5-G3:"`) matches ZERO tests and VACUOUSLY
  > PASSES (`1..0`, exit 0 — a bead could close having run nothing). The regex-safe selector
  > above keys on the unique metacharacter-free substring of the same test.
- **deps:** `b3-quorum-doctrine`

---

### B2 — Release changelog + tag

#### `b2-changelog-tag` — CHANGELOG v3.2.0 section + local tag, flagged BREAKING (C7)

- **why:** AFTER B3 so quorum wording matches ratified doctrine (B2-S4 ordering dep). Add a v3.2.0 section covering v3.1.0..HEAD listing the ~104-skill prune, provenance ledger, quorum context-floor rewrite, converge loop, codex dispatch, bd/Dolt->br tracker, BC6 (B2-S1); mark the quorum change BREAKING with the new default (fresh context + cross-family opt-in) consistent with B3 (B2-S2); cut local git tag `v3.2.0` at release-window HEAD (B2-S3). B2-G15 remote-tag push is operator-performed (this lane prepares + flags, does not push). B2-S4 is satisfied by this bead's blocks dep on `b3-quorum-doctrine`.
- **scenario_ref:** B2-S1
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-b-recon-actions.bats --filter "B2-S1:"
  ```
- **deps:** `b3-quorum-doctrine`

#### `b2-remote-tag-proof` — Remote v3.2.0 tag proof at HEAD (C7/B2-G15, operator-pushed)

- **why:** Hardens B2-S3 beyond a local tag: `git ls-remote --tags origin refs/tags/v3.2.0` must equal `git rev-parse HEAD` (B2-G15). The push is conductor/operator-performed — this lane prepares the tag (`b2-changelog-tag`) and flags the operator to push; the assertion is the gate. Split from `b2-changelog-tag` because its green requires the operator push, a distinct rollback/handoff boundary.
- **scenario_ref:** B2-G15
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-b-recon-actions.bats --filter "B2-G15:"
  ```
- **deps:** `b2-changelog-tag`

---

### B5 — Codex dispatch trust boundary (Go, cli/cmd/ao, sequenced same-package)

#### `b5-codex-trust-boundary` — Codex dispatch trust boundary + duplicate-sandbox rejection (C10)

- **why:** `cli/cmd/ao/codex.go`: before any `sh -c` packet command, assert the packet originates from the operator-trusted local repo dir; a packet relocated to `t.TempDir()` is refused with `'trust boundary'`/`'operator-trusted'`/`'untrusted packet'` before the fake codex marker is written (B5-S1/S2). The same boundary gates the second `sh -c` surface `runCodexRequiredCommands` so a smuggled `'echo ok; touch <pwned>'` never runs (B5-G2). Add a distinct duplicate-sandbox validator rejecting >1 `--sandbox` occurrence with literal `'duplicate sandbox'` before exec (B5-G1). Keep the normal in-repo trusted packet dispatching to a PASS receipt (B5-S4 guardrail). Same package as `b5-symlink-paths`/`b5-provenance-doc` — sequenced to avoid collisions.
- **scenario_ref:** B5-S1
- **ACCEPTANCE:**
  ```
  cp docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/go-acceptance/recon_acceptance_codex_test.go.txt cli/cmd/ao/recon_acceptance_codex_gen_test.go && cp docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/go-acceptance/recon_acceptance_codex_helpers_test.go.txt cli/cmd/ao/recon_acceptance_codex_helpers_gen_test.go && ( cd cli && go test ./cmd/ao/ -run '^TestReconAcceptanceB5S1S2UntrustedPacketRefused$' -v ); st=$?; rm -f cli/cmd/ao/recon_acceptance_codex_gen_test.go cli/cmd/ao/recon_acceptance_codex_helpers_gen_test.go; exit $st
  ```
- **deps:** _(none)_

#### `b5-symlink-paths` — Symlink/TOCTOU-safe dispatch output paths (C11, B5-G13)

- **why:** `resolveCodexDispatchPath` uses `filepath.Clean` + string-prefix only — a `final.md` symlink inside an allowed dir pointing outside the repo passes. Resolve symlinks (`EvalSymlinks` on candidate+parent or `O_NOFOLLOW` write) and re-check the resolved real path against allowed roots, guarding the TOCTOU window, across final-message/JSONL/receipt write paths; dispatch fails before writing with a path-boundary error, leaving the escape target absent (B5-G13). Same `cmd/ao` package as `b5-codex-trust-boundary` — sequenced after it to avoid file collisions.
- **scenario_ref:** B5-G13
- **ACCEPTANCE:**
  ```
  cp docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/go-acceptance/recon_acceptance_codex_test.go.txt cli/cmd/ao/recon_acceptance_codex_gen_test.go && cp docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/go-acceptance/recon_acceptance_codex_helpers_test.go.txt cli/cmd/ao/recon_acceptance_codex_helpers_gen_test.go && ( cd cli && go test ./cmd/ao/ -run '^TestReconAcceptanceB5G13SymlinkOutputPathRejected$' -v ); st=$?; rm -f cli/cmd/ao/recon_acceptance_codex_gen_test.go cli/cmd/ao/recon_acceptance_codex_helpers_gen_test.go; exit $st
  ```
- **deps:** `b5-codex-trust-boundary`

#### `b5-provenance-doc` — Provenance keying decision doc (C12, B5-S3)

- **why:** Lowest-cost satisfier: write `docs/provenance/keying-decision.md` documenting the unkeyed-SHA-256 + git-as-anchor design with the explicit rationale string `'git history is the real tamper-evidence anchor'` (B5-S3). (Alternative keyed-digest symbol path is available but the doc satisfies the scenario today.) Same `cmd/ao` test package run — sequenced after the other B5 `cmd/ao` beads so the grouped go test goes fully green.
- **scenario_ref:** B5-S3
- **ACCEPTANCE:**
  ```
  cp docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/go-acceptance/recon_acceptance_codex_test.go.txt cli/cmd/ao/recon_acceptance_codex_gen_test.go && cp docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/go-acceptance/recon_acceptance_codex_helpers_test.go.txt cli/cmd/ao/recon_acceptance_codex_helpers_gen_test.go && ( cd cli && go test ./cmd/ao/ -run '^TestReconAcceptanceB5S3ProvenanceKeyingDecided$' -v ); st=$?; rm -f cli/cmd/ao/recon_acceptance_codex_gen_test.go cli/cmd/ao/recon_acceptance_codex_helpers_gen_test.go; exit $st
  ```
- **deps:** `b5-codex-trust-boundary`, `b5-symlink-paths`

---

### B4 — Deferred decomposition epic

#### `b4-decompose-cmd-ao` — Decompose cli/cmd/ao into a codex/ sub-package (C13, DEFERRED EPIC)

- **why:** Lowest priority, deferred epic. Move `codex.go` responsibilities into a `cli/cmd/ao/codex/` sub-package so the top-level `cli/cmd/ao` `.go` file count drops below 633 (B4-S3); keep `go build`/`vet`/`test` green (B4-S1 guardrail) and the command surface unchanged via `regen-all --check` (B4-S2 guardrail). NEVER co-waved with the B5 `cmd/ao` beads (same package, caller-migration collision) — depends on all of them so it lands last.
- **scenario_ref:** B4-S3
- **ACCEPTANCE:**
  ```
  bats docs/plans/bdd-foundry/behavior-first-planning-for-the-recon-re/acceptance-tests/stream-b-recon-actions.bats --filter "B4-S3:"
  ```
- **deps:** `b5-codex-trust-boundary`, `b5-symlink-paths`, `b5-provenance-doc`

---

## Dependency DAG (by key)

```
a0-vendor-govern ──┬─> a0-govern-harden
                   ├─> a1-model-pin
                   ├─> a2-empty-guard
                   ├─> a3-escalate-repair
                   └─> a4-scope-since
a5-monitor-guidance        (root, no deps)
b1-doc-skill-refs          (root, no deps)
b3-quorum-doctrine ─┬─> b5-forged-ratification
                    └─> b2-changelog-tag ─> b2-remote-tag-proof
b5-codex-trust-boundary ─┬─> b5-symlink-paths ─┐
                         └────────────────────> b5-provenance-doc
b5-{codex-trust-boundary,symlink-paths,provenance-doc} ─> b4-decompose-cmd-ao
```

- **cycle_free:** true
- **rejected:** (none)

## Waving guidance (collision-aware)

- **Wave 1 (parallel roots):** `a0-vendor-govern`, `a5-monitor-guidance`, `b1-doc-skill-refs`, `b3-quorum-doctrine`, `b5-codex-trust-boundary`. Disjoint write surfaces (`.claude/workflows/` + ledger; `docs/memory/`; CI/doc-refs; doctrine docs/memories; `cli/cmd/ao/codex.go`).
- **Stream A fan-out** (`a0-govern-harden`, `a1-model-pin`, `a2-empty-guard`, `a3-escalate-repair`, `a4-scope-since`) all edit `codebase-recon.js` / governance — **sequence**, do not co-wave (shared file).
- **B5 `cmd/ao` chain** (`b5-codex-trust-boundary` -> `b5-symlink-paths` -> `b5-provenance-doc`) is the same Go package — **sequential** by design.
- **`b4-decompose-cmd-ao`** lands LAST (caller-migration collision with all B5 `cmd/ao` beads); never co-waved with them.
- **B2 chain** (`b2-changelog-tag` -> `b2-remote-tag-proof`) crosses a handoff boundary: `b2-remote-tag-proof` green requires the operator/conductor push of `v3.2.0`.
