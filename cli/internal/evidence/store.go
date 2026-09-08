package evidence

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/evidencepath"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

type collisionError struct{ path string }

func (e *collisionError) Error() string { return "integrity collision at " + e.path }

type storedFile struct {
	Path    string
	Existed bool
}

func compareStored(handle *os.Root, rel, target string, payload []byte) (storedFile, error) {
	info, err := handle.Lstat(rel)
	if err != nil {
		return storedFile{}, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return storedFile{}, &collisionError{target}
	}
	b, err := handle.ReadFile(rel)
	if err != nil {
		return storedFile{}, err
	}
	if !bytes.Equal(b, payload) {
		return storedFile{}, &collisionError{target}
	}
	return storedFile{target, true}, nil
}

// preflightBytes checks an exact address without creating its parent or file.
// The publication path repeats these checks; this is not a writer lock.
func preflightBytes(root, rel string, payload []byte, excludedGitRoots ...string) (err error) {
	real, err := checkDestination(root, rel, excludedGitRoots...)
	if err != nil {
		return err
	}
	handle, err := os.OpenRoot(real)
	if err != nil {
		return err
	}
	defer func() { err = joinCleanupError(err, handle.Close()) }()
	if _, err := handle.Lstat(rel); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	_, err = compareStored(handle, rel, filepath.Join(real, filepath.FromSlash(rel)), payload)
	return err
}

// checkDestination is read-only, including for paths that do not exist yet.
func checkDestination(root, rel string, excludedGitRoots ...string) (string, error) {
	real, err := evidencepath.Validate(root, excludedGitRoots...)
	if err != nil {
		return "", err
	}
	normalized, err := normalize(rel)
	if err != nil || normalized != rel || rel == "." {
		return "", fmt.Errorf("output must be a canonical path relative to evidence root")
	}
	current := real
	for _, part := range strings.Split(rel, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("evidence destination crosses a symlink")
		}
		if info.IsDir() {
			if _, err = evidencepath.Validate(current, excludedGitRoots...); err != nil {
				return "", err
			}
		}
	}
	return real, nil
}

// storeBytes publishes with a no-replace hard link after syncing the temporary
// file, so concurrent writers cannot overwrite a corrupt content address.
// It reuses the canonical storage package's directory durability barrier.
func storeBytes(root, rel string, payload []byte, excludedGitRoots ...string) (result storedFile, err error) {
	real, err := checkDestination(root, rel, excludedGitRoots...)
	if err != nil {
		return storedFile{}, err
	}
	handle, err := os.OpenRoot(real)
	if err != nil {
		return storedFile{}, err
	}
	defer func() { err = joinCleanupError(err, handle.Close()) }()
	target := filepath.Join(real, filepath.FromSlash(rel))
	compare := func() (storedFile, error) {
		return compareStored(handle, rel, target, payload)
	}
	if _, err := handle.Lstat(rel); err == nil {
		return compare()
	} else if !os.IsNotExist(err) {
		return storedFile{}, err
	}
	dir := filepath.ToSlash(filepath.Dir(rel))
	if err = handle.MkdirAll(dir, 0700); err != nil {
		return storedFile{}, err
	}
	// Recheck the destination after directory creation; no payload has been written.
	if _, err = checkDestination(real, rel, excludedGitRoots...); err != nil {
		return storedFile{}, err
	}
	temp := dir + "/.evidence-" + rand.Text() + ".tmp"
	f, err := handle.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return storedFile{}, err
	}
	defer func() {
		if cleanupErr := handle.Remove(temp); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			err = joinCleanupError(err, fmt.Errorf("remove evidence temporary file: %w", cleanupErr))
		}
	}()
	_, err = f.Write(payload)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return storedFile{}, err
	}
	if closeErr != nil {
		return storedFile{}, closeErr
	}
	if err = handle.Link(temp, rel); err != nil {
		if os.IsExist(err) {
			return compare()
		}
		return storedFile{}, err
	}
	if err = handle.Remove(temp); err != nil {
		return storedFile{}, err
	}
	if err = storage.FsyncDir(filepath.Join(real, filepath.FromSlash(dir))); err != nil {
		return storedFile{}, err
	}
	return storedFile{target, false}, nil
}

