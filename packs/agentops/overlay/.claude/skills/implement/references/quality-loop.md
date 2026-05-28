# Autonomous Quality Loop (Pre-Commit)

Before committing, run a fix-verify loop on all files modified in this session (max 3 iterations):

**Iteration N:**

1. **List modified files:** `git diff --name-only HEAD`
2. **Read each modified file completely** — do not skim
3. **Check for defects:**
   - Wrong variable references (copy-paste errors, stale names)
   - Silent error swallowing (`_ = err` or empty catch blocks)
   - Hardcoded values that should be configurable or constants
   - Missing edge cases identified during implementation
   - Inconsistencies with existing patterns in the codebase
   - Unused imports or variables
   - Complexity budget violations (function cyclomatic complexity >15)
4. **Lifecycle review (once per loop, first iteration only):**
   If `--no-lifecycle` is NOT set AND lifecycle tier is `standard` or `full` AND staged changes exist:
   ```
   Skill(skill="review", args="--diff --staged --quick")
   ```
   Merge review findings into the defect list. CRITICAL → HIGH, WARNING → MEDIUM, NIT → LOW.
   This runs EXACTLY ONCE (first iteration only) — do NOT re-run review after fixes.
   **Skip if:** `--no-lifecycle` flag, lifecycle tier is `minimal` or `fast`, no staged changes.

4a. **Complexity-triggered refactor check (once per loop, first iteration only):**
   If `--no-lifecycle` is NOT set AND lifecycle tier is `full` AND any modified function has cyclomatic complexity > 15:
   ```
   Skill(skill="refactor", args="<high-cc-function> --dry-run")
   ```
   Treat refactor suggestions as MEDIUM findings. Do NOT auto-apply — report only.
   **Skip if:** `--no-lifecycle` flag, lifecycle tier is not `full`, no function exceeds CC > 15.

5. **Report findings** as a numbered list with severity (HIGH/MEDIUM/LOW)
6. **HIGH findings:** Fix immediately, re-run tests, re-sweep (next iteration)
   - If a fix causes test regression: **revert the fix**, report as unresolvable, proceed
7. **MEDIUM/LOW findings:** Report in commit message, proceed

**Loop termination:**
- 0 HIGH findings → exit loop, proceed to Step 6
- 3 iterations exhausted with HIGH findings remaining → **BLOCK commit**. Report remaining HIGHs and stop. Do NOT proceed to Step 6.
  - Override: `--force-commit` allows proceeding with documented HIGHs (explicit opt-in only)

**Output:** Record iteration count, findings per iteration, and remaining items.

If no modified files or sweep finds zero issues on first pass, proceed directly to Step 5c.
