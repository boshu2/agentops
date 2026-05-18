// Package wiki is the unified bounded context for ao's .agents/-touching
// logic. It consolidates frontmatter parsing, corpus location, indexing, and
// the LLM-wiki pipeline behind one set of ports.
//
// This file implements FrontmatterCodec — the single, behavior-preserving
// frontmatter parser that the five legacy parsers (knowledge.ParseFrontmatter,
// lifecycle.ParseFrontmatter, search.ParseFrontmatterFromContent,
// search.ParseFrontmatterBlock, ratchet.parseYAMLFrontMatter) delegate to.
package wiki

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/boshu2/agentops/cli/internal/ports"
	"gopkg.in/yaml.v3"
)

// Confidence enum coercion targets. The .agents/ corpus mixes float
// confidences (e.g. confidence: 0.85) with enum confidences
// (confidence: high) authored by humans. Legacy parsers silently coerced
// the enum form to the 0.5 default, mis-ranking real content. The codec
// fixes this by mapping the enum vocabulary onto canonical floats.
const (
	// ConfidenceHigh is the canonical float for the "high" enum.
	ConfidenceHigh = 0.85
	// ConfidenceMedium is the canonical float for the "medium" enum.
	ConfidenceMedium = 0.55
	// ConfidenceLow is the canonical float for the "low" enum.
	ConfidenceLow = 0.30
	// ConfidenceDefault is the fallback when a confidence value is absent
	// or malformed.
	ConfidenceDefault = 0.5
)

// Confidence is the tolerant-parsed confidence of a wiki artifact.
// Value is always a canonical float in [0,1]. Raw preserves the original
// textual representation when the input was an enum or a malformed value,
// so callers can surface or audit the source data; Raw is empty when the
// input was already a clean float.
type Confidence struct {
	// Value is the canonical confidence float, always in [0,1].
	Value float64
	// Raw is the original string when the source was an enum
	// ("high"/"medium"/"low") or a malformed value; empty for clean floats.
	Raw string
}

// ParseConfidence coerces a raw frontmatter confidence value into a
// canonical Confidence. It accepts:
//
//   - float64 / int already in [0,1]: passed through (Raw empty).
//   - the enums "high"/"medium"/"low" (case-insensitive): mapped to
//     0.85 / 0.55 / 0.30 with Raw set to the original string.
//   - numeric strings in [0,1]: parsed to float (Raw empty).
//   - anything else (out-of-range numbers, unknown words, nil): 0.5 with
//     Raw set to the original string representation.
func ParseConfidence(v any) Confidence {
	switch typed := v.(type) {
	case nil:
		return Confidence{Value: ConfidenceDefault}
	case float64:
		if inUnitRange(typed) {
			return Confidence{Value: typed}
		}
		return Confidence{Value: ConfidenceDefault, Raw: strconv.FormatFloat(typed, 'g', -1, 64)}
	case float32:
		return ParseConfidence(float64(typed))
	case int:
		return ParseConfidence(float64(typed))
	case int64:
		return ParseConfidence(float64(typed))
	case uint64:
		// yaml.v3 decodes integers exceeding int64's range as uint64.
		return ParseConfidence(float64(typed))
	case string:
		return parseConfidenceString(typed)
	default:
		return Confidence{Value: ConfidenceDefault, Raw: fmt.Sprint(v)}
	}
}

// parseConfidenceString handles the string branch of ParseConfidence.
func parseConfidenceString(s string) Confidence {
	trimmed := strings.TrimSpace(s)
	switch strings.ToLower(trimmed) {
	case "high":
		return Confidence{Value: ConfidenceHigh, Raw: trimmed}
	case "medium":
		return Confidence{Value: ConfidenceMedium, Raw: trimmed}
	case "low":
		return Confidence{Value: ConfidenceLow, Raw: trimmed}
	}
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil && inUnitRange(f) {
		return Confidence{Value: f}
	}
	return Confidence{Value: ConfidenceDefault, Raw: trimmed}
}

// inUnitRange reports whether f is within the canonical [0,1] confidence band.
func inUnitRange(f float64) bool {
	return f >= 0 && f <= 1
}

// Document is the result of decoding a frontmatter-bearing markdown document.
// Fields is the parsed YAML map (empty, non-nil, when no valid frontmatter is
// present). Body is the content after the closing delimiter. HasFrontmatter
// reports whether a well-formed, successfully-parsed frontmatter block was
// found.
type Document struct {
	// Fields is the parsed frontmatter map; empty and non-nil on miss.
	Fields map[string]any
	// Body is the document content after the closing delimiter.
	Body string
	// HasFrontmatter reports whether a valid frontmatter block was parsed.
	HasFrontmatter bool
	// ContentStart is the 0-based line index of the first body line (the
	// line after the closing --- delimiter). It is 0 on a miss. This serves
	// line-oriented callers that need to resume scanning past the block.
	ContentStart int
}

// Confidence returns the tolerant-parsed confidence for the document,
// reading the "confidence" key. Absent key yields the 0.5 default.
func (d Document) Confidence() Confidence {
	return ParseConfidence(d.Fields["confidence"])
}

