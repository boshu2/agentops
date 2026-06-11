# Codebase Pattern Extraction — agentops repo audit

- **Skill executed:** `skills/codebase-pattern-extraction/SKILL.md` (Collect → Diff/Align → Abstract → Parameterize → Package)
- **Scope:** single repo `/Users/bo/dev/agentops`; cross-cutting surfaces treated as the skill's "projects": `cli/` (Go, 620 files in `cmd/ao`), `scripts/` (253 shell scripts, ~100 `check-*.sh`), `skills/` (169 skills), `lib/` (shared helpers), `tests/` (122 bats files), `.github/workflows/validate.yml` (15 jobs)
- **Mode:** strictly read-only; this report + a thin JSON result record are the only writes
- **Date:** 2026-06-11 (supersedes the earlier same-day draft at this path; its verified findings are folded in below)

## Method note

The skill's pipeline assumes multi-repo mining; here each subsystem is a "project" and the diff/align step compares recurrences across them. CASS history mining was skipped (worker scope = repo surface only); discovery used `rg`/`grep` sweeps per the skill's Discovery Techniques. Every extracted pattern clears the skill's **3+ instances** bar. Counts below were measured this run, not copied from docs.

## Summary verdict

The repo is unusually pattern-dense and **already practices extraction** (`lib/bats-common.bash`, `lib/ao-paths.sh`, generator-with-`--check` tools). Two findings dominate:

1. **The governing meta-pattern is Convention → Gate → Logged Escape Hatch** — nearly every convention is mechanically enforced and has a named, audit-trailed bypass.
2. **The dominant gap is not missing abstraction — it is missing back-application.** Shared artifacts get extracted, then most instances never migrate: `lib/bats-common.bash` has **3/122** bats consumers; `lib/ao-paths.sh` has ~11 references repo-wide against 253 scripts; the `practices:` provenance annotation covers 693 Go files and 166/169 skills but only **13/253** bash scripts. Closing adoption gaps is worth more than any new abstraction — exactly the skill's "Skip validation / apply back to source projects" anti-pattern, observed live.

---

## Meta-pattern 0: Convention → Gate → Logged Escape Hatch

**Source instances (sample of many):**
- Convention: command files need co-changed tests → Gate: `scripts/check-go-command-test-pair.sh` → Hatch: `AGENTOPS_SKIP_COMMAND_TEST_PAIR=1` (prints `SKIP: ... disabled via ...`)
- Convention: test counts never silently shrink → Gate: `scripts/check-test-count-regression.sh` → Hatch: `Test-Removal-Reason:` commit trailer OR `AGENTOPS_TEST_COUNT_NOREGRESS=skip` ("emergency bypass (logged)")
- Convention: generated catalogs match frontmatter → Gate: `scripts/check-skill-catalog-drift.sh` → Hatch: regenerate (the fix IS the hatch)

**Invariant core:** a convention is not real until a gate enforces it; a gate is not operable without an escape hatch; a hatch is not safe unless it leaves an audit trail (env var that logs, or a commit trailer that survives in history).

**Variance points:** enforcement scope (diff-scoped vs tree-scoped); bypass channel (env var / trailer / regenerate).

**Packaged as (recommendation):** a one-page "anatomy of a gate" reference in `skills/standards/references/` so new gates copy the triple, not just the check.

---

## Pattern 1: Drift Gate (generate → diff → fail with remediation)

**Source instances (7):** `scripts/check-skill-catalog-drift.sh` (thin wrapper → `generate-skill-catalog.sh --check`), `check-bounded-contexts-drift.sh` (YAML canonical vs two narrative docs, PASS/WARN/FAIL verdict + `--json`), `check-codex-parity-drift.sh`, `check-doc-hooks-drift.sh`, `check-registry-drift.sh`, `validate-context-map-drift.sh` (skill frontmatter → `docs/contracts/context-map.md`), `validate-sku-catalog-drift.sh`.

**Invariant core:** one canonical machine-readable source (frontmatter / YAML / disk state) + one generated projection + a gate that recomputes and fails on diff. The gate is a *stable named entry point* wrapping the generator — `check-skill-catalog-drift.sh` says it outright: "Thin wrapper ... so the workflow has a stable, named entry point."

**Variance points:** canonical source; projection (JSON catalog / markdown doc / count strings); severity model (binary vs PASS/WARN/FAIL).

**Already-applied evidence:** commit `0626f7a19` "derive skill count from disk SSOT, kill manual-edit doc-gate block" — the repo is actively converging on this shape.

