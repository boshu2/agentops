# Decline Templates

Templates for politely declining features that don't fit the project scope. Always use HEREDOC format for proper multi-line handling.

---

## Contents

| Template | Use When |
|----------|----------|
| [Scope Creep](#scope-creep-general) | Feature expands project beyond intended focus |
| [Different Direction](#different-direction) | Conflicts with project roadmap |
| [Breaking Changes](#would-require-breaking-changes) | Would disrupt existing users |
| [Maintenance Burden](#maintenance-burden-too-high) | Too much ongoing work for benefit |
| [Dependency Concerns](#dependency-concerns) | Would add unwanted dependencies |
| [Already Solved](#already-solved-differently) | Workaround exists |
| [Too Niche](#too-niche) | Benefits too few users |
| [Security Concerns](#security-concerns) | Would introduce security risks |
| [Out of Scope](#out-of-scope-polite-redirect) | Fundamentally wrong tool |
| [Complexity](#complexity-outweighs-benefit) | Edge case handling too complex |

---

## Scope Creep (General)

```bash
gh issue close N -R owner/repo -c "$(cat <<'EOF'
Thanks for the suggestion! After consideration, this would add scope I'm trying to avoid for this project.

The tool is intentionally focused on [core use case]. Adding [requested feature] would:
- Increase maintenance burden
- Add complexity for edge cases
- Risk feature creep

Feel free to fork if you need this functionality. Thanks for understanding!
EOF
)"
```

---

## Different Direction

```bash
gh issue close N -R owner/repo -c "$(cat <<'EOF'
Thanks for the thoughtful suggestion! This doesn't quite fit the direction I'm taking the project.

[Brief explanation of actual direction]

I appreciate the input—it's helpful to understand what users are looking for, even when I can't implement it.
EOF
)"
```

---

## Would Require Breaking Changes

```bash
gh issue close N -R owner/repo -c "$(cat <<'EOF'
Implementing this would require breaking changes that I'm not ready to make.

The current behavior is intentional and changing it would affect existing users. I'd rather maintain stability than add this feature.

If this is critical for your use case, forking might be the best path forward.
EOF
)"
```

---

## Maintenance Burden Too High

```bash
gh issue close N -R owner/repo -c "$(cat <<'EOF'
I appreciate the suggestion! Unfortunately, this feature would add significant maintenance burden relative to its benefit.

As a solo maintainer, I need to be selective about what I add. This falls outside what I can reasonably support long-term.

Thanks for understanding!
EOF
)"
```

---

## Dependency Concerns

```bash
gh issue close N -R owner/repo -c "$(cat <<'EOF'
Implementing this would require adding [dependency], which I'd like to avoid.

The project aims to minimize external dependencies for:
- Simpler installation
- Fewer security surfaces
- Easier maintenance

If you need this functionality, [alternative tool] might be a better fit.
EOF
)"
```

---

## Already Solved Differently

```bash
gh issue close N -R owner/repo -c "$(cat <<'EOF'
The use case you're describing can already be achieved with:

```bash
# Example workaround
existing-command | other-tool
```

I don't plan to add a dedicated feature for this since the current approach works well enough.
EOF
)"
```

---

## Too Niche

```bash
gh issue close N -R owner/repo -c "$(cat <<'EOF'
Thanks for the suggestion! This is a bit too niche for the main project—it would add complexity for a small subset of users.

That said, this could work well as:
- A wrapper script
- A separate tool that uses this one
- A fork with your specific needs

Happy to help if you go one of those routes!
EOF
)"
```

---

## Security Concerns

```bash
gh issue close N -R owner/repo -c "$(cat <<'EOF'
I'm declining this due to security concerns.

[Brief explanation of risk]

The project prioritizes security over convenience in cases like this. I understand it's less flexible, but I'd rather err on the side of caution.
EOF
)"
```

---

## Out of Scope (Polite Redirect)

```bash
gh issue close N -R owner/repo -c "$(cat <<'EOF'
Thanks for the suggestion! This is a bit outside what this tool is designed for.

This tool focuses on [core purpose]. For [requested functionality], you might want to check out:
- [Alternative tool 1]
- [Alternative tool 2]

Hope that helps!
EOF
)"
```

---

## Complexity Outweighs Benefit

```bash
gh issue close N -R owner/repo -c "$(cat <<'EOF'
I've thought about this, but the implementation complexity outweighs the benefit.

Handling [edge cases the feature would require]:
- [Edge case 1]
- [Edge case 2]
- [Edge case 3]

Would make the code significantly more complex without proportional value.

Thanks for the suggestion though!
EOF
)"
```

---

## Response Guidelines

1. **Thank them** — They took time to suggest something
2. **Explain clearly** — Brief reason why it doesn't fit
3. **Offer alternatives** — Fork, other tools, workarounds
4. **Stay positive** — Don't dismiss their use case as invalid
5. **Be firm but kind** — No means no, but say it nicely
6. **Use HEREDOC** — Always wrap multi-line responses

---

## Tone Calibration

| Too Harsh | Just Right | Too Soft |
|-----------|------------|----------|
| "This doesn't fit" | "This doesn't quite fit the direction..." | "Maybe someday..." |
| "No" | "After consideration, this would add scope I'm trying to avoid" | "I'll think about it" |
| "Wrong tool" | "This is outside what this tool is designed for" | "Interesting idea..." |

---

## When NOT to Decline

Don't decline if:
- The feature is simple and fits scope
- It fixes a real usability issue
- Multiple users have requested it
- It aligns with project direction
- The maintenance cost is low

→ In these cases, just implement it and close.
