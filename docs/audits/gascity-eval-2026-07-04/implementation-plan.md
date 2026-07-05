# Implementation Plan — Gas City v1.3 Port Slate

> Companion to [`report.md`](report.md). Six survivors of the 3-lens panel, sequenced as two
> waves of write-scope-disjoint slices. Everything is a **native build in `cli/internal/` +
> skills/standards** — no gascity dependency, no daemon, no new always-on surface. Gas City files
> cited are *design references* to read before building, per the fork's read-only charter.

## Framing

The slate has one theme: **promote membrane semantics that today live in bash conventions or
agent self-report into typed, fail-closed Go contracts.** That is the exact move gascity spent
the v1.3 line making (and paid for in production incidents we get to skip). Each item states
capability, acceptance (Given/When/Then), effort, write scope, and the gascity reference.

## Wave 1 — membrane core (3 lanes, write-scope disjoint)

### 1. `promptsafe` — injection + secret hygiene at prompt-assembly choke points  (S)

**Capability:** untrusted interpolated fields (bead titles/descriptions, Agent Mail bodies,
corpus/learning titles) cannot smuggle harness delimiter tags into assembled prompts; agent-invoked
child commands do not inherit secret-bearing env.

- New package `cli/internal/promptsafe`:
  - `SanitizeLeaf(s)` — strip literal harness delimiter tags (`<system-reminder>`, closing forms)
    from untrusted leaf strings, **looping to fixpoint** (single-pass strip can splice neighbors
    into a fresh tag — the incident gascity patched).
  - `StripSecretEnv(env)` — drop inherited env whose keys match secret patterns
    (`TOKEN|SECRET|API_KEY|PASSWORD|CREDENTIAL|PRIVATE`), preserving explicit operator allowlist.
- Wire at every prompt-assembly site: `ao session bootstrap`, `ao lookup` / `ao corpus inject`,
  MCP Agent Mail message rendering. One shared choke point, not per-caller regex.
- Standards addendum (trust boundaries): never interpolate untrusted data into `sh -c`; build
  argv + quote. Audit existing exec sites against it.

**Acceptance:**
- Given a bead title containing splice fragments (`</system-`, `reminder>` adjacents), When any
  choke point renders it, Then no reconstructable delimiter tag survives (adversarial vector table,
  fixpoint asserted).
- Given `AWS_SECRET_ACCESS_KEY` in the parent env, When a promptsafe-wrapped worker command runs,
  Then the child env lacks it while `PATH`/allowlisted values survive.

**Write scope:** new pkg + call-site edits in `cli/cmd/ao/{session,corpus,mcp}*.go`.
**Reference:** `gascity/internal/promptsafe/promptsafe.go`, `docs/reference/trust-boundaries.md`.

### 2. Degradation-aware typed decider — failure classes in `planpawl.Decide`  (S)

**Capability:** the Go windshield distinguishes "a reviewer's provider rate-limited/timed out"
(transient infra → BLOCKED, retryable) from "a reviewer genuinely refuted" (fail → REDO). Today
`planpawl.Decide` is degradation-blind; the distinction exists only as conventions in
`scripts/pawl-review.sh` (strict-no-degrade, NO-VERDICT, `degraded`). This closes the recorded
"warm pawl degrades → spurious REFUTED" escape class.

- Add `FailureClass` (`none|transient|hard`) + a small tested token table
  (`rate_limited`, `provider_unavailable`, `provider_timeout`, `transport_interrupted`, …);
  unknown/missing class fails safe to `transient` — never silent pass, never false refute.
- `Decide` outcomes become PASS / REDO (genuine refutation) / BLOCKED-DEGRADED (retryable);
  transient partial coverage yields BLOCKED, never PASS.
- `pawl-review.sh` keeps transport; Go owns the semantics (parity tests pin the bash conventions
  to the typed table).
- Absorbs `outcome-classifier-fail-closed` (planpawl already fail-closes unknown dispositions —
  add parity tests, not a new function) and the ELIGIBLE/DEGRADED/BLOCKED trichotomy concept from
  gascity's store preflight (`preflight-eligibility-ledger` generalization deferred).

