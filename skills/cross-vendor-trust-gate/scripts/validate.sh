#!/usr/bin/env bash
# validate.sh — cross-vendor-trust-gate behavioral validator.
# Confirms this skill's own artifacts are well-formed AND that the real tool it
# documents (trust-gate.sh) is reachable and still exposes the flags this skill
# teaches. bash-3.2-safe, shellcheck-clean, exit 0 on success.
set -u

script_dir="$(cd "$(dirname "$0")" && pwd)"
skill_dir="$(cd "${script_dir}/.." && pwd)"
skill_md="${skill_dir}/SKILL.md"
spec_json="${skill_dir}/skill.spec.json"
skill_name="cross-vendor-trust-gate"

fail() {
  printf '%s validation failed: %s\n' "${skill_name}" "$1" >&2
  exit 1
}

# --- 1. this skill's own artifacts ---
[ -f "${skill_md}" ] || fail "missing SKILL.md"
[ -f "${spec_json}" ] || fail "missing skill.spec.json"

grep -q '^name: cross-vendor-trust-gate$' "${skill_md}" || fail "SKILL.md name frontmatter wrong"
grep -q '^description:' "${skill_md}" || fail "SKILL.md missing description"

# required coverage triggers (cp-cdp): trust, cross-vendor, parity
for term in 'trust' 'cross-vendor' 'parity'; do
  grep -qi "${term}" "${skill_md}" || fail "SKILL.md missing required coverage term: ${term}"
done

# spec.json must be valid JSON; prefer jq, fall back to python3.
if command -v jq >/dev/null 2>&1; then
  jq -e . "${spec_json}" >/dev/null 2>&1 || fail "skill.spec.json is not valid JSON"
elif command -v python3 >/dev/null 2>&1; then
  python3 -m json.tool "${spec_json}" >/dev/null 2>&1 || fail "skill.spec.json is not valid JSON"
else
  fail "no JSON validator available (need jq or python3)"
fi

# --- 2. the real tool this skill operates ---
gate="${TRUST_GATE_PATH:-${HOME}/acfs/skill-pipeline/scripts/trust-gate.sh}"
[ -f "${gate}" ] || fail "trust-gate.sh not reachable at ${gate} (set TRUST_GATE_PATH to override)"
bash -n "${gate}" 2>/dev/null || fail "trust-gate.sh failed bash -n syntax check"

# every flag this skill documents must still exist in the tool's interface.
for flag in '--repo' '--skill-dir' '--codex-dir' '--out' '--require-cross'; do
  grep -q -- "${flag}" "${gate}" || fail "documented flag ${flag} not found in trust-gate.sh — skill drifted from tool"
done

# the three trust levels this skill describes must still be the tool's vocabulary.
for level in 'fresh' 'single-validated' 'cross-validated'; do
  grep -q -- "${level}" "${gate}" || fail "documented trust level '${level}' not found in trust-gate.sh"
done

printf '%s validation passed\n' "${skill_name}"
exit 0
