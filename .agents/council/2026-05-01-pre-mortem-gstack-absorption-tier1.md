---
id: pre-mortem-2026-05-01-gstack-absorption-tier1
type: pre-mortem
date: 2026-05-01
mode: quick
plan_path: .agents/plans/2026-05-01-gstack-absorption-tier1.md
epic: soc-irg1
scope_mode: expansion
---

# Pre-Mortem: gstack Absorption — Tier 1

## Council Verdict: WARN

**Recommendation:** Proceed to /crank after applying 4 small plan amendments documented below. The plan is fundamentally sound — atomic-write reuse is correctly mandated, codex parity invariant is enforced, dependency matrix is correct, hooks.json wave-exclusivity is preserved. The warnings address surface-enumeration gaps and defensive coding holes, not architectural flaws.

## Verdict Summary

| Concern | Verdict | Notes |
|---|---|---|
| I3 lock-file race risk | **PASS** | Plan explicitly mandates `SafeAtomicWrite` reuse; conformance check enforces grep |
| I5 mass-refactor safety | **WARN** | Surface enumeration incomplete; commit policy unspecified |
| Codex parity invariant | **PASS** | I3 ships 5 codex files + audit-codex-parity gate |
| Wave 2 file-overlap discipline | **PASS** | bd dependencies confirmed (soc-irg1.5 blocked-by .1+.3); single-worker explicit |
| hooks.json wave-exclusivity | **PASS** | Matrix correctly assigns single writer (I3) |
| Pending-findings sidecar handling | **PASS** | Plan explicitly defers registry edits until UU resolved |
| Council FAIL pattern 4 (Propagation blindness) | **WARN** | I5 surface may extend beyond 82 ao files |
| Council FAIL pattern 6 (Dead infra activation) | **WARN** | I3 hook smoke test verifies syntax, not behavior |
| Council FAIL pattern 1 (Mechanical verification) | **WARN** | I3 L2 scenarios lack concrete test commands |
| Mandatory check 2.8 (Input validation) | **WARN** | I3 hook missing malformed-JSON defense |
| Mandatory check 2.7 (Test pyramid) | **PASS** | L0 + L1 + L2 coverage explicit |
| Mandatory check 2.5 (Error/rescue map) | **PASS** | `ao scope unfreeze` rescues I3 lock; `git revert` rescues I5 |

## Findings (4 — all fixable with small plan amendments)

### Finding 1: I5 propagation surface enumeration is incomplete

**Pattern:** Council FAIL #4 — Propagation Surface Blindness
**Severity:** Significant (blast radius high; detection difficulty low — fixable now)

The plan enumerates `cli/cmd/ao/*.go (~82 files)` and `hooks/*.sh (~10)` as the I5 refactor surface. The actual surface likely extends to:

- `cli/cmd/ao/*_test.go` — test fixtures often contain hardcoded `.agents/` paths
- `cli/embedded/` — agentops embeds hooks/skills into the binary; embedded copies need re-sync via `make sync-hooks` (per CLAUDE.md, line ~62)
- `skills/*/SKILL.md` and `skills-codex/*/SKILL.md` — many skill prompts cite `.agents/` paths in their examples
- `docs/` — narrative docs reference `.agents/` layout
- `scripts/*.sh` — release/validation scripts
- `tests/` — install-smoke and integration tests
- `lib/schemas/` — schema files may reference `.agents/` paths

