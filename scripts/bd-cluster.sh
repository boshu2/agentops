#!/usr/bin/env bash
set -euo pipefail

# bd-cluster.sh — Analyze open beads for domain overlap and suggest consolidation groups.
#
# Usage: scripts/bd-cluster.sh [--json] [--apply]
#
#   --json   Emit structured JSON output instead of human-readable text
#   --apply  For each cluster, reparent non-epic beads under the epic using
#            `bd update <id> --parent <epic-id>`
#
# Exit 0 always (advisory tool — never blocks).

# ---------------------------------------------------------------------------
# Flags
# ---------------------------------------------------------------------------
JSON_OUTPUT=0
APPLY=0

for arg in "$@"; do
  case "$arg" in
    --json)  JSON_OUTPUT=1 ;;
    --apply) APPLY=1 ;;
    *)
      echo "Unknown argument: $arg" >&2
      echo "Usage: $0 [--json] [--apply]" >&2
      exit 1
      ;;
  esac
done

# ---------------------------------------------------------------------------
# Dependency guards
# ---------------------------------------------------------------------------
if ! command -v bd &>/dev/null; then
  echo "WARN: bd not found — skipping cluster analysis" >&2
  exit 0
fi

if ! command -v jq &>/dev/null; then
  echo "WARN: jq not found — skipping cluster analysis" >&2
  exit 0
fi

# ---------------------------------------------------------------------------
# Stop words (skip when tokenizing titles)
# ---------------------------------------------------------------------------
STOP_WORDS="the a an in to for of and or with is are be was were by on at from as this that it its into"

# is_stop_word <word> — returns 0 (true) if word is a stop word
is_stop_word() {
  local word="${1,,}"  # lowercase
  local sw
  for sw in $STOP_WORDS; do
    [[ "$word" == "$sw" ]] && return 0
  done
  return 1
}

# ---------------------------------------------------------------------------
# Data collection — fetch open beads and cache show output
# ---------------------------------------------------------------------------
TMPDIR_CACHE="$(mktemp -d)"
# shellcheck disable=SC2064
trap "rm -rf '$TMPDIR_CACHE'" EXIT

# Get list of open beads as JSON array
LIST_JSON="$TMPDIR_CACHE/list.json"
if ! bd list --status open --json >"$LIST_JSON" 2>/dev/null; then
  echo "WARN: bd list failed — skipping cluster analysis" >&2
  exit 0
fi

# Check we got a non-empty array
BEAD_COUNT="$(jq 'length' "$LIST_JSON" 2>/dev/null || echo 0)"
if [[ "$BEAD_COUNT" -lt 2 ]]; then
  if [[ "$JSON_OUTPUT" -eq 1 ]]; then
    echo '{"clusters":[],"unclustered":[],"message":"fewer than 2 open beads — nothing to cluster"}'
  else
    echo "Fewer than 2 open beads — nothing to cluster."
  fi
  exit 0
fi

# Collect bead IDs and titles from list output
mapfile -t BEAD_IDS < <(jq -r '.[].id' "$LIST_JSON")

# Fetch full show output for each bead (one per bead, cached)
declare -A BEAD_TITLE
declare -A BEAD_BODY
declare -A BEAD_LABELS
declare -A BEAD_IS_EPIC

show_jq() {
  local filter="$1"
  local file="$2"
  jq -r "(if type == \"array\" then .[0] // {} else . end) | ${filter}" "$file" 2>/dev/null || true
}

for id in "${BEAD_IDS[@]}"; do
  SHOW_FILE="$TMPDIR_CACHE/show_${id}.json"
  if ! bd show "$id" --json >"$SHOW_FILE" 2>/dev/null; then
    # Fall back to list data for title only
    title="$(jq -r --arg i "$id" '.[] | select(.id==$i) | .title // .subject // ""' "$LIST_JSON")"
    echo '{}' >"$SHOW_FILE"
    BEAD_TITLE["$id"]="$title"
    BEAD_BODY["$id"]=""
    BEAD_LABELS["$id"]=""
    BEAD_IS_EPIC["$id"]="0"
    continue
  fi

  BEAD_TITLE["$id"]="$(show_jq '.title // .subject // ""' "$SHOW_FILE")"
  BEAD_BODY["$id"]="$(show_jq '.body // .description // ""' "$SHOW_FILE")"
  BEAD_LABELS["$id"]="$(show_jq '(.labels // []) | join(" ")' "$SHOW_FILE")"
  # Treat as epic if type/kind field says so, or if it has children
  IS_EPIC="$(show_jq 'if (.type=="epic" or .kind=="epic" or .issue_type=="epic" or ((.children // []) | length > 0)) then "1" else "0" end' "$SHOW_FILE")"
  BEAD_IS_EPIC["$id"]="$IS_EPIC"
