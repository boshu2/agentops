package doctor

// RobotDocs returns the paste-ready agent handbook for the doctor surface, as a
// self-contained Markdown document. It ends with exactly one trailing newline
// and contains no color codes.
func RobotDocs() string {
	return `# ao doctor — Agent Handbook

` + "`ao doctor`" + ` diagnoses and (with ` + "`--fix`" + `) repairs AgentOps workspace state.
Every disk write under ` + "`fix`" + ` and ` + "`undo`" + ` flows through a single audited
chokepoint that backs up verbatim, hashes before and after, and records an
append-only ` + "`actions.jsonl`" + ` line. Diagnose is always read-only.

## CLI surface

` + "```" + `
ao doctor [SUBCOMMAND] [OPTIONS]

SUBCOMMANDS
  diagnose (default)   Run all detectors. Read-only.
  fix                  Run detectors, then apply fixers. Backs up before every mutation.
  undo <run-id>        Restore from .doctor/runs/<run-id>/backups/.  run-id may be "latest".
  explain <finding-id> Expand one finding with full evidence.
  capabilities         Machine-readable contract (use --json).
  health               Cheap one-line liveness summary.
  robot-docs           This handbook.
  gc                   Prune old runs. Requires --yes AND --before <date>.
  ls                   List runs in .doctor/runs/.
  diff                 Show what --fix would change. Read-only.

COMMON FLAGS
  --json / --robot     Stable JSON to stdout.
  --fix                (diagnose) Apply fixers for findings.
  --dry-run            With --fix: print the plan, change nothing.
  --only <id,...>      Scope to detectors/subsystems.
  --skip <id,...>      Inverse of --only.
  --since <run-id>     Diff findings against an earlier run.
  --online             Enable network probes (default: offline-only).
  --quick              Fast-path detectors only (< 200 ms). For pre-commit.
  --severity <P0..P3>  Minimum severity to emit.
  --explain <id>       Same as the explain subcommand.
  --robot-triage       Emit the mega-command triage JSON.
` + "```" + `

## Exit codes

| Code | Meaning |
|------|---------|
| 0  | healthy / fix complete / undo complete |
| 1  | findings present (no --fix) |
| 2  | fix partial |
| 3  | fix failed and rolled back |
| 4  | refused (unsafe state — see finding) |
| 5  | concurrency: another doctor holds the lock |
| 6  | --online required for at least one finding |
| 64 | usage error |
| 66 | no input (not a recognized project) |
| 73 | could not create .doctor/runs/<run-id>/ |
| 74 | filesystem I/O error |

## JSON pointers

- ` + "`ao doctor --json`" + ` — diagnose report: ` + "`schema_version`" + `, ` + "`findings[]`" + `,
  ` + "`summary`" + `, ` + "`exit_code`" + `, ` + "`next_steps`" + `.
- ` + "`ao doctor --fix --json`" + ` — adds ` + "`actions_taken`" + `, ` + "`actions_jsonl_path`" + `,
  ` + "`backups_dir`" + `, ` + "`undo_command`" + `.
- ` + "`ao doctor capabilities --json`" + ` — detectors, fixers, exit codes, env vars,
  write scopes, schema URLs.
- Run artifacts live under ` + "`.doctor/runs/<ISO8601>__<run-id>/`" + `:
  ` + "`report.json`" + `, ` + "`report.md`" + `, ` + "`actions.jsonl`" + `, ` + "`backups/`" + `,
  ` + "`quarantine/`" + `, ` + "`stderr.log`" + `, ` + "`stdout.json`" + `, ` + "`undo.sh`" + `.

## Canonical examples

1. Healthy workspace:
   ` + "`ao doctor`" + ` → prints checks + findings, exit 0.
2. Broken — see findings as JSON:
   ` + "`ao doctor --json | jq '.findings'`" + ` → exit 1.
3. Broken — plan a fix without touching disk:
   ` + "`ao doctor --dry-run --fix`" + ` → prints "[dry-run] would mutate ...".
4. Broken — apply fixes with backups:
   ` + "`ao doctor --fix`" + ` → writes ` + "`actions.jsonl`" + `, emits ` + "`undo_command`" + `.
5. Undo the most recent fix run:
   ` + "`ao doctor undo latest`" + ` → restores byte-identical from backups.
   Inspect a single finding: ` + "`ao doctor explain <finding-id>`" + `.

## Things ao doctor will NEVER do

- Delete a file during ` + "`diagnose`" + `, ` + "`fix`" + `, or ` + "`undo`" + `. "Delete" is always a
  ` + "`Rename`" + ` into the run's ` + "`quarantine/`" + ` directory — the user decides what to
  remove later.
- Run destructive shell (` + "`rm -rf`" + `, ` + "`git reset --hard`" + `, ` + "`git clean -f`" + `).
- Write outside the documented ` + "`write_scopes`" + ` (see ` + "`capabilities --json`" + `).
  An out-of-scope write refuses with exit 4.
- Start, stop, signal, or restart any process.
- Edit ` + "`.git/**`" + `, shell rc files, ` + "`$PATH`" + `, systemd units, user config YAML,
  or the ` + "`ao`" + ` binary itself.
- Rewrite or "auto-correct" user-authored JSON/YAML — corrupt user files are
  backed up and quarantined, never guessed at.

**The one deletion exception:** ` + "`ao doctor gc --before <date> --yes`" + ` prunes
old ` + "`.doctor/runs/<run-id>/`" + ` directories (and their ` + "`backups/`" + `). It is
gated on an explicit cutoff plus ` + "`--yes`" + `, so the user knowingly accepts the
loss of undo capability for those pruned runs. It never deletes silently.

## Learn more

` + "`ao doctor capabilities --json`" + ` is the machine-readable source of truth.
`
}
