# Task Proposals (2026-04-29)

The following tasks are based on direct file inspection in this repository and are intentionally scoped for execution in one PR each.

## 1) Typo fix task
**Title:** Fix `recieve` misspelling in quick-fix workflow example

**Evidence:** `docs/workflows/quick-fix.md` includes `recieve` in both the user quote and example commit message.

**Scope:**
- Change `recieve` → `receive` in the two example lines.
- Keep the rest of the example unchanged.

**Acceptance criteria:**
- `rg -n "recieve" docs/workflows/quick-fix.md` returns no matches.
- Example remains a typo-fix walkthrough.

---

## 2) Bug fix task
**Title:** Avoid non-digit phase labels in daemon reconcile tests when `phase >= 10`

**Evidence:** Test IDs are built using ASCII arithmetic (`string(rune('0'+phase))`) in `cli/internal/daemon/reconcile_test.go`.
For two-digit values this generates punctuation (`:` etc.) instead of numeric strings.

**Scope:**
- Replace rune math with integer formatting (`strconv.Itoa(phase)` or `fmt.Sprintf("%d", phase)`).
- Update all affected test-field builders in that file.

**Acceptance criteria:**
- IDs/labels in tests render correctly for phase values above 9.
- `cd cli && go test ./internal/daemon -run Reconcile` passes.

---

## 3) Documentation discrepancy task
**Title:** Remove duplicate install command in AGENTS install section

**Evidence:** `AGENTS.md` repeats the same `install.sh` command twice under “Other agents (for example Cursor): install only selected skills”.

**Scope:**
- Deduplicate repeated command.
- Clarify whether the command installs all skills or selected skills, matching actual behavior.

**Acceptance criteria:**
- No duplicate command remains in the subsection.
- Wording aligns with command behavior.

---

## 4) Test improvement task
**Title:** Add regression test for tool-event scanners to ignore plain-text mentions

**Evidence:** `cli/testdata/transcripts/real-2.4mb.jsonl` contains both regular content strings and structured tool events; scanners that grep raw text can false-positive on tool-name mentions.

**Scope:**
- Add/extend a parser-level test that distinguishes:
  - structured tool events (`"type":"tool_use"` + event shape), and
  - incidental mentions inside free-form text.
- Use existing transcript fixtures or add a compact targeted fixture.

**Acceptance criteria:**
- Test fails with naive string matching and passes with structured-event parsing.
- Existing behavior for real tool-use events remains covered.
