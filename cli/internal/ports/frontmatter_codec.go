package ports

// FrontmatterDocument is the port-level shape of a decoded frontmatter
// document. It mirrors wiki.Document without importing the wiki package, so
// callers of the port stay decoupled from the concrete codec. Fields is the
// parsed YAML map (empty, non-nil, on a miss); Body is the content after the
// closing delimiter; HasFrontmatter reports whether a well-formed block was
// parsed.
type FrontmatterDocument struct {
	// Fields is the parsed frontmatter map; empty and non-nil on miss.
	Fields map[string]any
	// Body is the document content after the closing delimiter.
	Body string
	// HasFrontmatter reports whether a valid frontmatter block was parsed.
	HasFrontmatter bool
}

// FrontmatterConfidence is the port-level shape of a tolerant-parsed
// confidence value. Value is always a canonical float in [0,1]; Raw holds the
// original textual form when the source was an enum or malformed value, and
// is empty for clean floats.
type FrontmatterConfidence struct {
	// Value is the canonical confidence float, always in [0,1].
	Value float64
	// Raw is the original string for enum/malformed inputs; empty for floats.
	Raw string
}

// FrontmatterCodecPort is the BC seam for frontmatter parsing. It exists so
// the five legacy frontmatter parsers (knowledge/lifecycle/search/ratchet)
// route their YAML-block extraction and confidence coercion through one
// consolidated implementation, and so future callers can be exercised against
// an in-memory adapter.
//
// Contract:
//
//   - Decode MUST return a FrontmatterDocument with a non-nil Fields map on
//     every input. On a miss (no/malformed frontmatter) Fields is empty and
//     HasFrontmatter is false.
//   - DecodeLines is the line-oriented equivalent of Decode; results MUST
//     match Decode for the same logical input.
//   - ParseConfidence MUST always return a Value in [0,1]; unknown or
//     out-of-range inputs MUST yield the 0.5 default with Raw populated.
//
// The production implementation is wiki.FrontmatterCodec.
type FrontmatterCodecPort interface {
	// Decode parses a leading YAML frontmatter block from text.
	Decode(text string) FrontmatterDocument
	// DecodeLines is the line-oriented form of Decode.
	DecodeLines(lines []string) FrontmatterDocument
	// ParseConfidence coerces a raw confidence value to a canonical form.
	ParseConfidence(v any) FrontmatterConfidence
}