**Required pseudocode amendment** (add to I5's Implementation section):

```bash
# I5 Step 0 — Pre-flight surface enumeration (mandatory before refactor)
echo "=== Surface enumeration baseline ==="
for surface in cli/cmd/ao cli/internal cli/embedded hooks lib scripts tests skills skills-codex docs; do
  count=$(grep -rln '"\.agents/' "$surface/" 2>/dev/null | wc -l)
  printf "%-20s  %d files\n" "$surface/" "$count"
done > .agents/research/2026-05-XX-i5-baseline.txt
cat .agents/research/2026-05-XX-i5-baseline.txt
# Refactor only proceeds AFTER this baseline is committed as artifact.
# scripts/check-paths-resolver-coverage.sh asserts the same surfaces post-refactor.
```

**Acceptance amendment for I5:**
- New: `cli/embedded/` is re-synced via `cd cli && make sync-hooks` after migration
- New: baseline file `.agents/research/2026-05-XX-i5-baseline.txt` committed alongside refactor for diff
- The "≥80% reduction" target now applies to the FULL surface, not just `cli/cmd/ao`

### Finding 2: I3 hook activation test is too weak

**Pattern:** Council FAIL #6 — Dead Infrastructure Activation
**Severity:** Significant (the whole point of I3 is the hook firing; if it doesn't fire post-install, value = 0)

I3's acceptance includes `bash -n hooks/edit-scope-guard.sh` (syntax check only) and the L2 spawn-2-workers scenario (which validates the Go subcommand and lock semantics, but not the hook installation path). Nothing verifies the hook ACTUALLY fires when registered in `hooks/hooks.json`.

**Required pseudocode amendment** (add to I3's Tests section):

```bash
# I3 L2 hook-activation test
# tests/hooks/test-edit-scope-guard-fires.sh
set -e
TEST_REPO=$(mktemp -d)
cp hooks/hooks.json "$TEST_REPO/hooks.json"
cp hooks/edit-scope-guard.sh "$TEST_REPO/edit-scope-guard.sh"
echo '{"frozen_dirs":["protected/"],"acquired_at":"...","acquired_by":"test"}' > "$TEST_REPO/.agents/scope.lock"

# Simulate Claude Code hook input for an Edit tool call to a frozen path
HOOK_INPUT='{"tool":{"name":"Edit","params":{"file_path":"protected/foo.go"}}}'
echo "$HOOK_INPUT" | bash "$TEST_REPO/edit-scope-guard.sh"
test $? -ne 0 || (echo "FAIL: hook did not block frozen-dir edit"; exit 1)

# And confirm allowed path passes
HOOK_INPUT='{"tool":{"name":"Edit","params":{"file_path":"unprotected/foo.go"}}}'
echo "$HOOK_INPUT" | bash "$TEST_REPO/edit-scope-guard.sh"
test $? -eq 0 || (echo "FAIL: hook blocked unprotected edit"; exit 1)
echo "PASS"
```

**Acceptance amendment for I3:**
- New: `tests/hooks/test-edit-scope-guard-fires.sh` exists and passes
- New: `bash tests/hooks/test-edit-scope-guard-fires.sh` is in the I3 conformance commands list

### Finding 3: I3 hook missing malformed-input defense

**Pattern:** Mandatory check 2.8 — Input validation for enum-like / structured fields
**Severity:** Informational (defensive; failure mode is "blocks all edits" if malformed input crashes hook)

`hooks/edit-scope-guard.sh` reads JSON tool input from stdin. If the input is malformed (jq fails, missing fields, etc.), the hook MUST NOT crash — that would block all edits. Plan doesn't specify defensive behavior.

**Required pseudocode amendment** (add to I3's hook script spec):

```bash
# hooks/edit-scope-guard.sh — top of script
# Defensive parse: if input is malformed, log warning and exit 0 (do not block).
HOOK_INPUT=$(cat)
if ! echo "$HOOK_INPUT" | jq -e . >/dev/null 2>&1; then
  echo "edit-scope-guard: malformed JSON input, allowing edit (fail-open)" >&2
  exit 0
fi
TARGET_PATH=$(echo "$HOOK_INPUT" | jq -r '.tool.params.file_path // .tool.params.command // empty')
if [ -z "$TARGET_PATH" ]; then
  exit 0   # nothing to check; allow
fi
# ... continue with scope-lock check
```

**Acceptance amendment for I3:**
- New: hook fail-open behavior on malformed input is tested (`echo "garbage" | hook` exits 0)

### Finding 4: I5 commit-per-package policy missing

**Pattern:** Bisectability hygiene (not in top-8, but observed in similar mass refactors)
**Severity:** Informational (operability; not blocking)

I5 is a single-worker mass refactor of ~82+ files. A single commit covering all of them is hard to bisect if a regression appears days later. The plan doesn't mandate an incremental commit cadence.

**Required pseudocode amendment** (add to I5's Implementation Notes):

```text
I5 commit cadence (mandatory):
  - 1 commit per package family (cli/cmd/ao, cli/internal, hooks, lib)
  - Each commit must independently pass `cd cli && go test ./...`
  - Final commit adds scripts/check-paths-resolver-coverage.sh, GOALS.md gate, and pattern doc
  - Total: ~5 commits, not 1
  - Rationale: enables `git bisect` if a path resolution regression surfaces post-merge
```

**Acceptance amendment for I5:**
- New: PR contains ≥4 commits, each individually green on `make test`

## Decision Gate

**Verdict:** WARN
**Action required before /crank:**
1. Apply 4 amendments above to plan + relevant bd issues (I3 and I5)
2. Update I3 conformance to include `tests/hooks/test-edit-scope-guard-fires.sh`
3. Update I5 conformance to include surface baseline + cli/embedded re-sync
4. (No bd issue re-creation needed; these are amendments to existing issue bodies)

**Risk acceptance path** (if amendments deferred):
- Defer Finding 4 (commit cadence) to operator discretion at execution time
- Defer Finding 3 (malformed input) to L2 test phase — implementer adds during write
- BLOCKING: Findings 1 + 2 must be addressed in plan before /crank

## Findings to persist (for `.agents/findings/registry.jsonl` after UU resolved)

Three findings emerge from this pre-mortem itself, additive to the 3 from /research:

1. `dedup_key: pre-mortem-pattern|propagation-surface-must-include-cli-embedded|mass-refactor` — When a Go-CLI mass refactor touches paths embedded into the binary (`cli/embedded/`), surface enumeration MUST include the embedded tree and acceptance MUST require `make sync-hooks` re-run. Otherwise the embedded copies drift silently.

2. `dedup_key: pre-mortem-pattern|hook-activation-test-must-verify-firing|hook-port` — Porting a hook from another tool must include an L2 test that verifies the hook FIRES post-install (simulate the harness's stdin contract; assert non-zero on block path AND zero on allow path). `bash -n` is insufficient — it only validates parse.

3. `dedup_key: pre-mortem-pattern|input-handling-fail-open-vs-fail-closed-must-be-explicit|hook-design` — Hooks reading structured input from stdin MUST declare fail-open vs fail-closed behavior on malformed input. Default fail-open for advisory hooks (don't block); fail-closed only for security-critical hooks with audit logging.

These will be appended to `.agents/findings/pending-2026-05-01-gstack-absorption.jsonl` (sidecar) so they merge into the registry once the UU state is resolved.

## Council FAIL Pattern Coverage Score

| # | Pattern | Plan Status |
|---|---|---|
| 1 | Missing mechanical verification (38%) | WARN — fix Finding 2 |
| 2 | Self-assessment instead of external gates (22%) | PASS |
| 3 | Context rot and hallucination (15%) | PASS |
| 4 | Propagation surface blindness (14%) | WARN — fix Finding 1 |
| 5 | Plan oscillation (11%) | PASS |
| 6 | Dead infrastructure activation (8%) | WARN — fix Finding 2 |
| 7 | Missing rollback/rescue map (6%) | PASS |
| 8 | Four-surface closure gap (5%) | PASS (minor: examples for /scope skill optional) |

## Notes

- This pre-mortem ran in `--quick` mode (single-agent inline review). Proportionate to standard-complexity 5-issue plan. No `--deep` council needed; the warnings are concrete and fixable.
- Sources cited: `cli/internal/llmwiki/scope_guard.go:76`, `cli/internal/storage/file.go:255`, `lib/hook-helpers.sh:84-88`, `hooks/hooks.json` schema, `scripts/audit-codex-parity.sh`, `scripts/sync-skill-counts.sh`, CLAUDE.md (sync-hooks instruction), GOALS.md Directive 1+2.
- Per pre-mortem-skill Step 4.6, the 4 pseudocode fixes above must be copied verbatim into bd issues `soc-irg1.3` (Findings 2+3) and `soc-irg1.5` (Findings 1+4) by the orchestrator before /crank.
