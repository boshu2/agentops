package sourceread

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

// Options carries finite caller-selected bounds. ThroughByte and the expected
// prefix digest are paired to bind every continued read to one frozen prefix.
type Options struct {
	File               string
	Context            config.ContextRequest
	StartByte          int64
	MaxBytes           int64
	ThroughByte        *int64
	ExpectPrefixSHA256 string
	ExpectFileIdentity string
	AllowOversize      bool
}

type Observation struct {
	Identity   string `json:"identity"`
	Size       int64  `json:"size"`
	ModifiedNS int64  `json:"modified_unix_nano"`
}

type Result struct {
	SchemaVersion           string        `json:"schema_version"`
	File                    string        `json:"file"`
	SourceID                string        `json:"source_id"`
	ProjectID               string        `json:"project_id"`
	OwnerScope              string        `json:"owner_scope"`
	TaskRef                 string        `json:"task_ref"`
	ModelRef                string        `json:"model_ref"`
	DestinationRef          string        `json:"destination_ref"`
	ContentScope            string        `json:"content_scope"`
	AccessEnforcement       string        `json:"access_enforcement"`
	PolicySHA256            string        `json:"source_policy_sha256"`
	Before                  Observation   `json:"file_before"`
	After                   Observation   `json:"file_after"`
	FileIdentityExpectation string        `json:"file_identity_expectation"`
	CapturedThrough         int64         `json:"captured_through"`
	StartByte               int64         `json:"start_byte"`
	EndByte                 int64         `json:"end_byte"`
	NextByte                int64         `json:"next_byte"`
	PrefixSHA256            string        `json:"prefix_sha256"`
	SpanSHA256              string        `json:"span_sha256"`
	BytesBase64             string        `json:"bytes_base64"`
	TextView                string        `json:"text_view"`
	TextViewEncoding        string        `json:"text_view_encoding"`
	Profile                 OutputProfile `json:"output_profile"`
	OversizeOverride        bool          `json:"oversize_override"`
	ProfileBoundSatisfied   bool          `json:"profile_bound_satisfied"`
	HostDelivery            string        `json:"host_delivery"`
	SemanticProcessing      string        `json:"semantic_processing"`
	CompleteReading         bool          `json:"complete_reading"`
	SerializedBytes         int64         `json:"serialized_bytes"`
}

func validateOptions(o Options) error {
	if o.StartByte < 0 || o.MaxBytes <= 0 {
		return fmt.Errorf("read-source: start-byte must be nonnegative and max-bytes must be positive")
	}
	if o.File == "" {
		return fmt.Errorf("read-source: file is required")
	}
	if (o.ThroughByte == nil) != (o.ExpectPrefixSHA256 == "") {
		return fmt.Errorf("read-source: through-byte and expect-prefix-sha256 must be supplied together")
	}
	if o.ThroughByte != nil && (*o.ThroughByte < 0 || o.StartByte > *o.ThroughByte || !verdictcheck.ValidDigest(o.ExpectPrefixSHA256)) {
		return fmt.Errorf("read-source: invalid frozen boundary or prefix digest")
	}
	return nil
}

// Run emits exactly one compact JSON result after authorization, integrity and
// actual serialized-size checks. A successful write is only emitted output;
// downstream host observation and semantic acknowledgements remain separate.
func Run(ctx context.Context, o Options, out io.Writer) error { return (Service{}).Run(ctx, o, out) }
func (s Service) Run(ctx context.Context, o Options, out io.Writer) error {
	if err := validateOptions(o); err != nil {
		return err
	}
	a, err := s.authorize(ctx, o)
	if err != nil {
		return err
	}
	// The base64 content alone cannot fit when max-bytes exceeds this bound.
	// Reject before source open; do not allocate an arbitrarily large request.
	if !o.AllowOversize && o.MaxBytes > a.policy.OutputProfile.MaxSerializedBytes {
		return fmt.Errorf("read-source: requested bounds exceed selected serialized-output profile; reduce max-bytes or explicitly allow oversize")
	}
	result, err := s.read(ctx, o, a)
	if err != nil {
		return err
	}
	payload, err := serialize(&result)
	if err != nil {
		return err
	}
	if int64(len(payload)) > a.policy.OutputProfile.MaxSerializedBytes {
		if !o.AllowOversize {
			return fmt.Errorf("read-source: serialized output exceeds selected profile; no source bytes emitted")
		}
		result.ProfileBoundSatisfied = false
		payload, err = serialize(&result)
		if err != nil {
			return err
		}
	}
	n, err := out.Write(payload)
	if err != nil || n != len(payload) {
		return fmt.Errorf("read-source: output write incomplete; host delivery unverified")
	}
	return nil
}

func serialize(r *Result) ([]byte, error) {
	// serialized_bytes includes its own digits and the one newline. Iterate to
	// a fixed point; the decimal width can only grow finitely.
	for {
		b, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}
		b = append(b, '\n')
		if r.SerializedBytes == int64(len(b)) {
			return b, nil
		}
		r.SerializedBytes = int64(len(b))
	}
}

func observe(f *os.File, info os.FileInfo) Observation {
	return Observation{Identity: nativeIdentity(f, info), Size: info.Size(), ModifiedNS: info.ModTime().UnixNano()}
}

