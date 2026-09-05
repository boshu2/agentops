#!/usr/bin/env python3
"""Structural conformance for the Cathedral Cut product boundary."""

from __future__ import annotations

import ast
import hashlib
import html
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from urllib.parse import unquote_to_bytes

import yaml


ROOT = Path(__file__).resolve().parents[1]
CORE = ("rpi", "plan", "implement", "validate")
CORE_SCHEMAS = (
    "subject-manifest.v1.schema.json",
    "verdict.v2.schema.json",
    "rpi-report.v1.schema.json",
)
COMPATIBILITY_SCHEMAS = (
    "plan-packet.v1.schema.json",
    "candidate-packet.v1.schema.json",
    "revision-packet.v1.schema.json",
)
PACKET_FREE_NARRATIVE = (
    "GOALS.md",
    "PROGRAM.md",
    "docs/software-factory.md",
    "docs/seed-definition.md",
    "docs/INCIDENT-RUNBOOK.md",
    "docs/templates/README.md",
    "docs/templates/intent-issue.md",
    "docs/templates/slice-validation.md",
)
LEGACY_PACKET_TOKENS = (
    "PlanPacket", "CandidatePacket", "RevisionPacket",
    "plan-packet.v1", "candidate-packet.v1", "revision-packet.v1",
)
FORBIDDEN_STATE = {
    "owner", "ready", "claim", "priority", "attempt", "attempts", "queue",
    "lease", "admission", "next_action", "next-action", "close", "closure",
    "release", "delivery", "budget", "retry", "retries",
}
FORBIDDEN_SCHEMA_STATE = {
    "retry", "retries", "budget", "queue", "claim", "lease", "admission",
    "next_action", "next-action", "closure", "release", "delivery",
}
# ADR-0017 (loop as control flow, not knowledge): `crank` is restored as a thin
# wave skill — one caller-selected wave per invocation, forwarding the caller's
# repair bound to RPI. The `ao crank` ROOT COMMAND stays removed and is still
# tombstoned in REMOVED_COMMANDS below; `converge` stays removed as a skill
# because its criterion now lives inside RPI's convergence law.
REMOVED_SKILLS = {
    "discovery", "behavior-first-planning", "goal-design", "converge",
    "evolve", "gc-membrane", "pawl-review", "push", "release", "pr-prep",
    "beads-br", "beads-bv",
    # ADR-0018 (Train 2 retirements): goals was a dead alias for fitness, shared
    # a tombstone with no consumer, and scope's checks folded into plan.
    # Reintroducing any of the three fails this gate.
    "goals", "shared", "scope",
}
REMOVED_MORTEM_ALIASES = {
    "pre-mortem", "pre_mortem", "post-mortem", "post_mortem",
}
REMOVED_COMMANDS = {
    "pawl", "plan-pawl", "land", "done", "close", "governor", "yield",
    "claim", "next-work", "state", "worktree", "validate", "converge",
    "reconcile", "membrane", "crank", "flywheel",
}
RETIRED_SCHEMAS = {
    "verdict.v1.schema.json", "pawl-verdict.v1.schema.json",
    "validation-receipt.v1.schema.json", "validation-request.v1.schema.json",
    "execution-packet.schema.json", "next-work-batch.v1.schema.json",
    "next-work-item.v1.schema.json", "yieldledger-event.v1.schema.json",
    "claim-registry.v1.schema.json", "verdict-ledger.v1.schema.json",
}
LINKED_SKILL_REFERENCE_PATTERNS = (
    (
        "retired AgentOps product identity",
        re.compile(
            r"\bAgentOps is (?:the|an?|your) (?:seven-move )?"
            r"(?:operating[ -]loop|operating system|global control plane|"
            r"execution orchestrator|software factory)\b",
            re.IGNORECASE,
        ),
    ),
    (
        "retired knowledge-flywheel identity",
        re.compile(r"\bknowledge[ -]flywheel\b", re.IGNORECASE),
    ),
    (
        "retired AgentOps flywheel command",
        re.compile(r"\bao\s+flywheel\b", re.IGNORECASE),
    ),
    (
        "retired corpus-installation claim",
        re.compile(r"\binstalls the corpus\b", re.IGNORECASE),
    ),
    (
        "retired operating-loop section label",
        re.compile(r"^#{1,6}\s+operating[- ]loop\s+use\s*$", re.IGNORECASE),
    ),
)
MARKDOWN_FENCE_OPEN = re.compile(r"^[ ]{0,3}(`{3,}|~{3,})")
MARKDOWN_RAW_HTML_BLOCK_OPEN = re.compile(
    r"^[ ]{0,3}<(?:pre|script|style|textarea)(?=[ \t>]|$)",
    re.IGNORECASE,
)
MARKDOWN_RAW_HTML_BLOCK_CLOSE = re.compile(
    r"</(?:pre|script|style|textarea)>",
    re.IGNORECASE,
)
MARKDOWN_LIST_ITEM = re.compile(r"^[ \t]{0,3}(?:[-+*]|\d{1,9}[.)])[ \t]+")
MARKDOWN_HEADING = re.compile(r"^[ \t]{0,3}#{1,6}(?:[ \t]+|$)")
MARKDOWN_SETEXT_UNDERLINE = re.compile(r"^[ ]{0,3}(?:=+|-+)[ \t]*$")
MARKDOWN_INLINE_HTML_TAG = re.compile(r"</?[A-Za-z][^<>]*>")
MARKDOWN_EMPHASIS = re.compile(
    r"(?<!\w)(\*{1,3}|_{1,3})(?=\S)(.+?\S)\1(?!\w)",
)
MARKDOWN_ESCAPABLE = frozenset(r'''!"#$%&'()*+,-./:;<=>?@[\]^_`{|}~''')
ENCODED_PATH_SEPARATOR = re.compile(r"%(?:2f|5c)", re.IGNORECASE)


def frontmatter(name: str) -> dict:
    text = (ROOT / "skills" / name / "SKILL.md").read_text(encoding="utf-8")
    parts = text.split("---", 2)
    if len(parts) != 3:
        raise AssertionError(f"{name}: missing frontmatter")
    value = yaml.safe_load(parts[1]) or {}
    if not isinstance(value, dict):
        raise AssertionError(f"{name}: invalid frontmatter")
    return value


def normalize_reference_label(label: str) -> str:
    """Apply CommonMark's case-insensitive, whitespace-collapsed label shape."""
    return " ".join(markdown_unescape_destination(label).split()).casefold()


def blank_preserving_lines(value: str) -> str:
    """Blank Markdown syntax without moving subsequent source line numbers."""
    return "".join(char if char in "\r\n" else " " for char in value)


def markdown_character_is_escaped(text: str, offset: int) -> bool:
    """Return whether an odd run of backslashes escapes text[offset]."""
    backslashes = 0
    cursor = offset - 1
    while cursor >= 0 and text[cursor] == "\\":
        backslashes += 1
        cursor -= 1
    return backslashes % 2 == 1


def markdown_link_is_active(text: str, offset: int) -> bool:
    """Exclude escaped links and image syntax from reference discovery."""
    if markdown_character_is_escaped(text, offset):
        return False
    if offset > 0 and text[offset - 1] == "!":
        return markdown_character_is_escaped(text, offset - 1)
    return True