// FrontmatterCodec is the single frontmatter parser for the wiki bounded
// context. The five legacy parsers delegate their YAML-block extraction and
// confidence handling to a FrontmatterCodec so behavior stays consistent
// across knowledge/, lifecycle/, search/, and ratchet/.
//
// The zero value is ready to use.
type FrontmatterCodec struct{}

// NewFrontmatterCodec returns a ready-to-use codec.
func NewFrontmatterCodec() FrontmatterCodec { return FrontmatterCodec{} }

// Decode parses a leading YAML frontmatter block from text. A frontmatter
// block is the content between a leading "---" line and the next "---" line.
// On any miss (no leading delimiter, no closing delimiter, invalid YAML) the
// returned Document has an empty Fields map, HasFrontmatter false, and Body
// equal to a sensible fallback (see DecodeLines).
func (FrontmatterCodec) Decode(text string) Document {
	return decodeLines(strings.Split(text, "\n"))
}

// DecodeLines is the line-oriented form of Decode for callers that already
// hold a []string. Behavior matches Decode.
func (FrontmatterCodec) DecodeLines(lines []string) Document {
	return decodeLines(lines)
}

// decodeLines locates the frontmatter block in lines and unmarshals it.
//
// Boundary rules (chosen to be the superset of the five legacy parsers):
//
//   - The first line must be "---" (after trimming) and a later "---" line
//     must close the block. Otherwise HasFrontmatter is false.
//   - On a miss, Body is the verbatim input (all lines re-joined) so that
//     callers preserving the original text see no change.
//   - On invalid YAML inside a well-formed block, Fields is empty, Body is
//     the trimmed content after the closing delimiter, and HasFrontmatter is
//     false.
//   - On success, Body is the trimmed content after the closing delimiter.
func decodeLines(lines []string) Document {
	miss := Document{Fields: map[string]any{}, Body: strings.Join(lines, "\n")}

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return miss
	}

	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return miss
	}

	body := strings.TrimSpace(strings.Join(lines[closeIdx+1:], "\n"))
	fmText := strings.Join(lines[1:closeIdx], "\n")
	contentStart := closeIdx + 1

	var fields map[string]any
	if err := yaml.Unmarshal([]byte(fmText), &fields); err != nil || fields == nil {
		return Document{Fields: map[string]any{}, Body: body, ContentStart: contentStart}
	}
	return Document{Fields: fields, Body: body, HasFrontmatter: true, ContentStart: contentStart}
}

// PortDecode adapts Decode to the ports.FrontmatterCodecPort signature.
func (c FrontmatterCodec) PortDecode(text string) ports.FrontmatterDocument {
	return c.Decode(text).toPort()
}

// PortDecodeLines adapts DecodeLines to the ports.FrontmatterCodecPort signature.
func (c FrontmatterCodec) PortDecodeLines(lines []string) ports.FrontmatterDocument {
	return c.DecodeLines(lines).toPort()
}

// PortParseConfidence adapts ParseConfidence to the ports.FrontmatterCodecPort
// signature.
func (FrontmatterCodec) PortParseConfidence(v any) ports.FrontmatterConfidence {
	conf := ParseConfidence(v)
	return ports.FrontmatterConfidence{Value: conf.Value, Raw: conf.Raw}
}

// toPort converts a Document into the port-level FrontmatterDocument shape.
func (d Document) toPort() ports.FrontmatterDocument {
	return ports.FrontmatterDocument{
		Fields:         d.Fields,
		Body:           d.Body,
		HasFrontmatter: d.HasFrontmatter,
	}
}

// PortCodec wraps a FrontmatterCodec so it satisfies ports.FrontmatterCodecPort.
// The port interface uses port-level types; this thin adapter bridges them.
type PortCodec struct{ Codec FrontmatterCodec }

// NewPortCodec returns a PortCodec satisfying ports.FrontmatterCodecPort.
func NewPortCodec() PortCodec { return PortCodec{Codec: NewFrontmatterCodec()} }

// Decode implements ports.FrontmatterCodecPort.
func (p PortCodec) Decode(text string) ports.FrontmatterDocument {
	return p.Codec.PortDecode(text)
}

// DecodeLines implements ports.FrontmatterCodecPort.
func (p PortCodec) DecodeLines(lines []string) ports.FrontmatterDocument {
	return p.Codec.PortDecodeLines(lines)
}

// ParseConfidence implements ports.FrontmatterCodecPort.
func (p PortCodec) ParseConfidence(v any) ports.FrontmatterConfidence {
	return p.Codec.PortParseConfidence(v)
}

// ExtractStringFields scans a frontmatter block for the named keys and
// returns their string values with surrounding quotes stripped. Only keys
// found inside the block are present in the result. This serves the
// field-prefix extraction shape used by search.ParseFrontmatterFromContent.
func (FrontmatterCodec) ExtractStringFields(lines []string, fields ...string) map[string]string {
	result := make(map[string]string)
	inFrontmatter := false
	dashCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			dashCount++
			if dashCount == 1 {
				inFrontmatter = true
				continue
			}
			if dashCount == 2 {
				break
			}
		}
		if !inFrontmatter {
			continue
		}
		for _, field := range fields {
			prefix := field + ":"
			if strings.HasPrefix(trimmed, prefix) {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
				result[field] = strings.Trim(val, "\"'")
			}
		}
	}
	return result
}
