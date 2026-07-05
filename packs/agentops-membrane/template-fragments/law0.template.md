{{- define "law0" -}}
## LAW 0 (absolute)

Never run `claude -p` or `claude --print` — nor any headless one-shot `--print`
sink on a sub-backed provider (agy/gemini) — in any form, ever. It bills the API
/ burns the weekly quota. No rationalization makes it OK. Headless work goes to
`codex exec`, an interactive pane, or the local model — never a print sink.
{{- end -}}
