#!/bin/sh
set -eu
mkdir -p "$1/out"
printf '{"complete":true}\n' > "$1/out/result.json"
