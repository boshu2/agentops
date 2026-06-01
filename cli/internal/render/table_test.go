package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestTable_BasicOutput(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf, "NAME", "AGE", "STATUS")
	tbl.AddRow("alice", "30", "active")
	tbl.AddRow("bob", "25", "inactive")
	if err := tbl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := buf.String()

	// Verify headers present
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "AGE") || !strings.Contains(out, "STATUS") {
		t.Errorf("missing headers in output:\n%s", out)
	}

	// Verify separator
	if !strings.Contains(out, "----") {
		t.Errorf("missing separator in output:\n%s", out)
	}

	// Verify data rows
	if !strings.Contains(out, "alice") || !strings.Contains(out, "bob") {
		t.Errorf("missing data rows in output:\n%s", out)
	}

	// Should have 4 lines (header, separator, 2 data) + trailing newline
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Errorf("expected 4 lines, got %d:\n%s", len(lines), out)
	}
}

func TestTable_EmptyTable(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf, "A", "B")
	if err := tbl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// No rows added means no output at all (no headers either)
	if buf.Len() != 0 {
		t.Errorf("expected empty output for table with no rows, got:\n%s", buf.String())
	}
}

func TestTable_MaxWidth(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf, "ID", "VALUE")
	tbl.SetMaxWidth(0, 8)
	tbl.AddRow("abcdefghijklmnop", "ok")
	if err := tbl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "abcde...") {
		t.Errorf("expected truncated ID, got:\n%s", out)
	}
	if strings.Contains(out, "abcdefghijklmnop") {
		t.Errorf("ID should have been truncated:\n%s", out)
	}
}

func TestTable_MissingValues(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf, "A", "B", "C")
	tbl.AddRow("only-one")
	if err := tbl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "only-one") {
		t.Errorf("expected value in output:\n%s", out)
	}
}

func TestTable_TruncateMaxLessThanThree(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf, "ID", "VALUE")
	tbl.SetMaxWidth(0, 2) // max <= 3 triggers raw slice without "..."
	tbl.AddRow("abcdef", "ok")
	if err := tbl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := buf.String()
	// With max=2, "abcdef" should be truncated to "ab" (no "..." suffix)
	if !strings.Contains(out, "ab") {
		t.Errorf("expected truncated 'ab' in output:\n%s", out)
	}
	// Should NOT contain ellipsis since max <= 3
	if strings.Contains(out, "...") {
		t.Errorf("max <= 3 should not add '...' suffix:\n%s", out)
	}
	// Should NOT contain the full string
	if strings.Contains(out, "abcdef") {
		t.Errorf("ID should have been truncated:\n%s", out)
	}
}

func TestTable_TruncateExactlyAtMax(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf, "ID", "VALUE")
	tbl.SetMaxWidth(0, 5)
	tbl.AddRow("abcde", "ok") // len == max, should NOT truncate
	if err := tbl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "abcde") {
		t.Errorf("string at exactly max should not be truncated:\n%s", out)
	}
}

func TestTable_SeparatorMatchesHeaderLength(t *testing.T) {
	var buf bytes.Buffer
	tbl := NewTable(&buf, "SHORT", "LONGHEADER")
	tbl.AddRow("x", "y")
	if err := tbl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// The separator line fields should match header lengths
	sepFields := strings.Fields(lines[1])
	if len(sepFields) != 2 {
		t.Fatalf("expected 2 separator fields, got %d: %q", len(sepFields), lines[1])
	}
	if sepFields[0] != "-----" {
		t.Errorf("expected 5 dashes for SHORT, got %q", sepFields[0])
	}
	if sepFields[1] != "----------" {
		t.Errorf("expected 10 dashes for LONGHEADER, got %q", sepFields[1])
	}
}

// --- Benchmarks ---

func BenchmarkTableRender(b *testing.B) {
	for range b.N {
		var buf bytes.Buffer
		tbl := NewTable(&buf, "Name", "Value", "Status")
		tbl.SetMaxWidth(0, 20)
		for j := 0; j < 10; j++ {
			tbl.AddRow("some-item", "some-value", "active")
		}
		_ = tbl.Render()
	}
}
