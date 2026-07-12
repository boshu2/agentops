---
name: release
description: "Run release readiness validation."
---
# Release Skill

> **Purpose:** Take a project from "code is ready" to "tagged, pushed by the operator, and verified green on the exact tagged SHA."

Pre-flight validation, changelog from git history, version bumps across package files, release commit, annotated tag, curated release notes, and post-push exact-SHA CI verification. Local preparation is reversible. Publishing (including the GitHub Release page) is CI's job.

## Constraints

- **Keep preparation reversible and publication operator-owned.** This skill may create local release artifacts, a commit, and an annotated tag, but it never pushes, publishes, or triggers CI because those actions cross the reversible local boundary.
- **Require deterministic pre-flight.** Run the full release gate for the confirmed version and treat `--skip-checks` as an explicit degraded operator choice because an untested tag cannot become a trustworthy release boundary.
- **Bind completion to the tagged commit.** Record the tag SHA, exact-SHA CI run, and reconciliation result because a green branch run or an unverified tag does not prove the released artifact.
- **Keep release claims evidence-backed.** Derive changelog, notes, version choice, and audit only from the selected git range and generated artifacts because invented or copied-forward claims corrupt both audiences.
- **Consult the pawl before raising the andon.** WARN, FAIL, or REFUTED release evidence repairs and reruns automatically because ordinary rejection identifies incomplete preparation; only a real publication breaker may enter HOLD or consume the helper lane.

## Breaker State Machine

- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** repair the owned release artifact or route the defect back to its producing bead, then rerun pre-flight and pawl; plain rejection never enters HOLD and never consumes the helper lane.
- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** freeze tag or publication guidance when an irreversible remote action, ambiguous artifact identity, or unavailable release authority prevents safe progress, then route exactly one bounded helper consultation with the audit packet.
- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, apply the bounded recovery, and re-earn local validation, exact-SHA evidence, reconciliation, and the pawl verdict.
- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and send the helper-provided release evidence to the operator.
- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** skip the helper and route directly to the operator; these are the only direct-human states.

---

## Quick Start

```bash
$release 1.7.0                # full release: changelog + bump + commit + tag
$release 1.7.0 --dry-run      # show what would happen, change nothing
$release --check               # readiness validation only (GO/NO-GO)
$release                       # suggest version from commit analysis
```

---

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `version` | No | Semver string (e.g., `1.7.0`). If omitted, suggest based on commit analysis |
| `--check` | No | Readiness validation only — don't generate or write anything |
| `--dry-run` | No | Show generated changelog + version bumps without writing |
| `--skip-checks` | No | Skip pre-flight validation (tests, lint) |
| `--changelog-only` | No | Only update CHANGELOG.md — no version bumps, no commit, no tag |

---

## Modes

| Mode | Invocation | Behavior |
|---|---|---|
| **Full Release** | `$release [version]` | Pre-flight → changelog → release notes → version bump → user review → write → release commit → tag → push guidance → exact-SHA CI verification. |
| **Check** | `$release --check` | Pre-flight checks only; reports GO/NO-GO. Composable with `$validate`. No writes. |
| **Changelog Only** | `$release X.Y.Z --changelog-only` | Updates `CHANGELOG.md` only — no version bumps, no commit, no tag. |

---

## Workflow

**Read [references/release-workflow-detail.md](references/release-workflow-detail.md) for the full per-step procedure** — bash commands, check tables, expected output, audit-record template, and worked examples. The index below is for orientation only; the agent must execute against `release-workflow-detail.md` for correctness.

