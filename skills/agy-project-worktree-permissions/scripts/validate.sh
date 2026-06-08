#!/usr/bin/env bash
# Behavioral validator for the agy-project-worktree-permissions skill.
# bash-3.2-safe (macOS default), shellcheck-clean. Exit 0 on pass, 1 on fail.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skill_dir="$(cd "${script_dir}/.." && pwd)"
skill_md="${skill_dir}/SKILL.md"
spec_json="${skill_dir}/skill.spec.json"
skill_name="agy-project-worktree-permissions"

fail() {
  printf '%s validation failed: %s\n' "${skill_name}" "$1" >&2
  exit 1
}

# 1. Required files exist.
[ -f "${skill_md}" ] || fail "missing SKILL.md"
[ -f "${spec_json}" ] || fail "missing skill.spec.json"

# 2. skill.spec.json is valid JSON and self-consistent.
if command -v python3 >/dev/null 2>&1; then
  python3 - "${spec_json}" "${skill_name}" <<'PY' || fail "skill.spec.json invalid"
import json, sys
spec = json.load(open(sys.argv[1], encoding="utf-8"))
name = sys.argv[2]
assert spec.get("name") == name, "spec name mismatch"
assert spec.get("skill_api_version") == 1, "skill_api_version must be 1"
trig = spec.get("metadata", {}).get("triggers", [])
for term in ("agy", "worktree", "permissions", "project"):
    assert term in trig, f"spec triggers missing required term: {term}"
assert spec.get("output_contract"), "output_contract empty"
assert spec.get("evidence", {}).get("sources"), "evidence.sources empty"
PY
else
  python -m json.tool "${spec_json}" >/dev/null 2>&1 || fail "skill.spec.json not valid JSON"
fi

# 3. SKILL.md has closed YAML frontmatter with name + the four trigger terms.
head -1 "${skill_md}" | grep -q '^---$' || fail "SKILL.md missing opening frontmatter"
grep -q "^name: ${skill_name}\$" "${skill_md}" || fail "frontmatter name mismatch"
grep -q '^skill_api_version: 1$' "${skill_md}" || fail "missing skill_api_version: 1"
grep -q '^output_contract:' "${skill_md}" || fail "missing output_contract"

# Frontmatter must be closed (>= 2 '---' delimiter lines).
delims="$(grep -c '^---$' "${skill_md}" || true)"
[ "${delims}" -ge 2 ] || fail "frontmatter not closed"

# 4. Required coverage/trigger terms appear in the doc body (case-insensitive).
for term in agy worktree permissions project; do
  grep -qi "${term}" "${skill_md}" || fail "SKILL.md missing coverage term: ${term}"
done

# 5. Behavioral substance: the isolation invariants must actually be documented,
#    not just named. Each of these is the load-bearing mechanic of the skill.
grep -q -- '--add-dir' "${skill_md}" || fail "missing AGY scope primitive (--add-dir)"
grep -qi 'dcg' "${skill_md}" || fail "missing dcg guard reference"
if ! grep -qi 'author' "${skill_md}" || ! grep -qi 'judge' "${skill_md}"; then
  fail "missing author/judge role split"
fi
grep -qiE 'disjoint|non-overlapping|share no path|overlap' "${skill_md}" \
  || fail "missing scope-disjointness invariant"
grep -q 'cp-c6k.3.4' "${skill_md}" || fail "missing cp-c6k.3.4 proof linkage"

printf '%s validation passed\n' "${skill_name}"
