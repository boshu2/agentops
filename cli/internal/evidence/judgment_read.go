package evidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/evidencepath"
	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

const judgmentPrefix = "judgment-receipt:"
const maxJudgmentBytes = 16 << 20

var profileFields = []string{"id", "runtime", "model", "family", "effort"}

// LoadJudgeProfiles loads the consumer's explicit required profiles. Provider
// authorization is deliberately supplied separately; no profile can grant it.
func LoadJudgeProfiles(path string) ([]JudgeProfile, error) {
	raw, err := ReadObject(path)
	if err != nil {
		return nil, err
	}
	if err = verdictcheck.ExactFields(raw, []string{"profiles"}, nil); err != nil {
		return nil, err
	}
	entries, ok := raw["profiles"].([]any)
	if !ok {
		return nil, fmt.Errorf("profiles must be an array")
	}
	for _, entry := range entries {
		if err = strictJudgeProfile(entry); err != nil {
			return nil, err
		}
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var result struct {
		Profiles []JudgeProfile `json:"profiles"`
	}
	if err = json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return result.Profiles, nil
}

func strictJudgmentObject(value any, fields []string) error {
	obj, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("expected receipt object")
	}
	return verdictcheck.ExactFields(obj, fields, nil)
}

func strictJudgeProfile(value any) error {
	if err := strictJudgmentObject(value, profileFields); err != nil {
		return err
	}
	obj := value.(map[string]any)
	for _, field := range profileFields {
		if _, ok := obj[field].(string); !ok {
			return fmt.Errorf("profile %s must be a string", field)
		}
	}
	return nil
}

func decodeJudgmentReceipt(b []byte) (*JudgmentReceipt, error) {
	raw, err := verdictcheck.DecodeObject(b)
	if err != nil {
		return nil, err
	}
	if err = verdictcheck.ExactFields(raw, []string{"version", "requested", "subject_manifest_digest", "acceptance_digest", "author_context_id", "transcript", "exit_code", "timed_out", "truncated", "cleanup_verified", "omissions"}, nil); err != nil {
		return nil, err
	}
	if err = strictJudgeProfile(raw["requested"]); err != nil {
		return nil, err
	}
	if err = strictJudgmentObject(raw["transcript"], []string{"path", "start", "end", "sha256"}); err != nil {
		return nil, err
	}
	if _, ok := raw["omissions"].([]any); !ok {
		return nil, fmt.Errorf("omissions must be an explicit array")
	}
	var receipt JudgmentReceipt
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&receipt); err != nil {
		return nil, err
	}
	if receipt.Version != "1" {
		return nil, fmt.Errorf("unsupported judgment receipt version")
	}
	return &receipt, nil
}

// privateRead confines candidate-selected references to the independently
// supplied evidence root. It never follows an outside-root symlink, retrieves
// a URL or opens a FIFO/device. Files must be private regular files.
func privateRead(root, path string) (payload []byte, err error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("evidence reference must be an absolute canonical private path")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	path = filepath.Join(parent, filepath.Base(path))
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("evidence reference escapes authorized root")
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { err = joinCleanupError(err, handle.Close()) }()
	info, err := handle.Lstat(rel)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("evidence reference must be a private regular file")
	}
	f, err := handle.Open(rel)
	if err != nil {
		return nil, err
	}
	defer func() { err = joinCleanupError(err, f.Close()) }()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("evidence reference must be a private regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxJudgmentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxJudgmentBytes {
		return nil, fmt.Errorf("evidence reference exceeds 16 MiB read bound")
	}
	return b, nil
}

func readJudgmentReceipt(root, ref string) (*JudgmentReceipt, error) {
	if !strings.HasPrefix(ref, judgmentPrefix) {
		return nil, fmt.Errorf("not a judgment receipt reference")
	}
	path, digest, ok := strings.Cut(strings.TrimPrefix(ref, judgmentPrefix), "#sha256=")
	if !ok || !verdictcheck.ValidDigest(digest) {
		return nil, fmt.Errorf("invalid receipt reference digest")
	}
	b, err := privateRead(root, path)
	if err != nil {
		return nil, err
	}
	if Hash(b) != digest {
		return nil, fmt.Errorf("receipt_digest_mismatch")
	}
	return decodeJudgmentReceipt(b)
}

func readNativeSpan(root string, binding TranscriptBinding) ([]byte, error) {
	b, err := privateRead(root, binding.Path)
	if err != nil {
		return nil, err
	}
	if binding.Start < 0 || binding.End <= binding.Start || binding.End > int64(len(b)) {
		return nil, fmt.Errorf("invalid transcript_span")
	}
	if binding.Start > 0 && b[binding.Start-1] != '\n' {
		return nil, fmt.Errorf("transcript_span must start at native line boundary")
	}
	if binding.End < int64(len(b)) && b[binding.End-1] != '\n' {
		return nil, fmt.Errorf("transcript_span must end at native line boundary")
	}
	span := b[binding.Start:binding.End]
	if Hash(span) != binding.SHA256 {
		return nil, fmt.Errorf("transcript_digest_mismatch")
	}
	return span, nil
}

func judgmentRoot(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("missing independently supplied evidence root")
	}
	return evidencepath.Validate(root)
}
