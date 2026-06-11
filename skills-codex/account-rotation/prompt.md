# Execution Profile — account-rotation

When the user hits a usage/rate limit and wants to switch accounts (or to spread
swarm lanes across accounts):

1. Detect host (`uname`) and which agent CLI is being rotated.
2. Route: macOS+Claude → `claude-acct`; macOS+Codex/Gemini or any Linux/WSL → `caam`.
3. macOS+Claude only: a full account = Keychain token + `~/.claude.json` `.oauthAccount`;
   `claude-acct use <name>` swaps both. caam is a no-op there (it swaps a file Claude ignores).
4. Capturing a new account: log out / use Incognito first (the browser captures the
   currently-signed-in account); verify by account email, not token hash.
5. Rotation affects new processes, not the live session — relaunch to move yourself.
