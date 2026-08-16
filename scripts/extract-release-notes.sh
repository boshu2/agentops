#!/usr/bin/env bash
# Assemble the GitHub Release body for a given version.
# Body = curated highlights + the version's CHANGELOG section embedded in <details>.
#
# Usage: scripts/extract-release-notes.sh v2.9.2 [v2.9.0]
#   $1 = current tag (required)
#   $2 = previous tag (optional, for footer link)
#
# Requires: docs/releases/YYYY-MM-DD-v<version>-notes.md curated highlights AND a
# matching CHANGELOG.md section. There is NO fallback to a raw CHANGELOG dump — a
# missing curated file or CHANGELOG entry is a hard error (3.0.0/3.0.1 shipped raw
# dumps precisely because this used to fall back). Structure is enforced by
# scripts/validate-release-notes.sh.
#
# Output: writes release-notes.md to repo root

set -euo pipefail

TAG="${1:?Usage: extract-release-notes.sh TAG [PREV_TAG]}"
PREV_TAG="${2:-}"
VERSION="${TAG#v}"
REPO="boshu2/agentops"

# GitHub's release renderer preserves soft line breaks in prose. Curated notes
# are hard-wrapped for readable diffs, so copying them byte-for-byte produces a
# narrow ragged column in the published release. Reflow prose and list
# continuations into logical Markdown lines while leaving structural Markdown
# (headings, tables, code, HTML, block quotes, and explicit hard breaks) intact.
normalize_markdown() {
  awk '
    function leading_spaces(value, count) {
      count = 0
      while (substr(value, count + 1, 1) == " ") {
        count++
      }
      return count
    }
    function trailing_spaces(value, count, pos) {
      count = 0
      pos = length(value)
      while (pos > 0 && substr(value, pos, 1) == " ") {
        count++
        pos--
      }
      return count
    }
    function strip_markdown_indent(value, count) {
      count = leading_spaces(value)
      if (count > 3) {
        count = 3
      }
      return substr(value, count + 1)
    }
    function flush_flow() {
      if (flow != "") {
        print flow
        flow = ""
        flow_kind = ""
      }
    }
    function append_flow(value, kind, hard_break, indent, piece, spaces) {
      spaces = trailing_spaces(value)
      hard_break = (spaces >= 2 || value ~ /\\$/)
      piece = value
      sub(/^[ \t]+/, "", piece)
      sub(/[ \t]+$/, "", piece)
      if (flow == "") {
        indent = leading_spaces(value)
        flow = substr(value, 1, indent) piece
        flow_kind = kind
      } else {
        flow = flow " " piece
      }
      if (hard_break) {
        while (spaces > 0) {
          flow = flow " "
          spaces--
        }
        flush_flow()
      }
    }
    function is_list_marker(value, indent, text, pos, char, spaces) {
      indent = leading_spaces(value)
      text = substr(value, indent + 1)
      char = substr(text, 1, 1)

      if ((char == "-" || char == "+" || char == "*") &&
          substr(text, 2, 1) ~ /[ \t]/) {
        pos = 2
      } else {
        pos = 1
        while (substr(text, pos, 1) ~ /[0-9]/ && pos <= 9) {
          pos++
        }
        if (pos == 1 ||
            (substr(text, pos, 1) != "." && substr(text, pos, 1) != ")") ||
            substr(text, pos + 1, 1) !~ /[ \t]/) {
          return 0
        }
        pos++
      }

      spaces = 0
      while (substr(text, pos, 1) ~ /[ \t]/) {
        spaces++
        pos++
      }
      marker_indent = indent
      marker_content_indent = indent + pos - 1
      return 1
    }
    function is_thematic_or_setext(value, compact) {
      compact = value
      gsub(/[ \t]/, "", compact)
      return ((length(compact) >= 3 && compact ~ /^-+$/) ||
              (length(compact) >= 3 && compact ~ /^\*+$/) ||
              (length(compact) >= 3 && compact ~ /^_+$/) ||
              compact ~ /^=+$/)
    }
    function is_table_delimiter(value, compact) {
      compact = value
      gsub(/[ \t]/, "", compact)
      return compact ~ /^\|?:?-+:?(\|:?-+:?)+\|?$/
    }
    function fence_open(value, text, char, count, rest) {
      text = strip_markdown_indent(value)
      char = substr(text, 1, 1)
      if (char != "`" && char != "~") {
        return 0
      }
      count = 0
      while (substr(text, count + 1, 1) == char) {
        count++
      }
      if (count < 3) {
        return 0
      }
      rest = substr(text, count + 1)
      if (char == "`" && index(rest, "`") != 0) {
        return 0
      }
      next_fence_char = char
      next_fence_length = count
      return 1
    }
    function fence_close(value, text, count, rest) {
      text = strip_markdown_indent(value)
      if (substr(text, 1, 1) != fence_char) {
        return 0
      }
      count = 0
      while (substr(text, count + 1, 1) == fence_char) {
        count++
      }
      rest = substr(text, count + 1)
      return (count >= fence_length && rest ~ /^[ \t]*$/)
    }
    function html_mode_for(value, lower) {
      lower = tolower(value)
      if (index(value, "<!--") == 1) return "comment"
      if (index(value, "<![CDATA[") == 1) return "cdata"
      if (index(value, "<?") == 1) return "processing"
      if (lower ~ /^<(script|pre|style|textarea)([ \t>]|$)/) return "raw"
      return "blank"
    }
    function html_closed(value, mode, lower) {
      lower = tolower(value)
      if (mode == "comment") return index(value, "-->") != 0
      if (mode == "cdata") return index(value, "]]>") != 0
      if (mode == "processing") return index(value, "?>") != 0
      if (mode == "raw") return lower ~ /<\/(script|pre|style|textarea)[ \t>]/
      return index(lower, "</") != 0
    }

    in_fence {
      print
      if (fence_close($0)) {
        in_fence = 0
        fence_char = ""
        fence_length = 0
      }
      next
    }
    html_mode != "" {
      print
      if (html_closed($0, html_mode) ||
          (html_mode == "blank" && $0 ~ /^[[:space:]]*$/)) {
        html_mode = ""
      }
      next
    }
    in_link_definition {
      print
      if ($0 ~ /^[[:space:]]*$/) {
        in_link_definition = 0
      }
      next
    }
    in_table && $0 ~ /^[[:space:]]*$/ {
      in_table = 0
    }
    in_table {
      if (index($0, "|") != 0) {
        print
        next
      }
      in_table = 0
    }
    /^[[:space:]]*$/ {
      flush_flow()
      print ""
      next
    }

    {
      markdown = strip_markdown_indent($0)
      indent = leading_spaces($0)
    }
    fence_open($0) {
      flush_flow()
      print
      in_fence = 1
      fence_char = next_fence_char
      fence_length = next_fence_length
      next
    }
    markdown ~ /^</ {
      flush_flow()
      print
      html_mode = html_mode_for(markdown)
      if (html_closed(markdown, html_mode)) {
        html_mode = ""
      }
      next
    }
    markdown ~ /^\[[^]]+\]:([ \t]|$)/ {
      flush_flow()
      print
      in_link_definition = 1
      next
    }
    is_table_delimiter(markdown) {
      flush_flow()
      print
      in_table = 1
      if (list_content_indent > 0 && indent < list_content_indent) {
        list_content_indent = 0
      }
      next
    }
    markdown ~ /^>/ || markdown ~ /^#+([ \t]|$)/ ||
    is_thematic_or_setext(markdown) {
      flush_flow()
      print
      if (list_content_indent > 0 && indent < list_content_indent) {
        list_content_indent = 0
      }
      next
    }
    is_list_marker($0) &&
    (marker_indent <= 3 ||
     (list_content_indent > 0 && marker_indent < list_content_indent + 4)) {
      flush_flow()
      list_content_indent = marker_content_indent
      append_flow($0, "list")
      next
    }
    /^\t/ || /^    / {
      if (list_content_indent == 0 || indent >= list_content_indent + 4) {
        flush_flow()
        print
        next
      }
    }
    {
      if (flow == "" && list_content_indent > 0 && indent < list_content_indent) {
        list_content_indent = 0
      }
      append_flow($0, flow_kind == "list" ? "list" : "prose")
    }
    END {
      flush_flow()
    }
  '
}

