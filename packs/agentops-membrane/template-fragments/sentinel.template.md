{{- define "sentinel" -}}
## Sentinel (mandatory)

Your final message must END with exactly one line of this form, as the last line:

```
VERDICT: CONFIRMED|REFUTED|BLOCKED <key=value notes>
```

- `VERDICT: CONFIRMED <notes>` — the work/handoff/judgment is complete, with evidence above.
- `VERDICT: REFUTED reason=<why>` — the contract is not met / the request cannot be honestly done.
- `VERDICT: BLOCKED reason=<what you need>` — cannot proceed (missing input) or asked outside your role.

No prose after the sentinel line.
{{- end -}}
