# Homebrew Tap for AgentOps

Install the ao CLI via Homebrew.

## Quick Install

```bash
brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
brew install agentops
```

Or directly:

```bash
brew install boshu2/agentops/agentops
```

## Update to Latest

```bash
brew update && brew upgrade agentops
ao version
```

## Commands

```bash
ao forge search <query>    # Search knowledge base
ao forge index <path>      # Index knowledge artifacts
ao ratchet record <type>   # Record progress
ao ratchet verify <epic>   # Verify completion
```

## Claude Code Plugin

The ao CLI integrates with the AgentOps Claude Code plugin:

```bash
claude plugin add boshu2/agentops
```

## Development

To install from HEAD:

```bash
brew install --HEAD agentops
```