def blank_inline_code(text: str) -> str:
    """Blank matched backtick code spans while preserving every newline."""
    output = list(text)
    cursor = 0
    while cursor < len(text):
        if text[cursor] != "`" or markdown_character_is_escaped(text, cursor):
            cursor += 1
            continue
        opener_end = cursor + 1
        while opener_end < len(text) and text[opener_end] == "`":
            opener_end += 1
        width = opener_end - cursor
        search = opener_end
        closing_end = None
        while search < len(text):
            if text[search] != "`":
                search += 1
                continue
            run_end = search + 1
            while run_end < len(text) and text[run_end] == "`":
                run_end += 1
            if run_end - search == width:
                closing_end = run_end
                break
            search = run_end
        if closing_end is None:
            cursor = opener_end
            continue
        output[cursor:closing_end] = blank_preserving_lines(text[cursor:closing_end])
        cursor = closing_end
    return "".join(output)


def markdown_line_is_indented_code(line: str) -> bool:
    """Return whether leading spaces/tabs reach CommonMark's four columns."""
    columns = 0
    for character in line:
        if character == " ":
            columns += 1
        elif character == "\t":
            columns += 4 - (columns % 4)
        else:
            break
        if columns >= 4:
            return True
    return False


def markdown_list_item_content(line: str) -> tuple[str, int] | None:
    """Return a list item's first-block content and its continuation indent."""
    cursor = 0
    columns = 0
    while cursor < len(line) and line[cursor] in " \t":
        width = 1 if line[cursor] == " " else 4 - (columns % 4)
        if columns + width > 3:
            break
        columns += width
        cursor += 1

    marker_start = cursor
    if cursor < len(line) and line[cursor] in "-+*":
        cursor += 1
    else:
        digits_start = cursor
        while cursor < len(line) and line[cursor].isdigit():
            cursor += 1
        if (
            cursor == digits_start
            or cursor - digits_start > 9
            or cursor >= len(line)
            or line[cursor] not in ".)"
        ):
            return None
        cursor += 1
    if cursor == marker_start or cursor >= len(line) or line[cursor] not in " \t":
        return None

    marker_columns = columns + len(line[marker_start:cursor])
    whitespace_start = cursor
    whitespace_columns = marker_columns
    while cursor < len(line) and line[cursor] in " \t":
        if line[cursor] == " ":
            whitespace_columns += 1
        else:
            whitespace_columns += 4 - (whitespace_columns % 4)
        cursor += 1
    spacing = whitespace_columns - marker_columns
    if spacing > 4:
        cursor = whitespace_start + 1
        whitespace_columns = marker_columns + (
            1 if line[whitespace_start] == " " else 4 - (marker_columns % 4)
        )
    return line[cursor:], whitespace_columns


def markdown_container_contents(line: str) -> list[tuple[str, int]]:
    """Return raw and list-item block candidates with continuation indents."""
    candidates = [(line, 0)]
    content = line
    total_indent = 0
    for _ in range(8):
        item = markdown_list_item_content(content)
        if item is None:
            break
        content, item_indent = item
        total_indent += item_indent
        candidates.append((content, total_indent))
    return candidates


def markdown_strip_indent(line: str, columns: int) -> str | None:
    """Remove a container continuation indent without consuming prose."""
    cursor = 0
    consumed = 0
    while cursor < len(line) and consumed < columns and line[cursor] in " \t":
        width = 1 if line[cursor] == " " else 4 - (consumed % 4)
        consumed += width
        cursor += 1
        if consumed > columns:
            return " " * (consumed - columns) + line[cursor:]
    if consumed < columns:
        return None
    return line[cursor:]


def markdown_fence_opening(line: str) -> tuple[str, int, int] | None:
    """Return a valid fence character, width, and list-container indent."""
    for content, container_indent in markdown_container_contents(line):
        opening = MARKDOWN_FENCE_OPEN.match(content)
        if opening is None:
            continue
        fence = opening.group(1)
        info_string = content[opening.end(1):]
        if fence[0] == "`" and "`" in info_string:
            continue
        return fence[0], len(fence), container_indent
    return None


def markdown_raw_html_opening(line: str) -> int | None:
    """Return the continuation indent for a CommonMark type-1 HTML block."""
    for content, container_indent in markdown_container_contents(line):
        if MARKDOWN_RAW_HTML_BLOCK_OPEN.match(content):
            return container_indent
    return None


def blank_html_comments(line: str, active: bool) -> tuple[str, bool]:
    """Blank HTML comment spans on one line and carry multiline state."""
    output = list(line)
    cursor = 0
    if active:
        closing = line.find("-->")
        if closing < 0:
            return blank_preserving_lines(line), True
        output[:closing + 3] = " " * (closing + 3)
        cursor = closing + 3
        active = False
    while cursor < len(line):
        opening = line.find("<!--", cursor)
        if opening < 0:
            break
        closing = line.find("-->", opening + 4)
        if closing < 0:
            output[opening:] = " " * (len(line) - opening)
            active = True
            break
        output[opening:closing + 3] = " " * (closing + 3 - opening)
        cursor = closing + 3
    return "".join(output), active


def markdown_blockquote_content(line: str) -> tuple[str, int]:
    """Strip nested CommonMark blockquote markers and return container depth."""
    cursor = 0
    depth = 0
    while cursor < len(line):
        probe = cursor
        spaces = 0
        while probe < len(line) and line[probe] == " " and spaces < 3:
            probe += 1
            spaces += 1
        if probe >= len(line) or line[probe] != ">":
            break
        depth += 1
        cursor = probe + 1
        if cursor < len(line) and line[cursor] in " \t":
            cursor += 1
    return (line[cursor:] if depth else line), depth