type SnapshotResult struct {
	AcceptanceDigest string `json:"acceptance_digest"`
	Idempotent       bool   `json:"idempotent"`
	IntentRef        string `json:"intent_ref"`
}

func SnapshotIntent(root string, payload []byte, excludedGitRoots ...string) (*SnapshotResult, error) {
	digest := Hash(payload)
	stored, err := storeBytes(root, "intents/sha256/"+digest+".intent", payload, excludedGitRoots...)
	if err != nil {
		return nil, err
	}
	return &SnapshotResult{digest, stored.Existed, stored.Path}, nil
}
func SnapshotSource(root, source string, stdin io.Reader, excludedGitRoots ...string) (*SnapshotResult, error) {
	// Root validation precedes even input consumption.
	if _, err := checkDestination(root, "intents/sha256/pending.intent", excludedGitRoots...); err != nil {
		return nil, err
	}
	var b []byte
	var err error
	if source == "-" {
		b, err = io.ReadAll(stdin)
	} else {
		b, err = os.ReadFile(source)
	}
	if err != nil {
		return nil, err
	}
	return SnapshotIntent(root, b, excludedGitRoots...)
}
func StoreDocument(root, relative string, value any, excludedGitRoots ...string) (string, error) {
	b, err := Canonical(value)
	if err != nil {
		return "", err
	}
	result, err := storeBytes(root, relative, append(b, '\n'), excludedGitRoots...)
	return result.Path, err
}

type RuntimeFacts struct {
	AuthorContextID     string
	ValidatorContextID  string
	FreshnessSource     string
	FreshnessAttesterID string
	ScopeResult         string
}
type StoreOptions struct {
	ExcludedGitRoots []string
	EvidenceRoot     string
	Root             string
	Draft            string
	IntentSource     string
	SubjectManifest  string
	BaseManifest     string
	Facts            RuntimeFacts
}
type StoreResult struct {
	AcceptanceDigest         string `json:"acceptance_digest"`
	ArtifactDigest           string `json:"artifact_digest"`
	Idempotent               bool   `json:"idempotent"`
	IntentRef                string `json:"intent_ref"`
	IntentSnapshotIdempotent bool   `json:"intent_snapshot_idempotent"`
	Path                     string `json:"path"`
	Verdict                  string `json:"verdict"`
}

func addFinding(draft map[string]any, id, summary, ref string) {
	findings, _ := draft["findings"].([]any)
	draft["findings"] = append(findings, map[string]any{"id": id, "summary": summary, "evidence_refs": []string{ref}})
}
func integrity(draft map[string]any, summary string) {
	draft["verdict"] = "NOT_PROVEN"
	addFinding(draft, "validate.integrity", summary, "verdict-store")
}

