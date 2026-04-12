> Extracted from council/SKILL.md on 2026-04-11.

# Council Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| "Error: --quick and --debate are incompatible" | Both flags passed together | Use `--quick` for fast inline check OR `--debate` for multi-round review, not both |
| "Error: --debate is only supported with validate mode" | Debate flag used with brainstorm/research | Remove `--debate` or switch to validate mode — brainstorming/research have no PASS/FAIL verdicts |
| Council spawns fewer agents than expected | `--explorers=N` exceeds MAX_AGENTS (12) | Agent auto-scales judge count. Check report header for actual judge count. Reduce `--explorers` or use `--count` to manually set judges |
| `--mixed` hard-errors before spawning judges | Codex CLI not on PATH or not runnable | Install/fix Codex CLI (`brew install codex`) or rerun without `--mixed`. Model uses user's configured default unless `COUNCIL_CODEX_MODEL` is set. |
| No output files in `.agents/council/` | Permission error or disk full | Check directory permissions with `ls -ld .agents/council/`. Council auto-creates missing dirs. |
| Agent timeout after 120s | Slow file reads or network issues | Increase timeout with `--timeout=300` or check `COUNCIL_TIMEOUT` env var. Default: 120s. |

## Migration from judge

`/council` replaces the old judge skill. Migration:

| Old | New |
|-----|-----|
| judge recent | `/council validate recent` |
| judge 2 opus | `/council recent` (default) |
| judge 3 opus | `/council --deep recent` |

**Deprecated:** The /judge skill was replaced by `/council` in v2.8. The judge skill will be removed in v3.0. Migrate all judge invocations to `/council`.
