# AgentOps — Codebase Pattern Extraction (2026-07-02)

> **Skill:** `codebase-pattern-extraction` · **Run:** 2026-07-02
> **Scope:** intra-repo mining — architectural patterns that recur **≥3×** across AgentOps packages and generalize into reusable artifacts (a shared Go helper, a skill, or a cross-project template). Every pattern below is backed by verified instances (grep-confirmed), per the skill's "3+ instances, never abstract from 1-2" rule.
> CASS session-history mining was out of scope for this run; the instances are all in-tree.

The through-line: **AgentOps repeatedly reaches for the same small set of primitives to make stochastic work trustworthy — append-only tamper-evidence, deterministic deciders over model proposals, fail-closed absence handling, and drift-proof generated contracts.** These are the extractable assets.

---

## Pattern 1 — Hash-chained append-only ledger (`payload_hash` + `prev_hash`)

**Instances (5 packages):** `cli/internal/provenancegraph/edge.go:218` · `cli/internal/turnstate/turnstate.go` · `cli/internal/rpi/ledger.go` · `cli/internal/drwitness/drwitness.go:205` · `cli/internal/drrebuild/drrebuild.go`

**Invariant core (identical across all):**
```
payload_hash = sha256(canonical_json(record WITHOUT chain/join fields))
hash         = sha256(payload_hash + "\n" + prev_hash)
```
Plus: canonical field ordering for byte-identical re-export; idempotent append (dedupe on an identity key); a `VerifyChain` that walks the whole file; and the rule "the committed JSONL is the audit authority; any DB projection loses on disagreement" (`drwitness.CrossCheck` re-derives and byte-compares).

**Variance:** the record schema (provenance edge vs turn transition vs run entry), the identity key for dedup, the join keys excluded from the payload hash (`bead_id`/`merge_sha`).

**Why it recurs:** every place AgentOps needs *tamper-evident* history reaches for this — provenance, turn lifecycle, run tracking. It is the mechanical substrate of "no verdict = not done."

**Package as:** a `cli/internal/hashchain` helper — `Seal(record, prevHash) → (payloadHash, hash)`, `VerifyChain(io.Reader) error`, `AppendIdempotent(store, record, identity)`. Would collapse ~5 near-duplicate implementations into one tested core. **This is the single highest-value extraction in the repo** — the logic is security-critical and currently duplicated. Note `.claude/rules/go.md` already documents the SHA-1-vs-SHA-256 gosec suppression dance for the git-object variant (`drrebuild.go`), so the helper must carry the dual `#nosec`/`nosemgrep` annotations.

---

## Pattern 2 — Typed error → process exit code = the verdict

**Instances (9 sentinel types):** `cli/cmd/ao/root.go:85-167` unwraps `gateExitError`, `doctorExitError`, `planPawlExitError`, `pawlReviewExitError`, `governorExitError`, `beadsExitError`, `corpusScanExitError`, `wikiHealthExitError`, `tickExitError` (+ `AgentsLintError`).

**Invariant core:** a command returns a typed error carrying an int exit code; `Execute()` does `errors.As` on each and calls `os.Exit(code)`, so **the exit code is the machine-readable verdict** (`0 CONFIRMED · 3 REFUTED · 3 HARDEN · 4 BLOCKED`, etc.). Shells, CI, and hooks branch on the number, never on parsed stdout.

**Variance:** which codes each command emits (pawl 0/3/4, governor 0/3, tick 3/4/5/6/8/10).

**Why it recurs:** it is the contract by which every gate-style command talks to the outside. It is *the* agent-ergonomics primitive.

**Package as:** already near-uniform; the extraction is a **shared `ExitError` interface** (`ExitCode() int`) so `Execute()` unwraps one interface instead of nine concrete types — and, per audit finding **A7**, feed those codes into `ao capabilities` so the published contract matches emission. Generalizable to any agent-facing CLI (the pattern already appears in sibling tools `bv`/`br`/`dcg`).

---

## Pattern 3 — Determinism inversion: the model *proposes*, a pure predicate *checks*

**Instances (3+):** `cli/internal/planpawl/decide.go:125` (family-judge verdicts → PASS/REDO/BLOCKED, no model calls) · `cli/internal/orchestration/shape.go:57` (`ValidateShape` — model proposes an orchestration shape, predicates check it against live writer count + write-sets) · `cli/internal/gates/checks/constraints.go` (advisory prompt vs mechanical FAIL) · `cli/internal/evidencedturn/evidencedturn.go:88` (7 predicates judge, no LLM).

**Invariant core:** a stochastic agent produces a *proposal*; a **pure, side-effect-free, fail-closed function** renders the binding decision from observable ground truth. "Trust the environment, not the agent." Unrecognized/malformed inputs count as the *unsafe* verdict (FAIL/REDO), never a pad.

**Variance:** the decision domain (plan verdict / orchestration shape / done-ness) and the ground-truth inputs.

**Why it recurs:** it is the core architectural stance of the whole product — every gate that matters is a windshield, not a chatbot.

