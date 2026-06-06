# AGY loop workflow + hooks + subagents (paste targets)

Concrete bodies referenced by `SKILL.md`. AGY-native. Adjust `br`/`bd` to the repo's tracker.

## 1. `.agents/workflows/agy-loop.md` — the slash workflow

```markdown
# /agy-loop — claim -> work -> validate -> close -> persist

> One bead per invocation. The author NEVER judges its own work (Rule 1/3).

## Steps
1. CLAIM: `br ready` (or `bd ready`). Pick exactly one bead. `br update <id> --claim`.
   Read its acceptance examples (Given/When/Then) and declared write scope.
2. WORK: implement the smallest vertical slice. TDD per slice. Stay strictly
   inside the declared write scope (Rule 4). Record evidence onto the bead as you go.
3. VALIDATE: dispatch the `validator` subagent with a CLEAN context. Pass it only
   the bead id, the acceptance examples, and the diff/proof surfaces — NOT your
   reasoning. It re-derives PASS/FAIL from evidence alone. On tie/ambiguity,
   dispatch the `tie-break` subagent. (Rules 1, 2, 3.)
4. CLOSE: only if the validator returns PASS with every Given/When/Then mapped to
   a captured proof. `br close <id>`. Otherwise `br update <id>` with the gap and
   reopen — do NOT close.
5. PERSIST: write evidence + learning to the bead, append to `.agents/ratchet/`,
   and emit an AGY artifact (the validator verdict). THEN commit the scoped change.
   The turn is not done until persist completes (Rule 5).
6. STOP. One bead per turn.
```

Invoke headless: `agy -p "/agy-loop <intent>"` (use `--print-timeout` for long slices).

## 2. Subagent definitions (author != judge)

Define as AGY subagents (clean-context children). Minimum two:

```markdown
# subagent: validator
Role: independent judge. Input: bead id + acceptance examples + proof surfaces ONLY.
Do NOT trust or read the author's narrative. For each Given/When/Then, confirm a
captured proof surface demonstrates it. Output: PASS or FAIL + the evidence map.
Never PASS on "looks good". If a proof is missing, FAIL with the specific gap.
```

```markdown
# subagent: tie-break
Role: arbiter when the validator's verdict is ambiguous or two runs disagree.
Re-run the acceptance examples against the proof surfaces from scratch. Output a
single binding PASS/FAIL + one-line rationale.
```

A worktree-isolated agent (AGY worktree mode) is an acceptable alternative isolation for the validator.

## 3. Hook stanza — extend `~/.gemini/settings.json`

The file already has a `BeforeTool` `dcg` guard. MERGE; do not overwrite. Add a
close-guard (Rule 2/5) and a scope-guard (Rule 4):

```json
{
  "hooks": {
    "BeforeTool": [
      { "matcher": "run_shell_command",
        "hooks": [
          { "name": "dcg", "type": "command", "command": "/Users/bo/.local/bin/dcg", "timeout": 5000 },
          { "name": "flywheel-scope-guard", "type": "command",
            "command": "/Users/bo/acfs/staged-skills/agy-rules-workflows/scripts/scope-guard.sh", "timeout": 5000 }
        ]
      }
    ],
    "AfterTool": [
      { "matcher": "run_shell_command",
        "hooks": [
          { "name": "flywheel-close-guard", "type": "command",
            "command": "/Users/bo/acfs/staged-skills/agy-rules-workflows/scripts/close-guard.sh", "timeout": 5000 }
        ]
      }
    ]
  }
}
```

- `scope-guard.sh`: inspect a `git commit`/`git add` command; reject paths outside the claimed bead's write scope.
- `close-guard.sh`: inspect a `br close`/`bd close`; reject if the bead has no captured evidence linked.

Both ship with this skill under `scripts/` as executable policy stubs — wired and safe (they no-op until you implement the marked tracker-read section, then they ENFORCE). The `command` paths above point at this skill's staged location; re-point them after the skill is installed to its final path.

## 4. Optional `plugin.json` (bundle for `agy plugin install`)

```json
{
  "name": "agy-rules-workflows",
  "version": "0.1.0",
  "description": "Flywheel laws + operating loop as AGY-native rules, workflow, hooks, subagents.",
  "rules": [".agents/rules/flywheel-laws.md"],
  "workflows": [".agents/workflows/agy-loop.md"],
  "subagents": ["validator", "tie-break"],
  "hooks": "hooks.json"
}
```

Validate with `agy plugin validate <path>` before `agy plugin install`.

## Cross-harness law parity

Same five laws, different skin per turnout:

| Law | Claude | Codex | AGY (this skill) |
|-----|--------|-------|------------------|
| Author != judge | subagent validator | codex-team reviewer | `validator` subagent / worktree |
| Evidence-gated close | bead-crank close-guard | gate script | `close-guard.sh` AfterTool hook |
| No self-grade | separate judge | separate reviewer | rule + validator subagent |
| Scoped commits | scope check | scope check | `scope-guard.sh` BeforeTool hook |
| Persist before done | ratchet + artifact | ratchet | bead + ratchet + AGY artifact |