def markdown_without_code(text: str) -> str:
    """Blank Markdown code and inactive raw HTML while retaining line positions."""
    output: list[str] = []
    fence_character = ""
    fence_width = 0
    fence_container_indent = 0
    fence_quote_depth = 0
    raw_html_active = False
    raw_html_container_indent = 0
    raw_html_quote_depth = 0
    html_comment_active = False
    paragraph_active = False
    previous_quote_depth = 0
    for line in text.splitlines(keepends=True):
        line_without_ending = line.rstrip("\r\n")
        line_ending = line[len(line_without_ending):]
        content, quote_depth = markdown_blockquote_content(line_without_ending)
        normalized_line = content + line_ending
        if quote_depth != previous_quote_depth:
            paragraph_active = False
        previous_quote_depth = quote_depth

        while True:
            if fence_character:
                contained = markdown_strip_indent(content, fence_container_indent)
                if quote_depth < fence_quote_depth or (
                    contained is None and content.strip()
                ):
                    fence_character = ""
                    fence_width = 0
                    fence_container_indent = 0
                    paragraph_active = False
                    continue
                closing_content = contained if contained is not None else ""
                closing = re.match(
                    rf"^[ ]{{0,3}}{re.escape(fence_character)}"
                    rf"{{{fence_width},}}[ \t]*$",
                    closing_content,
                )
                output.append(blank_preserving_lines(normalized_line))
                if closing is not None:
                    fence_character = ""
                    fence_width = 0
                    fence_container_indent = 0
                    paragraph_active = False
                break

            if raw_html_active:
                contained = markdown_strip_indent(content, raw_html_container_indent)
                if quote_depth < raw_html_quote_depth or (
                    contained is None and content.strip()
                ):
                    raw_html_active = False
                    raw_html_container_indent = 0
                    paragraph_active = False
                    continue
                html_content = contained if contained is not None else ""
                output.append(blank_preserving_lines(normalized_line))
                if MARKDOWN_RAW_HTML_BLOCK_CLOSE.search(html_content):
                    raw_html_active = False
                    raw_html_container_indent = 0
                    paragraph_active = False
                break

            content = blank_inline_code(content)
            content, html_comment_active = blank_html_comments(
                content,
                html_comment_active,
            )
            normalized_line = content + line_ending
            if not content.strip():
                output.append(normalized_line)
                paragraph_active = False
                break

            raw_html_indent = markdown_raw_html_opening(content)
            if raw_html_indent is not None:
                raw_html_active = True
                raw_html_container_indent = raw_html_indent
                raw_html_quote_depth = quote_depth
                output.append(blank_preserving_lines(normalized_line))
                if MARKDOWN_RAW_HTML_BLOCK_CLOSE.search(content):
                    raw_html_active = False
                    raw_html_container_indent = 0
                paragraph_active = False
                break

            opening = markdown_fence_opening(content)
            if opening is not None:
                fence_character, fence_width, fence_container_indent = opening
                fence_quote_depth = quote_depth
                output.append(blank_preserving_lines(normalized_line))
                paragraph_active = False
                break
            if markdown_line_is_indented_code(content):
                if paragraph_active:
                    output.append(normalized_line)
                else:
                    output.append(blank_preserving_lines(normalized_line))
                break

            output.append(normalized_line)
            if MARKDOWN_HEADING.match(content) or MARKDOWN_SETEXT_UNDERLINE.match(content):
                paragraph_active = False
            else:
                paragraph_active = True
            break
    return blank_inline_code("".join(output))


def markdown_link_label_end(text: str, start: int) -> int | None:
    """Return the closing bracket for a possibly nested Markdown link label."""
    depth = 0
    cursor = start
    while cursor < len(text):
        character = text[cursor]
        if character == "\\" and cursor + 1 < len(text):
            cursor += 2
            continue
        if character == "[":
            depth += 1
        elif character == "]":
            depth -= 1
            if depth == 0:
                return cursor
        cursor += 1
    return None


def markdown_link_title_end(text: str, start: int) -> int | None:
    """Return the offset after a quoted or parenthesized inline-link title."""
    opener = text[start]
    closer = {"\"": "\"", "'": "'", "(": ")"}.get(opener)
    if closer is None:
        return None
    cursor = start + 1
    while cursor < len(text):
        if text[cursor] == "\\" and cursor + 1 < len(text):
            cursor += 2
            continue
        if text[cursor] == closer:
            return cursor + 1
        cursor += 1
    return None


def markdown_inline_destination(text: str, opener: int) -> tuple[int, str] | None:
    """Parse one inline-link destination and return (link end, destination)."""
    cursor = opener + 1
    while cursor < len(text) and text[cursor] in " \t\r\n":
        cursor += 1
    if cursor >= len(text):
        return None

    if text[cursor] == ")":
        return cursor + 1, ""

    if text[cursor] == "<":
        destination_start = cursor
        cursor += 1
        while cursor < len(text):
            if text[cursor] == "\\" and cursor + 1 < len(text):
                cursor += 2
                continue
            if text[cursor] in "\r\n":
                return None
            if text[cursor] == "<":
                return None
            if text[cursor] == ">":
                cursor += 1
                destination = text[destination_start:cursor]
                break
            cursor += 1
        else:
            return None
    else:
        destination_start = cursor
        depth = 0
        while cursor < len(text):
            character = text[cursor]
            if character == "\\" and cursor + 1 < len(text):
                cursor += 2
                continue
            if character == "(":
                depth += 1
                cursor += 1
                continue
            if character == ")":
                if depth == 0:
                    return cursor + 1, text[destination_start:cursor]
                depth -= 1
                cursor += 1
                continue
            if character in " \t\r\n" and depth == 0:
                break
            cursor += 1
        if cursor >= len(text) or depth != 0:
            return None
        destination = text[destination_start:cursor]

    whitespace_start = cursor
    while cursor < len(text) and text[cursor] in " \t\r\n":
        cursor += 1
    if cursor < len(text) and text[cursor] == ")":
        return cursor + 1, destination
    if cursor == whitespace_start or cursor >= len(text):
        return None
    title_end = markdown_link_title_end(text, cursor)
    if title_end is None:
        return None
    cursor = title_end
    while cursor < len(text) and text[cursor] in " \t\r\n":
        cursor += 1
    if cursor >= len(text) or text[cursor] != ")":
        return None
    return cursor + 1, destination


def markdown_inline_links(text: str) -> list[tuple[int, int, str]]:
    """Return (start, end, destination) for syntactically complete inline links."""
    links: list[tuple[int, int, str]] = []
    cursor = 0
    while cursor < len(text):
        start = text.find("[", cursor)
        if start < 0:
            break
        label_end = markdown_link_label_end(text, start)
        if label_end is None or label_end + 1 >= len(text) or text[label_end + 1] != "(":
            cursor = start + 1
            continue
        parsed = markdown_inline_destination(text, label_end + 1)
        if parsed is None:
            cursor = start + 1
            continue
        end, destination = parsed
        links.append((start, end, destination))
        cursor = end
    return links


def blank_markdown_spans(text: str, spans: list[tuple[int, int]]) -> str:
    """Blank the supplied half-open spans without changing source line offsets."""
    output = list(text)
    for start, end in spans:
        output[start:end] = blank_preserving_lines(text[start:end])
    return "".join(output)


def markdown_reference_destination(text: str, start: int) -> tuple[int, str] | None:
    """Parse a reference-definition destination at start."""
    if start >= len(text):
        return None
    cursor = start
    if text[cursor] == "<":
        destination_start = cursor
        cursor += 1
        while cursor < len(text):
            if text[cursor] == "\\" and cursor + 1 < len(text):
                cursor += 2
                continue
            if text[cursor] in "<\r\n":
                return None
            if text[cursor] == ">":
                return cursor + 1, text[destination_start:cursor + 1]
            cursor += 1
        return None

    destination_start = cursor
    depth = 0
    while cursor < len(text):
        character = text[cursor]
        if character == "\\" and cursor + 1 < len(text):
            cursor += 2
            continue
        if character == "(":
            depth += 1
        elif character == ")":
            if depth == 0:
                return None
            depth -= 1
        elif character in " \t\r\n":
            break
        cursor += 1
    if cursor == destination_start or depth != 0:
        return None
    return cursor, text[destination_start:cursor]