// BindVerdict injects independent runtime facts and preserves the reference
// writer's identity/scope downgrades. It never invents criteria or freshness.
func BindVerdict(draft map[string]any, intent []byte, m *Manifest, f RuntimeFacts) (map[string]any, error) {
	raw, err := object(draft)
	if err != nil {
		return nil, err
	}
	// Reject fields before overriding facts; a candidate cannot smuggle a dialect.
	allowed := []string{"schema_version", "artifact_digest", "acceptance_digest", "subject_manifest_digest", "author_context_id", "validator_context_id", "freshness_attestation"}
	if err = verdictcheck.ExactFields(raw, []string{"verdict", "criteria", "findings", "evidence_refs", "checked", "not_checked", "validated_at"}, allowed); err != nil {
		return nil, err
	}
	for _, k := range []string{"criteria", "findings", "evidence_refs", "checked", "not_checked"} {
		if _, ok := raw[k].([]any); !ok {
			return nil, fmt.Errorf("%s must be an array", k)
		}
	}
	if freshness, ok := raw["freshness_attestation"].(map[string]any); ok {
		if err := verdictcheck.ExactFields(freshness, nil, []string{"source", "attester_identity"}); err != nil {
			return nil, err
		}
	}
	problems := []string{}
	if intent == nil {
		problems = append(problems, "runtime intent source is missing")
	} else {
		raw["acceptance_digest"] = Hash(intent)
	}
	if m == nil {
		problems = append(problems, "runtime subject manifest is missing")
	} else {
		value, err := object(m)
		if err != nil {
			return nil, err
		}
		if _, err = ParseManifest(value); err != nil {
			return nil, err
		}
		raw["subject_manifest_digest"] = m.Digest
	}
	for _, field := range []struct{ key, value string }{{"author_context_id", f.AuthorContextID}, {"validator_context_id", f.ValidatorContextID}} {
		if strings.TrimSpace(field.value) == "" {
			problems = append(problems, "runtime "+field.key+" is missing")
			raw[field.key] = nil
		} else {
			raw[field.key] = field.value
		}
	}
	raw["freshness_attestation"] = nil
	if (f.FreshnessSource != "runtime" && f.FreshnessSource != "caller") || strings.TrimSpace(f.FreshnessAttesterID) == "" {
		problems = append(problems, "runtime freshness attestation is missing or invalid")
	} else {
		raw["freshness_attestation"] = map[string]any{"source": f.FreshnessSource, "attester_identity": f.FreshnessAttesterID}
	}
	if f.ScopeResult == "FAIL" {
		raw["verdict"] = "FAIL"
		addFinding(raw, "validate.scope", "runtime-derived changed paths are outside intent scope", "runtime-scope")
	} else if f.ScopeResult != "PASS" {
		problems = append(problems, "runtime changed-path scope is not proven")
	}
	if f.AuthorContextID != "" && f.AuthorContextID == f.ValidatorContextID {
		problems = append(problems, "author and validator context IDs collide")
	}
	problems, err = verdictPassProblems(raw, problems)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		integrity(raw, strings.Join(problems, "; "))
	}
	raw["schema_version"] = "verdict.v2"
	return sealVerdict(raw)
}
func sealVerdict(raw map[string]any) (map[string]any, error) {
	delete(raw, "artifact_digest")
	digest, err := Digest(raw)
	if err != nil {
		return nil, err
	}
	raw["artifact_digest"] = digest
	b, err := Canonical(raw)
	if err != nil {
		return nil, err
	}
	if err = verdictcheck.VerifyArtifact(b, digest); err != nil {
		return nil, err
	}
	return raw, nil
}
func StoreVerdict(o StoreOptions) (*StoreResult, error) {
	// Check every output branch before any mkdir/temp/write, even intent storage.
	for _, rel := range []string{"intents/sha256/pending.intent", "verdicts/sha256/pending.json"} {
		if _, err := checkDestination(o.EvidenceRoot, rel, o.ExcludedGitRoots...); err != nil {
			return nil, err
		}
	}
	m, err := LoadManifest(o.SubjectManifest)
	if err != nil {
		return nil, err
	}
	if len(m.Entries) == 0 {
		return nil, fmt.Errorf("subject manifest has no entries; nonempty implementation candidate required")
	}
	var base *Manifest
	if o.BaseManifest != "" {
		base, err = LoadManifest(o.BaseManifest)
		if err != nil {
			return nil, err
		}
	}
	if err = VerifyManifest(o.Root, m, base); err != nil {
		return nil, err
	}
	intent, err := os.ReadFile(o.IntentSource)
	if err != nil {
		return nil, err
	}
	draft, err := ReadObject(o.Draft)
	if err != nil {
		return nil, err
	}
	artifact, err := BindVerdict(draft, intent, m, o.Facts)
	if err != nil {
		return nil, err
	}
	// Bind first so preflight checks the actual content addresses, including
	// the selected integrity-recovery address, before storing the intent.
	if err = preflightBytes(o.EvidenceRoot, "intents/sha256/"+Hash(intent)+".intent", intent, o.ExcludedGitRoots...); err != nil {
		return nil, err
	}
	preflightVerdict := func() error {
		b, err := Canonical(artifact)
		if err != nil {
			return err
		}
		return preflightBytes(o.EvidenceRoot, "verdicts/sha256/"+artifact["artifact_digest"].(string)+".json", append(b, '\n'), o.ExcludedGitRoots...)
	}
	recovered := false
	recoverArtifact := func(collision *collisionError) error {
		integrity(artifact, collision.Error())
		var err error
		artifact, err = sealVerdict(artifact)
		recovered = true
		return err
	}
	var collision *collisionError
	if err = preflightVerdict(); err != nil {
		if !errors.As(err, &collision) {
			return nil, err
		}
		if err = recoverArtifact(collision); err != nil {
			return nil, err
		}
		if err = preflightVerdict(); err != nil {
			return nil, err
		}
	}
	snapshot, err := SnapshotIntent(o.EvidenceRoot, intent, o.ExcludedGitRoots...)
	if err != nil {
		return nil, err
	}
	persist := func() (storedFile, error) {
		b, err := Canonical(artifact)
		if err != nil {
			return storedFile{}, err
		}
		return storeBytes(o.EvidenceRoot, "verdicts/sha256/"+artifact["artifact_digest"].(string)+".json", append(b, '\n'), o.ExcludedGitRoots...)
	}
	stored, err := persist()
	// Preserve one recovery attempt if a collision appeared after preflight.
	if !recovered && errors.As(err, &collision) {
		if err = recoverArtifact(collision); err != nil {
			return nil, err
		}
		stored, err = persist()
	}
	if err != nil {
		return nil, err
	}
	return &StoreResult{snapshot.AcceptanceDigest, artifact["artifact_digest"].(string), stored.Existed, snapshot.IntentRef, snapshot.Idempotent, stored.Path, artifact["verdict"].(string)}, nil
}

