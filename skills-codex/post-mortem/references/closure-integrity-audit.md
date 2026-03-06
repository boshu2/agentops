# Closure Integrity Audit

**Mechanically verify that closed beads represent real completed work, not premature or phantom closures.**

This audit catches four failure modes discovered in production:

1. **Multi-wave regressions** — A later wave's worker removes code that an earlier wave added. Each wave passes tests independently, but the net result is incomplete.
2. **Phantom closures** — Beads closed with generic/empty descriptions ("task"), no spec, no git evidence.
3. **Orphaned children** — Child beads exist in `bd list` but aren't linked to parent in `bd show <parent>`.
4. **Stretch goals closed without work** — Items marked "stretch" bulk-closed when epic closes, with no implementation or documented deferral rationale.

## When to Run

- **Step 2.3** of post-mortem (Reconcile Plan vs Delivered Scope)
- After `/crank` completes (before closing epic)
- During `/retro` when reviewing multi-wave epics

## Audit Procedure

### Check 1: Git Evidence Per Child

For each closed child bead, verify at least one commit references it or touches its declared files.

```bash
EPIC_ID="<epic-id>"
FAILURES=""

# Get all children
for child in $(bd children "$EPIC_ID" 2>/dev/null | grep -oP '\S+' | head -1); do
  # Check for commits mentioning this bead ID
  COMMITS=$(git log --oneline --all --grep="$child" 2>/dev/null | wc -l | tr -d ' ')

  if [ "$COMMITS" -eq 0 ]; then
    # Fallback: check if child has file metadata and those files were modified
    CHILD_DESC=$(bd show "$child" 2>/dev/null)
    FILES_IN_SCOPE=$(echo "$CHILD_DESC" | grep -oP '`[^`]+\.(go|py|ts|sh|md|yaml)`' | tr -d '`')

    if [ -z "$FILES_IN_SCOPE" ]; then
      FAILURES="${FAILURES}\n- NO EVIDENCE: $child — zero commits, no file metadata"
    else
      # Check if any scoped files were modified in recent commits
      TOUCHED=0
      for f in $FILES_IN_SCOPE; do
        if git log --oneline --diff-filter=M -- "$f" 2>/dev/null | head -1 | grep -q .; then
          TOUCHED=1
          break
        fi
      done
      [ "$TOUCHED" -eq 0 ] && FAILURES="${FAILURES}\n- NO EVIDENCE: $child — scoped files not modified"
    fi
  fi
done
```

**Verdict:**
- 0 failures → PASS
- 1-2 failures → WARN (include in council packet, continue)
- 3+ failures → FAIL (block epic closure, investigate)

### Check 2: Phantom Bead Detection

Flag children with no meaningful description or title.

```bash
for child in $(bd children "$EPIC_ID" 2>/dev/null | grep -oP '\S+' | head -1); do
  TITLE=$(bd show "$child" 2>/dev/null | head -1 | sed 's/^.*· //' | sed 's/ \[.*$//')
  DESC=$(bd show "$child" 2>/dev/null | sed -n '/^DESCRIPTION$/,/^$/p' | tail -n +2)

  # Generic titles: "task", "fix", "update", single word
  if echo "$TITLE" | grep -qP '^(task|fix|update|todo|item|work)$'; then
    FAILURES="${FAILURES}\n- PHANTOM: $child — generic title '$TITLE', no spec"
  fi

  # Empty or minimal description
  DESC_WORDS=$(echo "$DESC" | wc -w | tr -d ' ')
  if [ "$DESC_WORDS" -lt 5 ]; then
    FAILURES="${FAILURES}\n- PHANTOM: $child — description has $DESC_WORDS words (min 5)"
  fi
done
```

**Why this matters:** Phantom beads inflate completion metrics without representing real work. In the na-oh2 audit, 11 children all had "task" as their title — only the git commit revealed what they actually did.

### Check 3: Orphaned Children

Verify all children in `bd list` are linked to parent.

```bash
# Children from parent's perspective
PARENT_CHILDREN=$(bd show "$EPIC_ID" 2>/dev/null | grep '↳' | grep -oP '\w+-\w+\.\d+')

# Children from list (matching prefix)
LIST_CHILDREN=$(bd list --all 2>/dev/null | grep "^. ${EPIC_ID}\." | grep -oP '\w+-\w+\.\d+')

# Find orphans (in list but not in parent)
for child in $LIST_CHILDREN; do
  if ! echo "$PARENT_CHILDREN" | grep -q "^${child}$"; then
    FAILURES="${FAILURES}\n- ORPHAN: $child — exists in bd list but not linked to $EPIC_ID"
  fi
done
```

### Check 4: Multi-Wave Regression Detection

For multi-wave epics (crank), compare each wave's additions against the next wave's deletions.

```bash
# Get wave commits from crank notes
WAVE_COMMITS=$(bd show "$EPIC_ID" 2>/dev/null | grep 'CRANK_WAVE' | grep -oP 'at \K\S+')

# For each consecutive pair, check if Wave N+1 deleted lines Wave N added
PREV_COMMIT=""
for commit in $WAVE_COMMITS; do
  if [ -n "$PREV_COMMIT" ]; then
    # Lines added in previous wave
    ADDED=$(git diff "$PREV_COMMIT"^.."$PREV_COMMIT" 2>/dev/null | grep '^+[^+]' | sort)
    # Lines removed in current wave
    REMOVED=$(git diff "$commit"^.."$commit" 2>/dev/null | grep '^-[^-]' | sort)

    # Intersection = regressions
    REVERTED=$(comm -12 <(echo "$ADDED" | sed 's/^+//') <(echo "$REMOVED" | sed 's/^-//') 2>/dev/null | head -10)

    if [ -n "$REVERTED" ]; then
      FAILURES="${FAILURES}\n- REGRESSION: Wave removed lines that prior wave added:\n$(echo "$REVERTED" | head -5)"
    fi
  fi
  PREV_COMMIT="$commit"
done
```

**Origin:** na-vs9.4 — Wave 1 added vibe checkpoint detection (15 lines), Wave 2 removed it entirely. Both waves passed tests independently. The orphaned checkpoint writer in crank was only caught by manual audit.

### Check 5: Stretch Goal Audit

For children tagged "stretch" that were closed, verify either implementation exists or deferral is documented.

```bash
for child in $(bd children "$EPIC_ID" 2>/dev/null | grep -i 'stretch' | grep -oP '\S+' | head -1); do
  STATUS=$(bd show "$child" 2>/dev/null | grep -oP 'CLOSED')
  CLOSE_REASON=$(bd show "$child" 2>/dev/null | grep 'Close reason:')
  COMMITS=$(git log --oneline --all --grep="$child" 2>/dev/null | wc -l | tr -d ' ')

  if [ -n "$STATUS" ] && [ "$COMMITS" -eq 0 ]; then
    if ! echo "$CLOSE_REASON" | grep -qi 'defer\|stretch\|intentional\|not needed'; then
      FAILURES="${FAILURES}\n- STRETCH CLOSED WITHOUT RATIONALE: $child — no commits, no deferral reason"
    fi
  fi
done
```

## Output Format

Write results into the post-mortem report under `## Closure Integrity`:

```markdown
## Closure Integrity

| Check | Result | Details |
|-------|--------|---------|
| Git Evidence | PASS/WARN/FAIL | N children verified, M without evidence |
| Phantom Beads | PASS/WARN | N phantom beads detected |
| Orphaned Children | PASS/WARN | N orphans found |
| Multi-Wave Regression | PASS/FAIL | N regressions detected |
| Stretch Goals | PASS/WARN | N stretch goals closed without rationale |

### Findings
- <specific findings from each check>
```

## Integration with Council

Include closure integrity results in the council packet:

```json
{
  "context": {
    "closure_integrity": {
      "git_evidence_failures": [...],
      "phantom_beads": [...],
      "orphaned_children": [...],
      "wave_regressions": [...],
      "stretch_audit": [...]
    }
  }
}
```

The `plan-compliance` judge uses these to assess whether the epic should actually be marked complete.
