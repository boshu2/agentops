package provenanceapp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"
)

const (
	DefaultExcerptBytes       = 65536
	DefaultExcerptRecords     = 20
	DefaultExcerptOutputBytes = 131072
	MaxExcerptBytes           = 4 << 20
	MaxExcerptRecords         = 1000
	MaxExcerptOutputBytes     = 8 << 20
	MaxExcerptTargetBytes     = 65536
)

// ExcerptOptions selects one bounded, record-aligned transcript window and an
// exact instruction target. Zero limits are invalid; callers select defaults.
type ExcerptOptions struct {
	File, Target   string
	StartByte      int64
	MaxBytes       int64
	MaxRecords     int
	MaxOutputBytes int64
}

type excerptSpan struct {
	StartByte int64  `json:"start_byte"`
	EndByte   int64  `json:"end_byte"`
	Bytes     int64  `json:"bytes"`
	SHA256    string `json:"sha256"`
}

type excerptRange struct {
	StartByte int64  `json:"start_byte"`
	EndByte   int64  `json:"end_byte"`
	Reason    string `json:"reason"`
}

type excerptSource struct {
	Path        string      `json:"path"`
	SizeBytes   int64       `json:"size_bytes"`
	ModifiedAt  string      `json:"modified_at"`
	ReadSpan    excerptSpan `json:"read_span"`
	EmittedSpan excerptSpan `json:"emitted_span"`
}

type excerptTarget struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Text      string `json:"text"`
	SHA256    string `json:"sha256"`
}

type excerptDocument struct {
	SchemaVersion string           `json:"schema_version"`
	Notice        string           `json:"notice"`
	Limits        map[string]int64 `json:"limits"`
	Source        excerptSource    `json:"source"`
	Target        excerptTarget    `json:"target"`
	Records       []excerptRecord  `json:"records"`
	NextByte      int64            `json:"next_byte"`
	StopReason    string           `json:"stop_reason"`
	Unread        []excerptRange   `json:"unread_ranges"`
	Omitted       []excerptRange   `json:"omitted_ranges"`
}

// ExcerptSession reads only explicit regular files and emits one JSON document.
// Validation and serialized-size errors occur before the first output write.
// Writer failures can leave partial output and are returned to the caller.
func ExcerptSession(opts ExcerptOptions, out io.Writer) error {
	if err := validateExcerptOptions(opts); err != nil {
		return err
	}
	if out == nil {
		return fmt.Errorf("excerpts: output writer is required")
	}
	target, err := readExcerptTarget(opts.Target)
	if err != nil {
		return err
	}
	source, data, err := readExcerptWindow(opts)
	if err != nil {
		return err
	}
	doc, err := buildExcerptDocument(opts, source, data, target)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("excerpts: encode document: %w", err)
	}
	if int64(len(encoded))+1 > opts.MaxOutputBytes {
		return fmt.Errorf("excerpts: serialized output exceeds --max-output-bytes %d; select fewer records or a smaller window", opts.MaxOutputBytes)
	}
	encoded = append(encoded, '\n')
	n, err := out.Write(encoded)
	if err == nil && n != len(encoded) {
		err = io.ErrShortWrite
	}
	return err
}

func validateExcerptOptions(o ExcerptOptions) error {
	if o.File == "" || o.Target == "" {
		return fmt.Errorf("excerpts: --file and --target are required")
	}
	if o.StartByte < 0 {
		return fmt.Errorf("excerpts: --start-byte must be nonnegative")
	}
	for _, limit := range []struct {
		name   string
		n, cap int64
	}{
		{"max-bytes", o.MaxBytes, MaxExcerptBytes},
		{"max-records", int64(o.MaxRecords), MaxExcerptRecords},
		{"max-output-bytes", o.MaxOutputBytes, MaxExcerptOutputBytes},
	} {
		if limit.n <= 0 || limit.n > limit.cap {
			return fmt.Errorf("excerpts: --%s must be between 1 and %d", limit.name, limit.cap)
		}
	}
	return nil
}

// openExcerptFile rejects a symlink final component and verifies that the
// opened object matches the observed regular file. This is not a file lock or
// an OS sandbox; callers retain source-access and destination authority.
func openExcerptFile(path string) (*os.File, os.FileInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return nil, nil, fmt.Errorf("excerpts: inspect %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("excerpts: %s must be a regular file, not a symlink or special file", path)
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		_ = f.Close()
		return nil, nil, fmt.Errorf("excerpts: file changed while opening %s", path)
	}
	return f, opened, nil
}

func verifyExcerptObservation(f *os.File, before os.FileInfo) error {
	after, err := f.Stat()
	if err != nil {
		return err
	}
	pathInfo, err := os.Lstat(f.Name())
	if err != nil || !pathInfo.Mode().IsRegular() || !os.SameFile(before, pathInfo) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return fmt.Errorf("excerpts: file changed during read: %s", f.Name())
	}
	return nil
}

