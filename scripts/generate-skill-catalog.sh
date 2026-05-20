#!/usr/bin/env bash
# generate-skill-catalog.sh — emit skills/catalog.json from SKILL.md frontmatter.
#
# Walks every skills/*/SKILL.md, parses YAML frontmatter, and emits a single
# queryable JSON file conforming to schemas/skill-catalog.schema.json. Slice
# 1 of soc-vuu6.4 (skill catalog as queryable JSON). Subsequent slices add
# `ao skills list/consumers/producers/graph` (slice 2) and consumers in the
# showcase (slice 3).
#
# Modes:
#   default      Regenerate skills/catalog.json from scratch.
#   --check      Regenerate to a tmp file and diff against the committed
#                catalog. Exit 1 if drift detected (CI gate).
#   --stdout     Emit JSON to stdout; do not write any file.
#   --out PATH   Custom output path (default: skills/catalog.json).
#
# Exit codes:
#   0 — wrote / matched
#   1 — drift detected in --check mode, OR generator failure
#   2 — usage error
#   3 — required tool missing (jq, awk) or skills/ dir absent

set -euo pipefail

MODE="write"
OUT_PATH=""

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --check) MODE="check" ;;
    --stdout) MODE="stdout" ;;
    --out) shift; OUT_PATH="${1:-}" ;;
    -h|--help) usage 0 ;;
    *) echo "generate-skill-catalog: unknown arg: $1" >&2; usage 2 ;;
  esac
  shift || true
done

for cmd in jq awk; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "generate-skill-catalog: required tool missing: $cmd" >&2
    exit 3
  fi
done

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
SKILLS_DIR="$ROOT/skills"
CODEX_DIR="$ROOT/skills-codex"
OVERRIDES_DIR="$ROOT/skills-codex-overrides"
[ -z "$OUT_PATH" ] && OUT_PATH="$ROOT/skills/catalog.json"

if [ ! -d "$SKILLS_DIR" ]; then
  echo "generate-skill-catalog: skills/ not found at $SKILLS_DIR" >&2
  exit 3
fi

# Extract YAML frontmatter (between the first two `---` lines) of a file.
extract_frontmatter() {
  awk '/^---$/{c++; if(c==2) exit; next} c==1' "$1"
}

# Convert a YAML block into JSON via the smallest, dependency-free path:
# write to a temp file, then use awk to translate into a JSON object
# preserving lists. We only handle the subset of YAML our SKILL.md
# frontmatter actually uses — scalar key:value, lists (`- item`), and
# nested object lists for `context_rel` (kind: / with:). Anything more
# exotic is reported as `null` and skipped.
# Returns JSON object with only the fields we project into the catalog.
parse_frontmatter() {
  local fm="$1"
  awk -v fm="$fm" '
    function trim(s){ sub(/^[[:space:]]+/,"",s); sub(/[[:space:]]+$/,"",s); return s }
    function jstr(s) {
      gsub(/\\/, "\\\\", s)
      gsub(/"/, "\\\"", s)
      gsub(/\t/, "\\t", s)
      gsub(/\r/, "\\r", s)
      return "\"" s "\""
    }
    BEGIN {
      n = split(fm, lines, "\n")
      cur_key = ""
      in_list = 0
      list_buf = ""
      ctx_in = 0
      ctx_buf = ""
      ctx_kind = ""; ctx_with = ""
      out["name"] = ""; out["description"] = ""
      out["hexagonal_role"] = ""; out["user_invocable"] = "false"
      list_vals["consumes"] = ""
      list_vals["produces"] = ""
      list_vals["practices"] = ""
      ctx_list = ""
    }
    {
      # Re-iterate over the lines stored in `lines[]`.
    }
    END {
      for (i = 1; i <= n; i++) {
        line = lines[i]
        if (line ~ /^[[:space:]]*$/) { continue }
        # Top-level scalar (key: value)
        if (line ~ /^[A-Za-z_][A-Za-z0-9_-]*:[[:space:]]*[^[:space:]].*$/) {
          # Flush any pending list state, including a half-built context_rel
          # entry — without this, the LAST entry in a context_rel block is
          # silently dropped when we transition to the next scalar key.
          if (ctx_in && ctx_kind != "") {
            ctx_list = ctx_list (ctx_list == "" ? "" : ",") "{" jstr("kind") ":" jstr(ctx_kind) "," jstr("with") ":" jstr(ctx_with) "}"
            ctx_kind = ""; ctx_with = ""
          }
          in_list = 0; cur_key = ""
          ctx_in = 0
          key = line; sub(/:.*/, "", key)
          val = line; sub(/^[^:]*:[[:space:]]*/, "", val); val = trim(val)
          # Strip surrounding quotes if quoted
          if (val ~ /^".*"$/) { val = substr(val, 2, length(val) - 2) }
          if (val ~ /^'\''.*'\''$/) { val = substr(val, 2, length(val) - 2) }
          if (key == "name") out["name"] = val
          else if (key == "description") out["description"] = val
          else if (key == "hexagonal_role") out["hexagonal_role"] = val
          else if (key == "user-invocable" || key == "user_invocable") {
            out["user_invocable"] = (val == "true" ? "true" : "false")
          }
          continue
        }
        # Top-level key with no value (list opener: `consumes:`)
        if (line ~ /^[A-Za-z_][A-Za-z0-9_-]*:[[:space:]]*$/) {
          key = line; sub(/:.*/, "", key)
          # Reset state for new list/section
          if (ctx_in && ctx_kind != "") {
            ctx_list = ctx_list (ctx_list == "" ? "" : ",") "{" jstr("kind") ":" jstr(ctx_kind) "," jstr("with") ":" jstr(ctx_with) "}"
            ctx_kind = ""; ctx_with = ""
          }
          ctx_in = (key == "context_rel" ? 1 : 0)
          if (ctx_in) { cur_key = "" ; in_list = 0; continue }
          in_list = (key == "consumes" || key == "produces" || key == "practices") ? 1 : 0
          cur_key = key
          continue
        }
        # List item — `- value` (simple scalar)
        if (in_list && line ~ /^-[[:space:]]+[^[:space:]]/) {
          item = line; sub(/^-[[:space:]]+/, "", item); item = trim(item)
          if (list_vals[cur_key] == "") list_vals[cur_key] = jstr(item)
          else list_vals[cur_key] = list_vals[cur_key] "," jstr(item)
          continue
        }
        # context_rel item kickoff: `- kind: alias-of` (one entry start)
        if (ctx_in && line ~ /^-[[:space:]]+kind:/) {
          if (ctx_kind != "") {
            ctx_list = ctx_list (ctx_list == "" ? "" : ",") "{" jstr("kind") ":" jstr(ctx_kind) "," jstr("with") ":" jstr(ctx_with) "}"
          }
          ctx_kind = line; sub(/^-[[:space:]]+kind:[[:space:]]*/, "", ctx_kind); ctx_kind = trim(ctx_kind)
          ctx_with = ""
          continue
        }
        # context_rel continuation: `  with: foo`
        if (ctx_in && line ~ /^[[:space:]]+with:[[:space:]]*/) {
          ctx_with = line; sub(/^[[:space:]]+with:[[:space:]]*/, "", ctx_with); ctx_with = trim(ctx_with)
          continue
        }
      }
      if (ctx_in && ctx_kind != "") {
        ctx_list = ctx_list (ctx_list == "" ? "" : ",") "{" jstr("kind") ":" jstr(ctx_kind) "," jstr("with") ":" jstr(ctx_with) "}"
      }
      printf "{"
      printf "%s:%s,", jstr("name"), jstr(out["name"])
      printf "%s:%s,", jstr("description"), jstr(out["description"])
      printf "%s:%s,", jstr("hexagonal_role"), jstr(out["hexagonal_role"])
      printf "%s:%s,", jstr("user_invocable"), out["user_invocable"]
      printf "%s:[%s],", jstr("consumes"), list_vals["consumes"]
      printf "%s:[%s],", jstr("produces"), list_vals["produces"]
      printf "%s:[%s],", jstr("practices"), list_vals["practices"]
      printf "%s:[%s]", jstr("context_rel"), ctx_list
      printf "}"
    }
  '
}

