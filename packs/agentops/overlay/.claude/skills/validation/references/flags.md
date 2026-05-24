# /validation Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--complexity=<level>` | auto | Force complexity level (`fast` / `standard` / `full`). Matches `/rpi` and `/discovery` syntax. |
| `--interactive` | off | Human gates in validation report review (before writing summary). Does NOT override `/vibe` council autonomy. |
| `--no-lifecycle` | off | Skip ALL lifecycle checks in STEP 1.7 (test, deps, review, perf) |
| `--lifecycle=<tier>` | matches complexity | Controls which lifecycle skills fire: `minimal` (test only), `standard` (+deps, +review), `full` (+perf) |
| `--no-retro` | off | Skip retro step only |
| `--no-forge` | off | Skip forge step only |
| `--no-budget` | off | Disable phase time budgets |
| `--strict-surfaces` | off | Make all 4 surface failures blocking (FAIL instead of WARN). Passed automatically by `/rpi --quality`. |
| `--allow-critical-deps` | off | Allow shipping with CVSS >= 9.0 vulnerabilities (acknowledged risk acceptance) |
| `--release-context` | auto | Force STEP 1.7.5 release-readiness gates on. Auto-detected from branch name (`release/*`, `v*-prep`, `v*-evolve-run`, `v[0-9]+\.[0-9]+*`). |
| `--skip-release-gates` | off | Bypass STEP 1.7.5 (operator-acknowledged risk for non-release validation hitting the release-shaped branch heuristic) |
