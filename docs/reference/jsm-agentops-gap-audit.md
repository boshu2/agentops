# JSM-Informed AgentOps Skill Gap Audit

This audit compares current AgentOps skill structure to the JSM-style package patterns captured on 2026-05-16. It is structural only and does not import JSM skill content.

## Baseline

Command snapshot:

```bash
find skills -mindepth 1 -maxdepth 1 -type d
find skills -path '*/SELF-TEST.md' -type f
find skills -mindepth 2 -maxdepth 2 -type d -name assets
find skills -mindepth 2 -maxdepth 2 -type d -name subagents
find skills -path '*/scripts/*' -type f -perm -111
```

Observed AgentOps state:

| Measure | Value |
|---|---:|
| Skill directories | 77 |
| Skills with `SELF-TEST.md` | 1 |
| Skills with `assets/` | 1 |
| Skills with `subagents/` | 0 |
| Skills with `scripts/` | 71 |
| Skills with `references/` | 64 |
| Executable files under `*/scripts/*` | 111 |
| Skills over 50 files | 0 |
| Total files under `skills/` | 643 |

## Gap Summary

| Gap | Impact | Recommendation |
|---|---|---|
| Minimal `SELF-TEST.md` coverage | weak trigger and behavior proof for most market-facing skills | continue adding self-tests to pilot skills |
| Almost no `assets/` usage | templates and payloads may bloat prompts or stay implicit | introduce assets only for reusable templates and report skeletons |
| No `subagents/` usage | broad orchestration skills lack explicit role packets | add only for high-complexity delegation skills |
| Executable repo scripts | good for repo-runtime, incompatible with JSM export validator | use temporary export copy with mode normalization |
| No export validator | no repeatable JSM package-readiness proof | use `scripts/check-jsm-export.sh` |
| Older absorption matrix | covers 45-skill history, not full current corpus | keep as historical and use current standards snapshot |

## Structural Strengths

- AgentOps already uses `references/` heavily.
- Most skills are compact enough for package-clean file-count limits.
- Existing repo gates cover frontmatter, docs, skill integrity, and Codex artifact parity.
- Skills already have scripts for repo-native validation and operational workflows.

## Priority Upgrade Queue

| Skill | Why first | Suggested upgrade |
|---|---|---|
| `standards` | central quality contract | first self-test added; next pass should tighten trigger description and decide export posture |
| `research` | used for corpus analysis and discovery | add self-test around prior-art lookup and output contract |
| `plan` | creates issue-ready decomposition | add self-test for baseline audit and mechanical checks |
| `validation` | closes implementation work | add self-test for four-surface closure and budget guards |
| `reverse-engineer-rpi` | directly aligned with product reverse-engineering | add assets/report template and self-test |

## Deferred Gaps

- Broad `subagents/` adoption should wait until a specific orchestration skill needs stable role packets.
- Mega-skill packaging is not urgent because no AgentOps skill currently exceeds 50 files.
- CASS-based mining is deferred because `jsm cass status` reports unavailable locally.

## Verification

Use:

```bash
scripts/inventory-jsm-skills.sh --root skills --json
scripts/check-jsm-export.sh --json skills/standards
bash skills/heal-skill/scripts/heal.sh --strict
bash scripts/validate-skill-frontmatter.sh --strict
bash tests/docs/validate-doc-release.sh
```

Observed export smoke:

| Skill | Classification | Notes |
|---|---|---|
| `skills/quickstart` | `package_clean` | temporary export copy validated successfully |
| `skills/standards` | `validation_failed` | JSM validator flags protected-methodology and possible-secret warnings; keep as repo-runtime until explicitly reviewed for export |

---

**Source:** Pattern-only comparison of AgentOps skill structure with the user-local JSM corpus. No proprietary source text copied.
