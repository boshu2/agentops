> Extracted from council/SKILL.md on 2026-04-11.

# Council Flags & Environment Variables

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `COUNCIL_TIMEOUT` | 120 | Agent timeout in seconds |
| `COUNCIL_CODEX_MODEL` | gpt-5.3-codex | Override Codex model for --mixed. Set explicitly to pin Codex judge behavior; omit to use user's configured default. |
| `COUNCIL_CLAUDE_MODEL` | sonnet | Claude model for judges (sonnet default — use opus for high-stakes via `--profile=thorough`) |
| `COUNCIL_EXPLORER_MODEL` | sonnet | Model for explorer sub-agents |
| `COUNCIL_EXPLORER_TIMEOUT` | 60 | Explorer timeout in seconds |
| `COUNCIL_R2_TIMEOUT` | 90 | Maximum wait time for R2 debate completion after sending debate messages. Shorter than R1 since judges already have context. |
| `AGENTOPS_MODEL_TIER` | (none) | Global default model tier. Overridden by skill-specific env vars and explicit flags. |
| `AGENTOPS_COUNCIL_MODEL_TIER` | (none) | Council-specific model tier override. Maps to COUNCIL_CLAUDE_MODEL via tier→profile mapping. |

## Flags

| Flag | Description |
|------|-------------|
| `--deep` | 3 Claude agents instead of 2 |
| `--mixed` | Spawn matched Claude + Codex pairs per perspective (default 3 perspectives = 6 judges). Same prompt, same packet, same persona — only the vendor differs. Scales to 2N judges when a preset has N != 3 perspectives. |
| `--debate` | Enable adversarial debate round (2 rounds via backend messaging, same agents). Incompatible with `--quick`. |
| `--evidence` | **Falsifiable-assertion mode** (alias: `--tdd`). Requires every finding to include `test_assertions` — concrete, mechanical checks (grep, stat, go test, etc.) that would prove the finding is real. Consolidation clamps verdict to at least WARN if any finding lacks assertions. Works with all modes; strongest pairing is `validate`. See [evidence-mode.md](evidence-mode.md). |
| `--commit-ready` | **Also write the consolidated report to `docs/council-log/YYYY-MM-DD-<mode>-<target-slug>.md`** in addition to the usual `.agents/council/` transient path. Use when the verdict is load-bearing for a merged commit or a decision that should survive rebases. See `docs/council-log/README.md`. |
| `--timeout=N` | Override timeout in seconds (default: 120) |
| `--perspectives="a,b,c"` | Custom perspective names (each name sets the judge's system prompt to adopt that viewpoint) |
| `--perspectives-file=<path>` | Load named perspectives from a YAML file (see Named Perspectives in SKILL.md) |
| `--preset=<name>` | Built-in persona preset (security-audit, architecture, research, ops, code-review, plan-review, doc-review, retrospective, product, developer-experience, leadership-quartet) |
| `--count=N` | Override agent count per vendor (e.g., `--count=4` = 4 Claude, or 4+4 with --mixed). Subject to MAX_AGENTS=12 cap. |
| `--explorers=N` | Explorer sub-agents per judge (default: 0, max: 5). Max effective value depends on judge count. Total agents capped at 12. |
| `--explorer-model=M` | Override explorer model (default: sonnet) |
| `--technique=<name>` | Brainstorm technique (scamper, six-hats, reverse). Case-insensitive. Only applicable to brainstorm mode — error if combined with validate/research. If omitted, unstructured brainstorm (current behavior). See `brainstorm-techniques.md`. |
| `--profile=<name>` | Model quality profile (balanced, budget, fast, inherit, quality, thorough). Error if unrecognized name. Overridden by `COUNCIL_CLAUDE_MODEL` env var (highest priority), then by explicit `--count`/`--deep`/`--mixed`. See `model-profiles.md`. |
| `--tier=<name>` | Cost tier alias for --profile (quality, balanced, budget). Maps to profile names. See `model-profiles.md`. |
