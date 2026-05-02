---
id: vibe-2026-05-01-soc-irg1
type: vibe
date: 2026-05-01
mode: quick
target: recent (10 commits, 4cb8b8a5..882f87a2)
epic: soc-irg1
---

# Vibe: soc-irg1 gstack absorption Tier 1

## Council Verdict: PASS

**Recommendation:** Ship as-is. All 6 specific quality concerns flagged for evaluation pass. Mechanical gates all green. Test coverage shape is healthy. The 3 complexity hotspots in `cli/internal/skillshealth/audit.go` are over the warn threshold (15) but well under the fail threshold (25); they are dense parser/comparator functions where the complexity is intrinsic to the work. No refactor required.

## Scope

- Commits: 10 (4cb8b8a5..882f87a2)
- Files changed (excluding .agents/ artifacts): 66 source files, +6536/-71 LOC (artifact volume dominated by `.agents/research/gstack/contracts/repo-contract.json` from the discovery commit)
- Languages: Go, shell, markdown, JSON, YAML
- Mode: --quick (single-agent inline structured review)

## Complexity Hotspots (gocyclo -over 10)

| File:Line | Function | Score | Verdict |
|---|---|---:|---|
| `cli/internal/skillshealth/audit.go:314` | `descriptionsClose` | 17 | OK (B) — token-overlap matcher; complexity is intrinsic |
| `cli/internal/skillshealth/audit.go:221` | `findBrokenRefs` | 17 | OK (B) — reference link validator; intrinsic |
| `cli/internal/skillshealth/audit.go:145` | `ParseFrontmatter` | 16 | OK (B) — YAML frontmatter parser; intrinsic |
| `cli/cmd/ao/skills.go:51` | `runSkillsCheck` | 13 | OK (B) — RunE dispatcher |
| `cli/internal/skillshealth/audit.go:50` | `Audit` | 13 | OK (B) — orchestrator |
| `cli/internal/scope/scope.go:71` | `Write` | 11 | OK (B) — has 2 fallback branches by design |

All under 25 (fail threshold per CLAUDE.md). Three at 16-17 are above 15 (warn threshold) but the agentops policy is "warn at 15, fail at 25" — these warn-level scores are acceptable for parser/comparator functions.

## Test Pyramid

| Level | Count | Files |
|---|---:|---|
| L0 (compile/lint/parity) | 3 | go vet, audit-codex-parity, build |
| L1 (unit, table-driven) | 19 | paths_test, scope_test, skillshealth/audit_test (frontmatter parser, parity comparator) |
| L2 (integration) | 7-8 | TestShellGoAgreement (3 subtests, cross-language), TestWrite_AtomicityUnderConcurrency (100 goroutines), TestSkillsCheck_JSONOutputSchema, TestSkillsCheck_StrictExitsNonZeroOnMissingFrontmatter, TestScopeFreezeThenStatus_NonJSON, TestScopeStatusJSON_EmptyLock, L2 audit against real skills tree |
| L3 (e2e) | 0 | n/a — no e2e surface in scope |
| L4 (fresh-context smoke) | 1 | tests/hooks/test-edit-scope-guard-fires.sh (7-case behavioral) |
| **Weighted score** | **0.33** | (3+19+24+0+5)/155 — just above the 0.3 PASS threshold |

`satisfaction_score: 0.33` (source: test-pyramid-weighted)

Healthy shape. The L4 hook-fires test is particularly strong — it simulates Claude Code's actual stdin contract.

## Specific Concerns Evaluated

### 1. Atomic-write reuse — ✓ PASS
`cli/internal/scope/scope.go:71` (`Write`) calls `llmwiki.SafeAtomicWrite` first; on `*llmwiki.WriteScopeError` (lock lives outside vault allowlist), falls back to `llmwiki.AtomicWriteFile` — the underlying primitive with same temp+rename guarantees. Code comment explicitly explains why the fallback is safe. **No new locking semantics.** Pre-mortem hand-off requirement satisfied. The `TestWrite_AtomicityUnderConcurrency` 100-goroutine test verifies torn-write impossibility.

### 2. Hook fail-open — ✓ PASS
`hooks/edit-scope-guard.sh:18-27` reads stdin, runs `jq -e .` to validate JSON; on parse failure logs warning to stderr and `exit 0` (fail-open). Empty target path also `exit 0`. Verbatim per pre-mortem Finding 3. Behavior verified by `tests/hooks/test-edit-scope-guard-fires.sh` case 3 (PASS).

### 3. Codex parity discipline — ✓ PASS
`/scope` skill ships in both `skills/scope/` AND `skills-codex/scope/` (with `prompt.md` + `references/lock-file-format.md`). `bash scripts/audit-codex-parity.sh --skill scope` PASS. Full audit also PASS.

### 4. Cobra registration — ✓ PASS
`cli/cmd/ao/cobra_commands_test.go` updated in two places (lines 422 + 482) to include `"scope"` and `"skills"`. `TestCobraCommandTreeRegistration` and `TestCobraExpectedCmdsMatchRegistration` both pass. `cli/docs/COMMANDS.md` regenerated to document the new commands.

### 5. Embedded sync — ✓ PASS
`cli/embedded/hooks/` and `cli/embedded/lib/` re-synced via `make sync-hooks` after I3 added the new hook AND after I5 migrated path-computing hooks. The post-Wave-1 commit (`4591da66`) explicitly handles the I3 sync; the I5 commits handle the migration sync (visible as 6 file changes in `cli/embedded/hooks/` in the diff stats).

### 6. Pre-existing broken-refs finding — ✓ correctly classified
`ao skills check` (I2) surfaces 6 broken references to `references/strict-delegation-contract.md` in `discovery/rpi/validation` SKILLs. These reference paths predate this epic — the new audit catches them on first run. **NOT a regression introduced by this epic.** Real bugs worth filing as a follow-up bd issue (`/discovery`, `/rpi`, `/validation` skills currently link to a missing file). Recommend filing under epic soc-irg1 as low-priority follow-up OR into a separate hygiene epic.

## Findings

| Severity | Finding | File | Action |
|---|---|---|---|
| informational | 3 functions at complexity 16-17 in `cli/internal/skillshealth/audit.go` | audit.go:145, 221, 314 | No action; intrinsic complexity for parser/comparator work, well under fail threshold |
| informational | 6 broken `references/strict-delegation-contract.md` links in discovery/rpi/validation SKILLs | skills/{discovery,rpi,validation}/SKILL.md | File follow-up bd issue. NOT this epic's bug. |
| informational | I5 metric script counts per-line occurrences (not per-file) | scripts/check-paths-resolver-coverage.sh | Already documented in I5 worker report; gives finer-grained signal than per-file count. Acceptable deviation. |

No critical, no significant findings.

## Recommendation

**PASS — ship as-is.** No regressions, all pre-mortem amendments applied, atomic-write reuse correct, hook fail-open correct, codex parity discipline held, embedded sync done. The follow-up bd issue for the 6 broken refs is operator discretion (separate hygiene work, not blocking).

## Suggested follow-up

```bash
bd create --title "Fix broken references/strict-delegation-contract.md links in /discovery, /rpi, /validation SKILLs" \
  --description "ao skills check (introduced by soc-irg1.2) surfaces 6 broken refs. Either create the missing reference file or remove the dead links. Not a regression." \
  --type bug --priority 3 --parent soc-irg1
```

(Operator may run this if desired; not blocking ship.)

## Decision

- [x] PASS — proceed to /post-mortem
- [ ] WARN — review concerns first
- [ ] FAIL — fix and re-run
