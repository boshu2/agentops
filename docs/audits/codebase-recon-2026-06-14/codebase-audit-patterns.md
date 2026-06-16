# Codebase Audit — PATTERNS mode (release-window scoped)

- **Skill executed:** `skills/review/SKILL.md` (absorbed `codebase-audit`), REPORT MODE = **patterns** — extract recurring change-shapes, name the invariant + variance points, cite ≥3 instances each, flag the archaeology of the window.
- **Scope (strict):** ONLY `git diff v3.1.0..HEAD` / `git log v3.1.0..HEAD`. Reference point `v3.1.0 = c98836977` (2026-06-10) → HEAD `ab6039808` (2026-06-14). 147 commits, 1602 files, **+37,404 / −109,471** (large net deletion). Unchanged code was NOT audited.
- **Mode:** strictly read-only. This report + a thin JSON result record are the only writes. `go build ./...` and `go vet ./...` were run (both clean) as evidence, not as mutation.
- **Date:** 2026-06-14.

## Method note

Patterns mode here treats the *release window* as the corpus. Each recurring change-shape below was measured this run from the commit log, `--numstat`/`--name-status`, and the actual added Go/shell sources — not copied from prior reports. Every named pattern clears the **3+ instances** bar. CASS/session mining was out of worker scope; discovery used `git log`/`git diff`/`rg` over the window only.

## Shape of the release at a glance

| Surface | net | read |
|---|---|---|
| `docs/` | **+14,210** | contracts (dispositions ledger), bdd-foundry plan/behavior/acceptance corpus, audits |
| `cli/` | **+6,842** | new Go features: codex hardening, converge/quorum, provenance, `skills retire` |
| `scripts/` | **+2,499** | 15 new gate/validator/install scripts |
| `tests/` | **+1,217** | new bats suites for the new gates |
| `skills/` | **−70,861** | skill-corpus prune (105→65) + Lane-A extraction to mt-olympus |
| `skills-codex/` | **−19,518** | codex twins of the pruned skills |
| `.agy-plugin/` | **−4,640** | agy-plugin skill twins pruned |

Commit-type histogram: **52 feat · 26 fix · 23 docs · 20 chore · 4 merge · 2 test.** The window is dominated by two motions running concurrently: **(A) a large teardown** (skill prune + bd/Dolt retirement) and **(B) a security/assurance build-out** (codex sandbox hardening, converge quorum, tamper-evident provenance). The −109k deletion is the teardown; the +37k is the build-out plus its docs/contract scaffolding.

Top epics by commit count: `ag-s43tg` (21, skill prune phase 2) · `ag-xwjlc` (16, seams) · `ag-p273x` (9, codex hardening) · `ag-codex…o0nds` (7, codex runtime) · `ag-pj51` (5, prune phase-1 closeout).

---

## Meta-pattern 0: Prove-the-gate-bites (the assurance signature of this window)

This is the single strongest *new* pattern of the release. A gate/check is treated as a **lie until it is shown to reject a planted positive** — the window repeatedly plants a known-bad fixture, asserts the gate rejects it, AND plants a known-good fixture, asserts the gate accepts it, before trusting any PASS.

**Source instances (≥3):**
1. **Two-sided canary entry gate** — `cli/cmd/ao/converge_canary.go`: before any judge dispatch, `convergeRunCanary` feeds the gate a planted self-judge verdict (must reject) AND an independent-context verdict (must accept); an all-reject *or* all-accept gate fails the canary and aborts the run (commit `10b93c5d8`).
2. **Tamper-evident provenance verify** — `cli/internal/provenancegraph/verify.go`: `VerifyFile` checks committed bytes exactly (no re-chain/re-sort) so a forged hash or reordered row is *caught* and the offending file line named (`479891017`).
3. **Pre-mortem failure-exercising tests** — `02018464f` explicitly registers "pre-mortem failure-exercising tests" as the disposition; `afb3535dc` / refuter findings harden converge from *pre-land* refuter output.
4. **Adversarial swarm fuzz on the MTO consumer** — `197180f5e`: a YAML-injection the formal Codex review missed was caught by adversarial swarm fuzz before land.

**Invariant core:** an empty/PASS result has no evidentiary value; only a gate observed *biting* a planted positive (and *not* biting a planted negative) is trusted. **Variance points:** what the planted fixture is (self-judge verdict / forged hash / injection payload); whether the canary runs inline (converge) or as a separate test (provenance).

**Why it matters:** this directly operationalizes the repo's own memory `quorum gate exists; producer is the build` and `prove the gate bites before trusting a PASS`. It is the assurance discipline maturing from doctrine into shipped Go.

---

## Pattern 1: Fail-closed validation, ordered before any side effect

Every new external-input surface validates **up front, before dispatch/execution/receipt creation**, and forbids opt-out.