// VerifySubject binds independently supplied immutable acceptance bytes to
// exact current content and a structurally valid stored PASS. No policy is
// inferred from the verdict and no evidence reference is semantically judged.
func VerifySubject(root, manifestFile, baseFile, verdictFile, intentFile string) error {
	m, err := LoadManifest(manifestFile)
	if err != nil {
		return err
	}
	if len(m.Entries) == 0 {
		return fmt.Errorf("subject manifest has no entries")
	}
	var base *Manifest
	if baseFile != "" {
		base, err = LoadManifest(baseFile)
		if err != nil {
			return err
		}
	}
	if err = VerifyManifest(root, m, base); err != nil {
		return err
	}
	b, err := os.ReadFile(verdictFile)
	if err != nil {
		return err
	}
	raw, err := verdictcheck.DecodeObject(b)
	if err != nil {
		return err
	}
	digest, ok := raw["artifact_digest"].(string)
	if !ok {
		return fmt.Errorf("missing artifact digest")
	}
	if err = verdictcheck.VerifyArtifact(b, digest); err != nil {
		return err
	}
	if filepath.Base(verdictFile) != digest+".json" {
		return fmt.Errorf("artifact_digest does not match filename")
	}
	intent, err := os.ReadFile(intentFile)
	if err != nil {
		return err
	}
	if raw["acceptance_digest"] != Hash(intent) {
		return fmt.Errorf("expected intent digest mismatch")
	}
	if raw["subject_manifest_digest"] != m.Digest {
		return fmt.Errorf("subject manifest digest mismatch")
	}
	if raw["verdict"] != "PASS" {
		return fmt.Errorf("supplied verdict is not PASS")
	}
	return nil
}

func VerifyStored(file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	digest := strings.TrimSuffix(filepath.Base(file), ".json")
	return verdictcheck.VerifyArtifact(b, digest)
}

func verdictPassProblems(raw map[string]any, problems []string) ([]string, error) {
	if raw["verdict"] == "PASS" {
		if len(raw["not_checked"].([]any)) > 0 {
			problems = append(problems, "PASS cannot contain not_checked items: unverified in-scope acceptance stays NOT_PROVEN; bounded proof belongs in criteria[].reason, declared non-goal in intent, residual risk in report")
		}
		criteria := raw["criteria"].([]any)
		if len(criteria) == 0 {
			problems = append(problems, "PASS requires at least one criterion and every criterion must PASS")
		}
		for _, entry := range criteria {
			c, ok := entry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("criterion must be object")
			}
			if c["result"] != "PASS" {
				problems = append(problems, "PASS requires every criterion to PASS")
			}
			refs, ok := c["evidence_refs"].([]any)
			if !ok || len(refs) == 0 {
				problems = append(problems, "PASS requires evidence for every criterion plus nonempty evidence_refs and checked")
			}
		}
		if len(raw["evidence_refs"].([]any)) == 0 || len(raw["checked"].([]any)) == 0 {
			problems = append(problems, "PASS requires evidence for every criterion plus nonempty evidence_refs and checked")
		}
	}
	return problems, nil
}

// joinCleanupError keeps the primary failure available to errors.Is/As and
// reports a failed close or cleanup even when the main operation succeeded.
func joinCleanupError(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	return errors.Join(primary, cleanup)
}
