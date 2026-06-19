#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
allowlist_file="${repo_root}/scripts/lint/codex-residual-allowlist.txt"

if [[ ! -f "${allowlist_file}" ]]; then
  echo "Missing allowlist file: ${allowlist_file}" >&2
  exit 1
fi

cd "${repo_root}"

if [[ ! -d "skills-codex" ]]; then
  echo "skills-codex directory not found; skipping codex runtime section lint."
  exit 0
fi

cross_runtime_file="${repo_root}/scripts/lint/codex-cross-runtime-skills.txt"
is_cross_runtime() {
  [[ -f "${cross_runtime_file}" ]] || return 1
  grep -vE '^[[:space:]]*#|^[[:space:]]*$' "${cross_runtime_file}" | grep -qxF "$1"
}

# parity_only twins are generator-verified (codex-sync byte-exact drift gate);
# only scan BESPOKE (hand-authored) twins for residual Claude mentions.
bespoke_skills="$(python3 -c "import json; d=json.load(open('${repo_root}/skills-codex-overrides/catalog.json')); print(chr(10).join(e['name'] for e in d.get('skills',[]) if e.get('treatment')=='bespoke'))" 2>/dev/null || true)"
is_bespoke() { grep -qxF "$1" <<<"${bespoke_skills}"; }

skill_files=()
while IFS= read -r file; do
  skill_name="$(basename "$(dirname "${file}")")"
  # Only bespoke twins reach the content scan; parity twins are drift-gated.
  is_bespoke "${skill_name}" || continue
  # Cross-runtime bespoke skills may still document non-Codex runtimes — exempt.
  if is_cross_runtime "${skill_name}"; then
    continue
  fi
  skill_files+=("${file}")
done < <(find skills-codex -type f -name "SKILL.md" | sort)

if [[ ${#skill_files[@]} -eq 0 ]]; then
  echo "No SKILL.md files found under skills-codex; skipping codex runtime section lint."
  exit 0
fi

awk -v allowlist_file="${allowlist_file}" '
function normalize_word_boundaries(pattern,    n, i, out, parts) {
  n = split(pattern, parts, /\\b/)
  if (n == 1) {
    return pattern
  }

  out = ""
  for (i = 1; i <= n; i++) {
    out = out parts[i]
    if (i < n) {
      if (i % 2 == 1) {
        out = out "(^|[^[:alnum:]_])"
      } else {
        out = out "([^[:alnum:]_]|$)"
      }
    }
  }

  return out
}

function is_allowlisted(line,    i) {
  for (i = 1; i <= allowlist_count; i++) {
    if (line ~ allowlist_patterns[i]) {
      return 1
    }
  }
  return 0
}

BEGIN {
  while ((getline raw < allowlist_file) > 0) {
    if (raw ~ /^[[:space:]]*#/ || raw ~ /^[[:space:]]*$/) {
      continue
    }
    allowlist_count++
    allowlist_patterns[allowlist_count] = normalize_word_boundaries(raw)
  }
  close(allowlist_file)
}

FNR == 1 {
  runtime_setup_count = 0
  first_runtime_setup_line = 0
}

{
  if ($0 ~ /^[[:space:]]*#{1,6}[[:space:]]+.*[Rr]untime[[:space:]]+[Ss]etup([[:space:][:punct:]].*)?$/) {
    runtime_setup_count++
    if (runtime_setup_count == 1) {
      first_runtime_setup_line = FNR
    } else {
      printf "%s:%d: duplicate runtime setup section (first occurrence at line %d)\n", FILENAME, FNR, first_runtime_setup_line
      violations++
    }
  }

  if ($0 ~ /(^|[^[:alnum:]_])([Cc]laude|[Aa]nthropic|team-create|send-message)([^[:alnum:]_]|$)/) {
    if (!is_allowlisted($0)) {
      printf "%s:%d: residual mixed-runtime marker found: %s\n", FILENAME, FNR, $0
      violations++
    }
  }
}

END {
  if (violations > 0) {
    printf "codex runtime section lint failed with %d violation(s)\n", violations > "/dev/stderr"
    exit 1
  }
  print "codex runtime section lint passed"
}
' "${skill_files[@]}"
