# cp-d8u8 Goals-Rot Fix — Evidence

**Branch:** fix/cp-d8u8-goals-rot  
**Repo:** /Users/bo/dev/agentops  
**Worktree:** /tmp/cp-d8u8-wt  
**Date:** 2026-06-10

---

## Fix 1 — hook-preflight gate: REMOVED (genuinely dead)

**Finding:** `scripts/validate-hook-preflight.sh` was deleted in commit
`e431339c4` ("feat(hookless)!: remove all hooks — skills + CLI only") on
2026-05-24. That commit removed the entire hooks product surface (AgentOps 3.0
goes hookless). The script never had a prior git history under that exact path
— it was created and then deleted as part of the hook infrastructure teardown.

**Decision:** REMOVED from GOALS.md per the prune-stale-gates discipline.
No successor script exists (checked `ls scripts/ | grep -i hook` and
`git log --all -- scripts/validate-hook-preflight.sh`).

**Baseline:** gate was failing with `exit 1` (script not found).  
**After:** gate row removed; GOALS.md now has 28 goals (down from 29).

---

## Fix 2 — skill-frontmatter gate: VALIDATOR FORK RECONCILED

### Diagnosis (verbatim)

The inline check in GOALS.md was:
```bash
bash -c 'for f in skills/*/SKILL.md; do
  head -5 "$f" | grep -q "^---"
  && head -10 "$f" | grep -q "^name:"
  && head -10 "$f" | grep -q "^description:"
  || { echo FAIL:$f; exit 1; }; done'
```

`scripts/validate-skill-frontmatter.sh` uses schema validation
(`schemas/skill-frontmatter.v2.schema.json`, which marks `name` and
`description` as `required`), extracting the full YAML frontmatter block
between the first two `---` delimiters — position-agnostic.

**The fork:** the inline check fails any skill whose `description:` field
appears after line 10. Two skills trigger this:

- `skills/bead-tracker-migration/SKILL.md`: `description:` is on line 12
  (pushed down by a 4-line `metadata:` block at lines 6–11)
- `skills/ripgrep-search-discipline/SKILL.md`: `description:` is on line 19
  (pushed down by a 9-line `metadata:` + `context:` block)

Both files have correct, schema-valid frontmatter. The script passes them:
```
OK    skills/bead-tracker-migration/SKILL.md  (missing: consumes produces context_rel)
OK    skills/ripgrep-search-discipline/SKILL.md  (missing: consumes produces context_rel)
```

**Is the script at-least-as-strict?** Yes. The JSON schema marks `name`,
`description`, `hexagonal_role`, and `practices` as `required`. The inline
check only tested for `name` + `description` in head-10. The script is
STRICTER (also requires `hexagonal_role` and `practices`; does full YAML
parse; catches YAML syntax errors).

**Resolution:** replaced the inline check command with
`bash scripts/validate-skill-frontmatter.sh` — one validator, one truth.
The script description is updated to note the reconciliation.

### Before/After

**Before:** `skill-frontmatter` gate — FAIL (2 false-failing skills, position-based check)  
**After:** `skill-frontmatter` gate — PASS (166/166 OK, schema-validated)

---

## Fix 3 — skills/bead-tracker-migration/SKILL.md: NO CODE CHANGE NEEDED

The file's frontmatter is already schema-valid. The `description:` is on
line 12 (inside the frontmatter block, before the closing `---` on line 20).
The only reason it failed was the stale inline check in GOALS.md (Fix 2).
After Fix 2, the skill passes both validators.

---

## Fix 4 — ao defrag: RAN, compile-freshness NOW PASSES

**Command run:**
```
ao defrag --prune --dedup
```

**Output:**
```
Defrag report: 2026-06-10T15:56:11Z
  Prune: 0 total learnings, 0 stale, 0 orphans
  Dedup: 0 checked, 0 duplicate pairs
```

**Artifact written:** `.agents/defrag/latest.json` and `.agents/defrag/2026-06-10.json`

**compile-freshness gate:** PASS (was FAIL before defrag)
```
PASS: Compile health OK (defrag 0h ago, stale=0/5)
```

### Why the nightly automation didn't run it

The nightly GitHub Actions workflow (`nightly.yml`) DOES run `ao defrag --prune
--dedup`, but writes to `${RUNNER_TEMP}/knowledge-cycle/compile/defrag` — NOT to
`.agents/defrag/`. That path is ephemeral to the CI runner and is never committed
back to the repo.

The `compile-freshness` gate checks `.agents/defrag/latest.json` (the local path).
Since `.agents/` is listed in `.gitignore` (line 48: `/.agents/`) and
`/.agents/defrag/` is NOT force-tracked (the force-include list covers only
`.agents/rpi/` and `.agents/nightly/`), the runtime artifact only exists on a
machine that has run `ao defrag` locally.

This is by design — GOALS.md tags `compile-freshness` as `runtime-artifact`:
> "such flips do not propagate across environments"

The GOALS.md description says: "Operator refresh: run `ao defrag`".

**Root cause of the 371h gap:** no one ran `ao defrag` locally on this checkout.
The nightly CI run doesn't write to the monitored path. This is a local ops gap,
not a scheduler bug. The `defrag-weekly.yaml` example schedule (`ao schedule add
--file examples/schedules/defrag-weekly.yaml`) would auto-run it, but `ao schedule`
is not implemented in this CLI version (`Error: unknown command "schedule" for "ao"`).

**Finding for cp-m8md (scheduling lane):** the local `ao schedule` command doesn't
exist yet. The example schedule file exists but cannot be registered. Until `ao
schedule` is implemented, the operator must run `ao defrag` manually (or add a
launchd/cron entry). This is out of scope for cp-d8u8.

---

## Gate Count: Before/After

| Metric | Before | After |
|--------|--------|-------|
| Total gates | 29 | 28 (hook-preflight removed) |
| Passing | 25 | 27 (skill-frontmatter + compile-freshness added) |
| Failing | 4 | 2 (flywheel-compounding + go-complexity-ceiling remain, out of scope) |
| Score | 85.8% | ~93% (expected) |

**Remaining failures (out of scope):**
- `flywheel-compounding`: requires multi-session citation activity (tagged `long-cycle, corpus-state`)
- `go-complexity-ceiling`: 2 functions exceed CC threshold (CC=23 for `reconcileReachAlwaysErrorBudgets`, CC=22 for `sealSkillEdit`)
