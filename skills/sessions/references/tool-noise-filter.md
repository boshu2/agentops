# Tool-Noise Filter Specification

> **Purpose:** Codify the rules for stripping non-conversational content from Claude Code JSONL transcripts when producing normalized session turns for retrieval. This is a versioned spec — amendments require a version bump and a note in the changelog at the bottom.

**Spec version:** `v1` (2026-04-11)
**Owner:** AO session-mining pipeline (Option D)
**Consumers:** Claude Code connector (cli/internal/parser + cli/internal/sessions turn writer)
**Related specs:** `.agents/research/2026-04-11-ao-session-mining-research-addendum.md`, `.agents/research/2026-04-11-sessions-privacy-policy.md`

---

## Scope

This spec covers **which parts of a Claude Code `.jsonl` transcript are kept vs stripped** when the transcript is consumed for **retrieval purposes** (i.e., for `ao inject` to surface relevant past turns for a new goal).

**Out of scope for this spec:**

- Knowledge extraction (already handled by `cli/internal/parser/Extractor` producing `storage.Session{Decisions, Knowledge, ...}`).
- Tool call telemetry (already handled by `TranscriptMessage.Tools []ToolCall`).
- Session-level metadata extraction (`cwd`, `gitBranch`, `model`, token counts) — those are written to frontmatter, not body.
- Privacy/credential redaction — handled separately in `.agents/research/2026-04-11-sessions-privacy-policy.md`.

---

## Design principle

The filter is **inclusive by default for conversational content** (user and assistant natural language) and **exclusive by default for structured tool invocations and system machinery**. When in doubt about a block type, **drop**. Retrieval quality is more sensitive to false positives (noisy hits) than to false negatives (missed hits) — the former degrade every query; the latter only matter if the dropped content was actually searchable.

---

## Field-level truth table — what survives the filter

Rows are combinations of top-level `type` field × content shape. Columns are actions. This is the **mechanically-verifiable conformance table** for parser tests.

### Level 1: Top-level message `type` routing

| `type` value | Route | Rationale |
|---|---|---|
| `user` | KEEP (pipe through Level 2 user rules) | Conversational input |
| `assistant` | KEEP (pipe through Level 2 assistant rules) | Conversational output |
| `system` | **DROP** | Internal system state, not conversational |
| `tool_use` | **DROP** | Structured tool invocation, captured separately via `Tools []ToolCall` telemetry |
| `tool_result` | **DROP** | Structured tool output, captured separately |
| `file-history-snapshot` | **DROP** | IDE state, not conversational |
| `attachment` | **DROP** in v1 | File attachment references; re-evaluate in v2 if attachment bodies become indexable |
| `permission-mode` | **DROP** | IDE permission state toggle |
| `last-prompt` | **DROP** | Sticky prompt state |
| *(unknown)* | **DROP with warning log** | Fail closed; unknown types are opaque |

### Level 2a: `user` message content shape

`user.message.content` can be either a string OR a list of blocks. Both forms must be handled.

| Content shape | Action | Output |
|---|---|---|
| `content: "some string"` | KEEP | Emit the string verbatim (after Level 3 XML strip) |
| `content: []` (empty list) | DROP (message has no content) | No output |
| `content: [{"type": "text", "text": "..."}]` | KEEP | Concatenate all `type:text` blocks' `text` fields with `\n\n` separator |
| `content: [{"type": "image", ...}]` | DROP image block | No text contribution; if message has NO text blocks, drop whole message |
| `content: [{"type": "document", ...}]` | DROP document block | Same as image |
| `content: [{"type": "<unknown>", ...}]` | DROP unknown block (log warning) | Fail closed |

### Level 2b: `assistant` message content shape

`assistant.message.content` is ALWAYS a list of blocks.

| Block type | Action | Rationale |
|---|---|---|
| `text` | KEEP `text` field | User-facing assistant output |
| `thinking` | **DROP** | Internal model reasoning, not user-facing; can leak debugging artifacts; users query for observable behavior not model chain-of-thought |
| `tool_use` | **DROP** | Tool invocation, captured via `Tools []ToolCall` |
| `tool_result` | **DROP** | Tool output, captured via `Tools []ToolCall` |
| `image` | **DROP** | Non-text; retrieval is over text only |
| `document` | **DROP** | Non-text |
| *(unknown)* | **DROP with warning log** | Fail closed |

Concatenate all kept `text` blocks with `\n\n` separator. If **all blocks are dropped**, the resulting message is empty — drop the whole message.

### Level 3: XML pseudo-block strip (applied to kept text content)

Claude Code injects several XML-like meta blocks into user messages via shell hooks and command processing. These are NOT real user intent and must be stripped:

