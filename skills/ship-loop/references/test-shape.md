# Test shape for /ship-loop

What the FIRST FAILING TEST must look like, and how to test the rationale without trapping local-only state.

## The failing-test contract

Per `.claude/rules/{go,python}.md`: **L2-first, L1-always.** L2 = integration / contract tests; L1 = unit. Write L2 first (where bugs are actually found), then L1 for regression safety.

The test must:

1. **Fail BEFORE the fix.** Confirm with `bats <test>` or `go test <pkg>` and read the failure message.
2. **Fail for the right reason.** Not "doesn't crash". Assert exact expected values (`== expected`), not "not the wrong one" (`!= wrong`).
3. **Reproduce the failure mode at the right layer.** F1 (dead vars in script) → shellcheck check, not a Go unit test. F-storage-fs (path traversal) → Go integration covering Save/Load, not a manual command-line probe.
4. **Have a clear rollback path.** If the fix is reverted, the test must turn red on the very next CI run.

## Structural-property assertions

Tests that just verify "the function returns the right value" go stale when the implementation changes. Tests that assert **structural properties of the fix** survive refactoring:

| Failure class | Don't | Do |
|---|---|---|
| Gate logic | `assert gate_passes_on_input(X)` | `assert "needs_check shell" NOT IN gate_source` (closes F1) |
| Script regex | `assert regex_matches("  foo")` | `assert "[[:space:]]{2,}" NOT IN script_source` (closes mawk class) |
| Error handling | `assert err is not None` | `assert errors.Is(err, ErrInvalidRunID)` (exact sentinel) |
| Rationale anchor | `assert os.path.exists(".agents/learnings/X.md")` | `assert "<slug>" in open(script).read()` (closes meta-pattern) |

The right-column patterns survive (a) the underlying implementation changing, (b) `.agents/` being gitignored, (c) the test being run in CI's fresh clone.

## Rationale-anchor assertions

When a fix is anchored to a learning (e.g., F1 closes PR #322's dead-var class), the test should ALSO assert that the learning reference remains in the script source. This way, if a future agent re-introduces the failure mode AND removes the learning reference, the test fails twice and surfaces the regression.

Concrete pattern:

```bats
@test "post-mortem learning anchor reference is in the script comment" {
    # The change is anchored to a durable lesson; verify the rationale link
    # remains in the script comment header. The actual learning file lives
    # in .agents/ (gitignored, local-only), so a file-existence check would
    # break in CI's fresh clone. Asserting the reference in the script body
    # instead keeps the rationale traceable without depending on local state.
    grep -q '2026-05-18-script-rewrites-leave-dead-variables' "$GATE"
}
```

The `grep -q '<slug>' "$SCRIPT"` form is the canonical anchor assertion. Memorize it.

## When NOT to write a failing test

Trivial doc fixes (typo correction, link repair) don't need a failing test — the pre-existing pre-push gate output is the proof.

Pre-commit hook policy: shell-only changes warn "no paired bats test"; the operator may bypass with a rationale in the commit body (e.g., "refactor with existing coverage", "docs-only change"). Don't abuse the bypass.

## Examples from the 2026-05-18 session

- **F1** — `pre-push-shellcheck-unconditional.bats` 6 structural tests: "step 30 NOT wrapped in `if needs_check shell`", "step 30 collects staged .sh via git diff --cached", "rationale anchor in script body"
- **F3** — `gh-merge-chain.bats` 7 tests: argument parsing, --help text, --dry-run lists PRs, set -euo pipefail in source, anchor reference in script
- **Storage** — `packet_repo_test.go::TestRepo_SaveRejectsUnsafeRunID` 10 path-traversal subcases each returning `errors.Is(err, ErrInvalidRunID)` AND leaving no file on disk
