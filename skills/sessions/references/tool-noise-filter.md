# Tool-Noise Filter Specification — v1

> **Purpose:** Rules for stripping non-conversational content from Claude Code JSONL transcripts when producing normalized turn content for retrieval. Versioned spec — amendments require version bump and changelog entry at the bottom.

**Spec version:** v1 (2026-04-11)
**Owner:** AO session-mining pipeline (Option D)
**Consumers:** Claude Code connector (cli/internal/sessions post-processor, W1/W2 waves)
**Related:** `.agents/research/2026-04-11-ao-session-mining-research-addendum.md`, `.agents/research/2026-04-11-sessions-privacy-policy.md`

---

## Scope

Covers **which parts of a Claude Code .jsonl transcript are kept vs stripped** when consumed for **retrieval purposes** (i.e., for `ao inject` to surface relevant past turns). 

**Out of scope:** knowledge extraction (already in `cli/internal/parser/Extractor`), tool call telemetry (already in `TranscriptMessage.Tools`), session metadata extraction, privacy redaction (handled separately in `.agents/research/2026-04-11-sessions-privacy-policy.md`).

## Design principle

**Inclusive by default** for conversational content (user + assistant natural language), **exclusive by default** for tool invocations and system machinery. When in doubt → drop. Retrieval is more sensitive to false positives (noisy hits) than false negatives (missed hits).

## Level 1 — Top-level message `type` routing

| `type` | Action |
|---|---|
| `user` | KEEP (pipe through Level 2a) |
| `assistant` | KEEP (pipe through Level 2b) |
| `system` | DROP (internal state) |
| `tool_use` | DROP (captured via `Tools []ToolCall`) |
| `tool_result` | DROP (captured via `Tools []ToolCall`) |
| `file-history-snapshot` | DROP (IDE state) |
| `attachment` | DROP in v1 |
| `permission-mode` | DROP |
| `last-prompt` | DROP |
| *unknown* | DROP with warning log (fail closed) |

## Level 2a — user `content` shape

| Content | Action | Output |
|---|---|---|
| `"string"` | KEEP | Emit verbatim (then Level 3 strip) |
| `[]` empty | DROP | None |
| `[{type:text,...}]` | KEEP text | Concatenate `type:text` `text` fields with `\n\n` |
| `[{type:image,...}]` | DROP image | No contribution; message dropped if no text blocks |
| `[{type:document,...}]` | DROP | Same |
| `[{type:unknown,...}]` | DROP with warn | Fail closed |

## Level 2b — assistant `content` blocks

| Block type | Action |
|---|---|
| `text` | KEEP `text` field |
| `thinking` | **DROP** (internal reasoning, leaks debug artifacts) |
| `tool_use` | DROP |
| `tool_result` | DROP |
| `image`, `document` | DROP |
| *unknown* | DROP with warn |

Concatenate kept blocks with `\n\n`. If all blocks dropped → drop message.

## Level 3 — XML pseudo-block strip

Claude Code injects shell-synthesized XML blocks into user content. These are NOT user intent.

| Pattern | Action |
|---|---|
| `<system-reminder>...</system-reminder>` | STRIP entire block |
| `<local-command-caveat>...</local-command-caveat>` | STRIP |
| `<command-name>...</command-name>` | STRIP |
| `<command-message>...</command-message>` | STRIP |
| `<command-args>...</command-args>` | KEEP content, strip tags (args are real intent) |
| `<local-command-stdout>...</local-command-stdout>` | STRIP |
| `<local-command-stderr>...</local-command-stderr>` | STRIP |
| `[Tool: ...]` line prefix | STRIP line |
| `[Bash: ...]` line prefix | STRIP line |

Regex-based. If tag is unclosed, strip from opening to end-of-message (fail-safe).

## Level 4 — Post-strip empty drop

If post-Level-3 message has no text, drop entire message.

## Level 5 — Content-length floor

Messages < 20 chars post-filter → DROP. Configurable via `min_turn_chars` (default 20).

## Turn block assembly

After per-message filtering, assemble into **TurnBlocks** — one per user→assistant pair:

```
{
  session_id, turn_index, parent_session_id,
  workspace, model, timestamp_start, timestamp_end,
  input_tokens, output_tokens,
  user_text, assistant_text
}
```

**Pairing rules:**
1. User without assistant response → DROP (incomplete turn)
2. Assistant without preceding user → DROP (rare; subagent re-entry)
3. Multiple users in a row → concat into one `user_text`; pair with next assistant
4. Multiple assistants in a row → concat into one `assistant_text`; pair with most recent user
5. Subagents → separate TurnBlocks with `parent_session_id` populated; do NOT inline into parent sequence