**Packaged as (recommendation):** a `drift-gate` scaffold (gate wrapper + generator stub with `--check`); today instances range from a 741B wrapper to 8.9K bespoke scripts.

---

## Pattern 2: Generator with `--check` Dual Mode

**Source instances (3+):** `generate-skill-catalog.sh --check`; `sync-skill-counts.sh --check` ("Dry-run: report mismatches without modifying files (exit 1 if any)"); `update-cli-surface-counts.sh`; the `snapshot-flywheel-compounding.sh` / `check-flywheel-compounding-snapshot.sh` writer/checker pair.

**Invariant core:** the tool that *writes* the derived artifact is the tool that *verifies* it — one code path, so fixer and checker can never disagree. CI calls `--check`; humans/agents call bare mode to fix.

**Variance points:** patched target (catalog.json, doc count strings, CLI surface tables, snapshots).

**Packaged as (recommendation):** convention requirement — any new `generate-*.sh`/`sync-*.sh` must support `--check`, or its drift gate forks logic.

---

## Pattern 3: Gate Script Contract (the `check-*.sh` anatomy)

**Source instances:** ~100 `scripts/check-*.sh`; representatives diffed: `check-doctor-health.sh`, `check-go-command-test-pair.sh`, `check-test-count-regression.sh`, `check-skill-catalog-drift.sh`, `check-bounded-contexts-drift.sh`.

**Invariant core (well-formed majority):**
1. `#!/usr/bin/env bash` + `set -euo pipefail` (203/253 scripts)
2. Header comment: name, one-line purpose, bead provenance (`(ag-h2z)`, `(soc-jhq6)`), explicit **exit-code table** (`0 - pass / not applicable`, `1 - failed`, `2 - tool error`)
3. Three outcome vocabularies: `SKIP: <reason>`, `PASS: <summary with counts>`, `FAIL: <what>` + remediation to stderr ("Add at least one cli/cmd/ao/*_test.go change in the same commit/push.")
4. Root via `git rev-parse --show-toplevel` with fallback (43 scripts re-derive this)
5. `mktemp` + `trap ... EXIT` cleanup

**Diff/align gap:** only **13** scripts use shared fail/counter helpers; only **24** support `--json`; root resolution and changed-file collection are re-derived per script — the same boilerplate drift `lib/bats-common.bash` was created to kill on the test side.

**Packaged as (highest-value new extraction):** **`lib/gate-common.sh`** — `gate_root`, `gate_pass`, `gate_fail`, `gate_skip`, `gate_json_findings`, `gate_changed_files <scope>`. Justified **only as a bridge**: the Go gate port (`ao gate check`, epic ag-3n71, 12/79 parity) is the strategic destination, so migrate opportunistically (new gates first), don't retrofit 100 scripts.

---

## Pattern 4: Changed-Surface Conditional Dispatch

**Source instances (3 independently-evolved implementations — the diff/align money shot):**
1. `scripts/pre-push-gate.sh` — `collect_all_changed()` with `--scope {upstream|staged|worktree|head|auto}` → 12 `HAS_<surface>` booleans (GO, SKILL, HOOK, DOCS, SHELL, LEARNING, EVAL, CONTRACT, CI_POLICY, CONTEXT_MAP, SWARM, CHANGELOG) → fast mode runs only hot surfaces
2. `.github/workflows/validate.yml` `changes` job — `dorny/paths-filter@v4` with 14 named filters + **force-full override** (release tag or `merge_group` ⇒ `release=true` short-circuits every filter)
3. `scripts/check-go-command-test-pair.sh` — its own `collect_changed_files()` with a fallback chain (upstream range → staged → worktree → HEAD)

**Invariant core:** compute changed files once; map paths → surface booleans; run only checks whose surface is hot; force-full on events where partial validation is unsafe (release, merge queue).

**Variance points:** scope-selection strategy (explicit flag / fallback chain / PR base); surface taxonomy; full-run trigger.

**Documented footgun to preserve (soc-7ovd, in-file):** `HAS_<surface>` defaults to 1 and is reset only inside the FAST_MODE branch — FULL-mode reads get stale-true, not a path filter. The script files its own fix: "compute the diff once at script start and populate HAS_<surface> regardless of FAST_MODE." Implementations 1 and 3 have already drifted in scope semantics — textbook evidence for extraction.

**Packaged as (recommendation):** `gate_changed_files <scope>` in `lib/gate-common.sh` + one surface-taxonomy map consumed by both pre-push and the CI filter, so local gate and CI can't disagree on what "the go surface" means.

