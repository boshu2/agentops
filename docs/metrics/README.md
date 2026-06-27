# Repo metrics harness

A longitudinal record of who looks at this repo and what they open. GitHub's
traffic API is a **14-day rolling window, top-10 only** — every read evaporates.
This harness snapshots it on a schedule so the signal accumulates into a trend
instead of a thin two-week guess.

## Files

- `traffic.jsonl` — one JSON record per capture (append-only). Each line carries
  the date, stars/forks/watchers/open-issues, 14-day views and clones (total +
  unique), the top-10 viewed paths, and the top-10 referrers.
- `../../scripts/capture-repo-metrics.sh` — the capture. Fails closed (writes no
  partial record) if `gh` is missing/unauthenticated or any endpoint fails.

## Run it

```bash
scripts/capture-repo-metrics.sh              # append a snapshot for boshu2/agentops
scripts/capture-repo-metrics.sh --dry-run    # preview the record, write nothing
scripts/capture-repo-metrics.sh owner/repo   # any repo you have push access to
```

Traffic endpoints need **push access** to the repo (that's a GitHub API rule, not
ours). Schedule it however you run out-of-session work — `cron`, `launchd`, or the
AgentOps substrate. AgentOps ships no daemon (ADR-0009); the script is the
artifact, the cadence is yours. Weekly is plenty (the window is 14 days, so weekly
captures overlap and lose nothing).

## Reading it — the one trap

**`clones` is not users.** It is dominated by CI, package mirrors, and scrapers —
a real window showed ~43k clones against ~1k views. Never cite clones as adoption.
Rank human interest by:

1. **unique PATH views** (`top_paths[].uniques`) — what people actually open.
2. **fork velocity** (`forks` delta between snapshots) — who clones to *use*.
3. **stars** delta — attention, not commitment (watchers is the commitment tell).

The record bakes this caveat into every line as `clones_note` so a future reader
can't miss it.

## Why this is part of "rock solid"

Distribution (stars) is not the same as value delivered. None of these numbers
measure whether the verification membrane caught anyone's bad "done" — they
measure attention and intent-to-try. This harness turns a one-shot screenshot
into the trend you'd need before claiming retention or value, and keeps the next
"what do people use?" answer grounded in accumulated data rather than a 14-day
snapshot.
