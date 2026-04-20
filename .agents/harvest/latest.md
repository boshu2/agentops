---
title: Cross-Disk .agents/ Harvest
generated: 2026-04-19
scope: WSL + /mnt/c + /mnt/d
mode: read-only inventory + content-hash dedup
---

# Cross-Disk .agents/ Harvest — 2026-04-19

Walked 92 `.agents/` directories across WSL home, `/mnt/c`, and `/mnt/d`.
Content-hashed every `.md` in `learnings/`, `patterns/`, `research/`,
`findings/`, `retros/`, `session-handoffs/` (normalized: frontmatter stripped,
lowercased, whitespace collapsed, SHA-256).

## Totals

| Metric | Value |
|---|---|
| Rigs scanned | 92 |
| Artifact files | 1,642 |
| Unique content hashes | 968 (59%) |
| Duplicate clusters | 324 (spanning 998 files, 41%) |
| Walk duration | 5.1s |

## By tier

| Tier | Rigs | Files | Learnings | Patterns | Research | Findings | Retros |
|---|---|---|---|---|---|---|---|
| `wsl_live` | 39 | 971 | 425 | 27 | 433 | 35 | 51 |
| `D_archive` | 43 | 532 | 191 | 18 | 272 | 20 | 31 |
| `wsl_home_hub` (~/.agents) | 1 | 123 | 96 | 12 | 15 | 0 | 0 |
| `cache` (plugin marketplace) | 2 | 30 | 13 | 6 | 9 | 2 | 0 |
| `C_home` (/mnt/c) | 6 | 1 | 1 | 0 | 0 | 0 | 0 |
| `wsl_backup` | 1 | 0 | — | — | — | — | — |

C: is effectively empty — just a stub `.agents/` with one learning, plus
plugin caches and temp sync garbage. The real work lives in WSL; D: is
cold-storage of the pre-consolidation Mac state.

## Top rigs by artifact count

| # | Tier | Files | Rig |
|---|---|---|---|
| 1 | wsl_live | 360 | `gt/agentops-mac/crew/nami` |
| 2 | D_archive | 280 | `mac-extract/gt/platform-lab` |
| 3 | wsl_live | 161 | `dev/personal/platform-lab` |
| 4 | wsl_live | 161 | `gt/platform-lab` |
| 5 | wsl_home_hub | 123 | `~` (global hub) |
| 6 | wsl_live | 104 | `gt/` (rig-root) |
| 7 | D_archive | 96 | `mac-extract/gt/` |
| 8 | D_archive | 68 | `mac-extract/gt/mom` |
| 9 | wsl_live | 37 | `dev/personal/agentops` |
| 10 | wsl_live | 33 | `gt/olympus-mac/crew/vegeta` |

## Notable findings

### 1. Two live platform-lab checkouts hold identical content
`/home/boful/dev/personal/platform-lab/.agents` (inode 840552) and
`/home/boful/gt/platform-lab/.agents` (inode 148757) both hold 161 files
with identical distributions. Different inodes — not a symlink or
bind mount. One of them is stale duplication. Pick a canonical home
and delete the other.

### 2. Mac resource-fork pollution on D:
Two "duplicate clusters" dominate the dedup output: a 165-copy cluster
and a 47-copy cluster, both composed entirely of macOS AppleDouble
resource-fork files (`._*.md`) under
`/mnt/d/archive/mac-extract/gt/12factor/.agents/`. These are Finder
metadata artifacts, not real content — they all normalize to the same
(essentially empty) body. One `find -name "._*" -delete` on the
mac-extract tree would drop ~200 phantom files from the catalog.

### 3. Genuine cross-tier duplication: 22 clusters
22 content clusters span both `wsl_live` and `D_archive` — real
learnings that were restored from the Mac archive into live WSL trees.
Mostly concentrated in `platform-lab` and `~/.agents` (4-copy clusters
visible there, meaning the same file lives in the home hub, a live
rig, and one or more archive locations).

### 4. Global hub is already curated
`~/.agents` (the authored vault root) holds 123 files — almost entirely
learnings (96) + patterns (12). That's the promotion target; nothing
needs to be promoted *into* the hub from the harvest because hub
content is largely distinct from rig content (low overlap with rigs
except the 22 cross-tier clusters noted above).

## Suggested next steps (not executed)

| Action | Scope | Risk |
|---|---|---|
| Delete AppleDouble files: `find /mnt/d/archive/mac-extract -name "._*.md" -delete` | D: archive hygiene | Low — metadata only |
| Consolidate platform-lab into one checkout (delete the other's `.agents/`) | wsl_live | Medium — verify git state first |
| `ao dedup --merge` inside `~/.agents` and major rigs | single-rig, in-place | Low (archives to `.agents/archive/dedup/`) |
| Promote remaining unique high-utility cross-rig patterns to `~/.agents/learnings/` | Manual curation — no `ao harvest` exists | Medium — needs review |

## Catalog artifacts

- `.agents/harvest/latest.json` — full machine-readable catalog
- `.agents/harvest/cross-disk-2026-04-19T19-52-54.json` — timestamped copy
- `.agents/harvest/latest.md` — this report

## Skill drift

`/harvest` expects `ao harvest --roots ...` but v2.37.2 ships no
`harvest` subcommand. This report was produced by a one-off Python
walker at `/tmp/harvest_all.py`. Recommend filing a bead either to
(a) implement `ao harvest` per the skill contract, or (b) update the
harvest skill to compose existing primitives (`ao mine` + `ao dedup`
in a cross-rig loop).
