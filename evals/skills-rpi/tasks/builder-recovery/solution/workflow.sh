#!/bin/bash
set -euo pipefail
r="$1"
repo="$r/repo"
ao="${AO_SKILL_BUILDER_BIN:-/usr/local/bin/ao}"
[[ -f "$repo/skills/recovery-pilot/SKILL.md" ]]
[[ -f "$repo/skills-codex/recovery-pilot" ]]
[[ "$(cat "$repo/skills-codex/recovery-pilot")" = INJECTED_PROJECTION_OBSTRUCTION ]]
rm -- "$repo/skills-codex/recovery-pilot"
python3 - "$repo/skills/recovery-pilot/SKILL.md" <<'PYCODE'
from pathlib import Path
import sys
p=Path(sys.argv[1]);s=p.read_text()
s=s.replace("description: 'TODO: state when this behavior applies.'", "description: 'Inspect current Git changes in a caller-selected repository.'")
s=s.replace('  authoring_state: scaffold\n','')
a=s.index('TODO: State the applicable request');b=s.index('Retained caller note:')
s=s[:a]+"""For a request to inspect current Git changes, run `git status --short` in the caller-selected repository. Report changed paths inline. Do not alter files or Git state. Finish after the command succeeds and the paths are reported; if Git fails or the directory is not a repository, report that error and stop.

"""+s[b:]
p.write_text(s)
PYCODE
"$ao" skills check-source --repo "$repo" --strict skills/recovery-pilot > "$r/out/check.txt" 2>&1
(cd "$repo"; python3 scripts/generate-skill-mesh.py; bash scripts/codex-sync.sh --only recovery-pilot; bash scripts/regen-codex-hashes.sh --only recovery-pilot) > "$r/out/projection.txt" 2>&1
"$ao" skills audit --repo "$repo" --strict "$repo/skills/recovery-pilot" > "$r/out/audit.json"
printf '%s\n' '{"source_retained":true,"failed_stage":"codex_projection","recovery_complete":true,"semantics_evaluated":false,"original_report_preserved":true}' > "$r/out/summary.json"
