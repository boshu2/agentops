# AgentOps CORE — Gemini / Antigravity (AGY) Image

The **65-skill AgentOps image** — the 61-skill CORE plus the 4 Gemini/AGY operator
skills (IMAGE-CORE.md §3c) — packaged for **Google Antigravity (AGY) / Gemini**.

> **KEY FINDING (IMAGE-CORE.md §2):** `SKILL.md` is **portable across all vendors**.
> The skill content shipped here is the **same** `skills/<slug>/SKILL.md` the Claude
> image ships — **zero conversion**. Only the *packaging shell* differs: a thin
> Antigravity plugin wrapper. (Codex is the only vendor that needs a format
> conversion — the `skills-codex/` twin.) `verify.sh` enforces byte-identity to
> source, so a drift = a broken zero-conversion guarantee.

This bundle is intentionally separate from the canonical source corpus: the source
of truth remains `skills/<slug>/SKILL.md` at the repo root; `images/gemini/skills/*`
is the installable AGY bundle, copied verbatim and verified `cmp`-identical.

## Model anchor

Modeled on the **proven** AgentOps Antigravity plugin package green at agentops
commit **`ed8f573e6`** (`.agy-plugin/`, bead `cp-c6k.3.1` / `.3.5`). The wrapper
shape — `plugin.json` + `skills/` + `agents/` + `rules/` + `hooks/hooks.json` +
`mcp_config.json`, validated by `agy plugin validate` — is reused directly; only
the packaged skill set (the 61-slug CORE + 4 AGY operator skills, vs. the proven
package's 27 tool-op skills) and the manifest `name`/`version` differ.

## Layout

```
images/gemini/
  plugin.json          # AGY plugin manifest (name, version, skills/agents/rules/hooks, Agent Mail MCP)
  mcp_config.json      # sidecar MCP payload for Antigravity/Gemini config import (Agent Mail over stdio)
  hooks.json           # sidecar hooks payload: AgentOps dcg guard + non-mutating closeout/evidence surface
  hooks/hooks.json     # AGY plugin hook payload layout processed by `agy plugin validate` (== hooks.json)
  agents/*.md          # AGY subagent templates (worker, validator)
  rules/*.md           # AGY rules (AgentOps loop law, Door-9 no-API-print)
  skills/<slug>/SKILL.md   # the 65 portable SKILL.md files (61 CORE + 4 AGY operator; verbatim copy of agentops/skills/<slug>/)
  verify.sh            # self-check: JSON validity + slug presence/identity + (if present) agy plugin validate
  README.md            # this file
```

## The 65 slugs packaged (61 CORE + 4 AGY operator)

### Method-core (35) — the operating loop

```
rpi  discovery  research  plan  implement  crank  swarm  validate  vibe
council  pre-mortem  red-team  ratchet  post-mortem  forge  compile  flywheel
goals  evolve  autodev  beads  bootstrap  brainstorm  design  handoff  recover
inject  push  scope  session-bootstrap  status  test  operating-loop-workflow
skill-builder  skill-auditor
```

### Tool-operator-core (26) — operating the substrate

```
beads-br  beads-bv  agent-mail  ntm  cass  cass-memory  dcg  caam  casr  ubs
ru-multi-repo-workflow  gh-triage-ru  rch  sbh  process-triage
system-performance-remediation  ssh  gcloud  gh-cli  gh-actions  planning-workflow
multi-model-triangulation  research-software  repeatedly-apply-skill  cc-hooks
vibing-with-ntm
```

(`slb` from the pruning target list is `~/.claude`-only and was never ingested into
the corpus, so it is not in the image CORE — see IMAGE-CORE.md §1c.)

### Gemini/AGY operator (4) — drive AGY's first-class control surface

```
agy-native  agy-rules-workflows  agy-mcp-plugins  agy-headless-evidence
```

These are the IMAGE-CORE.md §3c vendor-native operator skills the Unit-3 consumer
note ("operator skills from §3c") requires in the Gemini image. They are packaged
the same way as the CORE — direct, byte-identical `SKILL.md` copies, zero conversion.

## Verify

```bash
bash images/gemini/verify.sh
```

Confirms all manifest JSON is valid, every one of the 65 slugs (61 CORE + 4 AGY
operator) resolves to a bundled `skills/<slug>/SKILL.md` that is byte-identical to
the canonical source, the
agents/rules templates are present, and — if the `agy` CLI is installed —
`agy plugin validate` passes.

## Import path (AGY / Gemini)

```bash
agy plugin validate images/gemini       # validate the bundle
agy plugin install  images/gemini       # install locally
agy plugin enable   agentops-core-gemini # enable by manifest name
agy plugin list                         # confirm discovery
```

Gemini CLI consumes the same portable `SKILL.md` files directly via
`gemini skills install` / `gemini skills link` — **no content conversion** for
Gemini or AGY. The `agy plugin validate` path is the binding surface; `gemini
skills list` is supporting discovery evidence only.