def markdown_reference_definitions(
    text: str,
) -> list[tuple[int, int, str, str]]:
    """Return (start, end, label, destination) for reference definitions."""
    definitions: list[tuple[int, int, str, str]] = []
    line_start = 0
    while line_start < len(text):
        cursor = line_start
        spaces = 0
        while cursor < len(text) and text[cursor] == " " and spaces < 3:
            cursor += 1
            spaces += 1
        if cursor < len(text) and text[cursor] == "[":
            label_end = markdown_link_label_end(text, cursor)
            if (
                label_end is not None
                and label_end + 1 < len(text)
                and text[label_end + 1] == ":"
            ):
                label = text[cursor + 1:label_end]
                if label.strip() and "\n\n" not in label.replace("\r\n", "\n"):
                    destination_start = label_end + 2
                    while (
                        destination_start < len(text)
                        and text[destination_start] in " \t"
                    ):
                        destination_start += 1
                    if destination_start < len(text) and text[destination_start] in "\r\n":
                        if text.startswith("\r\n", destination_start):
                            destination_start += 2
                        else:
                            destination_start += 1
                        indentation_start = destination_start
                        while (
                            destination_start < len(text)
                            and text[destination_start] in " \t"
                        ):
                            destination_start += 1
                        if destination_start == indentation_start:
                            destination_start = len(text)
                    parsed = markdown_reference_destination(text, destination_start)
                    if parsed is not None:
                        destination_end, destination = parsed
                        definition_end = text.find("\n", destination_end)
                        if definition_end < 0:
                            definition_end = len(text)
                        else:
                            definition_end += 1
                        definitions.append(
                            (line_start, definition_end, label, destination),
                        )
        newline = text.find("\n", line_start)
        if newline < 0:
            break
        line_start = newline + 1
    return definitions


def markdown_reference_links(
    text: str,
) -> list[tuple[int, int, str, str]]:
    """Return (start, end, lookup label, rendered label) reference links."""
    links: list[tuple[int, int, str, str]] = []
    cursor = 0
    while cursor < len(text):
        start = text.find("[", cursor)
        if start < 0:
            break
        label_end = markdown_link_label_end(text, start)
        if label_end is None:
            cursor = start + 1
            continue
        rendered_label = text[start + 1:label_end]
        end = label_end + 1
        lookup_label = rendered_label
        if end < len(text) and text[end] == "(":
            cursor = end + 1
            continue
        if end < len(text) and text[end] == "[":
            reference_end = markdown_link_label_end(text, end)
            if reference_end is None:
                cursor = end + 1
                continue
            explicit_label = text[end + 1:reference_end]
            lookup_label = rendered_label if not explicit_label else explicit_label
            end = reference_end + 1
        links.append((start, end, lookup_label, rendered_label))
        cursor = end
    return links


def markdown_rendered_prose(value: str) -> str:
    """Normalize inactive-free Markdown prose toward its rendered text."""
    replacements: list[tuple[int, int, str]] = []
    for start, end, _ in markdown_inline_links(value):
        label_end = markdown_link_label_end(value, start)
        assert label_end is not None
        replacements.append((start, end, value[start + 1:label_end]))
    inline_spans = [(start, end) for start, end, _ in replacements]
    without_inline = blank_markdown_spans(value, inline_spans)
    for start, end, _, rendered_label in markdown_reference_links(without_inline):
        replacements.append((start, end, rendered_label))
    for start, end, rendered_label in sorted(replacements, reverse=True):
        value = value[:start] + rendered_label + value[end:]

    value = MARKDOWN_INLINE_HTML_TAG.sub("", value)
    protected: dict[str, str] = {}
    unescaped: list[str] = []
    cursor = 0
    while cursor < len(value):
        if (
            value[cursor] == "\\"
            and cursor + 1 < len(value)
            and value[cursor + 1] in MARKDOWN_ESCAPABLE
        ):
            escaped = value[cursor + 1]
            if escaped in "*_":
                placeholder = f"\ue000{len(protected)}\ue001"
                protected[placeholder] = escaped
                unescaped.append(placeholder)
            else:
                unescaped.append(escaped)
            cursor += 2
            continue
        unescaped.append(value[cursor])
        cursor += 1
    value = "".join(unescaped)
    while True:
        normalized = MARKDOWN_EMPHASIS.sub(r"\2", value)
        if normalized == value:
            break
        value = normalized
    for placeholder, escaped in protected.items():
        value = value.replace(placeholder, escaped)
    return " ".join(html.unescape(value).split())


def markdown_unescape_destination(value: str) -> str:
    """Decode CommonMark backslash escapes used in a link destination."""
    output: list[str] = []
    cursor = 0
    while cursor < len(value):
        if (
            value[cursor] == "\\"
            and cursor + 1 < len(value)
            and value[cursor + 1] in MARKDOWN_ESCAPABLE
        ):
            output.append(value[cursor + 1])
            cursor += 2
            continue
        output.append(value[cursor])
        cursor += 1
    return "".join(output)


def safe_percent_decode_local_destination(value: str) -> str | None:
    """Decode one URL-encoding layer without allowing path-shape changes."""
    if ENCODED_PATH_SEPARATOR.search(value):
        return None
    try:
        decoded = unquote_to_bytes(value).decode("utf-8")
    except UnicodeDecodeError:
        return None
    if "\x00" in decoded:
        return None
    if any(component == ".." for component in re.split(r"[/\\]", decoded)):
        return None
    return decoded


def markdown_link_targets(text: str) -> list[str]:
    """Return inline and actually-used reference-style Markdown destinations."""
    prose = markdown_without_code(text)
    inline_links = markdown_inline_links(prose)
    targets = [
        destination
        for start, _, destination in inline_links
        if markdown_link_is_active(prose, start)
    ]
    without_inline_links = blank_markdown_spans(
        prose,
        [(start, end) for start, end, _ in inline_links],
    )
    definition_rows = markdown_reference_definitions(without_inline_links)
    definitions: dict[str, str] = {}
    for _, _, label, destination in definition_rows:
        definitions.setdefault(normalize_reference_label(label), destination)
    without_definitions = blank_markdown_spans(
        without_inline_links,
        [(start, end) for start, end, _, _ in definition_rows],
    )
    for start, _, label, _ in markdown_reference_links(without_definitions):
        if not markdown_link_is_active(without_definitions, start):
            continue
        target = definitions.get(normalize_reference_label(label))
        if target is not None:
            targets.append(target)
    return targets


def markdown_prose_blocks(text: str) -> list[tuple[int, str]]:
    """Return normalized non-code prose blocks with their starting line."""
    prose = markdown_without_code(text)
    prose = blank_markdown_spans(
        prose,
        [
            (start, end)
            for start, end, _, _ in markdown_reference_definitions(prose)
        ],
    )
    blocks: list[tuple[int, str]] = []
    parts: list[str] = []
    start_line = 0

    def flush() -> None:
        nonlocal parts, start_line
        if parts:
            blocks.append((start_line, markdown_rendered_prose(" ".join(parts))))
        parts = []
        start_line = 0

    lines = prose.splitlines()
    index = 0
    while index < len(lines):
        line = lines[index]
        line_number = index + 1
        stripped = line.strip()
        if not stripped:
            flush()
            index += 1
            continue
        if index + 1 < len(lines) and MARKDOWN_SETEXT_UNDERLINE.match(lines[index + 1]):
            flush()
            blocks.append((line_number, "# " + markdown_rendered_prose(stripped)))
            index += 2
            continue
        if MARKDOWN_HEADING.match(line):
            flush()
            blocks.append((line_number, markdown_rendered_prose(stripped)))
            index += 1
            continue
        list_item = MARKDOWN_LIST_ITEM.match(line)
        if list_item is not None:
            flush()
            start_line = line_number
            parts.append(line[list_item.end():].strip())
            index += 1
            continue
        if stripped.startswith("|") and stripped.endswith("|"):
            flush()
            blocks.append((line_number, markdown_rendered_prose(stripped)))
            index += 1
            continue
        quote = re.match(r"^[ \t]{0,3}(?:>[ \t]?)+", line)
        content = line[quote.end():].strip() if quote is not None else stripped
        trailing_backslashes = len(content) - len(content.rstrip("\\"))
        if trailing_backslashes % 2 == 1:
            content = content[:-1]
        if not parts:
            start_line = line_number
        parts.append(content)
        index += 1
    flush()
    return blocks


