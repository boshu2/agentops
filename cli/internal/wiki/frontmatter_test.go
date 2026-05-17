package wiki

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

const floatTolerance = 1e-9

// TestFrontmatterCodec is the primary acceptance test for the codec: a
// learning whose frontmatter declares `confidence: high` must parse to a
// canonical Value of 0.85 with Raw preserving the original "high".
func TestFrontmatterCodec(t *testing.T) {
	codec := NewFrontmatterCodec()

	t.Run("enum_high_coerces_to_0.85", func(t *testing.T) {
		doc := codec.Decode("---\nid: L1\nconfidence: high\n---\nbody\n")
		if !doc.HasFrontmatter {
			t.Fatalf("HasFrontmatter = false, want true")
		}
		conf := doc.Confidence()
		if conf.Value != 0.85 {
			t.Errorf("Confidence.Value = %v, want 0.85", conf.Value)
		}
		if conf.Raw != "high" {
			t.Errorf("Confidence.Raw = %q, want %q", conf.Raw, "high")
		}
	})

	t.Run("enum_medium_and_low", func(t *testing.T) {
		med := ParseConfidence("medium")
		if med.Value != 0.55 || med.Raw != "medium" {
			t.Errorf("medium = {%v,%q}, want {0.55,medium}", med.Value, med.Raw)
		}
		low := ParseConfidence("low")
		if low.Value != 0.30 || low.Raw != "low" {
			t.Errorf("low = {%v,%q}, want {0.30,low}", low.Value, low.Raw)
		}
	})

	t.Run("float_passthrough_keeps_value_and_empty_raw", func(t *testing.T) {
		got := ParseConfidence(0.72)
		if got.Value != 0.72 {
			t.Errorf("Value = %v, want 0.72", got.Value)
		}
		if got.Raw != "" {
			t.Errorf("Raw = %q, want empty for clean float", got.Raw)
		}
	})

	t.Run("malformed_falls_back_to_0.5_and_records_raw", func(t *testing.T) {
		got := ParseConfidence("probably")
		if got.Value != 0.5 {
			t.Errorf("Value = %v, want 0.5 default", got.Value)
		}
		if got.Raw != "probably" {
			t.Errorf("Raw = %q, want %q", got.Raw, "probably")
		}
	})

	t.Run("out_of_range_float_falls_back", func(t *testing.T) {
		got := ParseConfidence(1.5)
		if got.Value != 0.5 {
			t.Errorf("Value = %v, want 0.5 default for out-of-range", got.Value)
		}
		if got.Raw != "1.5" {
			t.Errorf("Raw = %q, want %q", got.Raw, "1.5")
		}
	})

	t.Run("absent_confidence_yields_default", func(t *testing.T) {
		doc := codec.Decode("---\nid: L1\n---\nbody\n")
		conf := doc.Confidence()
		if conf.Value != 0.5 {
			t.Errorf("absent confidence Value = %v, want 0.5", conf.Value)
		}
	})
}

// TestFrontmatterCodec_Golden drives the L0 golden contract file: every line
// of testdata/frontmatter_golden.txt is one ParseConfidence case.
func TestFrontmatterCodec_Golden(t *testing.T) {
	path := filepath.Join("testdata", "frontmatter_golden.txt")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open golden: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	cases := 0
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		runGoldenCase(t, line)
		cases++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan golden: %v", err)
	}
	if cases == 0 {
		t.Fatal("golden file produced zero cases")
	}
}

// runGoldenCase parses and asserts a single golden line of the form
// "<input> => <value> | <raw>".
func runGoldenCase(t *testing.T, line string) {
	t.Helper()
	lhs, rhs, ok := strings.Cut(line, "=>")
	if !ok {
		t.Fatalf("malformed golden line (no =>): %q", line)
	}
	wantValStr, wantRaw, ok := strings.Cut(rhs, "|")
	if !ok {
		t.Fatalf("malformed golden line (no |): %q", line)
	}
	wantVal, err := strconv.ParseFloat(strings.TrimSpace(wantValStr), 64)
	if err != nil {
		t.Fatalf("bad expected value in %q: %v", line, err)
	}
	expectRaw := strings.TrimSpace(wantRaw)
	if expectRaw == `""` {
		expectRaw = ""
	}

	input := decodeGoldenInput(t, strings.TrimSpace(lhs))
	got := ParseConfidence(input)
	if math.Abs(got.Value-wantVal) > floatTolerance {
		t.Errorf("%q: Value = %v, want %v", line, got.Value, wantVal)
	}
	if got.Raw != expectRaw {
		t.Errorf("%q: Raw = %q, want %q", line, got.Raw, expectRaw)
	}
}

// decodeGoldenInput maps a golden input token to its typed Go value.
func decodeGoldenInput(t *testing.T, token string) any {
	t.Helper()
	switch {
	case strings.HasPrefix(token, "float:"):
		v, err := strconv.ParseFloat(strings.TrimPrefix(token, "float:"), 64)
		if err != nil {
			t.Fatalf("bad float token %q: %v", token, err)
		}
		return v
	case strings.HasPrefix(token, "int:"):
		v, err := strconv.Atoi(strings.TrimPrefix(token, "int:"))
		if err != nil {
			t.Fatalf("bad int token %q: %v", token, err)
		}
		return v
	case strings.HasPrefix(token, "enum:"):
		return strings.TrimPrefix(token, "enum:")
	default:
		t.Fatalf("unknown golden token prefix: %q", token)
		return nil
	}
}

