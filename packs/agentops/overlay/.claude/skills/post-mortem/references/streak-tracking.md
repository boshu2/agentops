# RPI Session Streak Tracking

> Reference for Step 1.5 of the post-mortem skill.

## Purpose

Track consecutive days of RPI usage to surface workflow adoption trends in post-mortem reports. This is a read-only metric — post-mortem does not write streak data.

## Data Source

**File:** `.agents/rpi/rpi-state.json`

This file is written by `/rpi` (the RPI orchestrator) on every session start and phase transition. Post-mortem reads it but never writes to it.

## Detection Logic

1. Read `.agents/rpi/rpi-state.json`
2. If absent or unparseable: **silent no-op** — skip streak reporting entirely
3. Extract fields:
   - `session_id` — current or most recent RPI session
   - `phase` — current phase (research, plan, implement, validate)
   - `started_at` — ISO 8601 timestamp of session start
   - `verdicts` — array of verdict objects from prior phases

## Streak Calculation

Count consecutive calendar days (UTC) with at least one `rpi-state.json` update:

```bash
# Read the state file
RPI_STATE=".agents/rpi/rpi-state.json"
if [ ! -f "$RPI_STATE" ]; then
  # Silent no-op — no streak data available
  exit 0
fi

# Extract last update timestamp
LAST_UPDATE=$(jq -r '.started_at // .updated_at // empty' "$RPI_STATE" 2>/dev/null)
if [ -z "$LAST_UPDATE" ]; then
  exit 0
fi

# Check if updated today (UTC)
LAST_DATE=$(date -jf "%Y-%m-%dT%H:%M:%S" "${LAST_UPDATE%%[.+Z]*}" +%Y-%m-%d 2>/dev/null || \
            date -d "${LAST_UPDATE}" +%Y-%m-%d 2>/dev/null)
TODAY=$(date -u +%Y-%m-%d)
```

For consecutive-day tracking, compare file modification dates of historical state snapshots in `.agents/rpi/`. Each day with at least one `rpi-state.json` write counts as an active day.

## JSON Schema for Streak Data

The streak summary is computed at read time and included in the post-mortem report. It is NOT persisted separately.

```json
{
  "current_streak_days": 5,
  "last_rpi_date": "2026-03-12",
  "total_rpi_sessions": 14
}
```

| Field | Type | Description |
|-------|------|-------------|
| `current_streak_days` | integer | Consecutive calendar days with at least 1 rpi-state.json update. Minimum 1 if state file exists. |
| `last_rpi_date` | string (YYYY-MM-DD) | Date of most recent rpi-state.json update. |
| `total_rpi_sessions` | integer | Count of distinct `session_id` values found in rpi state history. Falls back to 1 if only current state exists. |

## Counting Rules

- **Session boundary:** Defined by `session_id` in `rpi-state.json`. Do NOT infer sessions from timestamps.
- **Consecutive days:** A streak breaks if a calendar day (UTC) passes with zero `rpi-state.json` updates.
- **Minimum streak:** If `rpi-state.json` exists and is valid, the minimum streak is 1.
- **Historical data:** If `.agents/rpi/` contains archived state files or `outcomes.jsonl`, use those to compute multi-day streaks. Otherwise, streak = 1 (current session only).

## Fallback Behavior

| Condition | Behavior |
|-----------|----------|
| `rpi-state.json` absent | Silent no-op. No tweetable line in report. |
| `rpi-state.json` unparseable (invalid JSON) | Silent no-op. Log warning to stderr only. |
| `rpi-state.json` missing expected fields | Use available fields, default missing to safe values. |
| No historical state files | Streak = 1, total sessions = 1 (current only). |

## Tweetable Summary Format

When streak data is available, add this line to the TOP of the post-mortem report (before the verdict table):

```
> RPI streak: 5 consecutive days | Sessions: 14 | Last verdict: PASS
```

If no verdict history exists, omit the verdict portion:

```
> RPI streak: 1 consecutive days | Sessions: 1
```