1. **Pre-flight** — run `scripts/ci-local-release.sh` (blocking) plus version/lint/test/branch/changelog/SBOM/security checks. `--check` mode stops after this step.
2. **Determine range** — `<last-tag>..HEAD`. For non-HEAD cuts, see [references/release-cut-and-bump.md](references/release-cut-and-bump.md).
3. **Read git history** — `git log --oneline --no-merges <range>` plus stats for ambiguity resolution.
4. **Classify and group** — Added / Changed / Fixed / Removed for `CHANGELOG.md`. Notes prose uses the richer 8-label set per [references/release-notes.md](references/release-notes.md).
5. **Suggest version** — major if breaking, minor if features, patch if only fixes. Confirm with the user before proceeding.
6. **Generate changelog entry** — Keep-a-Changelog format, today's date, style-matched to the most recent existing entry.
7. **Detect and offer version bumps** — generic patterns (`package.json`, `pyproject.toml`, etc.) plus AgentOps-specific manifests per [references/release-cut-and-bump.md](references/release-cut-and-bump.md).
8. **User review** — show generated changelog and version diffs; ask the user to proceed. `--dry-run` stops here.
9. **Write changes** — `CHANGELOG.md` update + version file edits.
10. **Generate release notes** — curated `docs/releases/YYYY-MM-DD-v<version>-notes.md` per [references/release-notes.md](references/release-notes.md). MUST be staged before the release commit.
11. **Write audit trail** — `docs/releases/YYYY-MM-DD-v<version>-audit.md` resolved via `scripts/resolve-release-artifacts.sh`. Format in workflow-detail Step 16.
12. **Release commit** — `git commit -m "Release v<version>"` with all release artifacts staged.
13. **Tag** — annotated `git tag -a v<version> -m "Release v<version>"`.
14. **GitHub Release (CI handles this)** — do NOT `gh release create` locally; GoReleaser is sole creator.
15. **Post-push exact-SHA CI and reconciliation verification** — after the operator pushes, require both `scripts/verify-release-ci.sh v<version>` to print `GO release-ci` and `ao reconcile --json | bash skills/release/scripts/validate-reconcile.sh "v<version>"` to pass for that exact expected tag; exact-SHA CI alone never authorizes closeout.
16. **Post-release guidance** — show push commands and the verification command; do NOT push.
17. **Audit trail format** — see workflow-detail for the markdown template.

**Checkpoint:** before writing, confirm the operator approved the displayed changelog and version diff; before handoff, prove the release commit, annotated tag, audit, and notes agree on the version; after push, require exact-SHA CI plus reconciliation before declaring complete.

---

## Boundaries

### What this skill does

- Pre-flight validation (tests, lint, clean tree, versions, branch)
- Changelog generation from git history
- Semver suggestion from commit classification
- Version string bumps in package files
- Release commit + annotated tag
- Release notes (highlights + changelog) for GitHub Release page
- Curated release notes for CI to publish on GitHub Release page
- Post-release guidance plus exact-SHA CI verification instructions
- Audit trail

### What this skill does NOT do

- **No publishing** — no `npm publish`, `cargo publish`, `twine upload`. CI handles this.
- **No building** — no `go build`, `npm pack`, `docker build`. CI handles this.
- **No pushing** — no `git push`, no `git push --tags`. The user decides when to push.
- **No CI triggering** — the tag push (done by the user) triggers CI.
- **No monorepo multi-version** — one version, one changelog, one tag. Scope for v2.

Everything this skill does is local and reversible:
- Bad changelog → edit the file
- Wrong version bump → `git reset HEAD~1`
- Bad tag → `git tag -d v<version>`
- Bad release notes → edit `docs/releases/*-notes.md` before push

---

## Universal Rules

- **Don't invent** — only document what git log shows
- **No commit hashes** in the final output
- **No author names** in the final output
- **Concise** — one sentence per bullet, technical but readable
- **Adapt, don't impose** — match the project's existing style rather than forcing a particular format
- **User confirms** — never write without showing the draft first
- **Local only** — never push, publish, or trigger remote actions
- **Not done at tag** — after the user pushes, verify a green `validate.yml` run for the exact tagged SHA and record the run id plus conclusion in the handoff or release audit notes.
- **Two audiences** — CHANGELOG.md is for contributors (file paths, issue IDs, implementation detail). Release notes are for feed readers (plain English, user-visible impact, no insider jargon). Never copy-paste the changelog into the release notes.

---

## Output Specification

- **Path:** `docs/releases/YYYY-MM-DD-v<version>-audit.md`, paired with `docs/releases/YYYY-MM-DD-v<version>-notes.md` and annotated ref `refs/tags/v<version>`.
- **Filename convention:** `YYYY-MM-DD-v<version>-audit.md` and `YYYY-MM-DD-v<version>-notes.md`, where `<version>` is the confirmed SemVer without a duplicate `v` prefix.
- **Serialization/schema format:** Markdown audit with release heading, date, previous tag, commit count, local-CI artifact path, version bumps, pre-flight results, and remote CI verdict; the annotated Git tag targets the release commit, and the verdict records exact SHA, run id, status, and conclusion.
- **Validator command:** set `VERSION="<version>"`, then run `bash scripts/validate-release-audit-artifacts.sh --mode target --target-release "$VERSION" && bash scripts/verify-release-ci.sh "v$VERSION" && ao reconcile --json | bash skills/release/scripts/validate-reconcile.sh "v$VERSION"`; command success alone is insufficient because the reconciliation JSON must name that exact expected tag, be semantically green, and contain no medium/high release finding.
- **Downstream handoff:** before push, send the operator the release commit SHA, annotated tag SHA, audit/notes paths, rollback commands, and exact push/verification commands; after push, append the exact-SHA CI and reconciliation evidence before closeout.

