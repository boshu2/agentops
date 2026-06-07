# Request Info Templates

Templates for requesting more information from issue reporters. Use HEREDOC format for proper multi-line handling.

---

## Contents

| Template | Use When |
|----------|----------|
| [Cannot Reproduce](#cannot-reproduce-bug) | Bug doesn't occur on your system |
| [Unclear Feature](#unclear-feature-request) | Feature request lacks specifics |
| [Missing Context](#missing-context) | Report is too vague |
| [Ambiguous Report](#ambiguous-report) | Multiple interpretations possible |
| [Platform-Specific](#platform-specific-issue) | Might be OS/env dependent |
| [Performance Issue](#performance-report) | Slow behavior reported |
| [Version Mismatch](#version-mismatch-suspected) | Might be fixed in newer version |
| [Regression Report](#regression-report) | Something broke that worked before |

---

## Cannot Reproduce Bug

```bash
gh issue comment N -R owner/repo -b "$(cat <<'EOF'
I tried to reproduce this but couldn't. Could you provide:

- Exact steps to reproduce (start from a clean state)
- OS and version
- Full error output (copy-paste, not screenshot)
- Tool version (`tool --version`)
- Any relevant config files

Happy to investigate further with more details.
EOF
)"
```

---

## Unclear Feature Request

```bash
gh issue comment N -R owner/repo -b "$(cat <<'EOF'
Interesting idea. A few questions before I consider this:

1. What specific use case are you trying to solve?
2. What's your expected input/output?
3. How would this interact with existing features?

This would help me understand if/how to implement it.
EOF
)"
```

---

## Missing Context

```bash
gh issue comment N -R owner/repo -b "$(cat <<'EOF'
Thanks for reporting. Could you provide more context?

- What were you doing when this happened?
- What did you expect vs what actually happened?
- Any workarounds you've found?

This will help narrow down the issue.
EOF
)"
```

---

## Ambiguous Report

```bash
gh issue comment N -R owner/repo -b "$(cat <<'EOF'
I want to make sure I understand correctly. When you say "[X]", do you mean:

1. [Interpretation A]
2. [Interpretation B]
3. Something else?

The answer affects how I'd approach this.
EOF
)"
```

---

## Platform-Specific Issue

```bash
gh issue comment N -R owner/repo -b "$(cat <<'EOF'
This might be platform-specific. Could you confirm:

- OS: macOS / Linux / Windows?
- Architecture: x86_64 / arm64?
- Shell: bash / zsh / fish?
- How did you install the tool? (brew / cargo / binary)

I'll try to reproduce on a matching setup.
EOF
)"
```

---

## Performance Report

```bash
gh issue comment N -R owner/repo -b "$(cat <<'EOF'
Performance issues are tricky to debug remotely. Could you provide:

- Rough dataset size (files, records, etc.)
- Time it takes currently
- What you'd expect it to take
- System specs (RAM, CPU, SSD vs HDD)
- Output of `time tool command` if possible

This helps distinguish between genuine bugs and expected behavior at scale.
EOF
)"
```

---

## Version Mismatch Suspected

```bash
gh issue comment N -R owner/repo -b "$(cat <<'EOF'
This might be fixed in a newer version. Could you confirm:

- What version are you using? (`tool --version`)
- How did you install it?
- When did you last update?

The latest version is [X.Y.Z]. If you're on an older version, updating might resolve this.
EOF
)"
```

---

## Regression Report

```bash
gh issue comment N -R owner/repo -b "$(cat <<'EOF'
Thanks for the regression report. To help track this down:

1. What version last worked correctly?
2. What version first showed the bug?
3. Can you share a minimal example that reproduces it?

If you can narrow down the version range, I can bisect to find the culprit commit.
EOF
)"
```

---

## Config/Environment Issue

```bash
gh issue comment N -R owner/repo -b "$(cat <<'EOF'
This might be a configuration issue. Could you share:

- Contents of your config file (if any)
- Relevant environment variables
- Any shell aliases or functions that might interfere

Sometimes conflicts with other tools cause unexpected behavior.
EOF
)"
```

---

## Intermittent Issue

```bash
gh issue comment N -R owner/repo -b "$(cat <<'EOF'
Intermittent issues are tricky. To help debug:

- How often does this occur? (every time, sometimes, rarely)
- Is there a pattern? (time of day, system load, specific inputs)
- Does it correlate with any other activity?

Any additional logging output would be helpful too.
EOF
)"
```

---

## Response Guidelines

1. **Be specific** — Ask for exactly what you need
2. **Be concise** — Don't overwhelm with questions
3. **Be helpful** — Offer workarounds if known
4. **Set expectations** — Don't promise a fix, promise investigation
5. **One round** — Try to get all info in one ask
6. **Use HEREDOC** — Always wrap multi-line responses

---

## What to Ask For

| Issue Type | Essential Info |
|------------|----------------|
| Crash | Steps, OS, full error output |
| Wrong output | Input, expected vs actual |
| Performance | Dataset size, timing, specs |
| Install failure | OS, arch, install method, error |
| Config issue | Config file, env vars |

---

## Common Mistakes

| Mistake | Why It's Bad |
|---------|--------------|
| "Please provide more info" | Too vague—user doesn't know what |
| Asking multiple rounds | Wastes time, user loses interest |
| Not offering workaround | Leaves user stuck |
| Promising a fix | Creates expectation you can't guarantee |
| Screenshots requests | Screenshots hide critical info, ask for text |
