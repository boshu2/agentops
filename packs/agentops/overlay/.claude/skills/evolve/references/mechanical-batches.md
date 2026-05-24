# Mechanical Batch Work — Script vs Per-File Edit

A "mechanical batch" is work where the same edit is applied to N files mechanically, with no per-file judgement after the pattern is established. Examples from prior sessions: declaring `practices: [slug]` across 533 cli/ Go files, regenerating COMMANDS.md from a Cobra tree, bumping a version literal in N manifests.

The default per-file `Edit` tool path scales linearly with N and burns context. Switch to a script when ANY of these apply:

- N > ~20 files
- The transformation is **uniform** (same regex / same sed / same template field)
- A failure on file K does not change the strategy for files K+1..N
- Per-file output verification is cheap (`git diff --stat`, a single grep)

## Script-first pattern

```bash
# 1. Inventory candidates with the predicate the task implies
candidates=$(git ls-files 'skills/**/SKILL.md' | xargs -I{} bash -c '
  grep -L "^practices:" "{}" >/dev/null && echo "{}"
' | head -50)

# 2. Apply the mechanical edit
for f in $candidates; do
  awk '
    /^description:/ && !done { print; print "practices: [<slug>]"; done=1; next }
    { print }
  ' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
done

# 3. Verify in one shot
git diff --stat
grep -c "^practices:" $candidates | sort
```

## Per-file Edit pattern (when the above does NOT apply)

Use `Edit` per file only when:

- The transformation requires per-file decisions (which slug? which carrier?)
- N is small (< 10)
- Verification benefits from reading each diff in context

## Decision in Step 4

When `/rpi` discovery surfaces a per-file pattern with N > 20 uniform candidates, the implementation phase should generate a script under `scripts/` or use an inline awk/sed pass rather than queuing N `Edit` operations. The post-edit gate then verifies the batch atomically.

If the script needs different parameters per file, that is a signal the work isn't actually mechanical — fall back to per-file Edit and reduce N (split the cycle).

## Why this matters

A 533-file `practices:` backfill done via per-file Edit takes ~50× more context than the same backfill done via a script with `git diff --stat` verification. The context-budget exhausts well before the work completes, and the loop drops into a forced scout or idle state with most of the work still pending.

## Anti-patterns

- Running a script that mutates files but ignoring the result of `git diff` (silent partial application).
- Using `sed -i` on a tree-traversed find without `--null` handling (filename-with-spaces breakage).
- Mixing mechanical and per-file edits in the same commit (auditor cannot tell what was bulk-applied vs per-judged).
- Treating a 200-file batch as a single "atomic" change — split into review-sized waves (~30-100 files) with one commit per wave.