CHANGELOG="CHANGELOG.md"
if [[ ! -f "$CHANGELOG" ]]; then
  echo "ERROR: $CHANGELOG not found" >&2
  exit 1
fi

# Extract the section for this version from CHANGELOG.md.
# Matches from "## [VERSION]" to the next "## [" line (exclusive).
CHANGELOG_SECTION=$(awk -v ver="$VERSION" '
  /^## \[/ {
    if (found) exit
    if (index($0, "[" ver "]")) { found=1; next }
  }
  found { print }
' "$CHANGELOG")

# Strip leading + trailing blank lines so the section doesn't introduce
# double-blanks when wrapped in `echo ""` boilerplate by the formatters below.
CHANGELOG_SECTION="$(printf '%s\n' "$CHANGELOG_SECTION" \
  | awk 'NF { found = 1 } found' \
  | awk 'NF { last = NR } { line[NR] = $0 } END { for (i = 1; i <= last; i++) print line[i] }' \
  | normalize_markdown)"

if [[ -z "$CHANGELOG_SECTION" ]]; then
  echo "ERROR: No CHANGELOG entry for $VERSION — add entry before releasing" >&2
  exit 1
fi

# Check for manually curated release notes.
# These are plain-English highlights, not the raw changelog.
# Curated notes are REQUIRED — no silent fallback to the raw CHANGELOG. A missing
# or non-conforming file is a hard error (3.0.0/3.0.1 shipped raw CHANGELOG dumps
# precisely because this used to fall back). Structure is enforced separately by
# scripts/validate-release-notes.sh; here we only require the file to exist.
NOTES_FILE=$(find docs/releases -name "*-v${VERSION}-notes.md" 2>/dev/null | head -1 || true)
if [[ -z "$NOTES_FILE" || ! -f "$NOTES_FILE" ]]; then
  echo "ERROR: no curated release-notes file docs/releases/*-v${VERSION}-notes.md" >&2
  echo "       Write one per docs/contracts/release-notes.md before tagging." >&2
  echo "       The raw CHANGELOG is not an acceptable release body." >&2
  exit 1
