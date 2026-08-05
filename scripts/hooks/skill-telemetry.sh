#!/usr/bin/env bash
# skill-telemetry.sh — PostToolUse hook: append one JSONL record per Skill
# invocation to .agents/ao/skill-telemetry.jsonl.
#
# WHY: the eval architecture (docs/architecture/eval-architecture.md, D11)
# needs the join "skill X was loaded in session S" x "session S outcome" for
# the observational effectiveness signal. This file existed but was dead (one
# record); this hook revives it. Repo-local by design — wired via
# .claude/settings.json, NOT the shipped hooks/hooks.json, so plugin users
# never inherit telemetry.
#
# CONTRACT: never blocks, never fails the tool call (always exit 0); appends
# only; caps args at 200 chars (metadata, not payload capture). The hook JSON
# arrives on stdin, so the python body is passed via -c (a heredoc would
# consume stdin and starve json.load).
set -uo pipefail

python3 -c "
import json, sys, os, datetime
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
if d.get('tool_name') != 'Skill':
    sys.exit(0)
ti = d.get('tool_input') or {}
rec = {
    'ts': datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'),
    'session_id': d.get('session_id', ''),
    'skill': ti.get('skill', ''),
    'args': (ti.get('args') or '')[:200],
    'source': 'posttooluse-hook',
}
root = os.environ.get('CLAUDE_PROJECT_DIR') or os.getcwd()
path = os.path.join(root, '.agents', 'ao', 'skill-telemetry.jsonl')
os.makedirs(os.path.dirname(path), exist_ok=True)
with open(path, 'a', encoding='utf-8') as f:
    f.write(json.dumps(rec, ensure_ascii=False) + chr(10))
" 2>/dev/null || true
exit 0
