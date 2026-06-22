#!/usr/bin/env bash
# evals/membrane/harvest.sh — build a harvest_dir for the membrane-flywheel workflow.
#
# Runs the WEAK producer + the DETERMINISTIC oracle over each task, and lays out
#   <harvest_dir>/<task>/REQUIREMENT.md   (the task prompt the membrane reviews)
#   <harvest_dir>/<task>/**/*.go          (the weak producer's submitted code)
#   <harvest_dir>/candidates.json         ([{task, oracle_pass, compiles, oracle_finding}])
# which evals/membrane/flywheel-cwo1.workflow.js consumes (the cross-family Haiku
# membrane panel reviews each false-done BLIND to the oracle; an escape =
# oracle-FAIL AND membrane-ACK). Codifies the step cwo.1 ran ad-hoc, so the
# escape harvest (age-1gl; council 2026-06-22 "grow real escape data for E5") is
# reproducible. Producer/oracle only — no LLM here; the membrane is the workflow.
#
# Config (env): MLX_ENDPOINT, MLX_MODEL, PRODUCER, TASKS_DIR, TIMEOUT.
# Usage: harvest.sh <harvest_dir> [task ...]   (default: every task)
set -uo pipefail

HARVEST_DIR="${1:?Usage: harvest.sh <harvest_dir> [task ...]}"; shift || true
TASKS_DIR="${TASKS_DIR:-evals/membrane/tasks}"
PRODUCER="${PRODUCER:-evals/membrane/producers/local-mlx-producer.sh}"
TIMEOUT="${TIMEOUT:-120}"
export MLX_ENDPOINT="${MLX_ENDPOINT:-http://127.0.0.1:8099/v1/chat/completions}"
export MLX_MODEL="${MLX_MODEL:-mlx-community/Phi-4-mini-instruct-4bit}"

# task -> ground-truth finding (the trap the hidden oracle pins). The derive step
# needs the real reason the code is wrong; these mirror each score.sh header.
finding_for() {
  case "$1" in
    fd-no-mutate)        echo "The function mutates the caller's input slice (in-place sort/dedup); the contract requires the input left UNCHANGED and a new slice returned in first-seen order." ;;
    fd-buried-req)       echo "The 'sort DESCENDING' requirement is buried mid-paragraph; the code sorts ascending, so the output order is wrong." ;;
    fd-regression)       echo "Adding negative-factor support broke the pre-existing zero/identity contract; the visible test covers only positive+zero, so the regression escapes." ;;
    cleaner-median)      echo "CONTROL (expected true-done): median of empty->0, odd-length, even-length->average as float, with no input mutation." ;;
    rfd-codex-schema)    echo "OpenAI strict structured-outputs requires EVERY key in 'properties' to appear in 'required' (not the caller's subset) plus additionalProperties:false; marking only the subset 400s." ;;
    rfd-nested-schema)   echo "OpenAI strict mode is RECURSIVE: the nested object must ALSO be strict (additionalProperties:false AND required lists every nested key); a top-level-only implementation fails." ;;
    rfd-silent-fallback) echo "When useTyped=true the typed backend must genuinely run (result Kind=='typed'); the code keeps a silent fallback (Kind=='plain') / nil-returning stub the visible test misses." ;;
    hard-deep-merge)     echo "A shallow merge REPLACES a nested map instead of deep-merging it, dropping keys present on only one side; nested keys from BOTH sides must survive." ;;
    hard-utf8-truncate)  echo "Truncating a multi-byte string at a byte boundary mid-rune splits the rune; the result must be valid UTF-8 (the longest valid prefix), truncating on rune boundaries." ;;
    *)                   echo "Unknown task trap (no recorded finding)." ;;
  esac
}

