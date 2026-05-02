# Plan: gstack Absorption — Tier 1 Must-Port Candidates

- **Date:** 2026-05-01
- **Source research:** `.agents/research/gstack-absorption.md`
- **Pending findings:** `.agents/findings/pending-2026-05-01-gstack-absorption.jsonl`
- **Applied findings:** f-2026-05-01-005 (knowledge separation), f-2026-05-01-017 (hooks/daemon compose), pending f-2026-05-01-021/022/023 (this session's findings)
- **Complexity:** Standard (5 issues + 1 epic; spans hooks/skills/CLI; foundational refactor included)
- **Detail template:** Standard (3-6 issues, standard complexity per `references/detail-templates.md`)

## Context

The gstack absorption research identified 4 Tier 1 must-port candidates. This plan decomposes them into trackable bd issues organized into 2 waves. The state-path resolver (#3 in research) is treated as foundational — other Tier 1 items can land in parallel with it because file ownership is partitioned, but a follow-up Wave 2 issue migrates all consumers to source it.

### Goal

Land 4 Tier 1 ports + 1 foundational migration in a way that:
- Preserves agentops's `.agents/` (internal) vs raw+wiki (external) knowledge separation (f-2026-05-01-005)
- Treats hooks (session-boundary) and agentopsd (cron-cadence) as composing surfaces (f-2026-05-01-017)
- Maintains 4-runtime parity (every new skill ships in skills/ AND skills-codex/)
- Reuses existing atomic-replace patterns (`cli/internal/storage/file.go:255 atomicWrite`, `cli/internal/llmwiki/scope_guard.go:76 SafeAtomicWrite`); no new locking semantics

### Non-goals (explicit OUT of scope)

- Tier 2 / Tier 3 candidates from the research (deferred to future epics)
- Anything in the synthesis's "Already Shipped" table
- Bun adoption or porting the `browse` compiled binary (Tier 1 #2 lands as **contract-only** markdown skill + design doc; binary deferred)
- Host adapters beyond agentops's stated 4 runtimes (Claude Code / Codex / Cursor / OpenCode)
- Editing `.agents/findings/registry.jsonl` (in unmerged state from prior session — pending findings parked at sidecar JSONL)

## Baseline Audit

Quantified evidence gathered before decomposition:

```
$ grep -rln "\".agents/" cli/cmd/ao/*.go | wc -l
82                                          # ao subcmd files with hardcoded .agents/ strings

$ grep -n "ROOT.*\.agents" lib/hook-helpers.sh | wc -l
≥10                                         # paths scattered in lib/hook-helpers.sh too
                                            # (e.g., _HOOK_PACKET_ROOT, _EVIDENCE_ONLY_CLOSURE_DIR)

$ ls cli/internal/storage/file.go cli/internal/llmwiki/scope_guard.go
                                            # both export atomic-write helpers — REUSE these
                                            # storage/file.go:255 atomicWrite (private)
                                            # llmwiki/scope_guard.go:76 SafeAtomicWrite (exported, vault-bounded)

$ ls skills/ | wc -l && ls skills-codex/ | wc -l
70 (incl. SKILL-TIERS.md)   69              # 4-runtime parity invariant: 69 skills × 2

$ jq '.hooks | keys' hooks/hooks.json
[ConfigChange, PostToolUse, PreCompact, PreToolUse, SessionEnd, SessionStart, Stop, SubagentStop, TaskCompleted, UserPromptSubmit, WorktreeCreate, WorktreeRemove]
                                            # PreToolUse is the matcher for edit-scope guard

$ grep -rln "AgentsDir\|knowledgeRoot\|defaultRoot" cli/internal/*/ | wc -l
≥10                                         # ad-hoc resolver functions across packages
```

**Existing reusable surfaces (no rebuild):**
- `cli/internal/storage/file.go:273 atomicWrite` — temp-file + rename pattern
- `cli/internal/llmwiki/scope_guard.go:76 SafeAtomicWrite` — vault-bounded variant
- `lib/hook-helpers.sh` — already has path knowledge but does not own resolution as its mission
- `scripts/audit-codex-parity.sh` (Python under the hood) — invariant for skills-codex parity
- `scripts/sync-skill-counts.sh` — counts skills across docs; call from `ao skills check`
- `cli/cmd/ao/heal_skill.go` — repair side; `ao skills check` is the read-only audit complement

## Files to Modify (Inventory)

| File | Wave | Issue | Access | Notes |
|---|---|---|---|---|
| `lib/ao-paths.sh` | 1 | I1 | **CREATE** | New: sourceable shell resolver. `eval "$(./lib/ao-paths.sh)"` exports `AO_AGENTS_DIR`, `AO_KNOWLEDGE_ROOT`, `AO_HOOKS_DIR`, `AO_SCOPE_LOCK`, etc. |
| `cli/internal/paths/paths.go` | 1 | I1 | **CREATE** | New Go package. `Resolve() *Paths` returns struct with all roots. Honors `AO_HOME` override + `CLAUDE_PLUGIN_DATA` fallback. |
| `cli/internal/paths/paths_test.go` | 1 | I1 | **CREATE** | L1 unit + L2 env-override scenario |
| `cli/cmd/ao/skills.go` | 1 | I2 | **CREATE** | `ao skills check [--json]` subcommand registration + RunE |
| `cli/internal/skillshealth/audit.go` | 1 | I2 | **CREATE** | New package. `Audit(skillsDir, codexDir string) Report` walks both trees, validates frontmatter, checks references, codex parity drift |
| `cli/internal/skillshealth/audit_test.go` | 1 | I2 | **CREATE** | L1 unit (frontmatter parse, parity comparison); L2 (full skills/ tree audit) |
| `cli/cmd/ao/skills_test.go` | 1 | I2 | **CREATE** | L1 cobra registration test + JSON output schema test |
| `skills/scope/SKILL.md` | 1 | I3 | **CREATE** | `/scope` skill — `freeze <dir>...`, `unfreeze [<dir>]`, `status`, `guard` (combo) |
| `skills/scope/references/lock-file-format.md` | 1 | I3 | **CREATE** | Reference doc for the `.agents/scope.lock` JSON schema |
| `skills-codex/scope/SKILL.md` | 1 | I3 | **CREATE** | Codex parity (mirrors skills/scope/SKILL.md) |
| `skills-codex/scope/prompt.md` | 1 | I3 | **CREATE** | Codex-specific render (per audit-codex-parity convention) |
| `skills-codex/scope/references/lock-file-format.md` | 1 | I3 | **CREATE** | Codex reference parity |
| `hooks/edit-scope-guard.sh` | 1 | I3 | **CREATE** | PreToolUse hook script. Reads JSON tool input from stdin, compares target path against `.agents/scope.lock` entries, exits non-zero with structured reason if outside scope. |
| `hooks/hooks.json` | 1 | I3 | **edit** (single block append) | Register `edit-scope-guard.sh` under `PreToolUse` matcher `Edit\|Write\|Bash`. **WAVE-EXCLUSIVE: only I3 touches hooks.json.** |
| `cli/cmd/ao/scope.go` | 1 | I3 | **CREATE** | `ao scope freeze <dir>...`, `ao scope unfreeze [<dir>]`, `ao scope status` cobra commands |
| `cli/cmd/ao/scope_test.go` | 1 | I3 | **CREATE** | L1 (subcommand registration); L2 (scenario: freeze, attempted edit blocked, unfreeze, edit allowed) |
| `cli/internal/scope/scope.go` | 1 | I3 | **CREATE** | New package. Reuses `cli/internal/llmwiki/scope_guard.go:76 SafeAtomicWrite` for lock-file mutation. NO new locking primitives. |
| `cli/internal/scope/scope_test.go` | 1 | I3 | **CREATE** | L1 (atomic write through SafeAtomicWrite); L2 (concurrent freeze attempt — last writer wins, never tears) |
| `.agents/decisions/2026-05-01-browse-contract.md` | 1 | I4 | **CREATE** | Design doc only — no code. Decides: contract-only markdown skill vs Go wrapper vs deferral entirely. Outputs a follow-up bd issue if any. |
| `cli/cmd/ao/*.go` (subset) | 2 | I5 | **edit** | Migrate ad-hoc `.agents/` strings to `cli/internal/paths` resolver. Single-worker job (touches 82 files). |
| `hooks/*.sh` (subset) | 2 | I5 | **edit** | Migrate hardcoded `.agents/` to `eval "$(./lib/ao-paths.sh)"`. Single-worker job. |
| `lib/hook-helpers.sh` | 2 | I5 | **edit** | Replace `_HOOK_PACKET_ROOT`-style constants with sourced resolver. Single-worker job. |
| `scripts/check-paths-resolver-coverage.sh` | 2 | I5 | **CREATE** | Warn-only ratchet check: greps for hardcoded `.agents/` strings outside the resolver; counts; warns above threshold. Exit 0 always (warn-only first). |
| `GOALS.md` | 2 | I5 | **edit** (single section append) | New fitness gate `state-path-resolver-coverage` (warn-only initially per warn-then-fail-ratchet pattern) |
| `.agents/patterns/2026-05-01-state-path-resolver.md` | 2 | I5 | **CREATE** | Documented pattern: every new hook/subcommand MUST source the resolver. Promoted from finding f-2026-05-01-022. |

## Boundaries

**In scope:**
- New files for I1-I4 (foundational additions)
- Single-block edit to `hooks/hooks.json` by I3 (one new PreToolUse entry; no other matchers touched)
- Mass refactor in I5 (Wave 2, single-worker, no overlap with I1-I4)
- New GOALS.md fitness gate (warn-only first)

**Out of scope:**
- Tier 2 / Tier 3 candidates (file separately as backlog at end of plan)
- Bun runtime adoption
- `browse` compiled binary port (I4 produces design doc only; implementation issue created downstream if approved)
- Editing `.agents/findings/registry.jsonl` (in unmerged state)
- Refactoring `cli/internal/storage/file.go atomicWrite` or `cli/internal/llmwiki/scope_guard.go SafeAtomicWrite` (REUSE, do not modify)

## Issues

### I1 — State-path resolver foundation
**Wave:** 1
**Type:** task
**Priority:** P1
**Dependencies:** none

**Description:**
Create `lib/ao-paths.sh` (sourceable) and `cli/internal/paths/paths.go` (Go) as the canonical state-path resolver. Both share env-var precedence: `AO_HOME` > `CLAUDE_PLUGIN_DATA` > `${PWD}/.agents` default. Resolve `AO_AGENTS_DIR`, `AO_KNOWLEDGE_ROOT`, `AO_HOOKS_DIR`, `AO_SCOPE_LOCK` (`.agents/scope.lock`), `AO_RPI_DIR`, `AO_FINDINGS_DIR`, `AO_PLANS_DIR`.

**Implementation:**
- `lib/ao-paths.sh` emits `export <NAME>=<value>` lines on stdout (one per resolved root). Idempotent.
- `cli/internal/paths/paths.go`:
  - Type `Paths struct { Home, AgentsDir, KnowledgeRoot, HooksDir, ScopeLock, RPIDir, FindingsDir, PlansDir string }`
  - Function `Resolve() *Paths` reads env, falls back to defaults
  - Function `(p *Paths) Validate() error` ensures dirs exist or are creatable
- Tests: `paths_test.go` — table-driven cases for env precedence; one L2 scenario asserts shell + Go agree on resolution given identical env.

**Acceptance criteria:**
- Both resolvers agree on `AO_AGENTS_DIR` for at least 4 env permutations
- `lib/ao-paths.sh` is `chmod +x` and runs under `bash -e`
- `cli/internal/paths` exports `Resolve()` and `Paths` struct as the documented surface
- L1 unit + L2 cross-language scenario pass

**Files owned (write):** `lib/ao-paths.sh`, `cli/internal/paths/paths.go`, `cli/internal/paths/paths_test.go`

### I2 — `ao skills check` health dashboard
**Wave:** 1
**Type:** feature
**Priority:** P1
**Dependencies:** none (will source resolver post-I1 in I5; lands functional with hardcoded paths first, refactored in Wave 2)

**Description:**
New cobra subcommand `ao skills check [--json] [--strict]`. Walks `skills/` and `skills-codex/`, validates YAML frontmatter (name, description present; name matches dir), checks every `references/*.md` file is linked from SKILL.md, runs codex parity diff (skill exists in both; description hash matches via `scripts/audit-codex-parity.sh` heuristic), reports broken references. `--strict` exits 1 on any finding (CI mode).

**Implementation:**
- `cli/cmd/ao/skills.go`:
  - Cobra command `skillsCmd` with subcommand `checkCmd`
  - Flags `--json bool`, `--strict bool`
  - RunE delegates to `skillshealth.Audit(skillsDir, codexDir, opts)`
- `cli/internal/skillshealth/audit.go`:
  - Type `Report struct { Skills []SkillStatus, Errors []string, ParityDrift []string }`
  - Type `SkillStatus struct { Name, Path string, FrontmatterValid bool, BrokenRefs []string, CodexParity string }`
  - Function `Audit(skillsDir, codexDir string) Report`
- Reuses `scripts/audit-codex-parity.sh` invariants but reimplemented in Go for speed; consults the same hash conventions documented in `scripts/regen-codex-hashes.sh`.

**Acceptance criteria:**
- `ao skills check` exits 0 when repo is healthy
- `ao skills check --json` produces valid JSON with all 69 skills enumerated
- `ao skills check --strict` exits 1 if any frontmatter is missing `name:` or `description:`
- L1 unit tests for frontmatter parser, parity comparator
- L2 integration test runs against real `skills/` tree, asserts exit 0 and skill count

**Files owned (write):** `cli/cmd/ao/skills.go`, `cli/cmd/ao/skills_test.go`, `cli/internal/skillshealth/audit.go`, `cli/internal/skillshealth/audit_test.go`

### I3 — Edit-scope guard (port `/freeze` + `/unfreeze` + `/guard`)
**Wave:** 1
**Type:** feature
**Priority:** P1
**Dependencies:** none (will source resolver in I5; I3 lands with hardcoded `.agents/scope.lock` path)

**Description:**
Hard-block enforcement of declared edit scopes. New `/scope` skill (with skills-codex parity), new `cli/cmd/ao/scope.go` subcommand (`freeze <dir>...`, `unfreeze [<dir>]`, `status`), new `hooks/edit-scope-guard.sh` PreToolUse hook that consults `.agents/scope.lock` and rejects edits outside locked dirs.

**Implementation:**
- `cli/cmd/ao/scope.go`: cobra commands `freezeCmd`, `unfreezeCmd`, `statusCmd` under `scopeCmd`. RunE delegates to `cli/internal/scope`.
- `cli/internal/scope/scope.go`:
  - Type `Lock struct { FrozenDirs []string, AcquiredAt time.Time, AcquiredBy string }` (additive set semantics; supports multiple frozen dirs)
  - Function `Read(lockPath string) (*Lock, error)`
  - Function `Write(lockPath string, l *Lock) error` — **MUST use `cli/internal/llmwiki/scope_guard.go:76 SafeAtomicWrite`**, no new locking
  - Function `IsAllowed(l *Lock, targetPath string) bool` — true if `len(FrozenDirs) == 0` OR `targetPath` is under any frozen dir
- `hooks/edit-scope-guard.sh`:
  - Reads JSON from stdin (Claude Code hook input)
  - Extracts `tool.params.file_path` (Edit/Write) or `tool.params.command` (Bash — best-effort path extraction)
  - Sources `.agents/scope.lock` (later via `lib/ao-paths.sh` in I5; for now hardcoded)
  - Exits 2 with stderr reason if outside scope
- `hooks/hooks.json`: append one PreToolUse entry, matcher `Edit|Write|Bash`, timeout 5s
- `skills/scope/SKILL.md`: documents `/scope freeze <dir>...`, `/scope unfreeze`, `/scope status`. YAML frontmatter follows agentops convention.
- `skills-codex/scope/SKILL.md` + `prompt.md`: parity render

**Acceptance criteria:**
- L2 scenario: spawn 2 workers, freeze `cli/cmd/ao/`, second worker's edit attempt outside frozen scope rejected with structured reason on stderr
- L2 race scenario: 100 concurrent freeze calls converge to single valid lock file (atomic-write invariant)
- `ao scope status --json` returns current lock state
- `audit-codex-parity.sh` passes for new `scope` skill
- Hook registers cleanly in hooks.json (validate against schemas/)

**Files owned (write):** `skills/scope/SKILL.md`, `skills/scope/references/lock-file-format.md`, `skills-codex/scope/SKILL.md`, `skills-codex/scope/prompt.md`, `skills-codex/scope/references/lock-file-format.md`, `hooks/edit-scope-guard.sh`, `hooks/hooks.json` (single block append), `cli/cmd/ao/scope.go`, `cli/cmd/ao/scope_test.go`, `cli/internal/scope/scope.go`, `cli/internal/scope/scope_test.go`

### I4 — Browse contract design doc (decision artifact, no code)
**Wave:** 1
**Type:** task
**Priority:** P2
**Dependencies:** none

**Description:**
Per the synthesis recommendation #1, `/browse` (Tier 1 #2) needs a design decision before implementation. Produce `.agents/decisions/2026-05-01-browse-contract.md` covering: (a) contract-only markdown skill vs (b) Go wrapper around user-installed Chromium vs (c) defer entirely. Decision must reference PRODUCT.md persona analysis and weigh against agentops users being mostly CLI/library authors today.

**Implementation:**
- New `.agents/decisions/2026-05-01-browse-contract.md` with sections: Context, Options, Tradeoff Matrix, Recommendation, Follow-up bd issue (if approved)
- No code changes
- Follow-up issue (if approved) gets filed at end of this plan's epic

**Acceptance criteria:**
- Decision doc lands with three options enumerated and a recommendation
- If recommendation is "build", a follow-up bd issue is filed with scope estimate
- If recommendation is "defer", reasoning is documented and Tier 1 #2 archives to backlog notes

**Files owned (write):** `.agents/decisions/2026-05-01-browse-contract.md`

### I5 — Migrate consumers to source state-path resolver (warn-only ratchet)
**Wave:** 2
**Type:** task
**Priority:** P1
**Dependencies:** I1 (state-path resolver must exist), I3 (hooks.json registration must land first to avoid bulk-edit conflict)

**Description:**
Mass refactor: replace ad-hoc `.agents/` string concatenation in `cli/cmd/ao/*.go` (82 files), `hooks/*.sh` (~10 affected), and `lib/hook-helpers.sh` (≥10 sites) with calls to the resolver from I1. Add a warn-only fitness ratchet that counts remaining hardcoded sites and reports trend. Document the pattern.

**Implementation:**
- Single-worker job (high file overlap; not safely parallelizable)
- Refactor pass per consumer family:
  - `cli/cmd/ao/*.go`: import `cli/internal/paths`, replace `".agents/..."` literals with `paths.Resolve().AgentsDir + "/..."`
  - `hooks/*.sh`: replace `${ROOT}/.agents/...` with `eval "$(./lib/ao-paths.sh)"; "$AO_AGENTS_DIR/..."`
  - `lib/hook-helpers.sh`: replace `_HOOK_PACKET_ROOT="${ROOT}/.agents/ao/packets"` with sourced equivalent
- New `scripts/check-paths-resolver-coverage.sh`: greps for hardcoded `.agents/` outside the resolver, counts, prints metric. Exit 0 always (warn-only first per warn-then-fail-ratchet pattern).
- Append fitness gate to `GOALS.md`:
  ```yaml
  state-path-resolver-coverage:
    measure: scripts/check-paths-resolver-coverage.sh
    threshold: warn-only
    weight: 3
  ```
- `.agents/patterns/2026-05-01-state-path-resolver.md`: document the pattern, when-to-apply, why it matters; promoted from finding f-2026-05-01-022.

**Acceptance criteria:**
- `scripts/check-paths-resolver-coverage.sh` exits 0 (warn-only initially)
- After migration, hardcoded `.agents/` count drops by ≥80% (from ≥82 to ≤16)
- All `cli/cmd/ao/*_test.go` continue to pass
- All hooks continue to fire correctly (`tests/install/test-install-smoke.sh` passes)
- New pattern doc lands

**Files owned (write):** `cli/cmd/ao/*.go` (mass refactor), `hooks/*.sh` (mass refactor), `lib/hook-helpers.sh` (refactor), `scripts/check-paths-resolver-coverage.sh` (new), `GOALS.md` (single section append), `.agents/patterns/2026-05-01-state-path-resolver.md` (new)

## File Dependency Matrix (Step 5.5 mandatory)

| Task | File | Access | Notes / Conflict Resolution |
|---|---|---|---|
| I1 | `lib/ao-paths.sh` | write | New file, no conflict |
| I1 | `cli/internal/paths/paths.go` | write | New package, no conflict |
| I1 | `cli/internal/paths/paths_test.go` | write | New, no conflict |
| I2 | `cli/cmd/ao/skills.go` | write | New file, no conflict |
| I2 | `cli/cmd/ao/skills_test.go` | write | New, no conflict |
| I2 | `cli/internal/skillshealth/audit.go` | write | New package |
| I2 | `cli/internal/skillshealth/audit_test.go` | write | New |
| I2 | `scripts/audit-codex-parity.sh` | read | I2 reuses invariants, does not modify |
| I2 | `skills/`, `skills-codex/` | read | Read-only walk |
| I3 | `skills/scope/**` | write | New dir, no conflict |
| I3 | `skills-codex/scope/**` | write | New dir, no conflict |
| I3 | `hooks/edit-scope-guard.sh` | write | New file |
| I3 | `hooks/hooks.json` | **write (single-block append)** | **WAVE-EXCLUSIVE in Wave 1.** No other I1/I2/I4 issue may touch hooks.json. I5 (Wave 2) MUST land after I3. |
| I3 | `cli/cmd/ao/scope.go` | write | New file |
| I3 | `cli/cmd/ao/scope_test.go` | write | New |
| I3 | `cli/internal/scope/scope.go` | write | New package |
| I3 | `cli/internal/scope/scope_test.go` | write | New |
| I3 | `cli/internal/llmwiki/scope_guard.go` | read | REUSE `SafeAtomicWrite`. Do not modify. |
| I4 | `.agents/decisions/2026-05-01-browse-contract.md` | write | New file |
| I4 | `PRODUCT.md` | read | Persona reference for decision |
| I5 | `cli/cmd/ao/*.go` (~82 files) | **write (single-worker)** | Wave 2; serializes after Wave 1. Conflicts with I2/I3 if run in parallel — mandatory wave-2-only. |
| I5 | `hooks/*.sh` (~10 files) | **write (single-worker)** | Wave 2 only |
| I5 | `hooks/hooks.json` | read | I5 does NOT modify hooks.json (I3 already added entry) |
| I5 | `lib/hook-helpers.sh` | **write** | Wave 2; single-worker |
| I5 | `lib/ao-paths.sh` | read | Sourcing the resolver from I1 |
| I5 | `cli/internal/paths/paths.go` | read | Importing the resolver from I1 |
| I5 | `scripts/check-paths-resolver-coverage.sh` | write | New |
| I5 | `GOALS.md` | **write (single section append)** | Wave 2 only |
| I5 | `.agents/patterns/2026-05-01-state-path-resolver.md` | write | New |

**Cross-wave shared file registry:**
- `hooks/hooks.json` — modified by I3 (Wave 1) only. I5 (Wave 2) does NOT touch.
- `cli/cmd/ao/*.go` — I2 creates `skills.go` and `skills_test.go`; I3 creates `scope.go` and `scope_test.go`; I5 (Wave 2) edits the broader 82-file set INCLUDING the new I2/I3 files (must be merged before I5 starts).
- `lib/` — I1 creates `ao-paths.sh`; I5 (Wave 2) refactors `hook-helpers.sh`.

**Wave gating:** I5 starts only after I1, I2, I3 are merged to main. I5 explicitly serializes; runs as single-worker.

## Implementation Notes (cross-cutting)

### Atomic-write reuse contract
- I3's `cli/internal/scope/scope.go` MUST call `cli/internal/llmwiki/scope_guard.go:76 SafeAtomicWrite` for every lock-file mutation. Reason: the same failure class hit `agentopsd` queue-claim invariants (per pre-mortem hand-off requirement). New locking semantics are explicitly forbidden.
- I1's `cli/internal/paths/paths.go` is read-mostly; no atomic-write surface needed.

### Codex parity contract
- Every new skill in `skills/` MUST land with a sibling in `skills-codex/` in the same PR
- `scripts/audit-codex-parity.sh` runs in CI and fails if delta detected
- I3 specifically lists 5 codex parity files

### Warn-then-fail-ratchet for I5
- New fitness gate `state-path-resolver-coverage` lands as **warn-only** (per `pattern: pre-tag-ci-validation` + `warn-then-fail-ratchet`)
- After 2 weeks of baseline data, a follow-up issue flips threshold to blocking
- Rationale: prevents 82-file mass refactor from blocking CI on day-one drift

## Tests

### L0 (compile/lint, fastest)
- `go vet ./...` + `golangci-lint run` after each issue
- `bash -n hooks/edit-scope-guard.sh` + `actionlint` if applicable
- `scripts/audit-codex-parity.sh` after I3

### L1 (unit, fast)
- `cli/internal/paths/paths_test.go`: env precedence, default fallback (table-driven, 6 cases)
- `cli/internal/skillshealth/audit_test.go`: frontmatter parser, parity comparator (8+ cases)
- `cli/internal/scope/scope_test.go`: lock read/write, IsAllowed predicate (10+ cases)
- `cli/cmd/ao/skills_test.go`: cobra registration, JSON schema
- `cli/cmd/ao/scope_test.go`: cobra registration

### L2 (integration, where bugs are found)
- I1: cross-language resolver agreement scenario (shell+Go agree given same env)
- I2: full `skills/` tree audit against real repo, exit 0 with 69 skills enumerated
- I3: spawn-2-workers scenario (freeze, second-worker-edit-rejected, unfreeze, edit-allowed)
- I3: 100 concurrent freeze calls converge atomically (race test)
- I5: refactor preserves all existing test pass rates (run full `cd cli && make test` before/after)

### L3 (e2e/slow, gated)
- Not required for this plan (no shipped binary; install-smoke gate covers)

## Conformance Checks (per-issue, embedded in bd validation blocks)

```yaml
# I1 conformance
files_exist:
  - lib/ao-paths.sh
  - cli/internal/paths/paths.go
  - cli/internal/paths/paths_test.go
content_check:
  - file: lib/ao-paths.sh
    contains: "AO_AGENTS_DIR"
tests:
  - cd cli && go test ./internal/paths/...

# I2 conformance
files_exist:
  - cli/cmd/ao/skills.go
  - cli/internal/skillshealth/audit.go
tests:
  - cd cli && go test ./cmd/ao -run TestSkills
  - cd cli && go test ./internal/skillshealth/...
command:
  - cd cli && go build ./... && ./bin/ao skills check --json | jq -e '.skills | length >= 60'

# I3 conformance
files_exist:
  - skills/scope/SKILL.md
  - skills-codex/scope/SKILL.md
  - hooks/edit-scope-guard.sh
  - cli/cmd/ao/scope.go
  - cli/internal/scope/scope.go
content_check:
  - file: cli/internal/scope/scope.go
    contains: "SafeAtomicWrite"
tests:
  - cd cli && go test ./internal/scope/...
  - bash scripts/audit-codex-parity.sh --skill scope
  - bash -n hooks/edit-scope-guard.sh

# I4 conformance
files_exist:
  - .agents/decisions/2026-05-01-browse-contract.md
content_check:
  - file: .agents/decisions/2026-05-01-browse-contract.md
    contains: "Recommendation"

# I5 conformance
files_exist:
  - scripts/check-paths-resolver-coverage.sh
  - .agents/patterns/2026-05-01-state-path-resolver.md
command:
  - bash scripts/check-paths-resolver-coverage.sh   # exit 0 (warn-only)
  - cd cli && make test                              # no regressions
content_check:
  - file: GOALS.md
    contains: "state-path-resolver-coverage"
```

## Verification (Whole-Plan Acceptance)

After all 5 issues merge:
1. `cd cli && make build && make test` — green
2. `scripts/pre-push-gate.sh --fast` — green
3. `bash scripts/audit-codex-parity.sh` — green
4. `ao skills check --strict` — green (now self-validating)
5. `ao scope freeze cli/cmd/ao/ && echo test > /tmp/blocked && [ $? -ne 0 ]` — scope guard fires (manual smoke)
6. `bash scripts/check-paths-resolver-coverage.sh` — exits 0 with metric printed
7. New skill `scope` is invokable via `/scope freeze ...` from a Claude Code session
8. Codex parity: `/scope` skill loads from skills-codex/ in a Codex session

## Execution Order

```
Wave 1 (parallel, 4 workers, no file overlap):
  ├─ I1 — State-path resolver foundation     [worker A: lib/, cli/internal/paths/]
  ├─ I2 — ao skills check                    [worker B: cli/cmd/ao/skills*, cli/internal/skillshealth/]
  ├─ I3 — Edit-scope guard                   [worker C: skills/scope/, skills-codex/scope/, hooks/, cli/cmd/ao/scope*, cli/internal/scope/]
  └─ I4 — Browse contract design doc         [worker D: .agents/decisions/]

  Wave 1 merge gate: all 4 issues green; hooks.json validates; codex parity audit passes; ao skills check exits 0.

Wave 2 (single-worker, serialized after Wave 1 fully merged):
  └─ I5 — Migrate consumers to resolver       [single worker: refactor pass]

  Wave 2 merge gate: full make test passes; check-paths-resolver-coverage prints baseline; new pattern doc + GOALS.md fitness gate land.
```

## Planning Rules Compliance (PR-001 through PR-007)

| Rule | Status | Justification |
|---|---|---|
| **PR-001 — Mechanical enforcement** | PASS | I5 introduces `scripts/check-paths-resolver-coverage.sh` ratchet; I3 reuses existing atomic-write helpers as enforcement mechanism (no new locking); GOALS.md fitness gate is mechanical |
| **PR-002 — External validation** | PASS | I2 (`ao skills check`) and I3 (scope hook) are external validators of contracts; codex parity audit (`scripts/audit-codex-parity.sh`) is third-party check |
| **PR-003 — Feedback loops** | PASS | Each issue has acceptance test; I5 produces metric for ratchet; I4 feeds back into design backlog if "build" decision |
| **PR-004 — Knowledge separation** | PASS | All writes scoped to `.agents/` (decisions, patterns, plans) or repo files. No writes to raw+wiki/. Cited f-2026-05-01-005. |
| **PR-005 — Process gates** | PASS | Wave 2 serializes after Wave 1 merge gate; pre-mortem mandatory before Wave 1 starts; warn-then-fail ratchet for I5 |
| **PR-006 — Cross-layer consistency** | PASS | I1 enforces shell + Go agree on resolver; I3 enforces skill + skills-codex parity; I2 audits all skills uniformly |
| **PR-007 — Phased rollout** | PASS | Wave 1 lands additions (low risk); Wave 2 mass refactor lands warn-only first; threshold flip is a separate follow-up issue |

## Post-Merge Cleanup

After Wave 2 merge:
- File a follow-up bd issue (P3) to flip `state-path-resolver-coverage` from warn-only to blocking after 2 weeks of baseline data
- File a follow-up bd issue per I4 recommendation (if "build" decision) for browse implementation
- Run `scripts/sync-skill-counts.sh` to sync the new `scope` skill across all docs
- Update `docs/SKILLS.md` and `using-agentops/SKILL.md` to reference the new `/scope` skill
- File pending findings (021/022/023) into `.agents/findings/registry.jsonl` AFTER the registry's UU state is resolved

## Next Steps (after this plan)

1. `/pre-mortem .agents/plans/2026-05-01-gstack-absorption-tier1.md --quick` (next in /discovery DAG)
2. If pre-mortem PASS/WARN → `/crank` against the epic
3. If pre-mortem FAIL → re-plan with feedback (max 3 attempts per /discovery contract)

## Backlog (NOT in this plan; deferred for future epic)

- Tier 2 candidates from `.agents/research/gstack-absorption.md`:
  - #5 Telemetry/timeline/question/review log convention (medium leverage; touches existing JSONL writers — file when registry is unmerged-resolved)
  - #6 `/canary` post-deploy probe lane (depends on browse decision — file after I4 lands)
  - #7 `/investigate` systematic mode for `/bug-hunt` (refinement)
  - #8 `ao hooks install/uninstall` operator commands (medium leverage; install-path UX)
  - #9-#10 see synthesis
- Tier 3: defer-or-skip set
