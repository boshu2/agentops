# Output Templates

Expanded templates for specific tool types. Basic structure is in SKILL.md.

---

## CLI Tool (Expanded)

```markdown
## [Tool] vX.Y.Z (YYYY-MM-DD)

**Repo:** github.com/org/repo @ abc123

### Commands
| Task | Command | Notes |
|------|---------|-------|
| [task] | `[cmd]` | Added in vX.Y |

### Flags (Including Hidden)
| Flag | Description | Source |
|------|-------------|--------|
| `--flag` | [desc] | docs |
| `--hidden` | [desc] | source: file:123 |

### Config (`[filename]`)
```toml
[section]
option = "default"  # [description]
```

### Env Vars
| Variable | Default | Notes |
|----------|---------|-------|
| `VAR` | [default from code] | [notes] |

### Bleeding Edge (unreleased)
| Feature | PR | Status |
|---------|-----|--------|
| [feature] | #123 | merged, not released |

### Gotchas
- **[Issue]**: [fix]. Source: #456

### Patterns
```[lang]
// From: [tests/blog post]
[code]
```

### Sources
- Repo: [url] @ [commit]
- PRs: #123, #456
- Posts: [url] (2025-MM)
```

---

## Library/Framework

```markdown
## [Library] vX.Y.Z (YYYY-MM-DD)

**Install:** `[package manager command]`

### Core API
| Export | Purpose | Since |
|--------|---------|-------|
| `name` | [purpose] | vX.Y |

### New in Latest Release
| API | Description |
|-----|-------------|
| `name` | [desc] |

### Config
```[lang]
{
  option: "default", // [description]
}
```

### Patterns (2025-2026)
```[lang]
// Source: [blog/tests]
[code]
```

### Migration (from vX to vY)
- [breaking change]: [fix]

### Gotchas
- [issue]: [solution]
```

---

## Comparison

When researching alternatives:

```markdown
## [Tool A] vs [Tool B]

| Aspect | [A] | [B] |
|--------|-----|-----|
| Version | vX | vY |
| [aspect] | [A way] | [B way] |

### Use [A] when
- [scenario]

### Use [B] when
- [scenario]

### Migration A → B
1. [step]
```

---

## Minimal (Quick Research)

```markdown
## [Tool] (YYYY-MM-DD)

**Install:** `[cmd]`
**Key:** `[most common cmd]`
**Gotcha:** [one gotcha + fix]
**New:** [one 2025-2026 feature]
**Source:** [repo@commit]
```