**Package as:** a documented **"decider" contract** (`references/decider-pattern.md` in the domain skill): pure function, closed input vocabulary, fail-closed default, exhaustive test table. Already the de-facto shape; making it an explicit, cited pattern would guide new gates.

---

## Pattern 4 — Fail-closed on absence; "no verdict ≠ pass"

**Instances (6+ packages):** `cli/internal/verdictledger/loader.go:22` (missing ledger = empty, not pass) · `cli/internal/gates/checks/constraints.go:36` (malformed index/unreadable file → FAIL) · `cli/internal/planpawl/decide.go:170` (off-roster family → FAIL) · `docs/contracts/pawls.md:171` (timed-out review = *no verdict*, never CONFIRM) · `cli/internal/evidencedturn` (zero-evidence → `unknown`, not vacuous pass) · `cli/internal/goalsfitness/satisfaction.go:148` (zero-evidence → `unknown`).

**Invariant core:** absence of proof is treated as *not proven*, and the code path for "couldn't determine" routes to the safe/blocking side.

**Variance:** the safe side's name (FAIL / REDO / HOLD / unknown).

**Counter-instance worth noting (audit A1):** `cli/internal/gates/scriptrunner.go` maps a *missing blocking backing script* to `GateStatusUnknown`, which `ExitCode()` **excludes** from failure — a fail-**open** violation of this otherwise-pervasive pattern. The pattern's own consistency is the reason A1 is a defect and not a design choice: the extraction (a shared "absence → unsafe-verdict" helper) would have prevented it.

**Package as:** a lint/constraint check `no-fail-open-on-unknown` that flags any `Status == Unknown` path reachable by a `Blocking` check without escalation. Dogfoods the escape corpus (ADR-0011).

---

## Pattern 5 — `init()`-based decentralized self-registration

**Instances:** ~92 `rootCmd.AddCommand` sites across `cli/cmd/ao/` (343 total `AddCommand` calls incl. subcommands) · `cli/internal/gates/checks/seed.go` (~90 checks each self-registering into the `Default` registry via package `init()`).

**Invariant core:** no central switch/list. Each unit (command, gate check) declares a `var` + `func init()` that registers itself into a package-level registry; registration collision **panics at startup** (programmer error, not runtime). Adding a unit = one new file/struct literal, zero edits to a central file.

**Variance:** the registry type (cobra tree vs `map[string]Check`).

**Why it recurs:** it is the anti-monolith property that keeps a ~646-file command surface and a ~90-check gate registry maintainable and merge-conflict-free under swarm load.

**Package as:** a documented convention (already partially in `gates.go:53`); generalizable as a template for any large agent-CLI. Escape hatch: `All()` returns sorted-by-ID for determinism.

---

## Pattern 6 — Drift-proof generated contract: reflect over the live tree

**Instances (3+):** `cli/cmd/ao/capabilities.go:104` (`buildCapabilitiesDoc` walks `rootCmd.Groups()`/`Commands()`/`VisitAll`) · `cli/cmd/ao/robot_docs.go:87` (command-surface section generated from the live tree) · `cli/cmd/ao/flag_suggest.go` (typo suggestions from live flags) · the three regen registries (`registry.json`/`COMMANDS.md`/`cli-surface.*` generated from sources + `make regen-check` drift gate).

**Invariant core:** the machine-readable contract is **generated by reflecting over the executable's own live structure**, so it *cannot* drift from actual behavior; a separate `--check` mode is the CI drift gate.

**Variance:** the reflection source (cobra tree vs SKILL.md frontmatter vs cobra flags).

**Why it recurs:** a published agent contract that drifts is worse than none. This is how AgentOps keeps ~172 registry capabilities honest.

**Package as:** the pattern is the lesson — "never hand-write a contract you can derive from the live artifact." Worth a `standards` reference entry. Caveat (audit A7): the reflection currently captures the command *tree* but not the typed *exit codes* — extend it.

---

## Pattern 7 — Atomic write (tmp + rename) for every persisted artifact

**Instances (~19 packages):** `aostate`, `config`, `pool`, `ratchet`, `verdictledger` (`writer.go:91`), `search`, `storage`, `lifecycle`, `harvest`, `wiki`, `llmwiki`, `scenarioresults`, `feedbackcompiler`, `evalsubstrate`, `rpi`, `types`, `doctor`, `agentworker`, `llm`.

**Invariant core:** write to a `*.tmp` sibling, `fsync`, `os.Rename` over the target — so a crash never leaves a half-written ledger/state file. Ledger appends additionally take a cross-process advisory `flock` (`provenancegraph/store.go:167`) so concurrent appenders can't fork the chain.

**Variance:** whether an advisory lock wraps the append (ledgers yes, config no).

**Package as — CORRECTION (verified against the 2026-06-24 run, P1):** the canonical helper **already exists** — `storage.AtomicWriteFile` (`cli/internal/storage/atomicfile.go:26`). Only ~13 sites delegate to it; the ~19 packages above still carry **private** `os.Rename`-based writers (`search/util.go`, `doctor/{engine,mutate,fix_skills,runartifact}.go`, `wiki/index.go`, `feedbackcompiler`, `llm/session_writer.go`, `pool.atomicMove`, …). So this is an **adoption/migration** gap, not a missing extraction — my initial "extract a new helper" framing was an overclaim the diff caught. Real work: migrate the private writers onto `storage.AtomicWriteFile`, add a **parent-dir fsync after rename** (the current helper fsyncs the file + renames but does not dir-fsync — the last crash-durability gap), and note `pool.atomicMove` still omits fsync entirely (the original data-loss risk 06-24 flagged, still open).