done

# ---------------------------------------------------------------------------
# Tokenize a string into meaningful words (lowercase, skip stop words)
# ---------------------------------------------------------------------------
# tokenize <string> — prints one word per line
tokenize() {
  local input="$1"
  # Strip punctuation, lowercase, split on whitespace
  echo "$input" \
    | tr '[:upper:]' '[:lower:]' \
    | tr -cs 'a-z0-9/' '\n' \
    | while IFS= read -r word; do
        [[ ${#word} -lt 3 ]] && continue
        is_stop_word "$word" && continue
        echo "$word"
      done
}

# extract_paths <string> — prints file paths found in the string (one per line)
extract_paths() {
  local input="$1"
  # Match tokens that look like path/to/something (contain / and a dot or end with known exts)
  echo "$input" \
    | grep -oE '[a-zA-Z0-9_./-]+/[a-zA-Z0-9_./-]+' \
    | grep -E '\.' \
    || true
}

# ---------------------------------------------------------------------------
# Build keyword and path sets per bead
# ---------------------------------------------------------------------------
declare -A BEAD_KEYWORDS  # space-separated keyword list
declare -A BEAD_PATHS     # space-separated path list

for id in "${BEAD_IDS[@]}"; do
  combined="${BEAD_TITLE[$id]} ${BEAD_BODY[$id]}"
  mapfile -t kws < <(tokenize "$combined" | sort -u)
  BEAD_KEYWORDS["$id"]="${kws[*]+"${kws[*]}"}"

  mapfile -t paths < <(extract_paths "$combined" | sort -u)
  BEAD_PATHS["$id"]="${paths[*]+"${paths[*]}"}"
done

# ---------------------------------------------------------------------------
# Compute pairwise overlap scores and build clusters
# ---------------------------------------------------------------------------
# cluster_of[id] = cluster_index (0-based), or "" if unclustered
declare -A CLUSTER_OF
CLUSTER_COUNT=0
declare -a CLUSTER_MEMBERS  # CLUSTER_MEMBERS[i] = space-separated bead IDs

# score_overlap <id_a> <id_b> — prints integer overlap score to stdout
score_overlap() {
  local a="$1" b="$2"
  local score=0

  # Keyword overlap
  local kw_a kw_b shared_kw
  kw_a="${BEAD_KEYWORDS[$a]}"
  kw_b="${BEAD_KEYWORDS[$b]}"
  if [[ -n "$kw_a" && -n "$kw_b" ]]; then
    shared_kw="$(comm -12 \
      <(echo "$kw_a" | tr ' ' '\n' | sort) \
      <(echo "$kw_b" | tr ' ' '\n' | sort) \
      | wc -l | tr -d ' ')"
    score=$((score + shared_kw))
  fi

  # File path overlap — each shared path adds 2 (weighted higher than a keyword)
  local p_a p_b shared_p
  p_a="${BEAD_PATHS[$a]}"
  p_b="${BEAD_PATHS[$b]}"
  if [[ -n "$p_a" && -n "$p_b" ]]; then
    shared_p="$(comm -12 \
      <(echo "$p_a" | tr ' ' '\n' | sort) \
      <(echo "$p_b" | tr ' ' '\n' | sort) \
      | wc -l | tr -d ' ')"
    score=$((score + shared_p * 2))
  fi

  # Label overlap — each shared label adds 3
  local l_a l_b shared_l
  l_a="${BEAD_LABELS[$a]}"
  l_b="${BEAD_LABELS[$b]}"
  if [[ -n "$l_a" && -n "$l_b" ]]; then
    shared_l="$(comm -12 \
      <(echo "$l_a" | tr ' ' '\n' | sort) \
      <(echo "$l_b" | tr ' ' '\n' | sort) \
      | wc -l | tr -d ' ')"
    score=$((score + shared_l * 3))
  fi

  echo "$score"
}

THRESHOLD=2  # minimum score to consider two beads related

# Simple single-linkage clustering
for id_a in "${BEAD_IDS[@]}"; do
  for id_b in "${BEAD_IDS[@]}"; do
    [[ "$id_a" == "$id_b" ]] && continue
    # Only process each pair once (a < b lexicographically)
    [[ "$id_a" > "$id_b" ]] && continue

    score="$(score_overlap "$id_a" "$id_b")"
    [[ "$score" -lt "$THRESHOLD" ]] && continue

    # Determine cluster membership
    ca="${CLUSTER_OF[$id_a]:-}"
    cb="${CLUSTER_OF[$id_b]:-}"

    if [[ -z "$ca" && -z "$cb" ]]; then
      # New cluster
      idx="$CLUSTER_COUNT"
      CLUSTER_OF["$id_a"]="$idx"
      CLUSTER_OF["$id_b"]="$idx"
      CLUSTER_MEMBERS[idx]="$id_a $id_b"
      CLUSTER_COUNT=$((CLUSTER_COUNT + 1))
    elif [[ -n "$ca" && -z "$cb" ]]; then
      CLUSTER_OF["$id_b"]="$ca"
      CLUSTER_MEMBERS[ca]="${CLUSTER_MEMBERS[ca]} $id_b"
    elif [[ -z "$ca" && -n "$cb" ]]; then
      CLUSTER_OF["$id_a"]="$cb"
      CLUSTER_MEMBERS[cb]="${CLUSTER_MEMBERS[cb]} $id_a"
    elif [[ "$ca" != "$cb" ]]; then
      # Merge cluster cb into ca
      for mid in ${CLUSTER_MEMBERS[cb]}; do
        CLUSTER_OF["$mid"]="$ca"
      done
      CLUSTER_MEMBERS[ca]="${CLUSTER_MEMBERS[ca]} ${CLUSTER_MEMBERS[cb]}"
      CLUSTER_MEMBERS[cb]=""
    fi
  done
done

# ---------------------------------------------------------------------------
# Identify the best "representative" for each cluster
# (prefer existing epics; otherwise pick the oldest/first ID lexicographically)
# ---------------------------------------------------------------------------
find_representative() {
  local members="$1"
  local rep=""
  for mid in $members; do
    if [[ "${BEAD_IS_EPIC[$mid]:-0}" == "1" ]]; then
      echo "$mid"
      return
    fi
  done
  # No epic found — return lexicographically smallest id
  # shellcheck disable=SC2086
  echo "$members" | tr ' ' '\n' | sort | head -1
}

# ---------------------------------------------------------------------------
# Compute shared keywords per cluster for display
# ---------------------------------------------------------------------------
cluster_shared_keywords() {
  local members="$1"
  local first=1
  local intersect=""
  local kw_str
  for mid in $members; do
    kw_str="${BEAD_KEYWORDS[$mid]}"
    if [[ $first -eq 1 ]]; then
      intersect="$(echo "$kw_str" | tr ' ' '\n' | sort)"
      first=0
    else
      intersect="$(comm -12 \
        <(echo "$intersect") \
        <(echo "$kw_str" | tr ' ' '\n' | sort))"
    fi
    [[ -z "$intersect" ]] && break
  done
  echo "$intersect" | tr '\n' ' ' | sed 's/ $//'
}

# ---------------------------------------------------------------------------
# Output
# ---------------------------------------------------------------------------
if [[ "$JSON_OUTPUT" -eq 1 ]]; then
  # Build JSON
  CLUSTERS_JSON="[]"
  UNCLUSTERED_JSON="[]"

  for idx in $(seq 0 $((CLUSTER_COUNT - 1))); do
    members="${CLUSTER_MEMBERS[$idx]:-}"
    [[ -z "$members" ]] && continue

    # Count unique members
    mapfile -t member_arr < <(echo "$members" | tr ' ' '\n' | sort -u | grep -v '^$')
    [[ ${#member_arr[@]} -lt 2 ]] && continue

    rep="$(find_representative "${member_arr[*]}")"
    shared="$(cluster_shared_keywords "${member_arr[*]}")"

    # Build bead array for this cluster
    BEADS_ARRAY="[]"
    for mid in "${member_arr[@]}"; do
      title="${BEAD_TITLE[$mid]:-}"
      is_epic="${BEAD_IS_EPIC[$mid]:-0}"
      BEADS_ARRAY="$(echo "$BEADS_ARRAY" | jq \
        --arg id "$mid" \
        --arg t "$title" \
        --argjson e "$is_epic" \
        '. + [{"id":$id,"title":$t,"is_epic":($e=="1")}]')"
    done

    cluster_obj="$(jq -n \
      --arg rep "$rep" \
      --arg shared "$shared" \
      --argjson beads "$BEADS_ARRAY" \
      '{"representative":$rep,"shared_keywords":($shared | split(" ") | map(select(. != ""))),"beads":$beads}')"

    CLUSTERS_JSON="$(echo "$CLUSTERS_JSON" | jq --argjson c "$cluster_obj" '. + [$c]')"
  done

  for id in "${BEAD_IDS[@]}"; do
    [[ -n "${CLUSTER_OF[$id]:-}" ]] && continue
    title="${BEAD_TITLE[$id]:-}"
    UNCLUSTERED_JSON="$(echo "$UNCLUSTERED_JSON" | jq \
      --arg id "$id" \
      --arg t "$title" \
      '. + [{"id":$id,"title":$t}]')"
  done

  jq -n \
    --argjson clusters "$CLUSTERS_JSON" \
    --argjson unclustered "$UNCLUSTERED_JSON" \
    '{"clusters":$clusters,"unclustered":$unclustered}'

else
  # Human-readable output
  CLUSTER_NUM=0
  for idx in $(seq 0 $((CLUSTER_COUNT - 1))); do
    members="${CLUSTER_MEMBERS[$idx]:-}"
    [[ -z "$members" ]] && continue

    mapfile -t member_arr < <(echo "$members" | tr ' ' '\n' | sort -u | grep -v '^$')
    [[ ${#member_arr[@]} -lt 2 ]] && continue

    CLUSTER_NUM=$((CLUSTER_NUM + 1))
    rep="$(find_representative "${member_arr[*]}")"
    shared="$(cluster_shared_keywords "${member_arr[*]}")"
    # Derive a cluster label from the first few shared keywords
    label="$(echo "$shared" | tr ' ' '\n' | head -3 | tr '\n' ' ' | sed 's/ $//')"
    [[ -z "$label" ]] && label="overlapping beads"

    echo "Cluster ${CLUSTER_NUM}: \"${label}\" (${#member_arr[@]} beads)"
    for mid in "${member_arr[@]}"; do
      title="${BEAD_TITLE[$mid]:-}"
      epic_marker=""
      [[ "${BEAD_IS_EPIC[$mid]:-0}" == "1" ]] && epic_marker=" [epic]"
      echo "  ${mid}${epic_marker}: ${title}"
    done
    echo "  Shared keywords: ${shared:-none}"
    if [[ "${BEAD_IS_EPIC[$rep]:-0}" == "1" ]]; then
      echo "  Suggestion: Consolidate under ${rep} (existing epic)"
    else
      echo "  Suggestion: Consolidate under ${rep}"
    fi
    echo ""
  done

  # Count unclustered
  UNCLUSTERED_COUNT=0
  for id in "${BEAD_IDS[@]}"; do
    [[ -n "${CLUSTER_OF[$id]:-}" ]] && continue
    UNCLUSTERED_COUNT=$((UNCLUSTERED_COUNT + 1))
  done

  if [[ "$CLUSTER_NUM" -eq 0 ]]; then
    echo "No clusters found across ${BEAD_COUNT} open beads."
  else
    echo "No clusters found for ${UNCLUSTERED_COUNT} remaining bead(s)."
  fi
fi

# ---------------------------------------------------------------------------
# Apply: reparent non-representative beads under the cluster representative
# ---------------------------------------------------------------------------
if [[ "$APPLY" -eq 1 ]]; then
  echo ""
  echo "Applying consolidation..."
  APPLIED=0
  for idx in $(seq 0 $((CLUSTER_COUNT - 1))); do
    members="${CLUSTER_MEMBERS[$idx]:-}"
    [[ -z "$members" ]] && continue

    mapfile -t member_arr < <(echo "$members" | tr ' ' '\n' | sort -u | grep -v '^$')
    [[ ${#member_arr[@]} -lt 2 ]] && continue

    rep="$(find_representative "${member_arr[*]}")"
    for mid in "${member_arr[@]}"; do
      [[ "$mid" == "$rep" ]] && continue
      if bd update "$mid" --parent "$rep" 2>/dev/null; then
        echo "  Reparented ${mid} under ${rep}"
        APPLIED=$((APPLIED + 1))
      else
        echo "  WARN: Failed to reparent ${mid} under ${rep}" >&2
      fi
    done
  done
  echo "Applied ${APPLIED} reparenting operation(s)."
fi

exit 0
