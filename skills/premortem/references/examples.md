# Premortem Examples

> Extracted from premortem/SKILL.md on 2026-04-11.

## Validate a Plan (Default — Inline)

**User says:** `/premortem .agents/plans/2026-02-05-auth-system.md`

**What happens:**

1. Agent reads the auth system plan
2. Runs `/council --quick validate <plan-path>` (inline, no spawning)
3. Single-agent structured review finds missing error handling for token expiry
4. Council verdict: WARN
5. Output written to `.agents/council/2026-02-13-premortem-auth-system.md`

**Result:** Fast premortem report with actionable concerns. Use `--deep` for high-stakes plans needing multi-judge consensus.

## Cross-Vendor Plan Validation

**User says:** `/premortem --mixed .agents/plans/2026-02-05-auth-system.md`

**What happens:**

1. Agent runs mixed-vendor council (3 Claude + 3 Codex)
2. Cross-vendor perspectives catch platform-specific issues
3. Verdict: PASS with 2 warnings

**Result:** Higher confidence from cross-vendor validation before committing resources.

## Auto-Find Recent Plan

**User says:** `/premortem`

**What happens:**

1. Agent scans `.agents/plans/` for most recent plan
2. Finds `2026-02-13-add-caching-layer.md`
3. Runs inline council validation (no spawning, ~10% of full council cost)
4. Records ratchet progress

**Result:** Frictionless validation of most recent planning work.

## Deep Review for High-Stakes Plan

**User says:** `/premortem --deep .agents/plans/2026-02-05-migration-plan.md`

**What happens:**

1. Agent reads the migration plan
2. Searches knowledge flywheel for prior migration learnings
3. Checks PRODUCT.md for product context
4. Runs `/council --deep --preset=plan-review validate <plan-path>` (4 judges)
5. Council verdict with multi-perspective consensus

**Result:** Thorough multi-judge review for plans where the stakes justify spawning agents.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Council times out | Plan too large or complex for judges to review in allocated time | Split plan into smaller epics or increase timeout via council config |
| FAIL verdict on valid plan | Judges misunderstand domain-specific constraints | Add context via `--perspectives-file` with domain explanations |
| Product perspectives missing | PRODUCT.md exists but not included in council packet | Verify PRODUCT.md is in project root and no explicit `--preset` override was passed |
| Premortem gate blocks /crank | Epic has 3+ issues and no premortem ran | Run `/premortem` before `/crank`, or use `--skip-premortem` flag (not recommended) |
| Spec-completeness judge warns | Plan lacks Boundaries or Conformance Checks sections | Add SDD sections or accept WARN (backward compatibility — not a failure) |
| Mandatory for epics enforcement | Hook blocks /crank on 3+ issue epic without premortem | Run `/premortem` first, or set `AGENTOPS_SKIP_PREMORTEM_GATE=1` to bypass |
