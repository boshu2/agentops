# README Template for Documentation Folders

Use this template when generating READMEs for documentation folders.

## Standard Template (~50-80 lines)

```markdown
# [Folder Name]: [Brief Purpose]

**One sentence:** What this folder contains and why it exists.

---

## What's Here

| Document | Purpose | ~Tokens |
|----------|---------|---------|
| `file1.md` | Brief description | 2k |
| `file2.md` | Brief description | 1.5k |
| `subfolder/` | What the subfolder contains | - |

---

## Start Here

**New to this topic?** Read in this order:

1. `introduction.md` (X min) - Overview
2. `getting-started.md` (X min) - First steps
3. `reference.md` (X min) - Detailed reference

---

## Related Sections

- [../related-folder/](../related-folder/) - How this relates
- [../another-folder/](../another-folder/) - Connection point

---

**Last Updated:** YYYY-MM-DD
```

## Guidelines

### Title
- Use the folder name, humanized (e.g., `04-research` → "Research")
- Add a brief descriptor after colon

### One Sentence
- Explain what the folder contains
- Mention who it's for if relevant
- Keep under 20 words

### What's Here Table
- List all `.md` files and subfolders
- Brief 3-5 word description
- Approximate token count (words ÷ 0.75)

### Start Here
- Only include if there's a natural reading order
- Limit to 3-5 items
- Include time estimates

### Related Sections
- Link to 2-4 related folders
- Explain the relationship briefly

### Token Budget
- Target: 50-100 lines
- Max: 150 lines (for hub READMEs)
- Aim for ~400-800 tokens

## Variations

### Hub README (larger folders)
- Can be 100-150 lines
- Add "Quick Links" section
- Add "By Role" or "By Use Case" navigation

### Leaf README (simple folders)
- Can be 30-50 lines
- Skip "Start Here" if only 2-3 files
- Simpler "What's Here" (bullet list OK)

### Auto-Generated Index README
- Note it's auto-generated
- Include generation timestamp
- Don't edit manually (will be overwritten)
