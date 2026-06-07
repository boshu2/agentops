---
name: ripgrep-search-discipline
user-invocable: false
skill_api_version: 1
metadata:
  tier: library
  stability: stable
  dependencies:
    - ripgrep
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
output_contract: "A concise ripgrep search plan or command sequence that uses fast, precise rg flags and avoids slow grep/find patterns."
description: >-
  CLI-reference contract for using ripgrep (rg) optimally — the flags that make
  code search fast and precise, the slow patterns to avoid, and the search
  strategy an agent should default to instead of grep -r / find.
  Triggers: "search the codebase", "find where X is defined", "grep for", "rg",
  "ripgrep", "find all usages", "which files contain", "search for a string",
  "find function/symbol", "grep -r is slow", "search with context lines",
  "multiline / cross-line regex search", "machine-readable search output".
  This is a reference contract an agent greps in place, not a procedure to run end-to-end.
---
<!-- TOC: Critical Constraints | Why This Exists | Quick Start | Flag Reference | Strategy Decision Tree | Robot Mode | Exit Codes | Slow Patterns | Quality Rubric | Examples | Troubleshooting | See Also -->

# ripgrep-search-discipline — search code at ripgrep speed, not grep speed

> **Core insight:** `rg` is gitignore-aware, parallel, and Unicode-correct by default.
> Reaching for `grep -r` or `find | xargs grep` throws all three away. Default to `rg`.

## ⚠️ Critical Constraints

- **Default to `rg`, never `grep -r` or `find … -exec grep`.**
  **Why:** `rg` walks the tree in parallel, skips `.git/`, `node_modules/`, and
  everything in `.gitignore` automatically, and is typically 5–50× faster.
  ```bash
  # WRONG — slow, scans ignored dirs, no parallelism
  grep -rn "parseConfig" .
  find . -name '*.go' -exec grep -l "parseConfig" {} +
  # CORRECT
  rg "parseConfig"
  rg -t go "parseConfig"
  ```

- **Filter by type with `-t`/`--type`, not by piping `find` into rg.**
  **Why:** `-t go` is resolved internally against rg's built-in type map; spawning
  `find` first serializes the walk and defeats rg's own ignore logic.
  ```bash
  # WRONG
  find . -name '*.py' | xargs rg "TODO"
  # CORRECT
  rg -t py "TODO"
  ```

- **Quote the pattern; escape regex metacharacters or use `-F`.**
  **Why:** an unquoted pattern is glob-expanded by the shell, and `.` `(` `[` `*`
  `$` are regex operators. Literal searches must use `-F`/`--fixed-strings`.
  ```bash
  # WRONG — shell expands *, regex eats the dots and parens
  rg config.get(*)
  # CORRECT — literal search for the exact text
  rg -F 'config.get(*)'
  ```

- **Never use `-U`/`--multiline` as a reflex.** It buffers whole files and is far
  slower. **Why:** most "find the function" searches are single-line; only reach
  for `-U` when the match genuinely spans newlines.

- **For agent/tool consumption, emit `--json`, not human output.**
  **Why:** human output is heading-grouped and color-coded; `--json` is one
  stable NDJSON object per event, parseable without regex-scraping.

## Why This Exists

Agents repeatedly default to `grep -rn`, `find -exec`, or unfiltered `rg` over the
whole repo, then wait on (or drown in) results. The cost is real: scanning `.git/`
and vendored trees, missing the `-t` filter that would have narrowed 10k files to
40, or dumping thousands of lines into context when `-l` / `-c` would have answered
the question in one line. This contract encodes the small set of flags and the
decision tree that turn search from a bottleneck into a reflex. It is a **reference
you grep in place**, not a workflow you execute top-to-bottom.

## Quick Start

```bash
rg "needle"                       # recursive, gitignore-aware, line-numbered, grouped
rg -t go "func NewServer"         # only Go files
rg -i "deadline"                  # case-insensitive
rg -S "Deadline"                  # smart-case: literal-lower → insensitive, has-upper → sensitive
rg -l "import requests"           # files-with-matches only (cheap, great for "where is X used")
rg -c "TODO"                      # per-file match counts
rg -A3 -B1 "panic("              # 3 lines after, 1 before each hit
rg -o '\bv\d+\.\d+\.\d+\b'        # print only the matched substring (extraction)
rg -F 'a[b].c(' src/             # literal search, no regex interpretation
rg --json "needle" | jq -c .      # machine-readable NDJSON for tooling
```

## Flag Reference (the ones that matter)

| Need | Flag | Note |
|------|------|------|
| Limit by language/type | `-t TYPE` / `--type-not TYPE` | `rg --type-list` to see types; `--type-add` for custom |
| Limit by path pattern | `-g GLOB` / `--iglob` | `-g '!**/test/**'` to exclude; repeatable |
| Context | `-A N` / `-B N` / `-C N` | after / before / both |
| Files only (cheap) | `-l` / `--files-with-matches` | answer "where" without dumping bodies |
| Counts | `-c` / `--count-matches` | `-c` = lines/file; `--count-matches` = total hits |
| Literal (no regex) | `-F` / `--fixed-strings` | always use for strings with `.([*$` |
| Whole word | `-w` / `--word-regexp` | avoids `id` matching `valid` |
| Case | `-i` / `-s` / `-S` | insensitive / sensitive / smart |
| Extract substring | `-o` / `--only-matching` | print match, not whole line |
| Cross-line match | `-U` / `--multiline` (+`--multiline-dotall`) | slow; only when match spans `\n` |
| Search ignored files | `--no-ignore` / `--hidden` / `-uu` | opt back in to `.gitignore`d / dotfiles |
| Bound the search | `--max-count N` / `--max-depth N` | cap hits per file / dir depth |
| Replace in output | `-r REPLACEMENT` | preview substitutions (does NOT write files) |
| Perl regex | `-P` / `--pcre2` | lookaround/backrefs; slower, use only when needed |
| Stable order | `--sort path` | deterministic output (disables parallelism) |
| Machine output | `--json` | NDJSON event stream — parse this, not text |

