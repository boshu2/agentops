# Checkpoint-Policy Preflight

Validates the ratchet chain before running the post-mortem council. Ensures prior phases completed successfully and all artifacts are available.

## 1. Guard Clause

```bash
# Skip if --skip-checkpoint-policy flag is set
# Skip if chain file doesn't exist (standalone post-mortem is valid)
CHAIN_FILE=".agents/ao/chain.jsonl"
if [ ! -f "$CHAIN_FILE" ]; then
  echo "Checkpoint policy: SKIP (no chain file — standalone post-mortem)"
  # Continue to Step 1 without blocking
fi
```

## 2. Ratchet Chain Policy Checks

Load `chain.jsonl` and verify prior phases are locked:

1. **Parse entries using dual-schema:** Check for BOTH `gate` (old schema) and `step` (new schema) field names. Each line is a JSON object — use `jq` to extract either field:
   ```bash
   jq -r '.gate // .step' "$CHAIN_FILE"
   ```
2. **Required phases:** For each of `research`, `plan`, `pre-mortem`, `implement`/`crank`, `vibe`:
   - Check that at least one entry exists with `locked: true` or `status: "locked"`
   - Missing phases: **WARN** (logged, not blocking)
3. **Council verdict validation:** For `pre-mortem` and `vibe` entries:
   - Find the corresponding council report in `.agents/council/` (match by date and type in filename)
   - Read the `## Council Verdict:` line
   - If verdict is `FAIL`: **BLOCK** — do not proceed
4. **Cycle guard:** If `cycle > 1` in any entry, verify `parent_epic` is non-empty. Empty parent on multi-cycle: **WARN**

## 3. Artifact Availability Checks

For each chain entry's `output` path:

1. If output starts with `.agents/` or contains `/` (is a file path): verify file exists on disk
2. If output matches `epic:<id>` or `issue:<id>`: skip (not a file reference)
3. If output is `inline-pass`: skip (no artifact expected)
4. Missing artifacts: **WARN**

```bash
while IFS= read -r line; do
  output=$(echo "$line" | jq -r '.output // .artifact // empty')
  case "$output" in
    epic:*|issue:*|inline-pass|"") continue ;;
    *)
      if [[ "$output" == *"/"* ]] && [ ! -f "$output" ]; then
        echo "WARN: artifact missing: $output"
      fi
      ;;
  esac
done < "$CHAIN_FILE"
```

## 4. Idempotency Check

If an epic ID is provided, check `.agents/rpi/next-work.jsonl` for an existing entry with the same `source_epic`:

1. If found and `consumed: false`: **WARN** "Post-mortem already harvested for this epic. Re-running will create duplicate entries."
2. If found and `consumed: true`: **INFO** "Prior post-mortem consumed by `<consumed_by>`. Fresh harvest will be appended."
3. If not found: no action needed

```bash
NEXT_WORK=".agents/rpi/next-work.jsonl"
if [ -n "$EPIC_ID" ] && [ -f "$NEXT_WORK" ]; then
  existing=$(grep "\"source_epic\":\"$EPIC_ID\"" "$NEXT_WORK" | tail -1)
  if [ -n "$existing" ]; then
    consumed=$(echo "$existing" | jq -r '.consumed')
    if [ "$consumed" = "false" ]; then
      echo "WARN: Post-mortem already harvested for $EPIC_ID. Re-running will create duplicate entries."
    else
      consumed_by=$(echo "$existing" | jq -r '.consumed_by')
      echo "INFO: Prior post-mortem consumed by $consumed_by. Fresh harvest will be appended."
    fi
  fi
fi
```

## 5. Summary Report Table

Print the preflight summary before proceeding:

```
| Check              | Status    | Detail                    |
|--------------------|-----------|---------------------------|
| Chain loaded       | PASS/SKIP | path or "not found"       |
| Prior phases locked| PASS/WARN | list any unlocked         |
| No FAIL verdicts   | PASS/BLOCK| list any FAILed           |
| Artifacts exist    | PASS/WARN | list any missing          |
| Idempotency        | PASS/WARN/INFO | dedup status         |
```

## 6. Blocking Behavior

- **BLOCK** only on FAIL verdicts in prior gates (pre-mortem or vibe). If any check is BLOCK: stop post-mortem and report:
  > "Checkpoint-policy BLOCKED: `<reason>`. Fix the failing gate and re-run."
- **WARN** on everything else (missing phases, missing artifacts, idempotency). Warnings are logged, included in the council packet as `context.checkpoint_warnings`, and execution proceeds.
- **INFO** is purely informational — no action needed.
