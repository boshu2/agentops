package render

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// Table formats columnar output using tabwriter.
type Table struct {
	w             *tabwriter.Writer
	headers       []string
	maxWidth      map[int]int // column index -> max width (0 = unlimited)
	headerWritten bool
}

// NewTable creates a table that writes to w with the given column headers.
func NewTable(w io.Writer, headers ...string) *Table {
	return &Table{
		w:        tabwriter.NewWriter(w, 0, 0, 2, ' ', 0),
		headers:  headers,
		maxWidth: make(map[int]int),
	}
}

// SetMaxWidth sets the maximum display width for a column (0-indexed).
// Values exceeding the limit are truncated with "...".
func (t *Table) SetMaxWidth(col, width int) *Table {
	t.maxWidth[col] = width
	return t
}

// AddRow appends a data row. Extra values beyond the header count are ignored;
// missing values are filled with empty strings.
func (t *Table) AddRow(values ...string) {
	if !t.headerWritten {
		t.headerWritten = true
		t.writeHeaderAndSeparator()
	}

	cells := make([]string, len(t.headers))
	for i := range cells {
		if i < len(values) {
			cells[i] = t.truncate(i, values[i])
		}
	}

	for i, cell := range cells {
		if i > 0 {
			//nolint:errcheck // tabwriter output to stdout
			fmt.Fprint(t.w, "\t")
		}
		//nolint:errcheck // tabwriter output to stdout
		fmt.Fprint(t.w, cell)
	}
	//nolint:errcheck // tabwriter output to stdout
	fmt.Fprintln(t.w)
}

// Render flushes the underlying tabwriter. Must be called after all AddRow calls.
func (t *Table) Render() error {
	return t.w.Flush()
}

func (t *Table) writeHeaderAndSeparator() {
	for i, h := range t.headers {
		if i > 0 {
			//nolint:errcheck // tabwriter output to stdout
			fmt.Fprint(t.w, "\t")
		}
		//nolint:errcheck // tabwriter output to stdout
		fmt.Fprint(t.w, h)
	}
	//nolint:errcheck // tabwriter output to stdout
	fmt.Fprintln(t.w)

	for i, h := range t.headers {
		if i > 0 {
			//nolint:errcheck // tabwriter output to stdout
			fmt.Fprint(t.w, "\t")
		}
		//nolint:errcheck // tabwriter output to stdout
		fmt.Fprint(t.w, dashes(len(h)))
	}
	//nolint:errcheck // tabwriter output to stdout
	fmt.Fprintln(t.w)
}

func (t *Table) truncate(col int, s string) string {
	max, ok := t.maxWidth[col]
	if !ok || max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// dashes returns a string of n dashes.
func dashes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '-'
	}
	return string(b)
}
