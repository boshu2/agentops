# AgentOps Pre-Reduction Snapshot

Snapshot time: 2026-06-07T06:16:56Z

This records the AgentOps baseline immediately before the lean `ao` reduction
work starts. The reduction itself is planned in [REDUCTION.md](REDUCTION.md);
this file is the reversible reference point for comparing what was moved,
archived, or kept.

## Baseline Tag

Tag:

```text
ao-pre-reduction@5b334a13f
```

Target commit:

```text
5b334a13f20b37405c5ee3d7fba64e66027ae7a2 docs(ao): categorize reduction surface (#811)
```

Create or verify the tag with:

```bash
git tag -l 'ao-pre-reduction@5b334a13f'
git rev-parse 'ao-pre-reduction@5b334a13f^{}'
```

Compare a future lean image against this baseline with:

```bash
git diff 'ao-pre-reduction@5b334a13f'..HEAD -- \
  REDUCTION.md \
  PRE-REDUCTION-SNAPSHOT.md \
  docs/reduction/ao-file-buckets.tsv \
  cli/cmd/ao \
  cli/docs/COMMANDS.md
```

## Inventory

| Surface | Count |
|---|---:|
| Tracked files | 5024 |
| `cli/` tracked files | 1378 |
| `cli/cmd/ao/*.go` files | 637 |
| `skills/**/SKILL.md` files | 173 |
| `skills-codex/**/SKILL.md` files | 173 |
| `cli/embedded/skills/**/SKILL.md` files | 1 |
| `docs/` tracked files | 424 |
| `scripts/` tracked files | 272 |

The source and Codex skill counts are equal at this baseline. The extra
top-level `skills/_fixtures` directory is a test fixture directory, not a
source skill because it does not contain a direct `SKILL.md`.

## Command Surface

Generated command reference:

```text
cli/docs/COMMANDS.md
```

Baseline command counts:

| Command surface | Count |
|---|---:|
| Top-level `ao` commands in generated docs | 84 |
| Subcommands in generated docs | 201 |

Top-level commands at the baseline:

```text
ao agent
ao agents
ao anti-patterns
ao autodev
ao badge
ao beads
ao canon
ao capabilities
ao chaos-test
ao ci
ao citation
ao claim
ao close
ao codex
ao compile
ao completion
ao config
ao constraint
ao contradict
ao corpus
ao council-gate
ao cron
ao curate
ao dedup
ao defrag
ao demo
ao doctor
ao eval
ao extract
ao feedback-loop
ao findings
ao flywheel
ao forge
ao gate
ao goals
ao guard-status
ao handoff
ao harness
ao harvest
ao help
ao init
ao inject
ao install-guards
ao knowledge
ao lookup
ao loop
ao maturity
ao mcp
ao memory
ao metrics
ao mind
ao mine
ao next-work
ao notebook
ao operator
ao orchestrate
ao patterns
ao pool
ao provenance
ao quick-start
ao ratchet
ao ready
ao reconcile
ao redact
ao registry
ao retrieval-bench
ao robot-docs
ao rpi
ao scenario
ao scope
ao search
ao seed
ao session
ao sessions
ao skills
ao status
ao tick
ao trace
ao turn
ao validate
ao verdict-gate
ao version
ao vibe-check
ao wiki
```

## Reduction Census

The authoritative per-file reduction census is
[docs/reduction/ao-file-buckets.tsv](docs/reduction/ao-file-buckets.tsv).
It has one row per current `cli/cmd/ao/*.go` file.

| Bucket | Files | Meaning |
|---|---:|---|
| KEEP | 450 | Belongs in the lean local AO image. |
| RELOCATE | 146 | Belongs at the MTO/factory or image-adapter boundary. |
| ARCHIVE | 2 | Legacy or deprecated surface to remove from the lean image after callers are migrated. |
| RESEARCH | 39 | Ownership unclear; inspect before moving or cutting. |
| Total | 637 | Matches the `cli/cmd/ao/*.go` baseline count. |

The archive set is intentionally narrow:

```text
cli/cmd/ao/pool_migrate_legacy.go
cli/cmd/ao/pool_migrate_legacy_test.go
```

## Reproduction Commands

Run from the repository root at the target commit:

```bash
git rev-parse HEAD
git ls-files | wc -l
git ls-files cli | wc -l
find cli/cmd/ao -maxdepth 1 -name '*.go' | wc -l
find skills -mindepth 2 -maxdepth 2 -name SKILL.md | wc -l
find skills-codex -mindepth 2 -maxdepth 2 -name SKILL.md | wc -l
git ls-files docs | wc -l
git ls-files scripts | wc -l
rg '^### `ao ' cli/docs/COMMANDS.md | wc -l
rg '^#### `ao ' cli/docs/COMMANDS.md | wc -l
awk -F '\t' 'NR>1 {count[$2]++} END {for (k in count) print k, count[k]}' \
  docs/reduction/ao-file-buckets.tsv | sort
test "$(( $(wc -l < docs/reduction/ao-file-buckets.tsv) - 1 ))" -eq \
  "$(find cli/cmd/ao -maxdepth 1 -name '*.go' | wc -l)"
```
