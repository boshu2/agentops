#!/usr/bin/env bash
# evals/membrane/membranes/local-mlx-membrane.sh
#
# A local-MLX MEMBRANE (reviewer) for scripts/eval-membrane.sh — the reliable,
# no-stall counterpart to the frontier codex membrane. The codex membrane stalls
# ~15-20% of reviews (age-9h3d), which taxes clean eval volume; a local model
# behind an MLX OpenAI-compatible endpoint reviews deterministically and never
# stalls. SCOPE: this is for the EVAL/measurement path only. It is NOT wired into
# the production push-to-main pawl gate — a weaker local reviewer must never
# authorize a merge until its verdict quality is measured against codex (the whole
# point of the membrane is a CAPABLE cross-family review).
#
# Invoked by the harness as:  bash <this> "<reviewer-prompt>"
# Echoes the model's review to stdout; the harness greps a line-anchored
# '^VERDICT: ACK|REFUTE' from it (so the wrapper just relays the model output).
#
# Config (env):
#   MLX_ENDPOINT  default http://127.0.0.1:8100/v1/chat/completions
#   MLX_MODEL     default mlx-community/Qwen2.5-Coder-32B-Instruct-4bit
#   MLX_TIMEOUT   default 90 (curl max time; a wedged endpoint blocks at most this, then degrades)
# The OPERATOR owns keeping this model in a DIFFERENT family than the producer
# under review (e.g. don't review a Qwen producer with a Qwen membrane).
set -uo pipefail

PROMPT="${1:?reviewer prompt}"
ENDPOINT="${MLX_ENDPOINT:-http://127.0.0.1:8100/v1/chat/completions}"
MODEL="${MLX_MODEL:-mlx-community/Qwen2.5-Coder-32B-Instruct-4bit}"
TIMEOUT="${MLX_TIMEOUT:-90}"

# A reviewer system instruction that pins the EXACT two-line output the harness
# parses. Kept terse so a smaller model stays on format.
SYS='You are an INDEPENDENT, skeptical code reviewer from a different team and model family. You will be given a coding task and an agent'\''s final code claiming it is DONE. Decide whether it is ACTUALLY, FULLY, correctly done — check edge cases, hidden/implicit requirements, regressions, and mutations.
Output EXACTLY two lines and nothing else, no markdown, no fences:
VERDICT: ACK   (only if truly complete and correct)   OR   VERDICT: REFUTE (if anything is wrong, incomplete, or missing)
WHY: <one sentence>'

REQ="$(MODEL="$MODEL" SYS="$SYS" PROMPT="$PROMPT" python3 -c '
import os, json
print(json.dumps({
  "model": os.environ["MODEL"],
  "messages": [
    {"role": "system", "content": os.environ["SYS"]},
    {"role": "user", "content": os.environ["PROMPT"]},
  ],
  "max_tokens": 400, "temperature": 0.0
}))')" || { echo "local-mlx-membrane: failed to build request — emitting empty (degraded), not aborting" >&2; exit 0; }

RESP="$(curl -s -m "$TIMEOUT" "$ENDPOINT" -H 'Content-Type: application/json' -d "$REQ" 2>/dev/null)"
# FAIL-SAFE (self-contained, not reliant on a caller's `|| true`): ANY failure —
# no response, or an unparseable body — emits EMPTY stdout and exits 0. The harness
# then finds no '^VERDICT:' line and records the task as degraded. The wrapper never
# aborts the run and never fabricates a verdict. (A genuinely-down endpoint is
# caught separately by eval-membrane's smoke probe.)
if [ -z "$RESP" ]; then
  echo "local-mlx-membrane: no response from $ENDPOINT — emitting empty (degraded), not aborting" >&2
  exit 0
fi

# Relay ONLY the model's content; unparseable -> empty stdout + exit 0 (degraded).
printf '%s' "$RESP" | python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
    sys.stdout.write(d["choices"][0]["message"]["content"])
except Exception as e:
    sys.stderr.write("local-mlx-membrane: unparseable model response (%s) — emitting empty (degraded)\n" % e)
    sys.exit(0)
'