def linked_skill_references(root: Path = ROOT) -> list[Path]:
    """Resolve direct, local references links from canonical skill kernels."""
    root = root.resolve()
    linked: set[Path] = set()
    for skill_md in sorted((root / "skills").glob("*/SKILL.md")):
        skill_dir = skill_md.parent.resolve()
        references_dir = (skill_dir / "references").resolve()
        try:
            text = skill_md.read_text(encoding="utf-8")
        except OSError as exc:
            raise AssertionError(f"cannot read skill kernel {skill_md}: {exc}") from exc
        for target_text in markdown_link_targets(text):
            raw_target = target_text.strip()
            if raw_target.startswith("<") and ">" in raw_target:
                raw_target = raw_target[1:raw_target.index(">")]
            else:
                raw_target = raw_target.split(maxsplit=1)[0]
            raw_target = markdown_unescape_destination(raw_target)
            target = raw_target.split("#", 1)[0].split("?", 1)[0]
            target = safe_percent_decode_local_destination(target)
            if (
                not target
                or "://" in target
                or target.startswith(("/", "\\"))
            ):
                continue
            candidate = (skill_dir / target).resolve()
            try:
                candidate.relative_to(references_dir)
            except ValueError:
                continue
            if candidate.is_file():
                linked.add(candidate)
    return sorted(linked)


def check_linked_skill_reference_identity(root: Path = ROOT) -> None:
    """Linked current guidance must not reintroduce retired product terms."""
    root = root.resolve()
    findings: list[str] = []
    for reference in linked_skill_references(root):
        try:
            text = reference.read_text(encoding="utf-8")
        except (OSError, UnicodeError) as exc:
            raise AssertionError(f"cannot read linked skill reference {reference}: {exc}") from exc
        for line_number, block in markdown_prose_blocks(text):
            for label, pattern in LINKED_SKILL_REFERENCE_PATTERNS:
                if pattern.search(block):
                    path = reference.relative_to(root)
                    findings.append(f"{path}:{line_number}: {label}: {block}")
    assert not findings, (
        "linked skill references contain obsolete operations-layer terminology:\n"
        + "\n".join(findings)
    )


def property_names(value: object) -> set[str]:
    names: set[str] = set()
    if isinstance(value, dict):
        props = value.get("properties")
        if isinstance(props, dict):
            names.update(str(key) for key in props)
        for child in value.values():
            names.update(property_names(child))
    elif isinstance(value, list):
        for child in value:
            names.update(property_names(child))
    return names


def check_skill_graph() -> None:
    entries = {path.parent.name: frontmatter(path.parent.name) for path in (ROOT / "skills").glob("*/SKILL.md")}
    expected = {"rpi": {"anti-ceremony", "plan", "implement", "validate"}, "plan": set(), "implement": set(), "validate": set()}
    actual = {
        name: set((entries[name].get("metadata") or {}).get("dependencies") or [])
        for name in CORE
    }
    assert actual == expected, f"core dependency graph mismatch: {actual}"
    # ADR-0017: crank is the one non-core skill with a hard dependency, on rpi
    # alone; the skill mesh generator carries the same allowance.
    allowed_extra = {"crank": {"rpi"}}
    for name, entry in entries.items():
        deps = set((entry.get("metadata") or {}).get("dependencies") or [])
        if name != "rpi":
            assert deps == allowed_extra.get(name, set()), (
                f"{name}: only rpi (and crank on rpi, ADR-0017) may declare hard dependencies: {sorted(deps)}"
            )
    for name in REMOVED_SKILLS:
        assert not (ROOT / "skills" / name / "SKILL.md").exists(), f"removed skill is live: {name}"
        assert not (ROOT / "skills-codex" / name / "SKILL.md").exists(), f"removed Codex skill is live: {name}"
    for name in REMOVED_MORTEM_ALIASES:
        assert not (ROOT / "skills" / name).exists(), f"removed skill alias is live: {name}"
        assert not (ROOT / "skills-codex" / name).exists(), f"removed Codex alias is live: {name}"
    assert (ROOT / "skills" / "premortem" / "SKILL.md").is_file()
    assert (ROOT / "skills" / "postmortem" / "SKILL.md").is_file()
    swarm = entries["swarm"]
    assert not ((swarm.get("metadata") or {}).get("dependencies") or []), "swarm must remain optional"
    assert "dispatch_once" in (ROOT / "skills" / "swarm" / "SKILL.md").read_text(encoding="utf-8")


def check_generated_skill_inventory() -> None:
    source_names = {path.parent.name for path in (ROOT / "skills").glob("*/SKILL.md")}
    codex_names = {path.parent.name for path in (ROOT / "skills-codex").glob("*/SKILL.md")}
    catalog = json.loads((ROOT / "skills" / "catalog.json").read_text(encoding="utf-8"))
    rows = catalog.get("skills") or []
    catalog_names = [row.get("name") for row in rows]
    assert len(catalog_names) == len(set(catalog_names)), "catalog contains duplicate dispositions"
    assert set(catalog_names) == source_names, "catalog does not cover every current skill exactly once"
    assert source_names == codex_names, "source and Codex skill sets differ"
    for row in rows:
        assert isinstance(row.get("disposition"), str) and row["disposition"], (
            f"{row.get('name')}: missing generated disposition"
        )


def check_core_schemas() -> None:
    for path in sorted((ROOT / "schemas").glob("*.json")):
        schema = json.loads(path.read_text(encoding="utf-8"))
        bad = property_names(schema).intersection(FORBIDDEN_SCHEMA_STATE)
        assert not bad, f"{path.name}: retired lifecycle state {sorted(bad)}"
    for filename in CORE_SCHEMAS:
        path = ROOT / "schemas" / filename
        assert path.is_file(), f"missing core schema: {filename}"
        schema = json.loads(path.read_text(encoding="utf-8"))
        bad = property_names(schema).intersection(FORBIDDEN_STATE)
        assert not bad, f"{filename}: lifecycle state {sorted(bad)}"
    for filename in COMPATIBILITY_SCHEMAS:
        path = ROOT / "schemas" / filename
        assert path.is_file(), f"missing compatibility schema: {filename}"
        schema = json.loads(path.read_text(encoding="utf-8"))
        assert schema.get("deprecated") is True, f"{filename}: compatibility schema is not deprecated"
    verdict = json.loads((ROOT / "schemas" / "verdict.v2.schema.json").read_text())
    assert set(verdict["properties"]["verdict"]["enum"]) == {"PASS", "FAIL", "NOT_PROVEN"}
    for forbidden in ("WARN", "confidence", "disposition", "next_action", "NOT_BUILT", "NOT_PLANNED"):
        assert forbidden not in json.dumps(verdict), f"verdict.v2 retains {forbidden}"
    for filename in RETIRED_SCHEMAS:
        assert not (ROOT / "schemas" / filename).exists(), f"retired schema is live: {filename}"


