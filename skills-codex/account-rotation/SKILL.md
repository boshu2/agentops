---
name: account-rotation
description: "Use when you hit a usage/rate limit on a coding-agent subscription and need to switch accounts, or to spread swarm lanes across accounts. Routes by host+agent: macOS+Claude → claude-acct (Keychain swap); macOS+Codex/Gemini or any Linux/WSL → caam (file swap)."
---

# account-rotation — switch coding-agent accounts on a rate limit

Route by host+agent, then let the tool do the swap:

```
macOS + Claude            → claude-acct  (Keychain layer)
macOS + Codex/Gemini      → caam         (file layer)
Linux / WSL  + anything   → caam         (file layer)
```

**Why the route:** caam swaps the auth *file*, which is correct for file-based
auth (Codex, Gemini, Claude-on-Linux). But Claude Code on macOS reads the login
**Keychain** (`Claude Code-credentials`) and ignores that file — so caam is a
no-op for Claude-on-Mac. `claude-acct` swaps the Keychain token **and** the
`~/.claude.json` `.oauthAccount` identity (both required, or only the current
account works).

```bash
claude-acct {list|current|use <name>|login <name> [email]}   # macOS + Claude
caam {status|next|use} <tool>                                 # else
```

**Two capture traps:** (1) distinct token bytes ≠ distinct accounts — verify by
account *email*, not hash; (2) the browser captures whichever account the provider
is signed into — log out / use Incognito before each capture login.

**Live-session caveat:** rotation changes what a *new* process picks up, not the
running session. Rotate, then relaunch. Ideal for putting each swarm lane on its
own account for parallel quota.
