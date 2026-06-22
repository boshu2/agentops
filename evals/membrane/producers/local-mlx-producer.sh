#!/usr/bin/env bash
# evals/membrane/producers/local-mlx-producer.sh
#
# A WEAK-model PRODUCER for scripts/eval-membrane.sh, satisfying age-cwo.1's
# "deliberately weak producer on real tasks" requirement WITHOUT bushido: it
# drives a small local model behind an MLX OpenAI-compatible endpoint (e.g.
# `mlx_lm.server --model mlx-community/Phi-4-mini-instruct-4bit --port 8099`).
# The weak producer is what generates REAL escapes — a frontier producer aces
# the tasks (age-1gl diligence: frontier ceiling).
#
# Invoked by the harness as:  bash <this> <workdir> <prompt> [timeout]
# It asks the model for complete file contents and writes them into <workdir>.
#
# Config (env):
#   MLX_ENDPOINT  default http://127.0.0.1:8099/v1/chat/completions
#   MLX_MODEL     default mlx-community/Phi-4-mini-instruct-4bit
#
# Usage with eval-membrane:
#   scripts/eval-membrane.sh --producer-label phi-4-mini-mlx \
#     --producer-cmd 'bash evals/membrane/producers/local-mlx-producer.sh "$1" "$2" "$3"' ...
#
# NOTE (2026-06-21): mlx-community/gemma-4-e4b-it-4bit and gemma-3n-E4B-it-4bit
# do NOT load in mlx-lm (weight/loader mismatch); Phi-4-mini is the working weak
# producer on Apple MLX.
set -uo pipefail

WORKDIR="${1:?workdir}"; PROMPT="${2:?prompt}"; TIMEOUT="${3:-180}"
ENDPOINT="${MLX_ENDPOINT:-http://127.0.0.1:8099/v1/chat/completions}"
MODEL="${MLX_MODEL:-mlx-community/Phi-4-mini-instruct-4bit}"

SYS='You are a coding agent. Implement the task by writing COMPLETE file contents.
For EACH file you create or change, output EXACTLY this, with no markdown fences:
=== FILE: <relative/path> ===
<full file content>
=== END ===
Output ONLY these blocks. No prose, no explanation, no ```.'

REQ="$(MODEL="$MODEL" SYS="$SYS" PROMPT="$PROMPT" python3 -c '
import os, json
print(json.dumps({
  "model": os.environ["MODEL"],
  "messages": [{"role":"user","content": os.environ["SYS"] + "\n\nTASK:\n" + os.environ["PROMPT"]}],
  "max_tokens": 1500, "temperature": 0.3
}))')"

RESP="$(curl -s -m "$TIMEOUT" "$ENDPOINT" -H 'Content-Type: application/json' -d "$REQ" 2>/dev/null)"
[ -n "$RESP" ] || { echo "producer: no response from $ENDPOINT" >&2; exit 1; }

printf '%s' "$RESP" | WORKDIR="$WORKDIR" PROMPT="$PROMPT" python3 -c '
import sys, json, re, os
workdir = os.environ["WORKDIR"]
try:
    d = json.load(sys.stdin)
    content = d["choices"][0]["message"]["content"]
except Exception as e:
    sys.stderr.write("producer: unparseable model response: %s\n" % e); sys.exit(1)

blocks = re.findall(r"=== FILE:\s*(.+?)\s*===\s*\n(.*?)\n=== END ===", content, re.S)
if not blocks:
    # Weak models ignore the FILE schema and just emit a fenced code block.
    # Fallback: target path = first backticked token in the PROMPT that looks
    # like a source path (has an extension); body = the FIRST fenced block.
    prompt = os.environ.get("PROMPT", "")
    pm = re.search(r"`([^`]+\.[A-Za-z0-9]+)`", prompt)
    fences = re.findall(r"```[a-zA-Z0-9]*\n(.*?)```", content, re.S)
    if pm and fences:
        blocks = [(pm.group(1), fences[0])]
    else:
        sys.stderr.write("producer: model emitted no FILE blocks and no inferable fenced code\n")
        sys.exit(2)

written = 0
for path, body in blocks:
    path = path.strip().lstrip("/")
    if ".." in path.split("/"):   # never escape the workspace
        continue
    full = os.path.join(workdir, path)
    os.makedirs(os.path.dirname(full) or ".", exist_ok=True)
    body = re.sub(r"^```[a-zA-Z0-9]*\n", "", body)
    body = re.sub(r"\n```\s*$", "", body)
    with open(full, "w") as f:
        f.write(body if body.endswith("\n") else body + "\n")
    written += 1
sys.stderr.write("producer: wrote %d file(s)\n" % written)
sys.exit(0 if written else 2)
'