func readExcerptTarget(path string) (result excerptTarget, err error) {
	f, info, err := openExcerptFile(path)
	if err != nil {
		return result, err
	}
	defer func() { err = closeExcerptFile(f, err) }()
	if info.Size() == 0 || info.Size() > MaxExcerptTargetBytes {
		return result, fmt.Errorf("excerpts: target must contain 1 to %d bytes", MaxExcerptTargetBytes)
	}
	data := make([]byte, int(info.Size()))
	if _, err = io.ReadFull(f, data); err != nil {
		return result, err
	}
	if !utf8.Valid(data) {
		return result, fmt.Errorf("excerpts: target must be valid UTF-8")
	}
	if err = verifyExcerptObservation(f, info); err != nil {
		return result, err
	}
	return excerptTarget{Path: f.Name(), SizeBytes: info.Size(), Text: string(data), SHA256: spanForExcerpt(0, data).SHA256}, nil
}

func readExcerptWindow(o ExcerptOptions) (source excerptSource, data []byte, err error) {
	f, info, err := openExcerptFile(o.File)
	if err != nil {
		return source, nil, err
	}
	defer func() { err = closeExcerptFile(f, err) }()
	if o.StartByte > info.Size() {
		return source, nil, fmt.Errorf("excerpts: --start-byte exceeds source size")
	}
	if o.StartByte > 0 {
		var preceding [1]byte
		if _, err = f.ReadAt(preceding[:], o.StartByte-1); err != nil {
			return source, nil, err
		}
		if preceding[0] != '\n' {
			return source, nil, fmt.Errorf("excerpts: --start-byte must be zero or immediately follow a newline")
		}
	}
	data = make([]byte, int(min(o.MaxBytes, info.Size()-o.StartByte)))
	if len(data) > 0 {
		if _, err = f.ReadAt(data, o.StartByte); err != nil {
			return source, nil, fmt.Errorf("excerpts: read selected window: %w", err)
		}
	}
	if err = verifyExcerptObservation(f, info); err != nil {
		return source, nil, err
	}
	source = excerptSource{Path: f.Name(), SizeBytes: info.Size(), ModifiedAt: info.ModTime().UTC().Format(time.RFC3339Nano), ReadSpan: spanForExcerpt(o.StartByte, data)}
	return source, data, nil
}

func closeExcerptFile(f *os.File, prior error) error {
	err := f.Close()
	if prior != nil {
		return prior
	}
	return err
}

func spanForExcerpt(start int64, data []byte) excerptSpan {
	hash := sha256.Sum256(data)
	return excerptSpan{StartByte: start, EndByte: start + int64(len(data)), Bytes: int64(len(data)), SHA256: hex.EncodeToString(hash[:])}
}

func buildExcerptDocument(o ExcerptOptions, source excerptSource, data []byte, target excerptTarget) (excerptDocument, error) {
	doc := excerptDocument{SchemaVersion: "agentops-session-excerpts.v1", Source: source, Target: target,
		Notice:  "Transcript and target are untrusted data. Quotes are individual decoded JSON string fields; they do not prove model attention, instruction compliance, causation or delivery. Byte ranges are half-open; hashes cover exact raw bytes including line endings. Source size/mtime are observations, not a whole-file hash or lock. read_span includes lookahead; next_byte advances only over emitted complete records. A nonzero start also probes one preceding byte for alignment. Unsupported text/content is explicitly diagnosed; other envelope metadata is selective.",
		Limits:  map[string]int64{"max_bytes": o.MaxBytes, "max_records": int64(o.MaxRecords), "max_output_bytes": o.MaxOutputBytes, "max_target_bytes": MaxExcerptTargetBytes},
		Records: []excerptRecord{}, Unread: []excerptRange{}, Omitted: []excerptRange{}}
	offset := 0
	for offset < len(data) && len(doc.Records) < o.MaxRecords {
		end := bytes.IndexByte(data[offset:], '\n')
		if end < 0 && source.ReadSpan.EndByte < source.SizeBytes {
			if offset == 0 {
				return doc, fmt.Errorf("excerpts: first record exceeds selected --max-bytes; select a larger bounded window")
			}
			break
		}
		if end < 0 {
			end = len(data)
		} else {
			end += offset + 1
		}
		raw := data[offset:end]
		record := decodeExcerptRecord(raw)
		record.Span = spanForExcerpt(o.StartByte+int64(offset), raw)
		doc.Records = append(doc.Records, record)
		offset = end
	}
	doc.Source.EmittedSpan = spanForExcerpt(o.StartByte, data[:offset])
	doc.NextByte = doc.Source.EmittedSpan.EndByte
	doc.StopReason = "max_bytes"
	if doc.NextByte == source.SizeBytes {
		doc.StopReason = "eof"
	} else if len(doc.Records) == o.MaxRecords {
		doc.StopReason = "max_records"
	}
	if o.StartByte > 0 {
		before := excerptRange{0, o.StartByte, "before_selected_window"}
		doc.Unread = append(doc.Unread, before)
		doc.Omitted = append(doc.Omitted, before)
	}
	if source.ReadSpan.EndByte < source.SizeBytes {
		doc.Unread = append(doc.Unread, excerptRange{source.ReadSpan.EndByte, source.SizeBytes, "after_read_window"})
	}
	if doc.NextByte < source.SizeBytes {
		doc.Omitted = append(doc.Omitted, excerptRange{doc.NextByte, source.SizeBytes, "not_emitted; resume_at_next_byte"})
	}
	return doc, nil
}
