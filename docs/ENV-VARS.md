# Environment variables

The semantic loop needs no environment variables. Inputs belong in explicit
packets and arguments.

Repository tooling recognizes a small host-policy surface:

| Variable | Meaning |
|---|---|
| `AGENTOPS_GATE_DISABLED=1` | Explicit repository pre-push bypass. This does not create semantic evidence. |
| `AO_BIN` | Select the `ao` executable used by a deterministic gate subprocess. |
| `CODEX_HOME` | Codex runtime profile root used by the optional Codex adapter. |

Runtime-specific tools may define their own variables. Those variables are
substrate configuration and never become Plan, Candidate, RPI, or verdict state.