## Strategy Decision Tree

```
What do you need?
├── "Which files contain X?"        → rg -l "X"            (cheapest)
├── "How many / how often?"         → rg -c "X"  or  --count-matches
├── "Where is X, with context?"     → rg -C3 "X"           (then narrow with -t/-g)
├── "Only in Go/Py/etc.?"           → rg -t go "X"
├── "Exclude tests/vendor?"         → rg -g '!**/{test,vendor}/**' "X"
├── "Literal string with . ( [ ?"   → rg -F 'X'
├── "Just the matched token?"       → rg -o 'PATTERN'
├── "Match spans multiple lines?"   → rg -U 'A[\s\S]*B'    (slow; last resort)
├── "Need to parse results?"        → rg --json "X" | jq
└── "Too many hits?"                → add -t, -g, -w, or --max-count to NARROW first
```

**Default reflex:** start with `rg -l` or `rg -c` to scope the problem, then re-run
narrowed (`-t`, `-g`, `-w`) before pulling full match bodies into context.

## Robot Mode (machine-readable)

For tool/agent consumption, use NDJSON — one JSON object per event:

```bash
rg --json "pattern" path/
```

Event `type` values: `begin` (per file), `match` (one per matching line, carries
`data.path`, `data.line_number`, `data.lines.text`, `data.submatches[]`), `end`
(per-file stats), `summary` (totals). Parse with `jq -c 'select(.type=="match")'`.
`scripts/rg-json.sh PATTERN [PATH]` wraps this and emits `path:line:text` tuples —
**Execute** it (`bash {baseDir}/scripts/rg-json.sh "TODO" src/`) when you want
structured hits without writing jq inline.

## Exit Codes

| Code | Meaning | Agent action |
|------|---------|--------------|
| `0` | One or more matches found | Proceed; results are on stdout |
| `1` | No matches (clean, not an error) | Treat as "not found", not failure |
| `2` | Real error (bad regex, unreadable path, bad flag) | Fix the command; read stderr |

**Why:** code `1` is the normal "absent" signal — never treat a no-match as a crash.
Only `2` is an actual failure. Use `rg -q "X" && …` to branch on presence cheaply.

## Slow Patterns to Avoid

| Slow / wrong | Fast / right | Why |
|--------------|--------------|-----|
| `grep -rn "x" .` | `rg "x"` | parallel + gitignore-aware |
| `find . -name '*.js' \| xargs grep x` | `rg -t js "x"` | no serial find, uses type map |
| `rg "x"` then visually filter | `rg -t go -g '!**/vendor/**' "x"` | narrow at the source |
| `rg -U "x"` for single-line | `rg "x"` | multiline buffers whole files |
| `rg "a.b.c"` expecting literal | `rg -F "a.b.c"` | dots are regex wildcards |
| dumping all matches into context | `rg -l` / `-c` first | scope before you read bodies |
| `rg "x" $(find ...)` | `rg "x"` (let rg walk) | rg's walker is the fast path |

## Quality Rubric

- Did you pick the **narrowest** flag set first (`-l`/`-c` before full bodies; `-t`/`-g` before whole-repo)?
- Are literals searched with `-F` and words with `-w` so the regex engine isn't fighting you?
- Did you treat exit code `1` as "no match" (not an error) and `2` as the only real failure?
- For tooling output, did you use `--json` instead of scraping human text?

## Examples

```bash
# Find a function definition in Go, with surrounding context
rg -t go -A5 'func \(s \*Server\) Start'

# Locate every file importing a package, no bodies (fast survey)
rg -l 'github.com/boshu2/agentops'

# Count TODO/FIXME density per file, excluding tests
rg -c -g '!**/*_test.go' '\b(TODO|FIXME)\b'

# Extract all semver tags mentioned anywhere
rg -o --no-filename '\bv\d+\.\d+\.\d+\b' | sort -u

# Cross-line: a struct field block (multiline, last resort)
rg -U 'type Config struct \{[\s\S]*?\}'

# Machine-readable hits for a downstream tool
rg --json 'panic\(' cli/ | jq -c 'select(.type=="match") | {path:.data.path.text, line:.data.line_number}'
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| "No matches" but you know it's there | file is gitignored / hidden | add `--no-ignore` and/or `--hidden` (`-uu` for both) |
| Pattern matches too much | regex metachar in your literal | use `-F`, or escape `. ( [ * $` |
| `id` matches inside `valid` | substring match | add `-w` / `--word-regexp` |
| Search is slow | `-U` on large files, or huge unfiltered tree | drop `-U`; add `-t`/`-g`/`--max-depth` |
| Output unparseable in a script | scraping human format | switch to `--json` |
| Different runs reorder results | parallel walk | add `--sort path` for determinism |
| `-P` complains | ripgrep built without PCRE2 | rephrase without lookaround/backrefs |

## See Also

| For… | Reference |
|------|-----------|
| Codex/cross-vendor parity (dual-file install) | [references/codex-parity.md](references/codex-parity.md) |
| NDJSON wrapper to run | `scripts/rg-json.sh` (Execute) |
| Self-check this skill's structure | `scripts/validate.sh` (Execute) |