def check_packet_free_narrative() -> None:
    for relative in PACKET_FREE_NARRATIVE:
        text = (ROOT / relative).read_text(encoding="utf-8")
        advertised = [name for name in LEGACY_PACKET_TOKENS if name in text]
        assert not advertised, f"{relative}: advertises legacy packets {advertised}"

    schemas_doc = (ROOT / "docs" / "SCHEMAS.md").read_text(encoding="utf-8")
    assert "## Legacy compatibility" in schemas_doc, "docs/SCHEMAS.md: no legacy compatibility section"
    current_schema_section = schemas_doc.split("## Legacy compatibility", 1)[0]
    assert not any(name in current_schema_section for name in LEGACY_PACKET_TOKENS), (
        "docs/SCHEMAS.md: legacy packet listed as a current core schema"
    )

    contracts_doc = (ROOT / "docs" / "contracts" / "index.md").read_text(encoding="utf-8")
    assert "Deprecated compatibility contracts" in contracts_doc, (
        "docs/contracts/index.md: compatibility contracts are not labeled deprecated"
    )
    current_contract_section = contracts_doc.split("Deprecated compatibility contracts", 1)[0]
    assert not any(name in current_contract_section for name in LEGACY_PACKET_TOKENS), (
        "docs/contracts/index.md: legacy packet listed as a current public contract"
    )


def check_bounded_repair_contract() -> None:
    """RPI stops under the convergence law, not after exactly one validation.

    ADR-0017 narrows the 2026-07-14 cut: it removed the iterate loop along with
    the unproven compounding claim, although ADR-0011 demoted only the latter.
    The blanket "Stop regardless" assertion and the ban on any loop in
    `run_once.py` are replaced here by positive canaries — the law's four
    conditions must be present BY NAME, so a bounded repair phase is allowed
    while an unbounded grind still fails this gate.
    """
    raw = (ROOT / "skills" / "rpi" / "SKILL.md").read_text(encoding="utf-8")
    # Whitespace-normalized so a canary phrase may wrap across source lines.
    text = " ".join(raw.split())
    assert "Stop regardless" not in text, (
        "RPI still asserts the retired single-pass stop; ADR-0017 replaced it with the law"
    )
    for phrase in (
        "stop when converged, stopped by the law, or out of `repair_rounds`",
        "## The convergence law",
        "rounds_used < repair_rounds",
        "larger than the previous round",
        "No finding id closed in an earlier round reopens.",
        "the subject-manifest digest changed",
        "repair round N: k open findings",
    ):
        assert phrase in text, f"RPI contract is missing a convergence-law canary: {phrase}"
    # Plan and Implement keep their single-dispatch lock; only Validate repeats.
    assert "dispatches Plan and Implement at most once" in text
    assert "never extends the caller's" in text, "RPI does not disclaim the caller's repair bound"

    runner = ROOT / "skills" / "rpi" / "scripts" / "run_once.py"
    assert runner.is_file(), "RPI has no executable reference behavior"
    source = runner.read_text(encoding="utf-8")
    tree = ast.parse(source, filename=str(runner))
    functions = {
        node.name for node in ast.walk(tree) if isinstance(node, ast.FunctionDef)
    }
    # The bounded loop is now allowed, and required: it must live in the repair
    # phase, be fed already-produced validate rounds, and consult the law.
    assert {"run_repair_phase", "law_violation", "normalize_round"} <= functions, (
        "RPI reference behavior has no bounded repair phase implementing the law"
    )
    repair = next(
        node
        for node in ast.walk(tree)
        if isinstance(node, ast.FunctionDef) and node.name == "run_repair_phase"
    )
    loops = [node for node in ast.walk(repair) if isinstance(node, (ast.For, ast.While))]
    assert loops, "the repair phase must actually iterate the validation rounds"
    # Bounded, not merely looping: the caller's bound must gate the loop. The
    # repair function must compare against `repair_rounds` (condition 1) and
    # every loop must be a `for` over the supplied rounds, never `while True`.
    compares_bound = any(
        isinstance(node, ast.Compare)
        and any(isinstance(n, ast.Name) and n.id == "repair_rounds" for n in ast.walk(node))
        for node in ast.walk(repair)
    )
    assert compares_bound, "the repair phase never compares against the caller's repair_rounds bound"
    assert all(isinstance(node, ast.For) for node in loops), (
        "the repair phase may only iterate the supplied rounds; an open-ended while loop is an unbounded grind"
    )
    # Finite by construction: every loop iterates the `validations` parameter
    # (directly or via enumerate), never a synthetic range or a constant.
    def iterates_validations(node: ast.For) -> bool:
        target = node.iter
        if isinstance(target, ast.Call) and target.args:
            target = target.args[0]
        return isinstance(target, ast.Name) and target.id == "validations"
    assert all(iterates_validations(node) for node in loops), (
        "every repair-phase loop must iterate the supplied validation rounds; nothing else is finite by construction"
    )
    # Execute the law's canaries against the reference behavior itself: budget,
    # growth, reopen, no-change, and the flip-to-PASS case must all STOP.
    import importlib.util
    spec = importlib.util.spec_from_file_location("rpi_run_once_canary", runner)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    dg = lambda ch: ch * 64  # noqa: E731
    def leg(status, ids, digest, evidence=(), classes=None):
        classes = classes or {}
        return {
            "status": status,
            "findings": [
                dict({"id": i, "summary": i}, **({"class": classes[i]} if i in classes else {}))
                for i in ids
            ],
            "subject_digest": digest,
            "evidence_refs": list(evidence),
            "validator_family": "fresh",
        }
    canaries = {
        "repair_budget_exhausted": ([leg("FAIL", ["a"], dg("a")), leg("FAIL", ["a"], dg("b")), leg("FAIL", ["a"], dg("c"))], 1),
        "finding_set_grew": ([leg("FAIL", ["a"], dg("a")), leg("FAIL", ["a", "b"], dg("b"))], 2),
        "reopened_finding": ([leg("FAIL", ["a", "b"], dg("a")), leg("FAIL", ["b"], dg("b")), leg("FAIL", ["a"], dg("c"))], 3),
        # A flat id count hides a repair phase that renames the same KIND of
        # defect every round; the class key is what catches it.
        "class_reopened": (
            [
                leg("FAIL", ["a"], dg("a"), classes={"a": "seal.pinning"}),
                leg("FAIL", ["b"], dg("b")),
                leg("FAIL", ["c"], dg("c"), classes={"c": "seal.pinning"}),
            ],
            3,
        ),
        "no_subject_or_evidence_change": ([leg("FAIL", ["a"], dg("a")), leg("PASS", [], dg("a"))], 2),
        "converged": ([leg("FAIL", ["a"], dg("a")), leg("PASS", [], dg("b"))], 2),
    }
    for expected, (rounds, bound) in canaries.items():
        outcome = module.run_repair_phase(rounds, repair_rounds=bound)
        assert outcome["stop_reason"] == expected, (
            f"law canary {expected}: reference behavior stopped with {outcome['stop_reason']!r}"
        )
    flip = module.run_repair_phase(canaries["no_subject_or_evidence_change"][0], repair_rounds=2)
    assert flip["report"]["status"] == "NOT_PROVEN", "a PASS over unchanged bytes after a FAIL must not certify"
    # The bound must CONTROL admission, not merely be mentioned: a poison round
    # past repair_rounds is never normalized (it would raise), and rounds_used
    # never exceeds the bound.
    poison = module.run_repair_phase(
        [leg("FAIL", ["a"], dg("a")), leg("FAIL", ["a"], dg("b")), {"status": "poison-not-a-round"}],
        repair_rounds=1,
    )
    assert poison["stop_reason"] == "repair_budget_exhausted" and poison["rounds_used"] == 1, (
        "the repair phase consumed a round past the caller's bound"
    )
    assert canaries and module.run_repair_phase([leg("FAIL", ["a"], dg("a"))], repair_rounds=0)["stop_reason"] == "repair_budget_exhausted"
    # The class law must STOP the loop, not merely be reported at the end: a
    # poison round after the class reopen is never normalized (it would raise),
    # and the churning round's own FAIL never certifies the outcome.
    class_poison = module.run_repair_phase(
        [
            leg("FAIL", ["a"], dg("a"), classes={"a": "seal.pinning"}),
            leg("FAIL", ["b"], dg("b")),
            leg("FAIL", ["c"], dg("c"), classes={"c": "seal.pinning"}),
            {"status": "poison-not-a-round"},
        ],
        repair_rounds=9,
    )
    assert class_poison["stop_reason"] == "class_reopened" and class_poison["rounds_used"] == 2, (
        "the class law did not stop the repair phase before the next round"
    )
    assert class_poison["report"]["status"] == "NOT_PROVEN", (
        "a round that renamed the same finding class must not certify its own status"
    )
    # Classless findings are outside the class law entirely; the same shape with
    # no class runs on and is never diagnosed as a class reopen.
    classless = module.run_repair_phase(
        [leg("FAIL", ["a"], dg("a")), leg("FAIL", ["b"], dg("b")), leg("FAIL", ["c"], dg("c"))],
        repair_rounds=9,
    )
    assert classless["stop_reason"] == "not_converged", (
        "findings without a class must never trigger the class law"
    )
    assert not any(
        isinstance(node, (ast.For, ast.While))
        for name in ("invoke_once",)
        for func in [n for n in ast.walk(tree) if isinstance(n, ast.FunctionDef) and n.name == name]
        for node in ast.walk(func)
    ), "the one bounded experiment must not loop; only the repair phase may"
    for reason in (
        "repair_budget_exhausted",
        "reopened_finding",
        "class_reopened",
        "finding_set_grew",
        "no_subject_or_evidence_change",
        "diversity_unsatisfied",
        "converged",
    ):
        assert f'"{reason}"' in source, f"repair phase never reports the stop reason {reason}"

    report = json.loads((ROOT / "schemas" / "rpi-report.v1.schema.json").read_text())
    assert "next_action" not in property_names(report)
    # The law rides in the report's `checked` lines; it must not smuggle a
    # tenth key into rpi-report.v1.
    assert len(property_names(report)) == 9, "rpi-report.v1 is no longer the nine-key shape"


