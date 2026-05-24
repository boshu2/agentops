# Codex CLI Verified Commands

Verified in this repo on 2026-02-26 (local environment).

## Known-Good Command Shapes

```bash
# Discover CLI surface
codex --help
codex exec --help

# Non-interactive execution (prompt argument)
codex exec "Summarize current git status."

# Pin working directory
codex exec -C "$(pwd)" "List changed files and suggest next step."

# Structured output options
codex exec --json "Return one-line status"
codex exec -o /tmp/codex-last.txt "Return one-line status"
```

## Integration with `ao rpi phased`

```bash
# Preferred runtime override for phased runs
/path/to/ao rpi phased --from=implementation --runtime-cmd codex <bead-id>
```

Expected dry-run spawn shape from current `ao` implementation:

```text
codex "exec" "<prompt>"
```

## Known-Bad / Mismatched Patterns

```bash
# BAD: -p is profile, not prompt
codex -p "do work"
```

Observed failure mode:

```text
Error loading configuration: config profile ... not found
```

Other invalid assumptions to avoid:

- `codex -q` (not a valid quiet flag)
- `codex exec --quiet` (no such flag)