| Pattern | Action | Notes |
|---|---|---|
| `<system-reminder>...</system-reminder>` | STRIP (entire block including tags) | Hook-injected guidance, pattern-matches "Reminder: ..." content |
| `<local-command-caveat>...</local-command-caveat>` | STRIP | `/commands` shell hook context |
| `<command-name>...</command-name>` | STRIP | Name of a `/slash-command` being invoked |
| `<command-message>...</command-message>` | STRIP | Prompt from command definition |
| `<command-args>...</command-args>` | KEEP (unwrap tags, keep content) | Args are the actual user intent when invoking a slash command |
| `<local-command-stdout>...</local-command-stdout>` | STRIP | Shell command output captured by `/bashrun` hook |
| `<local-command-stderr>...</local-command-stderr>` | STRIP | Shell command stderr |
| `[Tool: ...]` prefix at line start | STRIP the line | Hook-synthesized tool-call summary line |
| `[Bash: ...]` prefix at line start | STRIP the line | Hook-synthesized bash-call summary line |

**Implementation note:** XML strip is regex-based, NOT HTML-parser based. Claude's injected blocks are well-formed in practice but we don't depend on strict parsing; a simple non-greedy `<tag>...</tag>` regex per tag name suffices. If a tag is unclosed (e.g., `<system-reminder>` with no closing), strip from opening tag to end-of-message (fail-safe default for malformed input).

### Level 4: Post-strip empty-content drop

After Level 3 XML strip, if a message has no remaining text (all content was wrapped in stripped tags), **drop the entire message**. Zero-content turns degrade retrieval by adding empty hits.

### Level 5: Content-length floor (retrieval noise)

Messages with fewer than **20 characters** of post-filter content are **dropped**. Rationale: ultra-short turns ("yes", "ok", "thanks") are retrieval noise, not information. Spec floor is 20 chars, configurable via `min_turn_chars` in AO config (default: 20).

---

## Turn block assembly

After individual messages are filtered, assemble them into **TurnBlocks** — one per user→assistant pair. Each TurnBlock has:

```
{
  session_id: <string>         // from parent .jsonl filename or "sessionId" field
  turn_index: <int>            // 0-based, increments per user→assistant pair within a session
  parent_session_id: <string|null>  // only for subagent turns (isSidechain:true)
  workspace: <string>          // from cwd field on user messages
  model: <string>              // from assistant.message.model (latest in session)
  timestamp_start: <RFC3339>   // user message timestamp
  timestamp_end: <RFC3339>     // assistant message timestamp
  input_tokens: <int|null>     // from assistant.message.usage.input_tokens
  output_tokens: <int|null>    // from assistant.message.usage.output_tokens
  user_text: <string>          // filtered user content
  assistant_text: <string>     // filtered assistant content
}
```

### Pairing rules

1. **User without assistant response (yet)** — DROP. Retrieval only surfaces completed turns.
2. **Assistant without preceding user** — DROP. (Rare; only appears in subagent re-entry scenarios.)
3. **Multiple user messages in a row** — concatenate into one `user_text` with `\n\n` separator; pair with the next assistant response.
4. **Multiple assistant messages in a row** — concatenate into one `assistant_text`; pair with the most recent user message.
5. **Subagent interleaving** — subagent sessions from `<session-id>/subagents/agent-*.jsonl` are loaded SEPARATELY and their turns are written as distinct TurnBlocks with `parent_session_id` set to the parent session's ID. Do NOT attempt to inline subagent turns into parent session turn sequence; retrieval treats them as peers.

### Subagent join (directory-based)

For a parent session at `~/.claude/projects/<project>/<id>.jsonl`:

1. Check if `~/.claude/projects/<project>/<id>/subagents/` directory exists
2. If yes, for each `agent-*.jsonl` file in that directory:
   - Parse the file the same way as the parent, apply filter rules, assemble TurnBlocks
   - Tag each subagent TurnBlock with `parent_session_id = <parent id>`
   - Write subagent TurnBlocks to the same `.agents/ao/sessions/turns/` directory with filename `<YYYY-MM-DD>-<parent-id>-sub-<agent-slug>-<turn-index>.md`
3. If no subagents directory, proceed with parent session turns only (log at debug level)

---

## Parser behavior audit (what `cli/internal/parser/` already does)

Per the research addendum, the existing parser already:

- Handles `user` and `assistant` message types
- Handles polymorphic `content` (string or block list) via `extractMessageContent`
- Splits `tool_use` and `tool_result` into separate `Tools []ToolCall` field
- Truncates content at 500 chars by default (`DefaultMaxContentLength = 500`) — **must be set to 0 for retrieval path via `parser.MaxContentLength = 0`, which `ao forge transcript` already does at forge.go:273**