---

## Pattern 5: Diff-Scoped Process Gates (co-change pairing + ratchets)

**Source instances (4+):** `check-go-command-test-pair.sh` (non-test `cli/cmd/ao/*.go` change requires co-changed `*_test.go`); `check-test-count-regression.sh` (per-package `^func Test` count at BASE_REF vs HEAD; decrease fails without `Test-Removal-Reason:` trailer; per-package summing so moving a test between files is net-zero); the scenario↔test linkage gate in the `skill-gates` CI job; `check-ratchet-r3-constraint.sh`; `check-retrieval-quality-ratchet.sh`.

**Invariant core:** the gate's subject is the **diff**, not the tree — a *process* invariant ("you can't change X without Y") rather than a state invariant. Ratchets compare a metric at base vs head and allow decreases only via an explicit, history-preserved reason.

**Variance points:** paired artifact (test / scenario link / metric); override channel (trailer / env var); granularity (the per-package choice is deliberate anti-false-positive design worth copying).

**Packaged as (recommendation):** parameterized `ratchet-gate` template: `(metric_fn, base_ref, override_trailer)`.

---

## Pattern 6: Snapshot/Baseline Gates

**Source instances (4):** `check-agents-hash-snapshot.sh`; `check-flywheel-compounding-snapshot.sh` + `snapshot-flywheel-compounding.sh`; `evolve-capture-baseline.sh`; `check-retrieval-quality-ratchet.sh`.

**Invariant core:** a checked-in snapshot of derived state + a gate that recomputes and compares; updating the snapshot is the deliberate act acknowledging the change. A snapshot gate is a drift gate (Pattern 1) whose canonical source is "the past" — covered by the same scaffold.

---

## Pattern 7: Go CLI Self-Registering Command Module

**Source instances:** the whole `cli/cmd/ao/` surface — 620 files, **184** `func init()` registrations; sampled `agent.go:52-58`, `agents.go`, `agents_doctor.go`, `beads.go:162-169`.

**Invariant core (per command):**
- One file per command cluster, named after the command (`badge.go`, `beads_resume.go`)
- Package-level cobra var + flag vars named `<cmd><Flag>` (e.g. `agentBundleJSON`)
- `func init()` in the *same file* does `rootCmd.AddCommand(...)` — **no central registry file to merge-conflict on** (load-bearing for a hundreds-of-commits/week multi-agent repo)
- Per-command `--json` BoolVar with "Emit machine-readable..." help (53 declarations across 40 files)
- Paired `<source>_test.go`, enforced by Pattern 5's pairing gate + `.claude/rules/go.md` naming rules

**Gap:** `--json` is hand-rolled per command; no shared `addJSONFlag(cmd, &v)` helper. 53 near-identical lines is well past the 3-instance bar.

**Packaged as (recommendation):** tiny helper in `cli/internal/cliutil` (or cmd/ao): `addJSONFlag(cmd *cobra.Command, v *bool)` standardizing name+help, plus a JSON-or-human emitter. Small, mechanical adoption.

---

## Pattern 8: Extracted Test Harness with Provenance Comments

**Source instances:**
- `cli/cmd/ao/testutil_test.go:143` — `captureStdout(t, fn)`: pipe-swap of `os.Stdout`, reader goroutine, panic-safe restore via both `t.Cleanup` and deferred `recover()` re-panic; carries `// Origin: rpi_verify_test.go` — explicit extraction provenance
- `lib/bats-common.bash` — `bats_repo_root`, `bats_init_repo` (deterministic git identity; `gc.auto 0`/`fsmonitor false` to kill a CI teardown race, soc-72gkw), `bats_stub_bin`; header documents the extraction rationale verbatim ("Every script-test bats file re-derived the same fixture boilerplate ... which drifted between files")
- Guard-fixture fidelity rule in `.claude/rules/go.md` (round-trip real persisted samples, ag-mjlg)

**Invariant core:** when ≥3 test files re-derive a fixture, extract one helper documenting (a) origin file, (b) the bug motivating each non-obvious line (bead IDs inline), (c) composability constraints ("Functions only — no top-level shell options, so sourcing never alters the caller's `set -e`").

**This is the codebase-pattern-extraction skill executed internally** — but see the adoption gap: **3/122** bats files actually source `bats-common.bash`. The extraction succeeded; the back-application stalled. Cite it from the skill's `references/EXAMPLES.md` as both exemplar and cautionary tale.

---

## Pattern 9: One Contract, Two Renderings, One Parity Gate