def check_validate_helper() -> None:
    path = ROOT / "skills" / "validate" / "scripts" / "validate.py"
    tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
    forbidden_imports = {"subprocess", "socket", "urllib", "http", "requests", "git", "dulwich"}
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                assert alias.name.split(".")[0] not in forbidden_imports, f"validate helper imports {alias.name}"
        elif isinstance(node, ast.ImportFrom):
            assert (node.module or "").split(".")[0] not in forbidden_imports, f"validate helper imports {node.module}"
        elif isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute):
            assert node.func.attr not in {"system", "popen", "spawn", "execv", "execve"}, f"validate helper launches {node.func.attr}"
    spec = importlib.util.spec_from_file_location("cathedral_validate_contract", path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    with tempfile.TemporaryDirectory() as raw:
        try:
            module.store_verdict({"verdict": "FAIL"}, Path(raw))
        except module.ContractError:
            pass
        else:
            raise AssertionError("Validate persisted an incomplete verdict.v2 draft")
        assert not list(Path(raw).iterdir()), "Validate wrote an invalid verdict artifact"
    with tempfile.TemporaryDirectory() as raw:
        subject = Path(raw)
        (subject / "value").write_text("same", encoding="utf-8")
        first = module.build_manifest(subject, ["."], [], git_metadata={"commit": "one"})
        second = module.build_manifest(subject, ["."], [], git_metadata={"commit": "two"})
        assert first["canonical_manifest_digest"] == second["canonical_manifest_digest"], (
            "optional Git metadata changes subject content identity"
        )


def check_tombstones() -> None:
    source = (ROOT / "cli" / "cmd" / "ao" / "removed_command_hint.go").read_text(encoding="utf-8")
    for name in REMOVED_COMMANDS:
        assert f'"{name}"' in source, f"missing tombstone: {name}"
    assert "cli/internal/" not in source and "internal/commands/" not in source
    assert "exec.Command" not in source and "os/exec" not in source
    removed_sources = {
        "pawl.go", "plan_pawl.go", "land.go", "done_composition.go", "close_module.go",
        "governor.go", "yield.go", "claim_module.go", "state.go", "worktree.go",
        "validate.go", "converge.go", "reconcile.go", "membrane.go",
    }
    for filename in removed_sources:
        assert not (ROOT / "cli" / "cmd" / "ao" / filename).exists(), f"old command implementation is live: {filename}"
    for filename in (
        "closeout.go",
        "inmemory_closeout.go",
        "convergence_check.go",
        "inmemory_convergence_check.go",
    ):
        assert not (ROOT / "cli" / "internal" / "ports" / filename).exists(), (
            f"lifecycle authority port remains live: {filename}"
        )


def check_dispatch_once() -> None:
    path = ROOT / "skills" / "swarm" / "scripts" / "dispatch_once.py"
    spec = importlib.util.spec_from_file_location("cathedral_dispatch_once", path)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    packets = [
        {"packet_id": "one", "write_scope": {"include": ["a"]}},
        {"packet_id": "two", "write_scope": {"include": ["b"]}},
    ]
    calls: list[str] = []

    def executor(packet: dict) -> str:
        calls.append(packet["packet_id"])
        if packet["packet_id"] == "two":
            raise RuntimeError("observed error")
        return "candidate"

    results = module.dispatch_once(packets, executor)
    assert calls == ["one", "two"], f"dispatch count/order mismatch: {calls}"
    assert results[0]["result"] == "candidate"
    assert results[1]["error"]["message"] == "observed error"
    try:
        module.dispatch_once(
            [
                {"packet_id": "wide", "write_scope": {"include": ["src/**"]}},
                {"packet_id": "nested", "write_scope": {"include": ["src/lib/**"]}},
            ],
            executor,
        )
    except ValueError:
        pass
    else:
        raise AssertionError("dispatch_once accepted overlapping glob scopes")


def probe_no_substrate_calls() -> None:
    helper = ROOT / "skills" / "validate" / "scripts" / "validate.py"
    rpi_runner = ROOT / "skills" / "rpi" / "scripts" / "run_once.py"
    with tempfile.TemporaryDirectory() as raw:
        temp = Path(raw)
        subject = temp / "subject"
        subject.mkdir()
        (subject / "value.txt").write_text("candidate\n", encoding="utf-8")
        fake_bin = temp / "bin"
        fake_bin.mkdir()
        called = temp / "called"
        for name in ("git", "ao", "br", "bd", "push", "release"):
            executable = fake_bin / name
            executable.write_text(f"#!/bin/sh\necho {name} >> '{called}'\nexit 97\n", encoding="utf-8")
            executable.chmod(0o755)
        env = dict(os.environ)
        env["PATH"] = str(fake_bin) + os.pathsep + env.get("PATH", "")
        result = subprocess.run(
            [sys.executable, str(helper), "manifest", "--root", str(subject), "--include", "."],
            cwd=temp, env=env, text=True, capture_output=True, check=False,
        )
        assert result.returncode == 0, result.stderr
        payload = json.loads(result.stdout)
        assert payload["schema_version"] == "subject-manifest.v1"
        spec = importlib.util.spec_from_file_location("cathedral_validate", helper)
        assert spec and spec.loader
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        rpi_spec = importlib.util.spec_from_file_location("cathedral_rpi", rpi_runner)
        assert rpi_spec and rpi_spec.loader
        rpi = importlib.util.module_from_spec(rpi_spec)
        rpi_spec.loader.exec_module(rpi)
        # The intent SOURCE is bytes; the acceptance identity is sha256 of those
        # bytes; the resolved mapping carries that identity as a declared fact.
        # Deriving the bytes from the mapping that already contains the digest
        # would be circular, and folding the two together is what let the
        # RPI/Validate digest disagreement hide: this probe used to set
        # `intent_bytes = canonical_bytes(resolved_intent)`, which is precisely
        # the one input where a canonical-JSON digest of the mapping and a byte
        # digest of the source coincide. Keeping them separate means the probe
        # exercises the identity rather than a coincidence.
        intent_source = {
            "intent_ref": "conversation:cathedral-probe",
            "acceptance": ["value.txt contains candidate"],
            "write_scope": {"include": ["value.txt"], "exclude": []},
        }
        intent_bytes = module.canonical_bytes(intent_source)
        resolved_intent = {
            **intent_source,
            "acceptance_digest": hashlib.sha256(intent_bytes).hexdigest(),
        }
        subject_facts = {
            "subject_manifest_digest": payload["canonical_manifest_digest"],
            "subject_manifest": payload,
            "checks": ["manifest"],
        }
        draft = {
            "acceptance_digest": "a" * 64,
            "subject_manifest_digest": payload["canonical_manifest_digest"],
            "author_context_id": "non-git-author",
            "validator_context_id": "non-git-validator",
            "freshness_attestation": {"source": "runtime", "attester_identity": "probe"},
            "verdict": "PASS",
            "criteria": [{"id": "acceptance", "result": "PASS", "evidence_refs": ["probe"]}],
            "findings": [],
            "evidence_refs": ["probe"],
            "checked": ["acceptance"],
            "not_checked": [],
            "validated_at": "2026-07-14T00:00:00Z",
        }
        verdict_dir = temp / ".agents" / "ao" / "verdicts" / "sha256"
        calls: list[str] = []

        def anti_ceremony_guard(_intent: object) -> dict:
            calls.append("anti-ceremony")
            return {
                "decision": "CONTINUE",
                "reason": "The frozen outcome still requires implementation proof.",
                "frozen_outcome": "Write and prove the non-Git candidate",
                "parked_process_work": [],
                "remaining_proof": ["manifest", "fresh validation"],
                "stop_condition": "Stop after one fresh validation result.",
            }

        def plan_phase(_intent: object) -> dict:
            calls.append("plan")
            return resolved_intent

        def implement_phase(received_intent: dict) -> dict:
            calls.append("implement")
            assert received_intent == resolved_intent
            return subject_facts

        def validate_phase(received_intent: dict, received_subject: dict) -> dict:
            calls.append("validate")
            assert received_intent == resolved_intent
            assert received_subject == subject_facts
            artifact, verdict_path, existed = module.store_verdict(
                draft,
                verdict_dir,
                intent_bytes,
                payload,
                "non-git-author",
                "PASS",
                "non-git-validator",
                "runtime",
                "non-git-validator",
            )
            assert not existed
            return {
                "verdict": artifact["verdict"],
                "acceptance_digest": artifact["acceptance_digest"],
                "subject_manifest_digest": artifact["subject_manifest_digest"],
                "verdict_digest": artifact["artifact_digest"],
                "verdict_ref": str(verdict_path),
                "author_context_id": artifact["author_context_id"],
                "validator_context_id": artifact["validator_context_id"],
                "freshness_attestation": artifact["freshness_attestation"],
                "checked": artifact["checked"],
                "not_checked": artifact["not_checked"],
            }

        rpi_report = rpi.invoke_once(
            "temporary non-Git experiment",
            anti_ceremony_guard,
            plan_phase,
            implement_phase,
            validate_phase,
        )
        verdict_path = Path(rpi_report["verdict_ref"])
        assert calls == ["anti-ceremony", "plan", "implement", "validate"], f"RPI dispatch trace is {calls}"
        assert rpi_report["status"] == "PASS" and verdict_path.is_file()
        assert verdict_path.parent == verdict_dir
        assert not called.exists(), "Validate helper invoked a Git, tracker, push, or delivery executable"



def check_operations_layer_identity() -> None:
    """The product category, its owners, and the retired surfaces stay aligned."""
    language = (ROOT / "docs" / "contracts" / "ubiquitous-language.md").read_text(encoding="utf-8")
    for term in (
        "Operations layer",
        "Federated integration graph",
        "Semantic work-and-proof protocol",
        "RPI traversal",
        "Forbidden conflations",
    ):
        assert term in language, f"ubiquitous language missing term: {term}"

    traversal = ROOT / "docs" / "architecture" / "rpi-traversal.md"
    assert traversal.is_file(), "rpi-traversal.md owner page missing"
    compat = (ROOT / "docs" / "architecture" / "operating-loop.md").read_text(encoding="utf-8")
    assert "rpi-traversal.md" in compat, "compatibility page must link the owner"
    assert len(compat.splitlines()) < 25, "compatibility page must stay a short redirect"
    assert "subject-manifest.v1" not in compat, "compatibility page must not duplicate the contract"

    for retired in (
        ROOT / "cli" / "internal" / "commands" / "flywheel",
        ROOT / "cli" / "internal" / "flywheelapp",
    ):
        assert not retired.exists(), f"retired knowledge-flywheel surface is live: {retired}"

    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    assert "operations layer for agentic engineering" in readme
    product = (ROOT / "PRODUCT.md").read_text(encoding="utf-8")
    assert "operations layer for agentic engineering" in product


def main() -> int:
    checks = (
        check_skill_graph,
        check_generated_skill_inventory,
        check_core_schemas,
        check_packet_free_narrative,
        check_bounded_repair_contract,
        check_validate_helper,
        check_tombstones,
        check_operations_layer_identity,
        check_linked_skill_reference_identity,
        check_dispatch_once,
        probe_no_substrate_calls,
    )
    failures: list[str] = []
    for check in checks:
        try:
            check()
        except (AssertionError, OSError, ValueError, json.JSONDecodeError, yaml.YAMLError) as exc:
            failures.append(f"{check.__name__}: {exc}")
    if failures:
        print("Cathedral Cut conformance failed:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        return 1
    print("Cathedral Cut conformance: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