**Source instances (codex hardening epic ag-p273x, ≥4):**
- `5c196ea64` — `validateCodexDispatchAuth` rejects forbidden env in BOTH ambient env and packet-provided env; `OPENAI_API_KEY` is *always* forbidden regardless of the packet's own `reject_env`/`forbid_api_key` — **a packet cannot weaken its own guard.**
- `3f5b39ffd` — `resolveCodexDispatchPath` rejects absolute paths and `..` traversal that escape the packet cwd and every declared `allowed_paths` root; `validateCodexDispatchPathBounds` runs **before auth checks, worker execution, and receipt creation.**
- `225e6203e` — raw packet JSON validated against the full embedded JSON Schema (`additionalProperties`, auth constants, enums) **before any unmarshal-based checks**, via `santhosh-tekuri/jsonschema/v6`.
- `ee70d603e` — `evidence.required_commands` is now *executed* (not documentary) and receipt validation fails if a declared required command is absent from `commands_run` — closes a "claimed-but-not-run" hole.
- Sibling in shell: `scripts/assay/consume-mto-recurrence.sh` (`78edf72c1`) — fail-closed `jq` type-checks, real calendar-date validation, class-name sanitization.

**Invariant core:** validate raw input against an explicit contract → reject before any irreversible action → never let the input relax its own guard. **Variance points:** runtime (Go schema-validation vs shell `jq`); the forbidden set (auth env / path traversal / type mismatch / injection).

**Risk removed:** packet-injection, path traversal, and auth-downgrade attack surface on the codex dispatcher — all newly closed this window. This is the largest *security-risk-reduction* of the release.

---

## Pattern 2: Schema-embedded-in-binary + on-disk parity twin

New contracts are added as JSON Schemas in **two byte-identical locations**: `schemas/` (repo SOT) and `cli/embedded/schemas/` (embedded into the Go binary), enforced by a parity test.

**Source instances (≥3, all added this window):**
- `schemas/codex-task-packet.schema.json` ↔ `cli/embedded/schemas/codex-task-packet.schema.json`
- `schemas/codex-run-receipt.schema.json` ↔ `cli/embedded/schemas/codex-run-receipt.schema.json`
- `cli/embedded/embed.go` (+`embed.go` glue) + `codex_schema.go`/`codex_schema_test.go` (parity-tested).

**Invariant core:** the validator ships *inside* the binary (no runtime file dependency, no drift between deployed binary and repo) while the human-editable SOT lives in `schemas/`; a parity test forbids the two diverging. **Variance point:** none yet — this is a fresh convention with exactly the codex pair as its first instances; it will only be a durable *pattern* once a third contract adopts it. **Flagged as an emerging convention, not yet a load-bearing one.**

---

## Pattern 3: Teardown via single-writer disposition ledger (not direct delete)

The −90k of skill deletion did **not** happen by ad-hoc `rm`. It routed through one canonical ledger, `docs/contracts/skill-dispositions.yaml` (the single biggest doc churn: **+1,161 / −831**), with `ao skills retire <slug> --into <target>` as the deterministic executor.

**Source instances (≥3):**
- `e936314e9` — `ao skills retire --into` deterministic retire operation (new Go: `cli/cmd/ao/skills_retire.go` +655, `_test.go` +601).
- `8140d2741` — fidelity hardening of the retire ledger-edit + `--into` validation.
- `94db74318` / `40ee1c34f` / `5e4f7e58a` — the three breaking `!` prune commits all cite the disposition ledger as authority; `0a0fb353d` routes the "cruft cross-reference sweep → disposition ledger."

**Invariant core:** a destructive corpus change is expressed as data (keep/fold/cut rows) in one ledger, executed by one tool that ripples the ~15 hand-maintained derived surfaces (registry, catalog, tiers, domain map, codex twins) — directly the repo memory `skill retire needs a tool, not a swarm`. **Variance points:** disposition verb (keep/fold/cut/extract-to-mt-olympus). **Risk removed:** orphaned references — `5e4f7e58a`/`40ee1c34f` cite validator-driven fixes; `85b7b97b1` made validators resolve skill paths via the ledger so a retired slug doesn't dangle.

---

## Pattern 4: New convention → new gate → new bats, in the same arc

Almost every behavioral convention added this window shipped with its enforcing gate **and** its bats test in the same epic — the repo's `Convention → Gate → Logged Escape Hatch` meta-pattern, now observed being *created* live rather than just maintained.

**Source instances (15 new `scripts/` gates, paired ≥3):**
- `check-workflow-governance.sh` (`d4111ec9e`) + `tests/scripts/check-workflow-governance.bats` (+112) — bidirectional `.js`↔ledger bijection.
- `check-bead-scenario-coverage` `--admission` mode (`075b48912`) + `check-bead-scenario-coverage.bats` (+185).
- `check-json-marshal-checked.sh` (`845b31446`) + the errcheck rule — fixes unchecked `json.Marshal/Unmarshal` returns (`agentops-tqc.3`).
- `resolve-skill-path.sh` + `resolve-skill-path.bats` (+240); `validate-skill-disposition-schema.sh` + `.bats` (+191); `check-doc-skill-refs.sh` + `.bats` (+99).

**Invariant core:** no convention lands without (a) a mechanical gate and (b) a bats proof the gate behaves. **Variance points:** gate severity (always-run blocking vs scoped); the SSOT it guards (workflow ledger / dispositions / doc refs / skill counts). New count gate `tests/docs/test-skill-count-ssot.sh` (+119) derives the skill count from disk SSOT and kills the manual-edit doc block (`0626f7a19`) — convergence on "derive, don't hand-maintain."

