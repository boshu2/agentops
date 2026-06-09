# Packaging Strategies

Use this guide to choose the right delivery format for an extracted pattern.

## 1) Library / Crate / Package
Use when:
- The pattern is mostly code logic (functions, types, algorithms).
- Multiple projects need identical behavior.

Checklist:
- [ ] Public API is small and stable
- [ ] Tests cover the invariant core + edge cases
- [ ] README shows minimal usage
- [ ] Versioning policy defined (semver)

Example:
- Shared retry/backoff module

---

## 2) Skill (Workflow)
Use when:
- The pattern is a repeatable process or decision flow.
- Output is instructions, prompts, or checklists.

Checklist:
- [ ] SKILL.md includes triggers + exact prompt
- [ ] Examples show real usage
- [ ] SELF-TEST verifies core steps
- [ ] Validation passes (no executables, correct structure)

Example:
- Code review workflow

---

## 3) Template / Scaffold
Use when:
- The pattern is a project structure or file layout.
- You want to bootstrap new repos consistently.

Checklist:
- [ ] Template variables documented
- [ ] Defaults work out of the box
- [ ] Generated project builds/tests cleanly

Example:
- New Rust CLI scaffold with standard flags

---

## 4) Shared Config Bundle
Use when:
- The pattern is configuration (linting, formatting, CI).

Checklist:
- [ ] Config is minimal and composable
- [ ] Clear opt-out/override instructions

Example:
- Shared ESLint + Prettier baseline

---

## Decision Matrix

| Pattern Type | Best Package | Why |
|--------------|-------------|-----|
| Code logic | Library | Reusable, testable API |
| Workflow | Skill | Instructional, prompt-driven |
| Structure | Template | Repeatable scaffolding |
| Config | Shared config | Centralized defaults |