---

## Pattern 8 — Graceful capability-degradation ladder (probe by capability, not `command -v`)

**Instances (3+):** `cli/internal/orchestration/select.go:17` (NTM → Claude-native → beads-floor, first match wins) · `cli/internal/orchestration/ntm_probe.go:67` (`ntm --robot-capabilities`, not presence-on-PATH) · `cli/cmd/ao/beads_tracker.go` (`br` present? else exit-0 warning) · `cli/cmd/ao/beads.go:63` (`bdAvailable` seam) · `cli/cmd/ao/done.go:150` (origin-ledger fallback when local lags).

**Invariant core:** an external dependency is detected **by asking whether it can do the job** (a capability probe), and every tier emits the **same output-contract shape** so degradation is correctness-preserving, not a different code path. "A binary being on PATH says nothing about whether it can run a swarm."

**Variance:** the ladder rungs and the probe command.

**Package as:** the `orchestration.Selector` is already the reference implementation; the extractable asset is the **output-contract-parity discipline** (the floor tier must emit the same schema as the top tier) — worth codifying in the BC6 substrate contract.

---

## Pattern 9 — No-self-grade invariant routed through one function

**Instances (6 packages, one invariant):** `cli/internal/liveness/*` exposes `Disjoint(author, validator)`; consumed by `cli/internal/evidencedturn/evidencedturn.go:351`, `cli/internal/governor`, `cli/internal/aostate`, `cli/internal/ports`, `cli/internal/adapters`. Mirrored in `docs/contracts/pawls.md` (`context_id != author`) and `scripts/pawl-verdict.sh` (family normalization).

**Invariant core:** an author may never validate their own work; the check is centralized in **one** function (`liveness.Disjoint`) so the invariant has a single definition, and family normalization (claude/fable/anthropic → claude; codex → gpt) is shared so "different model" can't be spoofed by a rename.

**Variance:** what "identity" means at each call site (context_id vs model family vs actor).

**Why it recurs:** separation-of-duties is the trust source of the whole membrane. Centralizing it means the rule can't rot differently in six places.

**Package as:** already centralized — the lesson is the *pattern* (one invariant, one function, many callers). A model to copy for any other cross-cutting rule.

---

## Pattern 10 — Skip as a first-class verdict (exit 75 / 77), never a silent pass

**Instances:** `cli/internal/gates/scriptrunner.go` (exit **75** → `GateStatusSkip`) · `cli/internal/goals/measure.go:39` (`SkipExitCode = 77`, autotools convention, e.g. flywheel-compounding gate when corpus dormant) · `cli/internal/gates/report.go` (SKIP rendered distinctly from PASS).

**Invariant core:** "not applicable / not enough data to judge" is a distinct, *loud* third state — not folded into PASS. A skip is visible in the report and, for fitness, counts as `skip` not `pass` (so a dormant gate can't inflate the score).

**Variance:** the numeric code (75 gate / 77 goals) — worth reconciling to one.

**Package as:** the tri-state (`PASS`/`FAIL`/`SKIP`, + `UNKNOWN` as its own axis) is the gate verdict vocabulary; document the 75-vs-77 split as intentional-or-reconcile.

---

## Prioritized extraction backlog

| # | Pattern | Extract into | Value | Effort |
|---|---------|--------------|-------|--------|
| 1 | Hash-chained ledger | `cli/internal/hashchain` | **Very high** (security-critical, 5× dup) | Medium |
| 7 | Atomic write — **finish adoption** (helper `storage.AtomicWriteFile` already exists; migrate private writers, add parent-dir fsync, fix `pool.atomicMove` no-fsync) | migration, not extraction | **High** (crash-safety; already flagged 06-24 P1) | Low |
| 4 | Absence → unsafe-verdict | `no-fail-open-on-unknown` gate | **High** (would catch audit A1) | Low |
| 2 | `ExitError` interface + capabilities feed | shared interface | Medium (fixes A7) | Low |
| 3 | Decider contract | `standards` reference | Medium (guides new gates) | Low |
| 6 | Reflect-over-live-tree | `standards` reference | Medium (already practiced) | Low |

**Rule applied throughout (per the skill):** each pattern above has ≥3 verified instances, a clear invariant, a clear variance axis, and an escape hatch. Pattern 1 (hashchain) is the one genuine missing *extraction*; Pattern 7 is a partially-done *adoption* (helper exists since the 06-24 run — corrected above); Pattern 4 is a *prevention* (a gate that would catch A1). The rest are already well-factored and documented here as reusable design lessons, not code to consolidate.

---

## Related artifacts (this run)

- `codebase-archaeology.md` · `codebase-report.md` · `codebase-audit.md` · `SYNTHESIS.md`