**Source instances (4):**
- `lib/ao-paths.sh` (eval-export resolver: emits sorted `export NAME=%q` lines; documented precedence `AO_HOME` > `CLAUDE_PLUGIN_DATA` > git-root; "Idempotent: re-running with identical environment produces identical output") ↔ the Go-side resolver, kept honest by `scripts/check-paths-resolver-coverage.sh`
- Claude vs Codex skill bundles ↔ `check-codex-parity-drift.sh` + `audit-codex-parity.py`
- CLI surface vs docs ↔ `check-cmdao-surface-parity.sh` (14.5K)
- bash `pre-push-gate.sh` vs Go `ao gate check` ↔ the `go-gate-shadow` CI job (ag-3n71)

**Invariant core:** when one contract has two language/runtime renderings, ship (1) a single documented precedence/spec, (2) deterministic output, (3) a **parity gate** so the renderings can't drift. The shadow-job variant is the freshest instance: keep the shadow as parity gate until cutover, then invert it.

**Adoption gap:** `ao-paths.sh` itself has only ~11 references repo-wide against 253 scripts that could use it — most scripts still hand-resolve paths.

---

## Pattern 10: Skill Anatomy (schema-gated document module) + `practices:` provenance

**Source instances:** 169 skill dirs. Field coverage: `skill_api_version`/`hexagonal_role`/`tier` **166**, `consumes` 128, `produces` 132, `user-invocable` 120; `references/` in 103; `.feature` acceptance in 65; `<!-- TOC: -->` in 23. The `practices:` provenance annotation spans three languages: **693** Go files (`// practices: [...]`), **166/169** skills (frontmatter), **13/253** bash scripts.

**Invariant core:** SKILL.md with schema-validated frontmatter ("What. Use when X, Y, or Z" trigger description; hexagonal_role/consumes/produces feeding the generated context map via Pattern 1's `validate-context-map-drift.sh`) + optional `references/` + optional gherkin acceptance; enforced by `audit-skill-metadata.sh` + the `skills-integrity` CI job; doc counts derived by `sync-skill-counts.sh` (Pattern 2).

**Gap (meta-pattern incomplete):** TOC comment (23/169), consumes/produces (~76%), and bash `practices:` (5%) are conventions without full gates. Either gate them or retire them from the template — convention should match enforcement. Do **not** hand-retrofit 240 scripts; a warn-only lint on *new* `check-*.sh` is enough.

---

## Recommendations ranked (the skill's Package step)

| # | Action | Type | Collapses | Effort |
|---|---|---|---|---|
| 1 | **Back-application sweep** — migrate bats files onto `lib/bats-common.bash` (3/122 today) and scripts onto `lib/ao-paths.sh`, opportunistically on touch | adoption | 119 bats fixtures; ~40 path resolutions | M (incremental) |
| 2 | `lib/gate-common.sh` (root/pass/fail/skip/json + `gate_changed_files <scope>`) — bridge until ao-gate Go port (ag-3n71) | library | 3 divergent changed-file collectors; 43 root resolutions; SKIP/PASS/FAIL formatting | M |
| 3 | `addJSONFlag` + JSON-or-human emitter in CLI | library | 53 flag declarations / 40 files | S |
| 4 | drift-gate scaffold (wrapper + `--check` generator stub) | template | 7 drift gates + future | S |
| 5 | ratchet-gate template (metric_fn, base_ref, override trailer) | template | 4 ratchet/snapshot gates | S |
| 6 | "anatomy of a gate" doc (the Convention→Gate→Hatch triple, exit-code table, outcome vocab) | reference | doctrine for all above | S |
| 7 | Convention/enforcement reconciliation (TOC, consumes/produces, bash `practices:`: gate or retire) | hygiene | 169 skills, 253 scripts | S |

**Validation plan (per the skill's checklist):** for #2–#3, apply the helper back to 3 source instances each and diff outputs byte-for-byte; for #4–#5, regenerate one existing gate from the template and confirm identical CI behavior; for #1, the existing bats suites are the regression net. All extractions must carry Pattern 8's provenance-comment style and preserve escape hatches.

## Anti-pattern compliance check (self-audit against the skill)

- 3+ instances per pattern: ✓ (weakest is Pattern 2 at 3-4)
- No over-parameterization: helpers start minimal ✓
- Context documented: bead IDs and footgun comments carried forward ✓
- Validation before publishing: specified per recommendation ✓
- "Apply back to source projects": elevated to the #1 recommendation — it is the repo's measured weak spot ✓