## Quality Checklist

- Changelog and notes contain only changes supported by the selected git range, use the repository style, and serve their distinct contributor and feed-reader audiences.
- Version files, release commit, annotated tag, audit, and notes all name the same confirmed SemVer and resolve to one release boundary.
- Full local release validation and artifact checks pass before handoff; exact-tag CI and reconciliation pass after the operator pushes.
- Ordinary rejection remains in AUTO-REDO; HOLD has exactly one helper, and operator escalation is limited to the declared human states.

---

## Examples

**User says:** `$release 1.7.0`
Agent runs pre-flight → reads `v1.6.0..HEAD` git history → classifies commits → drafts CHANGELOG.md entry + curated release notes → detects version files (package.json, version.go, plugin manifests) → presents draft for review → on approval, writes files, creates release commit, creates annotated tag, prints push guidance, then after the user pushes verifies `scripts/verify-release-ci.sh v1.7.0` and records the run id/conclusion.

**User says:** `$release --check`
Agent runs all pre-flight checks and outputs a GO/NO-GO summary table. No writes.

**User says:** `$release` (no version)
Agent classifies commits and suggests a version (major if breaking, minor if features, patch if fixes only) with reasoning, then asks the user to confirm or override.

**User says:** `$release 1.7.0 --dry-run`
Agent shows what the changelog entry + version bumps would look like, then stops without writing.

See `references/release-workflow-detail.md` for the full per-step example narration.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| "No commits since last tag" error | Working tree clean, no new commits | Commit pending changes or skip release |
| Version mismatch warning | `package.json` and `go` version disagree | Manually sync before release, or pick one as source of truth |
| Tests fail during pre-flight | Breaking change not caught earlier | Fix tests, or use `--skip-checks` (not recommended) |
| Dirty working tree warning | Uncommitted changes present | Commit or stash before release |
| GitHub Release page body is empty | GoReleaser conflict with existing draft | CI deletes existing releases before GoReleaser runs; do NOT `gh release create` locally |
| `ci-local-release.sh` hangs on agents-hash | `~/.agents/patterns` is large | Set `AGENTS_HUB_OVERRIDE=/tmp/empty-hub` before invocation |

See `references/release-workflow-detail.md` for the full troubleshooting matrix.

## See Also

- [security](../security/SKILL.md) — Dependency audit and vulnerability scanning (absorbs deps)

When wiring or auditing the CI workflow that backs `--check` mode (or the tag-triggered release pipeline that consumes the curated notes), pull the relevant patterns from `references/gh-actions-ci-patterns.md` (general CI) or `references/gh-actions-release-automation.md` (tag-triggered, draft flow, asset upload). When generating the curated release-notes file or auditing CHANGELOG.md drift, treat the changelog as an orientation layer and use `references/changelog-as-research-artifact.md` for the structured-section, breaking-change-callout, and notes-vs-changelog rules.

For release-prep sessions that span package registries, deploy hosts, multi-repo sync, or platform-specific publishers, use `references/release-preflight-and-publishers.md` to separate local readiness from remote publishing and to preserve rollback evidence.

## Reference Documents

- [references/release-workflow-detail.md](references/release-workflow-detail.md) — full per-step procedure
- [references/release-cut-and-bump.md](references/release-cut-and-bump.md) — non-HEAD cut + AgentOps-specific bump targets
- [references/release-notes.md](references/release-notes.md) — curated notes format + product-area taxonomy
- [references/release-preflight-and-publishers.md](references/release-preflight-and-publishers.md)
- [references/release-cadence.md](references/release-cadence.md)
- [references/changelog-as-research-artifact.md](references/changelog-as-research-artifact.md)
- [references/gh-actions-ci-patterns.md](references/gh-actions-ci-patterns.md)
- [references/gh-actions-release-automation.md](references/gh-actions-release-automation.md)

## Local Resources

### references/

- [references/release-cadence.md](references/release-cadence.md)
- [references/release-notes.md](references/release-notes.md)
- [references/release-preflight-and-publishers.md](references/release-preflight-and-publishers.md)
- [references/release-cut-and-bump.md](references/release-cut-and-bump.md)
- [references/release-workflow-detail.md](references/release-workflow-detail.md)
- [references/gh-actions-ci-patterns.md](references/gh-actions-ci-patterns.md)
- [references/gh-actions-release-automation.md](references/gh-actions-release-automation.md)
- [references/changelog-as-research-artifact.md](references/changelog-as-research-artifact.md)

### scripts/

- `scripts/validate.sh`