**Gaps the existing parser does NOT handle** (which Option D's turn-writer must fill):

- Subagent directory traversal (no `subagents` or `isSidechain` awareness in parser.go)
- XML pseudo-block strip (Level 3 above is net-new)
- `thinking` block strip (parser emits the raw content block; filtering happens downstream)
- Content-length floor (parser emits everything; floor is applied at turn-writer)
- XML pseudo-block strip below is net-new

**Implementation strategy:** do NOT modify `cli/internal/parser/parser.go`. Add a new post-processor in `cli/internal/sessions/` that consumes `parser.ParseResult.Messages` and applies Level 2b, Level 3, Level 4, Level 5 rules before assembling TurnBlocks. This keeps the existing forge knowledge-extraction path untouched.

---

## Conformance test coverage (what W1 test-first-mode must cover)

For spec v1, the following test cases MUST exist in `cli/internal/sessions/filter_test.go`:

1. **TestFilter_UserStringContent_Kept** — user message with `content: "fix the build"` → TurnBlock.user_text == "fix the build"
2. **TestFilter_UserListContent_TextOnly_Concatenated** — user with `[{type: text}, {type: text}]` → concatenated with `\n\n`
3. **TestFilter_UserImageBlock_Dropped_MessageSurvives** — user with `[{type: text, "..."}, {type: image, ...}]` → text kept, image dropped, message kept
4. **TestFilter_UserImageBlockOnly_MessageDropped** — user with `[{type: image, ...}]` only → whole message dropped (no text content)
5. **TestFilter_AssistantTextOnly_Kept** — assistant with `[{type: text}]` → kept
6. **TestFilter_AssistantThinkingBlock_Dropped** — assistant with `[{type: thinking}, {type: text}]` → thinking dropped, text kept
7. **TestFilter_AssistantThinkingOnly_MessageDropped** — assistant with `[{type: thinking}]` only → whole message dropped
8. **TestFilter_AssistantToolUseBlock_Dropped** — assistant with `[{type: tool_use}, {type: text}]` → tool_use dropped, text kept
9. **TestFilter_AssistantToolUseOnly_MessageDropped** — assistant with `[{type: tool_use}]` only → whole message dropped
10. **TestFilter_SystemMessage_Dropped** — top-level `type: system` → dropped
11. **TestFilter_FileHistorySnapshot_Dropped** — top-level `type: file-history-snapshot` → dropped
12. **TestFilter_PermissionMode_Dropped** — top-level `type: permission-mode` → dropped
13. **TestFilter_UnknownType_DroppedWithWarning** — top-level `type: mystery-type` → dropped, warning log captured
14. **TestFilter_XMLStrip_SystemReminder** — user text containing `foo <system-reminder>ignore me</system-reminder> bar` → "foo  bar"
15. **TestFilter_XMLStrip_CommandArgsUnwrapped** — user text with `<command-name>/rpi</command-name><command-args>build X</command-args>` → "build X"
16. **TestFilter_XMLStrip_UnclosedTag_FailSafe** — user text with `before <system-reminder>oops no close` → "before "
17. **TestFilter_ContentLengthFloor_ShortDropped** — post-filter content with 10 chars → message dropped
18. **TestFilter_ContentLengthFloor_ExactlyAtFloor_Kept** — post-filter content with exactly 20 chars → message kept
19. **TestTurnBlock_UserAssistantPair_Assembled** — one `user` + one `assistant` in sequence → one TurnBlock
20. **TestTurnBlock_UserOnly_Dropped** — `user` with no following assistant → no TurnBlock emitted
21. **TestTurnBlock_MultipleUsersInRow_Concatenated** — `user`, `user`, `assistant` → one TurnBlock with concatenated user_text
22. **TestTurnBlock_MultipleAssistantsInRow_Concatenated** — `user`, `assistant`, `assistant` → one TurnBlock with concatenated assistant_text
23. **TestSubagentJoin_DirectoryExists_TurnBlocksEmitted** — parent `.jsonl` + `subagents/agent-*.jsonl` → parent turns + subagent turns emitted as peers with `parent_session_id` populated
24. **TestSubagentJoin_DirectoryMissing_ParentOnly** — parent `.jsonl` alone → parent turns only, no error, debug log
25. **TestSubagentJoin_EmptySubagentDir_ParentOnly** — parent `.jsonl` + empty `subagents/` dir → parent turns only

These 25 tests are the **conformance matrix for spec v1**. Any change to filter rules requires amending this matrix and bumping the spec version.

---

## Versioning & amendments

- **v1 — 2026-04-11** — Initial spec. Covers Claude Code JSONL format as of 2026-04-11 (claude-opus-4-6 era). Derived from research addendum findings. Ratified by the plan document as the pre-mortem W0-1 artifact.

**Amendment process:**

1. Format drift in Claude JSONL requires a new spec version
2. New block types or new XML hook patterns require amending the tables above
3. Lowering the content-length floor requires a retrieval-quality eval re-run
4. Every amendment requires: (a) version bump, (b) changelog entry, (c) updated conformance test coverage, (d) retrieval-quality re-run on the 20-query baseline

---

## References

- `.agents/research/2026-04-11-ao-session-mining-research.md` — initial research (Findings 2, 7)
- `.agents/research/2026-04-11-ao-session-mining-research-addendum.md` — scope revision after discovering existing forge pipeline
- `cli/internal/parser/parser.go` — upstream parser (handles Level 1 + Level 2a/2b partially)
- `cli/internal/parser/extractor.go` — upstream knowledge extractor (different pipeline, not affected by this spec)
- `.agents/plans/2026-04-11-ao-session-mining-option-d.md` — the plan this spec supports
- `.agents/council/2026-04-11-consolidated-ao-session-mining.md` — council consensus that mandated this spec

## Changelog

- **2026-04-11 v1** — Initial ratified version. 25 conformance test cases. Covers user/assistant content filtering, XML pseudo-block strip, subagent directory join, content-length floor, TurnBlock assembly.
