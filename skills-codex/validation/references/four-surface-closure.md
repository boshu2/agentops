# Four-Surface Closure Check

> Mandatory validation step. Every completed feature must be checked across four surfaces.
> From spec-as-leverage-point analysis: code-only validation misses 40%+ of shipping gaps.

## The Four Surfaces

| Surface | What to Check | Gate Command |
|---------|---------------|--------------|
| **Code** | Implementation matches spec, tests pass, no regressions | `make test` or project-specific test command |
| **Documentation** | Docs reflect current behavior, no stale references | `validate-doc-release.sh`, grep for old behavior in docs |
| **Examples** | Usage examples work, CLI help is current | Run examples, check `--help` output |
| **Proof** | Acceptance criteria gates pass, new behavior has tests | Run acceptance criteria commands from plan |

## When to Run

This check runs as part of validation Step 1.5 (after vibe, before post-mortem):

```
Step 1: vibe (code quality)
Step 1.5: four-surface closure ← NEW
Step 2: post-mortem (learning extraction)
Step 3: retro
Step 4: forge
Step 5: phase summary
```

## Quick Check Script

```bash
# Surface 1: Code
echo "=== Code ==="
make test 2>&1 | tail -5

# Surface 2: Docs
echo "=== Documentation ==="
# Check for stale references to old behavior
git diff --name-only HEAD~5 | grep -E '\.(md|txt|rst)$' || echo "No doc changes"

# Surface 3: Examples
echo "=== Examples ==="
# Verify CLI help matches implementation
# Verify CLI help matches implementation (project-specific command)
echo "Check CLI --help output matches implementation"

# Surface 4: Proof
echo "=== Proof ==="
# Run acceptance criteria from plan (customize per project)
echo "Run acceptance criteria gates here"
```

## Verdict Rules

- **All 4 surfaces pass:** Proceed to post-mortem
- **Code passes, others fail:** WARN — complete documentation/examples/proof before closing
- **Code fails:** BLOCK — fix code before checking other surfaces
- **Proof missing:** WARN — acceptance criteria must be runnable, not just "verify it works"