**Acceptance:**
- Given one refuter lane timing out and none refuting, When `Decide` runs, Then outcome is
  BLOCKED(degraded, retryable), not REDO and not PASS.
- Given all lanes CONFIRMED but coverage below the pawl's floor due to a transient failure, Then
  BLOCKED, never PASS.
- Given an unknown `failure_class` token, Then it classifies as transient (fail-safe), with the
  off-roster value surfaced in the reason.

**Write scope:** `cli/internal/planpawl/` + parity tests touching `scripts/pawl-review.sh`
expectations only (no bash rewrite this wave).
**Reference:** `gascity/internal/reviewquorum/classify.go`, `finalize.go` (transient→blocked vs
hard→fail rollup); `internal/beads/contract/preflight.go` (trichotomy).

### 3. Finding synthesis — cross-lane dedup + family corroboration  (M)

**Capability:** a cross-family review returns ONE ranked, deduped findings list where each finding
carries the set of families that independently reported it — "claude AND gpt (2/2)" outranks
"gemini only (1/3)". Today the membrane decides at whole-change grain and multi-judge output is a
pile of per-judge notes; there is no Go primitive for finding-level merge (grep confirmed empty).

- New package `cli/internal/findingsynth`: canonical `findingKey` (normalized
  severity+title+file+span), `Merge(lanes) → []Finding` accumulating per-finding lane/family
  attribution, deterministic ordering (corroboration count desc, severity, key).
- Consumers: `/converge` judge rounds, pawl-review output, `/code-review`-style flows. The quorum
  *decision* stays in `planpawl.Decide` — this is the synthesis layer beneath it.

**Acceptance:**
- Given 3 lanes with overlapping findings phrased differently but normalizing to one key, When
  merged, Then one finding with `families: [claude, gpt, gemini]` and count 3.
- Given a finding reported by 2/3 families and another by 1/3, Then the 2/3 finding ranks first.
- Merge is deterministic: same lanes in any order → byte-identical output.

**Write scope:** new pkg + one consumer wiring (converge output path).
**Reference:** `gascity/internal/reviewquorum/finalize.go:184` (`mergeLaneFindings`), `:204`
(`findingKey`), `types.go` (`Finding.Lanes`).

## Wave 2 — orchestration craft (3 lanes, write-scope disjoint)

### 4. Group-terminality predicate — deterministic "is this epic/wave actually done"  (M)

**Capability:** replace agent self-report "the wave is done" with a deterministic predicate at
group granularity — the membrane's "no verdict = not done" applied to epics/waves. Steal the three
production-paid guards, not the daemon:

1. an unresolved/missing member resolves to an unknown-status placeholder that **never** counts
   as done;
2. a group with a deliberately-open descendant (human-gate/checkpoint bead) is NOT complete;
3. a zero-descendant, still-materializing group is skipped, not reported done.

- Surface: `ao beads epic-status <id> --terminal` (or fold into `ao verify`) reading via
  `BEADS_DIR="$(ao beads dir)" br` exports; emits a machine-readable verdict + human reason.
- Consumers: `/crank` wave close, `/validate` completion audits, drive-loop exhaust checks.

**Acceptance:** one Given/When/Then per guard (missing member → not done; open checkpoint child →
not done; materializing group → skipped), plus happy path (all terminal → done with member roll-up).

**Write scope:** `cli/cmd/ao/` new command + `cli/internal/` support pkg.
**Reference:** `gascity/internal/convoy/convoy.go:87`, `membership.go:96,146`,
`cmd/gc/molecule_autoclose.go:227`.

### 5. "Unreachable is not absent" — read-federation guard tests  (S)

**Capability:** any ao lookup that probes multiple sources (ledger + cache, corpus files + index)
preserves the FIRST hard (non-NotFound) error and returns NotFound only when every probe was a
clean miss. A failed cache read must never masquerade as "bead/learning not found" and drive a
wrong close/skip.

- Audit ao-side dual-source readers (`verdictledger`, corpus/lookup paths); add invariant guard
  tests; fix any flattening found. br's own SQLite-cache-over-JSONL read is external Rust — file
  an upstream issue if the audit shows the same trap there.

