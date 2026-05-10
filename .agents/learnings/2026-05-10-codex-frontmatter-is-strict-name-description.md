---
id: learning-2026-05-10-codex-frontmatter-is-strict-name-description
type: learning
date: 2026-05-10
category: architecture
confidence: high
maturity: provisional
utility: 0.7
---

# Learning: Codex twin SKILL.md frontmatter is strictly `name` + `description`

## What We Learned

`skills-codex/*/SKILL.md` files are enforced to have ONLY two frontmatter keys: `name` and `description`. Adding any other key (e.g., `practices:`, `metadata:`) fails `scripts/validate-codex-generated-artifacts.sh` with `<skill> has non-Codex frontmatter fields: <key>`. The check is regex-driven on the frontmatter block, not config-driven.

## Why It Matters

When extending Claude-side skill metadata (e.g., the practice-citation gate), the new field stays Claude-side only. Mirroring it to codex twins will break the bundle validator and the pre-push gate. The correct pattern: edit `skills/X/SKILL.md`, then run `scripts/regen-codex-hashes.sh` (which updates `skills-codex/X/.agentops-generated.json` markers WITHOUT touching the codex SKILL.md content). The marker's `source_hash` records that Claude-side content drifted; codex content stays stable.

## Source

soc-hdot pass-1 backfill (2026-05-10). Initially mirrored practices: into all 13 codex twins; pre-push gate's `agentops-core.distribution-install-update` canary failed with 13 `non-Codex frontmatter fields: practices` errors. Reverted codex twins; re-ran regen; gate passed.