func TestFrontmatterCodec_Decode(t *testing.T) {
	codec := NewFrontmatterCodec()

	t.Run("valid_block", func(t *testing.T) {
		doc := codec.Decode("---\nid: abc\nutility: 0.8\n---\nbody goes here\n")
		if !doc.HasFrontmatter {
			t.Fatal("HasFrontmatter = false, want true")
		}
		if doc.Fields["id"] != "abc" {
			t.Errorf("Fields[id] = %v, want abc", doc.Fields["id"])
		}
		if doc.Body != "body goes here" {
			t.Errorf("Body = %q, want %q", doc.Body, "body goes here")
		}
		if doc.ContentStart != 4 {
			t.Errorf("ContentStart = %d, want 4", doc.ContentStart)
		}
	})

	t.Run("no_frontmatter_returns_verbatim_body", func(t *testing.T) {
		in := "just a body\nand more"
		doc := codec.Decode(in)
		if doc.HasFrontmatter {
			t.Error("HasFrontmatter = true, want false")
		}
		if len(doc.Fields) != 0 {
			t.Errorf("Fields = %v, want empty", doc.Fields)
		}
		if doc.Fields == nil {
			t.Error("Fields is nil, must be non-nil empty map")
		}
		if doc.Body != in {
			t.Errorf("Body = %q, want verbatim input %q", doc.Body, in)
		}
	})

	t.Run("unclosed_delimiter_is_a_miss", func(t *testing.T) {
		in := "---\nid: abc\nno close here"
		doc := codec.Decode(in)
		if doc.HasFrontmatter {
			t.Error("HasFrontmatter = true, want false for unclosed block")
		}
		if doc.Body != in {
			t.Errorf("Body = %q, want verbatim input", doc.Body)
		}
	})

	t.Run("invalid_yaml_keeps_block_boundary_but_no_frontmatter", func(t *testing.T) {
		doc := codec.Decode("---\nid: [unclosed\n---\nbody\n")
		if doc.HasFrontmatter {
			t.Error("HasFrontmatter = true, want false for invalid YAML")
		}
		if len(doc.Fields) != 0 {
			t.Errorf("Fields = %v, want empty for invalid YAML", doc.Fields)
		}
		if doc.Body != "body" {
			t.Errorf("Body = %q, want %q", doc.Body, "body")
		}
		if doc.ContentStart != 3 {
			t.Errorf("ContentStart = %d, want 3", doc.ContentStart)
		}
	})
}

func TestFrontmatterCodec_DecodeLines(t *testing.T) {
	codec := NewFrontmatterCodec()
	lines := []string{"---", "maturity: candidate", "utility: 0.8", "---", "body text"}
	doc := codec.DecodeLines(lines)
	if !doc.HasFrontmatter {
		t.Fatal("HasFrontmatter = false, want true")
	}
	if doc.Fields["maturity"] != "candidate" {
		t.Errorf("Fields[maturity] = %v, want candidate", doc.Fields["maturity"])
	}
	if doc.Body != "body text" {
		t.Errorf("Body = %q, want %q", doc.Body, "body text")
	}
}

func TestFrontmatterCodec_ExtractStringFields(t *testing.T) {
	codec := NewFrontmatterCodec()
	lines := strings.Split("---\nid: \"L1\"\nmaturity: candidate\nutility: 0.8\n---\nbody\nid: ignored", "\n")
	got := codec.ExtractStringFields(lines, "id", "maturity", "missing")
	if got["id"] != "L1" {
		t.Errorf("id = %q, want L1 (quotes stripped)", got["id"])
	}
	if got["maturity"] != "candidate" {
		t.Errorf("maturity = %q, want candidate", got["maturity"])
	}
	if _, ok := got["missing"]; ok {
		t.Error("missing key should not be present")
	}
	// "id: ignored" appears after the closing delimiter and must be skipped.
	if got["id"] == "ignored" {
		t.Error("extraction leaked past the closing delimiter")
	}
}

// TestPortCodec_SatisfiesPort confirms PortCodec implements the port
// interface and the adapter preserves codec results.
func TestPortCodec_SatisfiesPort(t *testing.T) {
	var p ports.FrontmatterCodecPort = NewPortCodec()

	doc := p.Decode("---\nconfidence: high\n---\nbody\n")
	if !doc.HasFrontmatter {
		t.Fatal("port Decode HasFrontmatter = false, want true")
	}
	conf := p.ParseConfidence(doc.Fields["confidence"])
	if conf.Value != 0.85 || conf.Raw != "high" {
		t.Errorf("port ParseConfidence = {%v,%q}, want {0.85,high}", conf.Value, conf.Raw)
	}

	linesDoc := p.DecodeLines([]string{"---", "id: x", "---", "tail"})
	if linesDoc.Body != "tail" {
		t.Errorf("port DecodeLines Body = %q, want %q", linesDoc.Body, "tail")
	}
}