**Acceptance:** Given a reader whose primary source errors (permission/corrupt), When resolved,
Then the hard error surfaces (not NotFound); Given all sources clean-miss, Then NotFound.

**Write scope:** tests + spot fixes in `cli/internal/{verdictledger,corpus,...}` (no API change).
**Reference:** `gascity/internal/storeref/storeref.go` (`Resolve`'s first-hard-error discipline).

### 6. Migration-owner discipline — one fail-closed owner per breaking migration  (M)

**Capability:** every breaking retirement/migration (the documented "retired-surface debris is a
CLASS" pain: bd→br, skill retirement, registry drift) has exactly ONE owning check that documents
version staging (warn → alias → hard error), pairs detection with an atomic, format-preserving
`--fix`, and **refuses ambiguous fixes** (both old+new present → skip and surface; humans choose).

- Land as: (a) a standards reference codifying the discipline (including the
  idempotency-key / write-the-dedup-marker-LAST ordering for interruptible ledger writes —
  absorbing `crash-safe-ledger-write-ordering` and the `idempotent-spawn-epoch` principle); (b) a
  retrofit of ONE live migration (skill retirement debris via `ao doctor`/`ao skills retire`) to
  prove the shape.

**Acceptance:** Given a repo with a retired surface present, When the owning check runs, Then it
detects + fixes atomically; Given old AND new present, Then it refuses and reports; Given an
interrupted fix, Then re-run re-processes (marker written last) rather than skips.

**Write scope:** `cli/internal/doctor/` + `skills/standards/references/`.
**Reference:** `gascity/internal/doctor/autofix_skills.go`, `checks_legacy_suspended.go`;
`internal/convergence/handler.go` (write-ordering).

## Non-goals (judged and cut — do not resurrect without new evidence)

- No daemon, event bus, dashboard, pack registry, reaper, or poll-cadence work (platform-shaped;
  NTM/substrate owns runtime concerns — ADR-0009).
- No read-only-proof field on verdicts: the codex-exec sandbox *enforces* read-only, strictly
  stronger than a self-reported porcelain diff (all three judges concurred).
- No skill-content-hash verify yet: doctor's manifest-hash partially covers it and there is no
  drift incident; revisit on first incident.
- No epoch-CAS rework of br writes: principle lands as doctrine in item 6; mechanism only if a
  collision recurs after Agent Mail reservations.

## Sequencing, effort, rollback

- **Wave 1** (items 1–3): disjoint write scopes → one wave, three lanes, Agent Mail reservations
  per lane. Total ≈ 2 S + 1 M.
- **Wave 2** (items 4–6): after wave 1 lands; also disjoint. Total ≈ 1 S + 2 M.
- Every bead lands through the membrane: failing test first, pawl verdict bound to head SHA,
  `ao gate check --fast --scope head` before push. Item 2 must NOT alter `pawl-verdict.v1`
  semantics — additive fields only; version to v2 if a required field becomes necessary.
- Rollback: each item is an independent bead arc; revert = `git revert` of that arc. No data
  migrations in any item.

## Bead filing (run on approval)

```bash
B="$(ao beads dir)"
E=$(BEADS_DIR="$B" br create "Gas City v1.3 port slate: typed membrane contracts" \
  -t epic -d "Eval: docs/audits/gascity-eval-2026-07-04/. Two waves, six slices; design-steal only." | grep -o 'age-[a-z0-9]*')
for t in \
  "promptsafe: fixpoint sanitizer + secret-env strip at prompt-assembly choke points" \
  "planpawl: degradation-aware Decide (none/transient/hard failure classes)" \
  "findingsynth: cross-lane finding dedup + family corroboration" \
  "ao beads epic-status --terminal: group-terminality predicate w/ 3 guards" \
  "read-federation guard tests: unreachable is not absent" \
  "migration-owner discipline: standards ref + one doctor retrofit"; do
  BEADS_DIR="$B" br create "$t" -t task --parent "$E"
done
# wave-2 deps (4,5,6 after wave 1) added via: br dep add <child> --blocks <...>
```
