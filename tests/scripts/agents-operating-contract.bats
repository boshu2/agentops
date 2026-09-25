#!/usr/bin/env bats

@test "root contract routes native execution and optional RPI" {
  repo_root="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  contract="$repo_root/AGENTS.md"
  [ -f "$contract" ]

  for required in \
    "operations layer for agentic engineering" \
    "federated integration graph" \
    "zero mandatory AgentOps skills" \
    "Lean RPI operating charter" \
    "Accepted intent -> native implementation and checks -> fresh independent judgment -> finish" \
    "Persist machine evidence only for a caller request or declared consumer" \
    "It owns no aggregate retry controller" \
    "fresh independent judgment" \
    "unchanged accepted outcome and scope; acceptance changes need caller authority" \
    "Implement repairs ordinary known defects directly" \
    "use at most one bounded fresh helper" \
    "incident within authority and real remaining bounds" \
    "Cancellation, refusal and spent hard time/cost/quota skip help" \
    "compaction, helpers and new subjects never renew real limits" \
    "docs/architecture/rpi-traversal.md"; do
    grep -Fq -- "$required" "$contract"
  done

  # Native controls own aggregate allowances; the lean charter keeps direct
  # repair, bounded help and fresh judgment without adding delivery machinery.
  ! grep -Eq 'ao land|next-work|Plan-Pawl' "$contract"
}

@test "every relative file link in the root contract resolves" {
  repo_root="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  run python3 - "$repo_root" <<'EOF'
import re
import sys
from pathlib import Path

root = Path(sys.argv[1])
text = (root / "AGENTS.md").read_text(encoding="utf-8")
targets = re.findall(r"\]\(([^)\s]+)\)", text)
local = [t.split("#", 1)[0] for t in targets if "://" not in t and not t.startswith("#")]
missing = [t for t in local if t and not (root / t).exists()]
assert local, "AGENTS.md links no repository files"
if missing:
    print("missing:", missing)
    raise SystemExit(1)
EOF
  [ "$status" -eq 0 ]
}