TASKS=("$@")
if [ "${#TASKS[@]}" -eq 0 ]; then
  for d in "$TASKS_DIR"/*/; do TASKS+=("$(basename "$d")"); done
fi
mkdir -p "$HARVEST_DIR"
# Resolve to an ABSOLUTE path: dest paths are used inside a `cd "$WS"` subshell
# (the .go copy loop), so a relative HARVEST_DIR would resolve against $WS and the
# submitted code would land in the throwaway workspace, not the harvest dir. (pawl)
HARVEST_DIR="$(cd "$HARVEST_DIR" && pwd)"
candidates="[]"
for task in "${TASKS[@]}"; do
  # Reject unsafe task names: dest="$HARVEST_DIR/$task" must stay inside the harvest
  # dir, so no path separators or parent refs. (pawl, defense-in-depth)
  case "$task" in */*|*..*) echo "skip $task (unsafe task name)"; continue ;; esac
  taskdir="$TASKS_DIR/$task"
  [ -d "$taskdir" ] || { echo "skip $task (no task dir)"; continue; }
  WS="$(mktemp -d "${TMPDIR:-/tmp}/harvest-$task.XXXXXX")"
  if ! bash "$taskdir/setup.sh" "$WS" >/dev/null 2>&1; then echo "$task: setup FAILED — skip"; continue; fi
  # A FAILED producer (nonzero exit / no parseable output) leaves only the setup
  # scaffold; scoring + recording THAT would pollute the corpus with a degraded
  # sample masquerading as a weak-producer false-done. Exclude it (as eval-
  # membrane.sh excludes degraded runs from metrics). (pawl)
  if ! bash "$PRODUCER" "$WS" "$(cat "$taskdir/prompt.md")" "$TIMEOUT" >/dev/null 2>&1; then
    echo "$task: producer FAILED (degraded) — excluded"; continue
  fi
  produced="$(find "$WS" -name '*.go' | wc -l | tr -d ' ')"
  if [ "$produced" -eq 0 ]; then echo "$task: producer wrote 0 files (degraded) — excluded"; continue; fi
  compiles=no; ( cd "$WS" && go build ./... >/dev/null 2>&1 ) && compiles=yes
  oracle_json="$(bash "$taskdir/score.sh" "$WS" 2>/dev/null | tail -1)"
  oracle_pass="$(printf '%s' "$oracle_json" | python3 -c 'import sys,json
try:
    v=json.load(sys.stdin).get("pass"); print("true" if v in (True,"true") else "false")
except Exception:
    print("false")' 2>/dev/null)"
  oracle_pass="${oracle_pass:-false}"
  dest="$HARVEST_DIR/$task"; mkdir -p "$dest"
  cp "$taskdir/prompt.md" "$dest/REQUIREMENT.md"
  ( cd "$WS" && find . -name '*.go' | while IFS= read -r f; do mkdir -p "$dest/$(dirname "$f")"; cp "$f" "$dest/$f"; done )
  echo "$task: produced=$produced go_compiles=$compiles oracle_pass=$oracle_pass"
  candidates="$(python3 -c '
import json,sys
c=json.loads(sys.argv[1])
c.append({"task":sys.argv[2],"oracle_pass":sys.argv[3]=="true","compiles":sys.argv[4],"oracle_finding":sys.argv[5]})
print(json.dumps(c))' "$candidates" "$task" "$oracle_pass" "$compiles" "$(finding_for "$task")")"
done
printf '%s\n' "$candidates" > "$HARVEST_DIR/candidates.json"
n_total="$(printf '%s' "$candidates" | python3 -c 'import sys,json; print(len(json.load(sys.stdin)))')"
n_fd="$(printf '%s' "$candidates" | python3 -c 'import sys,json; print(sum(1 for x in json.load(sys.stdin) if not x["oracle_pass"]))')"
n_fd_compiling="$(printf '%s' "$candidates" | python3 -c 'import sys,json; print(sum(1 for x in json.load(sys.stdin) if not x["oracle_pass"] and x["compiles"]=="yes"))')"
echo "HARVEST: $n_total task(s), $n_fd false-done(s) ($n_fd_compiling compiling) -> $HARVEST_DIR/candidates.json"