# Build the catalog payload.
TMP_PAYLOAD="$(mktemp)"
trap 'rm -f "$TMP_PAYLOAD"' EXIT

skill_count=0
printf '[' > "$TMP_PAYLOAD"
first=1
for sd in "$SKILLS_DIR"/*/SKILL.md; do
  [ -r "$sd" ] || continue
  name="$(basename "$(dirname "$sd")")"
  fm="$(extract_frontmatter "$sd")"
  base="$(parse_frontmatter "$fm")"
  # Compute reference count + codex override presence.
  refs_dir="$SKILLS_DIR/$name/references"
  ref_count=0
  [ -d "$refs_dir" ] && ref_count="$(find "$refs_dir" -maxdepth 1 -type f -name '*.md' | wc -l | tr -d ' ')"
  codex_present="false"
  [ -d "$CODEX_DIR/$name" ] || [ -d "$OVERRIDES_DIR/$name" ] && codex_present="true"
  # Merge generator-only fields into base JSON via jq.
  entry="$(printf '%s' "$base" | jq -c --arg n "$name" --argjson rc "$ref_count" --argjson cp "$codex_present" '
    .name = (if .name == "" then $n else .name end) |
    .references_count = $rc |
    .codex_override_present = $cp
  ')"
  if [ "$first" -eq 1 ]; then first=0; else printf ',' >> "$TMP_PAYLOAD"; fi
  printf '%s' "$entry" >> "$TMP_PAYLOAD"
  skill_count=$((skill_count + 1))
done
printf ']' >> "$TMP_PAYLOAD"

NOW="$(date -u +%FT%TZ)"
CATALOG_JSON="$(jq -c \
  --arg ts "$NOW" \
  --argjson count "$skill_count" \
  --slurpfile skills "$TMP_PAYLOAD" \
  -n '{schema_version:"1", generated_at:$ts, skill_count:$count, skills:$skills[0]}')"

case "$MODE" in
  stdout)
    printf '%s\n' "$CATALOG_JSON" | jq .
    ;;
  check)
    if [ ! -r "$OUT_PATH" ]; then
      echo "generate-skill-catalog: catalog not committed at $OUT_PATH (run without --check to generate)" >&2
      exit 1
    fi
    # Compare ignoring generated_at (timestamp drifts every run).
    new_norm="$(printf '%s' "$CATALOG_JSON" | jq 'del(.generated_at)')"
    old_norm="$(jq 'del(.generated_at)' "$OUT_PATH")"
    if [ "$new_norm" = "$old_norm" ]; then
      echo "generate-skill-catalog: catalog up-to-date ($skill_count skills)"
      exit 0
    fi
    echo "generate-skill-catalog: DRIFT — committed catalog differs from regeneration" >&2
    diff <(printf '%s' "$old_norm") <(printf '%s' "$new_norm") | head -40 >&2 || true
    exit 1
    ;;
  write)
    mkdir -p "$(dirname "$OUT_PATH")"
    printf '%s\n' "$CATALOG_JSON" | jq . > "$OUT_PATH"
    echo "generate-skill-catalog: wrote $OUT_PATH ($skill_count skills)"
    ;;
esac