func (s Service) read(ctx context.Context, o Options, a authorization) (r Result, err error) {
	f, before, err := s.openChecked(o)
	if err != nil {
		return r, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	through := before.Size()
	if o.ThroughByte != nil {
		through = *o.ThroughByte
	}
	if through > before.Size() || o.StartByte > through {
		return r, fmt.Errorf("read-source: truncated source or start beyond captured boundary")
	}
	count := min(o.MaxBytes, through-o.StartByte)
	prefix, span, err := readPrefix(ctx, f, through, o.StartByte, count)
	if err != nil {
		return r, err
	}
	if o.ExpectPrefixSHA256 != "" && prefix != o.ExpectPrefixSHA256 {
		return r, fmt.Errorf("read-source: frozen prefix changed")
	}
	if s.afterRead != nil {
		s.afterRead()
	}
	after, err := verifyRead(ctx, f, o.File, before, through, prefix)
	if err != nil {
		return r, err
	}
	text, textEncoding := textView(span)
	expectation := "not-supplied; cross-invocation replacement not checked"
	if o.ExpectFileIdentity != "" {
		expectation = "matched"
	}
	req := o.Context
	return Result{SchemaVersion: "source-read.v1", File: o.File, SourceID: req.SourceID, ProjectID: req.ProjectID, OwnerScope: req.OwnerScope, TaskRef: req.TaskRef, ModelRef: req.ModelRef, DestinationRef: req.DestinationRef, ContentScope: a.permission.ContentScope, AccessEnforcement: a.route.AccessEnforcement, PolicySHA256: a.policyDigest, Before: observe(f, before), After: observe(f, after), FileIdentityExpectation: expectation, CapturedThrough: through, StartByte: o.StartByte, EndByte: o.StartByte + count, NextByte: o.StartByte + count, PrefixSHA256: prefix, SpanSHA256: digest(span), BytesBase64: base64.StdEncoding.EncodeToString(span), TextView: text, TextViewEncoding: textEncoding, Profile: a.policy.OutputProfile, OversizeOverride: o.AllowOversize, ProfileBoundSatisfied: true, HostDelivery: "host-delivery-unverified", SemanticProcessing: "not-established", CompleteReading: false}, nil
}

func (s Service) openChecked(o Options) (*os.File, os.FileInfo, error) {
	before, err := os.Lstat(o.File)
	if err != nil || !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("read-source: source is unavailable or not a regular file")
	}
	open := s.openSource
	if open == nil {
		open = os.Open
	}
	f, err := open(o.File)
	if err != nil {
		return nil, nil, fmt.Errorf("read-source: source open failed")
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		_ = f.Close()
		return nil, nil, fmt.Errorf("read-source: file replaced while opening")
	}
	identity := nativeIdentity(f, opened)
	if o.ExpectFileIdentity != "" && (identity == "unavailable" || o.ExpectFileIdentity != identity) {
		_ = f.Close()
		return nil, nil, fmt.Errorf("read-source: file identity changed or unavailable")
	}
	return f, opened, nil
}

func verifyRead(ctx context.Context, f *os.File, path string, opened os.FileInfo, through int64, prefix string) (os.FileInfo, error) {
	// Re-read the entire frozen prefix; appends beyond it are harmless while
	// corrections inside it invalidate this result, even with identical calls.
	check, _, err := readPrefix(ctx, f, through, 0, 0)
	if err != nil || check != prefix {
		return nil, fmt.Errorf("read-source: frozen prefix changed or short read during verification")
	}
	after, err := f.Stat()
	if err != nil || after.Size() < through {
		return nil, fmt.Errorf("read-source: source truncated during read")
	}
	pathInfo, err := os.Lstat(path)
	if err != nil || !pathInfo.Mode().IsRegular() || !os.SameFile(opened, pathInfo) {
		return nil, fmt.Errorf("read-source: source replaced during read")
	}
	return after, nil
}

func textView(span []byte) (string, string) {
	textEncoding := "utf-8"
	text := string(span)
	if !utf8.Valid(span) {
		text = strings.ToValidUTF8(text, "\uFFFD")
		textEncoding = "utf-8-replacement-view; bytes_base64 is authoritative"
	}
	return text, textEncoding
}

func readPrefix(ctx context.Context, f *os.File, through, start, count int64) (string, []byte, error) {
	h := sha256.New()
	var span bytes.Buffer
	buf := make([]byte, 32*1024)
	for offset := int64(0); offset < through; {
		if err := ctx.Err(); err != nil {
			return "", nil, fmt.Errorf("read-source: source read canceled")
		}
		n := min(int64(len(buf)), through-offset)
		got, err := f.ReadAt(buf[:n], offset)
		if err != nil || int64(got) != n {
			return "", nil, fmt.Errorf("read-source: short source read")
		}
		_, _ = h.Write(buf[:got])
		lo, hi := max(offset, start), min(offset+n, start+count)
		if hi > lo {
			_, _ = span.Write(buf[lo-offset : hi-offset])
		}
		offset += n
	}
	return fmt.Sprintf("%x", h.Sum(nil)), span.Bytes(), nil
}