---

## Pattern 5: Test-first Go features (every new feature ships its test twin)

**Source instances:** 11 new Go `_test.go` files added this window, each paired to its feature file and frequently *larger* than it:
- `converge.go` (353) + `converge_test.go` (159) + `converge_canary.go` (87).
- `provenance_verify.go` (74) + `_test.go` (106); `provenancegraph/verify.go` (120) + `_test.go` (208) — **test bigger than impl.**
- `codex_dispatch_test.go` (**854**), `codex_schema.go` (116) + `_test.go` (273), `codex_image_health_test.go` (339), `codex_packet_contract_test.go` (271), `codex_task_packet_smoke_test.go` (91).
- `skills_retire.go` (655) + `_test.go` (601).

**Invariant core:** the L2-first/L1-always shape from `.claude/rules/go.md` is being honored in practice — security-sensitive surfaces (dispatch, schema, retire) carry the heaviest assertion mass. **Variance point:** test-to-impl ratio scales with blast radius. **Evidence:** `go build ./...` and `go vet ./...` both exit 0 at HEAD.

---

## Archaeology of the window (notable history signals)

1. **The seams epic (ag-xwjlc) paid a 44% reconciliation tax.** Of its 16 commits, **7 are rebase splice-repair / re-registration / post-rebase reconciliation** (`24e229655` rebase-4, `b1934713c` rebase-3, `7066bbb67`, `c87528119`, etc.). The epic re-registered codex twins and re-ran `regen` after *every* rebase against a hot `main`. This is the single clearest "shared-checkout-on-a-hot-repo" friction signature in the window and matches the repo memory `check NTM panes, not just AM, before hot writes` — concurrent lanes forced this one through repeated splice repair.

2. **The `aug/*` merge train (`dfe6ba55a`..`4d7248136`, ~26 commits).** The `ag-s43tg` prune landed as a long sequence of single-skill "graft trigger phrases into <target>" commits followed by a wall of `Merge branch 'aug/<skill>'`. This is the graft-only fold pattern: absorb a retired skill's trigger surface into a survivor *before* deleting it, one branch per survivor. Clean, auditable, but high commit-count — a deliberate trade of log noise for atomic-revert granularity.

3. **Two genuine breaking changes (`!`), both teardowns, both ledger-backed.** `45ccff436` (br+bv canonical, bd/Dolt retired) and `94db74318`/`40ee1c34f`/`5e4f7e58a` (skill prune). Both inverted long-standing contracts (closeout-contract `19b`, the skill corpus). The breaking marker was used correctly and each cites a controlling artifact (closeout-contract inversion; disposition ledger).

4. **Doc growth is mostly *planning corpus*, not prose drift.** The +14k docs is dominated by the bdd-foundry artifacts (`behaviors.md` +1069, two `spec.md`, `acceptance-tests/*.bats`, `beads-manifest.md`) and `skill-dispositions.yaml`. This is acceptance-first planning output (the bdd-foundry discipline) committed as evidence — `evidence/` also gained 10 audit files (`skill-prune-*`, `skill-corpus-token-audit.md`). The repo is committing its proof surfaces, per the `evidence over comfort` principle.

5. **The North-star reframe is in this window.** `b0b7ff1c2` "set the navigator goal + self-hosting destination — the repo had a scope, not a destination," plus `93c2e8f59`/`ab6039808` establishing the AgentOps↔Mount Olympus two-factory separation-of-duties. The window closes with a doctrine clarification, not code — the navigator/GPS-for-agentic-work framing landing in `docs/product`.

---

## Risk ledger for the window

| Risk | Direction | Evidence |
|---|---|---|
| Codex dispatch attack surface (auth-injection, path traversal, claimed-not-run, schema-bypass) | **REMOVED** | ag-p273x.1–.4, schema-at-runtime `225e6203e` |
| Gate giving false-green (empty PASS trusted) | **REMOVED** | two-sided canary `10b93c5d8`, provenance tamper-verify `479891017` |
| Unchecked `json.Marshal/Unmarshal` returns | **REMOVED** | errcheck rule + fixes `845b31446` |
| Orphaned references after 935-file skill deletion | **MITIGATED** | validators resolve via ledger `85b7b97b1`; validator-driven fixes in prune commits |
| Emerging schema-parity convention with only one instance pair | **WATCH** | Pattern 2 — not yet 3 adopters |
| High rebase-reconciliation cost on hot shared checkout | **PERSISTENT** | seams epic 7/16 splice-repair commits (process risk, not code risk) |
| Two breaking contract inversions in one window | **ACCEPTED** | both `!`-marked + ledger-backed; not silent |

**Net assessment:** this is a healthy release window. The deletion is disciplined (ledger-routed, validator-checked) and the additions skew heavily toward *assurance* — every new external-input surface is fail-closed-validated and every new gate is canary-proven. Build/vet are green at HEAD. The one standing process cost (not a code defect) is the rebase-reconciliation tax on the hot shared checkout, already a known repo memory.
