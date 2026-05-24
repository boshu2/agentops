# Gate Hygiene — Pre-Gate Sync and Output Parsing

Two recurring gate-stage frictions that compound across long sessions: missing source-surface rebuilds (the gate runs against stale binaries) and trusting trailing status lines (the gate output mixes blocking and advisory signals).

## Pre-gate source-surface detection

Before invoking `pre-push-gate.sh` (or any binary-dependent gate), inspect the staged diff and rebuild downstream artifacts if any of these surfaces changed:

| Changed surface | Required pre-gate action |
|---|---|
| `cli/**/*.go` | `cd cli && make build && go install ./cmd/ao` — refreshes `cli/bin/ao` and `~/go/bin/ao` so the gate sees the new Go behaviour |
| `skills/**` or `hooks/**` | `cd cli && make sync-hooks` — refreshes `cli/embedded/{skills,hooks}/` so the embedded-parity check passes |
| `skills-codex/**` | `bash scripts/regen-codex-hashes.sh` — refreshes generated_hash values in the codex manifest |
| `schemas/**` and `docs/contracts/**` together | re-run contract validation locally first; do not let CI surface the drift |

Detection recipe:

```bash
changed=$(git diff --cached --name-only)
echo "$changed" | grep -q '^cli/.*\.go$' && (cd cli && make build && go install ./cmd/ao)
echo "$changed" | grep -qE '^(skills/|hooks/)' && (cd cli && make sync-hooks)
echo "$changed" | grep -q '^skills-codex/' && bash scripts/regen-codex-hashes.sh
```

Without these pre-gate steps, the gate may fail with stale-binary or embedded-drift errors that look like real regressions but are just plumbing. Each false failure costs a turn of recovery work.

## Gate output parsing

The pre-push gate (and similar two-pass scripts) emits both blocking and advisory results. Trusting only the trailing status line conflates them. Use a structured grep:

```bash
# Capture full output, then parse explicit failure markers
bash scripts/pre-push-gate.sh --fast 2>&1 | tee /tmp/gate.log

# Authoritative blocking failures (case-sensitive Pass N: FAILED|BLOCKED)
if grep -E '^.*Pass [0-9]+: (FAILED|BLOCKED)' /tmp/gate.log >/dev/null; then
  echo "BLOCKING failure detected"
  exit 1
fi

# Advisory issues — record but don't block
grep -E 'advisory|warning|WARN' /tmp/gate.log || true
```

Anti-pattern: reading `tail -1 /tmp/gate.log` and treating "passed (N skipped)" as authoritative. A run can show "Pass 1: FAILED" mid-output and "passed (X skipped)" at the end if Pass 2 ran in advisory-only mode against the worktree. The structural markers (`Pass N: FAILED|BLOCKED`) are the truth.

## When to use `PRE_PUSH_SKIP_EVAL=1`

Documented release valve for the eval canary lane only, when:
- The canary is flaking on pre-existing infra (filed as a tracked bead)
- The current diff is unrelated to evals/, schemas/eval-, or cli/cmd/ao/eval
- A recorded recent run has confirmed the canary is currently 50/50

Never use `--no-verify`. The pre-commit hook is a no-op for most diffs but the principle violation is durable in `git log` and surfaces in post-mortem.
