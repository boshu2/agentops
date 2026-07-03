#!/usr/bin/env bash
# verify.sh — confirm every skill in the Claude image manifest exists in the corpus.
#
# Reads the slug list from images/claude/manifest.json (core_skills + operator_skills)
# and asserts each skills/<slug>/SKILL.md is present at the agentops repo root.
# Exit 0 iff all present; exit 1 on any missing skill (or a malformed manifest).
#
# Unit 2 (cp-ytub) of the cp-gqu image EPIC. Spec: IMAGE-CORE.md §1 + §2a + §3a.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
manifest="$here/manifest.json"
# repo root = two levels up from images/claude/
repo_root="$(cd "$here/../.." && pwd)"

if [ ! -f "$manifest" ]; then
  echo "FAIL: manifest not found: $manifest" >&2
  exit 1
fi

# Extract every "slug" value from both core_skills and operator_skills.
# Prefer python3 (robust JSON); fall back to grep/sed if python3 is absent.
if command -v python3 >/dev/null 2>&1; then
  slugs="$(python3 -c '
import json, sys
d = json.load(open(sys.argv[1]))
for k in ("core_skills", "operator_skills"):
    for e in d.get(k, []):
        print(e["slug"])
' "$manifest")"
else
  slugs="$(grep -oE '"slug"[[:space:]]*:[[:space:]]*"[^"]+"' "$manifest" \
           | sed -E 's/.*"slug"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
fi

if [ -z "$slugs" ]; then
  echo "FAIL: no slugs parsed from manifest" >&2
  exit 1
fi

missing=0
count=0
while IFS= read -r slug; do
  [ -n "$slug" ] || continue
  count=$((count + 1))
  if [ ! -f "$repo_root/skills/$slug/SKILL.md" ]; then
    echo "MISSING: skills/$slug/SKILL.md" >&2
    missing=$((missing + 1))
  fi
done <<EOF
$slugs
EOF

echo "checked $count skills; missing $missing"
if [ "$missing" -ne 0 ]; then
  echo "FAIL: $missing skill(s) missing from corpus" >&2
  exit 1
fi

# Version guard: the Claude marketplace plugin manifest is the install entrypoint
# for this image. Assert .claude-plugin/plugin.json declares the expected version
# so a stale-version drift (plugin.json behind the release) fails the gate.
EXPECTED_VERSION="${AGENTOPS_EXPECTED_VERSION:-3.2.0}"
plugin_manifest="$repo_root/.claude-plugin/plugin.json"
if [ ! -f "$plugin_manifest" ]; then
  echo "FAIL: Claude plugin manifest not found: $plugin_manifest" >&2
  exit 1
fi
if command -v python3 >/dev/null 2>&1; then
  plugin_version="$(python3 -c '
import json, sys
print(json.load(open(sys.argv[1])).get("version", ""))
' "$plugin_manifest")"
else
  plugin_version="$(grep -oE '"version"[[:space:]]*:[[:space:]]*"[^"]+"' "$plugin_manifest" \
                    | head -1 | sed -E 's/.*"version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
fi
if [ "$plugin_version" != "$EXPECTED_VERSION" ]; then
  echo "FAIL: .claude-plugin/plugin.json version is '$plugin_version', expected '$EXPECTED_VERSION'" >&2
  exit 1
fi
echo "OK: Claude plugin manifest version $plugin_version matches expected $EXPECTED_VERSION"

echo "OK: all $count Claude-image skills present (61 CORE + operator)"
exit 0
