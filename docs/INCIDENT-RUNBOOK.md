# AgentOps Incident Runbook — Consumer Recovery

> **Audience:** Anyone responding to a broken AgentOps installation.
> **Assumption:** You are stressed and need copy-pasteable commands. Each section is self-contained.

---

## Table of Contents

1. [Emergency Kill Switches](#1-emergency-kill-switches)
2. [Scenario A: Broken Skills After Update](#2-scenario-a-broken-skills-after-update)
3. [Scenario B: Evolve Pushed Bad Code to Main](#3-scenario-b-evolve-pushed-bad-code-to-main)
4. [Scenario C: Skills Not Loading / CI Gate Failing](#4-scenario-c-skills-not-loading--ci-gate-failing)
5. [Rollback Options](#5-rollback-options)
6. [Root Cause Analysis](#6-root-cause-analysis)
7. [Prevention Checklist](#7-prevention-checklist)

---

## 1. Emergency Kill Switches

**Do these FIRST if sessions are broken. Restore functionality, then investigate.**

```bash
# Stop evolve from running (persistent across sessions)
mkdir -p ~/.config/evolve
echo "incident $(date -Iseconds)" > ~/.config/evolve/KILL
```

> **AgentOps 3.0 is hookless.** A default install ships **zero** hooks — nothing auto-runs at
> session start, so there is no global "disable hooks" recovery step to take. Orientation is
> explicit (`ao session bootstrap`, `ao inject`) and CI (`.github/workflows/validate.yml`) is the
> authoritative gate. If you authored your own hooks via the `hooks-authoring` skill, see the
> note under [Scenario C](#4-scenario-c-skills-not-loading--ci-gate-failing) for how to disable them.

---

## 2. Scenario A: Broken Skills After Update

**Symptom:** Consumer ran the install script and now Claude sessions are broken — skills don't load, `ao` subcommands error, or skill invocations fail.

### Triage (< 5 min)

```bash
# 1. Check what version was installed
cat ~/.claude/skills/agentops/plugin.json 2>/dev/null | jq -r '.version'
# Or check the marketplace cache
cat ~/.claude/plugins/marketplaces/agentops-marketplace/plugin.json 2>/dev/null | jq -r '.version'

# 2. Check if skills are symlinks (known failure mode)
ls -la ~/.claude/skills/ | head -20

# 3. Confirm the ao CLI is on PATH and runs
which ao && ao status
```

### Fix: Reinstall from a known-good version

```bash
# Remove broken installation
rm -rf ~/.claude/skills/agentops

# Remove any symlinks (known failure: installer cannot write through symlinks)
find ~/.claude/skills -maxdepth 1 -type l -delete

# Reinstall from latest Claude plugin
claude plugin marketplace update agentops-marketplace
claude plugin install agentops@agentops-marketplace

# OR reinstall marketplace + plugin
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace
```

### Fix: Nuke and reinstall (if pinning doesn't work)

```bash
# Nuclear option: remove everything and reinstall
rm -rf ~/.claude/skills/agentops
rm -rf ~/.claude/plugins/marketplaces/agentops-marketplace

# Reinstall plugin
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace
```

### Verify the fix

```bash
# Confirm skills resolve and the CLI runs
ls ~/.claude/skills/agentops/ | head
ao status
```

---

## 3. Scenario B: Evolve Pushed Bad Code to Main

**Symptom:** `/evolve` ran autonomously, committed code that breaks builds, tests, or other skills. The regression gate failed to catch it, or evolve committed before the gate ran.

### Triage (< 5 min)

```bash
# 1. Stop evolve immediately
mkdir -p ~/.config/evolve
echo "incident: bad code on main $(date -Iseconds)" > ~/.config/evolve/KILL

# Also set local stop in the repo
echo "emergency stop" > .agents/evolve/STOP

# 2. Check what evolve did
cat .agents/evolve/cycle-history.jsonl 2>/dev/null   # cycle outcomes
cat .agents/evolve/session-summary.md 2>/dev/null     # session wrap-up
ls -lt .agents/evolve/fitness-*.json 2>/dev/null      # fitness snapshots

# 3. Find evolve's commits
git log --oneline -20   # look for evolve/rpi commit messages
```

### Revert evolve's changes

```bash
# Find the last good commit (before evolve ran)
# Look at fitness snapshots for session_start_sha
jq -r '.cycle_start_sha' .agents/evolve/fitness-0.json 2>/dev/null

# Or find it manually
git log --oneline -30 | less

# Revert everything after the known-good SHA
GOOD_SHA="<paste sha here>"
git revert --no-commit ${GOOD_SHA}..HEAD
git commit -m "revert: evolve incident — rolling back to ${GOOD_SHA}"

# Verify
cd cli && go build ./cmd/ao && go test ./...
./tests/run-all.sh
```

### If evolve is mid-run (still executing)

```bash
# Kill switch stops it at the next cycle boundary
mkdir -p ~/.config/evolve
echo "emergency stop" > ~/.config/evolve/KILL

# If it's in a tmux session, also kill the process
# Find the session
tmux list-sessions | grep -i evolve
# Kill it
tmux kill-session -t <session-name>
```

### Re-enable evolve after fix

```bash
rm ~/.config/evolve/KILL
rm .agents/evolve/STOP 2>/dev/null
```

---

## 4. Scenario C: Skills Not Loading / CI Gate Failing

**Symptom:** A skill won't invoke (Claude reports the skill is missing or malformed), or a PR is blocked because a CI gate in `.github/workflows/validate.yml` fails. AgentOps 3.0 is hookless — there is no session-start hook to misfire, so a broken skill or a failing gate is the usual culprit.

### Triage (< 5 min)

```bash
# 1. Confirm the skill exists and has a valid manifest
ls ~/.claude/skills/agentops/skills/<skill-name>/SKILL.md
# Frontmatter must parse — a malformed SKILL.md silently fails to load
head -20 ~/.claude/skills/agentops/skills/<skill-name>/SKILL.md

# 2. If a push is rejected, see which gate failed (the local pre-push Go gate is the authority; CI is a backstop)
gh pr checks <pr-number>

# 3. Reproduce the failing gate locally (run the FULL job, not a subset)
cat .github/workflows/validate.yml | grep -n "run:" | head -40   # find the step
bash scripts/<failing-gate>.sh                                    # run it
```

### Common failures

**Malformed or unregistered skill:**
```bash
# Skills source of truth is skills/ in the repo; the installed copy lives under
# ~/.claude/skills/agentops/. A skill that isn't in the registry won't be offered.
ls skills/<skill-name>/SKILL.md          # source of truth
bash scripts/check-registry-drift.sh     # catches missing-from-registry skills
```

**CI gate disagreement (docs drifted from executable behavior):**
```bash
# Contracts/counts/context-map gates fail when a generated surface is stale.
# Regenerate the derived surfaces, then re-run the gate.
bash scripts/regen-all.sh 2>/dev/null || true
bash scripts/validate-context-map-drift.sh
```

**Missing binary (ao, jq):**
```bash
which ao    # CLI must be installed and on PATH
which jq    # required by several validation scripts
```

### Optional: user-authored hooks

AgentOps ships **no** hooks by default. If you opted in and authored your own hooks via the
`hooks-authoring` skill, hook troubleshooting applies to **those** files only — not to any shipped
default. For backward-compat, AgentOps-authored hooks still honor the `AGENTOPS_HOOKS_DISABLED=1`
environment variable as an opt-out:

```bash
# Only relevant if you authored your own hooks. Disables AgentOps-aware hooks for the session.
export AGENTOPS_HOOKS_DISABLED=1
# Then debug your own hook scripts wherever you installed them.
```

---

## 5. Rollback Options

### Option A: Reinstall latest Claude plugin

```bash
# Refresh marketplace source and reinstall plugin
claude plugin marketplace update agentops-marketplace
claude plugin install agentops@agentops-marketplace
```

### Option B: Pin to a specific commit

```bash
# Clone and install from a known-good commit
cd /tmp
git clone https://github.com/boshu2/agentops.git agentops-recovery
cd agentops-recovery
git checkout <known-good-sha>

# Copy skills manually
rm -rf ~/.claude/skills/agentops
cp -r . ~/.claude/skills/agentops
```

### Option C: Sync from marketplace cache

```bash
# The marketplace cache may have a working version
cd ~/.claude/plugins/marketplaces/agentops-marketplace
git log --oneline -10   # find a good state
git checkout <good-sha>

# Then reinstall plugin from marketplace source
claude plugin install agentops@agentops-marketplace
```

### Option D: Nuclear reinstall

```bash
# Remove everything AgentOps-related
rm -rf ~/.claude/skills/agentops
rm -rf ~/.claude/plugins/marketplaces/agentops-marketplace
find ~/.claude/skills -maxdepth 1 -type l -delete   # remove symlinks

# Clear any cached state
rm -rf ~/.config/evolve/KILL 2>/dev/null

# Fresh install (plugin path)
claude plugin marketplace add boshu2/agentops
claude plugin install agentops@agentops-marketplace

# Verify
cat ~/.claude/skills/agentops/.claude-plugin/plugin.json | jq -r '.version'
ao status
```

---

## 6. Root Cause Analysis

After restoring service, investigate what went wrong.

### Was it an evolve regression?

```bash
# Check evolve history
cat .agents/evolve/cycle-history.jsonl | jq -s '.'

# Check fitness snapshots for regressions
for f in .agents/evolve/fitness-*-post.json; do
  echo "--- $f ---"
  jq '[.goals[] | select(.result == "fail") | .id]' "$f" 2>/dev/null
done

# Check GOALS.yaml for broken check commands
cat GOALS.yaml
```

### Was it a bad commit?

```bash
# Use git bisect to find the breaking commit
git bisect start
git bisect bad HEAD
git bisect good <last-known-good-sha>

# For each step, run the relevant test
./tests/run-all.sh && git bisect good || git bisect bad

# When done
git bisect reset
```

### Was it a skill or CI-gate failure?

```bash
# AgentOps 3.0 is hookless — the local pre-push Go gate (ao gate check) is the authority; CI is a backstop. Reproduce it locally.
# Check which gate failed on the PR
gh pr checks <pr-number>

# Run the omnibus validation locally (the same job CI runs)
bash scripts/pre-push-gate.sh

# Check for recent skill / generated-surface changes
git log --oneline -20 -- skills/ docs/

# Run full test suite
./tests/run-all.sh
```

### Was it a dependency issue?

```bash
# Check if ao CLI is working
ao status
ao flywheel status

# Check Go CLI builds
cd cli && go build ./cmd/ao && go test ./...

# Check for missing system tools
for cmd in jq shellcheck git ao; do
  which "$cmd" >/dev/null 2>&1 && echo "OK: $cmd" || echo "MISSING: $cmd"
done
```

---

## 7. Prevention Checklist

### Before releasing a new version

- [ ] `./tests/run-all.sh` passes (all tiers)
- [ ] `./tests/smoke-test.sh` passes
- [ ] `cd cli && go build ./cmd/ao && go test -race ./...` clean
- [ ] CI validation passes locally: `bash scripts/pre-push-gate.sh`
- [ ] Generated/derived surfaces are in sync: `bash scripts/check-registry-drift.sh` and `bash scripts/validate-context-map-drift.sh` clean
- [ ] Plugin and marketplace versions match: `jq -r '.version' .claude-plugin/plugin.json` equals `jq -r '.metadata.version' .claude-plugin/marketplace.json`

### Before running evolve

- [ ] GOALS.yaml check commands all work: run each `check:` value manually
- [ ] Evolve kill switch is clear: `test ! -f ~/.config/evolve/KILL && echo "clear"`
- [ ] Git working tree is clean: `git status --porcelain` is empty
- [ ] Know the current HEAD: `git rev-parse HEAD` (save this for revert)
- [ ] Set a reasonable cycle cap: `--max-cycles=3` for first run

### After evolve completes

- [ ] Review `cycle-history.jsonl` for any regressions
- [ ] Run `./tests/run-all.sh` manually (don't trust evolve's self-assessment)
- [ ] Check `git log --oneline -20` for reasonable commit messages
- [ ] Run `git diff <pre-evolve-sha>..HEAD --stat` to see total scope of changes

### Optional hook authoring rules (only if you opt in)

AgentOps ships no hooks. If you author your own via the `hooks-authoring` skill, the safe pattern is:

- Honor `AGENTOPS_HOOKS_DISABLED=1` at the top so operators can opt out
- Fail open (`exit 0` on error, never `set -e`) — a broken hook must never wedge a session
- Guard all external commands: `command -v <tool> >/dev/null 2>&1 && ...`
- If a hook emits JSON, validate it with `jq .` before committing

---

## Quick Reference Card

```
STOP EVOLVE:         echo "stop" > ~/.config/evolve/KILL
CHECK CI GATE:       gh pr checks <pr-number>
RUN GATE LOCALLY:    bash scripts/pre-push-gate.sh
REINSTALL:           claude plugin marketplace update agentops-marketplace && claude plugin install agentops@agentops-marketplace
NUCLEAR REINSTALL:   rm -rf ~/.claude/skills/agentops ~/.claude/plugins/marketplaces/agentops-marketplace && claude plugin marketplace add boshu2/agentops && claude plugin install agentops@agentops-marketplace
REVERT EVOLVE:       git revert --no-commit <good-sha>..HEAD && git commit -m "revert: evolve incident"
VERSION CHECK:       jq -r '.version' ~/.claude/skills/agentops/.claude-plugin/plugin.json
```