**Subagent join (directory-based):**
For parent `~/.claude/projects/<project>/<id>.jsonl`:
1. Check `~/.claude/projects/<project>/<id>/subagents/` exists
2. For each `agent-*.jsonl`: parse + filter + assemble, tag with `parent_session_id = <parent-id>`
3. Filename for subagent turns: `<YYYY-MM-DD>-<parent-id>-sub-<agent-slug>-<turn-index>.md`
4. If dir missing: parent only, debug log

## Parser gap audit

The existing `cli/internal/parser/` (see research addendum) already handles:
- user/assistant types, polymorphic content, tool_use/tool_result splitting
- Content truncation (500 chars default; set 0 for retrieval path — forge.go:273 already does this)

**Gaps this spec fills (new code in `cli/internal/sessions/`):**
- Subagent directory traversal
- XML pseudo-block strip (Level 3)
- `thinking` block strip (parser emits raw blocks; filtering is downstream)
- Content-length floor
- TurnBlock assembly

**Strategy:** do NOT modify `cli/internal/parser/parser.go`. Add post-processor in `cli/internal/sessions/` that consumes `parser.ParseResult.Messages` and applies Levels 2b/3/4/5 before TurnBlock assembly.

## Conformance test matrix (W1 test-first)

25 required test cases in `cli/internal/sessions/filter_test.go`:

1. **TestFilter_UserStringContent_Kept** — string content preserved verbatim
2. **TestFilter_UserListContent_TextOnly_Concatenated** — text blocks joined with `\n\n`
3. **TestFilter_UserImageBlock_Dropped_MessageSurvives** — image dropped, text kept
4. **TestFilter_UserImageBlockOnly_MessageDropped** — message with only image → dropped
5. **TestFilter_AssistantTextOnly_Kept** — text-only assistant kept
6. **TestFilter_AssistantThinkingBlock_Dropped** — thinking dropped, text kept
7. **TestFilter_AssistantThinkingOnly_MessageDropped** — thinking-only message dropped
8. **TestFilter_AssistantToolUseBlock_Dropped** — tool_use dropped, text kept
9. **TestFilter_AssistantToolUseOnly_MessageDropped** — tool_use-only message dropped
10. **TestFilter_SystemMessage_Dropped** — type:system dropped
11. **TestFilter_FileHistorySnapshot_Dropped** — file-history-snapshot dropped
12. **TestFilter_PermissionMode_Dropped** — permission-mode dropped
13. **TestFilter_UnknownType_DroppedWithWarning** — unknown type dropped with warn log
14. **TestFilter_XMLStrip_SystemReminder** — `foo <system-reminder>ignore</system-reminder> bar` → `foo  bar`
15. **TestFilter_XMLStrip_CommandArgsUnwrapped** — `<command-name>/rpi</command-name><command-args>build X</command-args>` → `build X`
16. **TestFilter_XMLStrip_UnclosedTag_FailSafe** — `before <system-reminder>oops` → `before `
17. **TestFilter_ContentLengthFloor_ShortDropped** — 10-char content → dropped
18. **TestFilter_ContentLengthFloor_ExactlyAtFloor_Kept** — 20-char content → kept
19. **TestTurnBlock_UserAssistantPair_Assembled** — one user + one assistant → one TurnBlock
20. **TestTurnBlock_UserOnly_Dropped** — user with no assistant → no TurnBlock
21. **TestTurnBlock_MultipleUsersInRow_Concatenated** — user, user, assistant → one TurnBlock with concatenated user_text
22. **TestTurnBlock_MultipleAssistantsInRow_Concatenated** — user, assistant, assistant → one TurnBlock with concatenated assistant_text
23. **TestSubagentJoin_DirectoryExists_TurnBlocksEmitted** — parent + subagents/agent-*.jsonl → parent + subagent TurnBlocks with `parent_session_id`
24. **TestSubagentJoin_DirectoryMissing_ParentOnly** — parent alone → parent turns, no error
25. **TestSubagentJoin_EmptySubagentDir_ParentOnly** — parent + empty subagents/ → parent turns only

Any change to filter rules requires amending this matrix and bumping the spec version.

## Amendments

Format drift in Claude JSONL → new spec version. Every amendment requires: version bump, changelog entry, updated conformance tests, retrieval-quality re-run on the 20-query baseline.

## References

- `.agents/research/2026-04-11-ao-session-mining-research.md` (Findings 2, 7)
- `.agents/research/2026-04-11-ao-session-mining-research-addendum.md` (scope revision)
- `cli/internal/parser/parser.go` (upstream parser, handles Levels 1 + 2a/2b partially)
- `cli/internal/parser/extractor.go` (knowledge extractor, different pipeline, not affected)
- `.agents/plans/2026-04-11-ao-session-mining-option-d.md`
- `.agents/council/2026-04-11-consolidated-ao-session-mining.md`

## Changelog

- **v1 — 2026-04-11** — Initial ratified version. 25 conformance tests. Covers user/assistant content filtering, XML pseudo-block strip, subagent directory join, content-length floor, TurnBlock assembly.