fi
CURATED_NOTES=$(cat "$NOTES_FILE")
CURATED_NOTES="$(printf '%s' "$CURATED_NOTES" \
  | sed \
    -e "s#(../../CHANGELOG.md)#(https://github.com/${REPO}/blob/main/CHANGELOG.md)#g" \
    -e "s#(../CHANGELOG.md)#(https://github.com/${REPO}/blob/main/docs/CHANGELOG.md)#g" \
  | normalize_markdown)"
echo "Using curated release notes from $NOTES_FILE" >&2

# Build the release notes file
{
  # Header
  cat <<HEADER
\`brew update && brew upgrade agentops\` · \`cd ~/.local/share/agentops && git pull --ff-only && ao skills link\` · [checksums](https://github.com/${REPO}/releases/download/${TAG}/checksums.txt) · [verify provenance](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations/using-artifact-attestations-to-establish-provenance-for-builds)

---

HEADER

  # Curated highlights (required) + the full changelog tucked in a <details>.
  echo "$CURATED_NOTES"
  echo ""
  echo "---"
  echo ""
  echo "<details>"
  echo "<summary>Full changelog</summary>"
  echo ""
  echo "$CHANGELOG_SECTION"
  echo ""
  echo "</details>"

  echo ""
  echo "---"
  echo ""
  echo "**Full Changelog**: https://github.com/${REPO}/compare/${PREV_TAG:-v0.0.0}...${TAG}"
} > release-notes.md

echo "Release notes written to release-notes.md ($(wc -l < release-notes.md) lines)"
