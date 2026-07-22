package delivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

// NativeBinaries pins the only runtime executors accepted by the optional
// delivery binary. A caller cannot substitute an arbitrary lifecycle script:
// Beads/GC own substrate state, git owns worktree effects, and gh is the sole
// selected forge-native auto-merge actor.
type NativeBinaries struct{ GC, Beads, Git, GH, Bash, Delivery string }

const (
	canonicalToolchainLockDigest   = "aecc3e4b097e8e873f2a92478da4535b85bc70fdc31c9605aa4a4462a0482fa0"
	canonicalBeadsCapabilityDigest = "1ea4f0946fd713c02d5ef0855f47f41bf108fe6ca33c76959f173350816a1359"
	nativeCommandTimeout           = 30 * time.Second
	nativeOutputLimit              = 1 << 20
)

// NativeProviders deliberately fails closed until its fixed native command
// operations are supplied with the capability-qualified repository context.
// It is not a persistence fallback: the in-memory fake is unavailable here.
type NativeProviders struct {
	binaries  NativeBinaries
	context   NativeContext
	manifest  SubjectManifest
	candidate string
	requests  map[string]Request
	ghRun     func(context.Context, ...string) ([]byte, error)
}

func NewNativeProviders(binaries NativeBinaries, native NativeContext) (*NativeProviders, error) {
	bound := map[string]string{
		"gc": binaries.GC, "bd": binaries.Beads, "git": binaries.Git,
		"gh": binaries.GH, "bash": binaries.Bash, "agentops-gc-delivery": binaries.Delivery,
	}
	for name, value := range bound {
		if !filepath.IsAbs(value) {
			return nil, errors.New("all native delivery binaries must be absolute")
		}
		info, err := os.Stat(value)
		if err != nil {
			return nil, err
		}
		if info.IsDir() || info.Mode()&0o111 == 0 {
			return nil, errors.New("native delivery binary is not executable")
		}
		bytes, err := os.ReadFile(value)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(bytes)
		binding, ok := native.Executables[name]
		if !ok || binding.Path != value || binding.Digest != fmt.Sprintf("%x", sum) {
			return nil, errors.New("native executable does not match the context binding")
		}
	}
	for _, path := range []string{native.RepositoryDir, native.WorktreeRoot, native.BeadsDir} {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return nil, errors.New("native context directory is unavailable")
		}
	}
	if !filepath.IsAbs(native.RepositoryDir) || !filepath.IsAbs(native.WorktreeRoot) || !filepath.IsAbs(native.BeadsDir) {
		return nil, errors.New("native context paths must be absolute")
	}
	if overlap, err := pathsOverlap(native.RepositoryDir, native.WorktreeRoot); err != nil || overlap {
		return nil, errors.New("ephemeral worktree root must be disjoint from repository")
	}
	if err := verifyToolchainBindings(native, bound); err != nil {
		return nil, err
	}
	return &NativeProviders{binaries: binaries, context: native}, nil
}

// VerifySubject is read-only. It proves the certificate's candidate is a
// local, single-parent commit and that subject-manifest.v1 (not a Git-derived
// digest) exactly covers its parent-to-candidate inventory.
func (p *NativeProviders) VerifySubject(ctx context.Context, request Request) error {
	if len(request.SubjectBytes) == 0 {
		return nil
	}
	if request.NativeDigest == "" || !reflect.DeepEqual(request.NativeContext, p.context) {
		return errors.New("delivery request native context does not match the controller binding")
	}
	if err := p.verifyRepository(ctx); err != nil {
		return err
	}
	if overlap, err := pathsOverlap(p.context.WorktreeRoot, request.Root); err != nil || overlap {
		return errors.New("ephemeral worktree root must be disjoint from evidence root")
	}
	manifest := request.SubjectManifest
	candidate := request.Certificate.Candidate.Commit
	if tree, err := p.git(ctx, "rev-parse", candidate+"^{tree}"); err != nil || tree != request.Certificate.Candidate.Tree {
		return errors.New("candidate tree differs from certificate")
	}
	parentLine, err := p.git(ctx, "rev-list", "--parents", "-n", "1", candidate)
	if err != nil {
		return err
	}
	parts := strings.Fields(parentLine)
	if len(parts) != 2 {
		return errors.New("candidate must have exactly one parent")
	}
	parent := parts[1]
	if ok, err := p.git(ctx, "merge-base", "--is-ancestor", parent, request.Target.BaseOID); err != nil || ok != "" { // successful git exits without output
		if err != nil {
			return errors.New("candidate parent is not an ancestor of epoch base")
		}
	}
	changed, err := p.changedPaths(ctx, parent, candidate)
	if err != nil {
		return err
	}
	declared := map[string]ManifestEntry{}
	for _, entry := range manifest.Entries {
		declared[entry.Path] = entry
	}
	if len(changed) != len(declared) {
		return errors.New("subject manifest does not exactly cover candidate paths")
	}
	for path, deleted := range changed {
		entry, ok := declared[path]
		if !ok || deleted != (entry.Kind == "deletion") {
			return errors.New("subject manifest path or deletion differs from candidate")
		}
		if deleted {
			if err := p.verifyDeletion(ctx, parent, entry); err != nil {
				return err
			}
		} else if err := p.verifyEntry(ctx, candidate, entry); err != nil {
			return err
		}
	}
	p.manifest, p.candidate = manifest, candidate
	if p.requests == nil {
		p.requests = make(map[string]Request)
	}
	key := request.Target.DeliveryBeadID
	if key == "" {
		key = makePrepared(request).DeliveryBeadID
	}
	p.requests[key] = request
	return nil
}

func (p *NativeProviders) verifyDeletion(ctx context.Context, parent string, entry ManifestEntry) error {
	output, err := p.git(ctx, "ls-tree", parent, "--", entry.Path)
	if err != nil || output == "" {
		return errors.New("manifest deletion did not exist in candidate parent")
	}
	parts := strings.Fields(output)
	if len(parts) < 3 || (parts[0] != "100644" && parts[0] != "100755" && parts[0] != "120000") || entry.Executable != (parts[0] == "100755") {
		return errors.New("manifest deletion executable mode differs from candidate parent")
	}
	return nil
}

func (p *NativeProviders) verifyComposedManifest(ctx context.Context, epoch string) error {
	if p.manifest.CanonicalManifestDigest == "" {
		return errors.New("epoch composition has no verified subject manifest")
	}
	for _, entry := range p.manifest.Entries {
		if entry.Kind != "deletion" {
			if err := p.verifyEntry(ctx, epoch, entry); err != nil {
				return fmt.Errorf("composed manifest entry %s: %w", entry.Path, err)
			}
			continue
		}
		output, err := p.git(ctx, "ls-tree", epoch, "--", entry.Path)
		if err != nil || output != "" {
			return errors.New("composed tree retains a declared deletion")
		}
	}
	return nil
}

// verifyToolchainBindings makes the lock and capability receipt active
// constraints rather than unattested digests.  The toolchain receipt is the
// existing materializer output: its runtime gc/bd paths and hashes must agree
// with the fixed executable bindings before a reducer can reach an effect.
func verifyToolchainBindings(native NativeContext, bound map[string]string) error {
	lock, capability, receipt, err := loadBoundToolchainDocuments(native)
	if err != nil {
		return err
	}
	if stringAt(capability, "selected_representation") != native.BeadsRepresentation || stringAt(capability, "toolchain", "lock_sha256") != native.ToolchainLock {
		return errors.New("successor capability does not bind the selected locked representation")
	}
	if numberAt(receipt, "schema_version") != 3 || !exactObjectKeys(receipt, "runtime", []string{"agentops-gc-delivery", "ao", "bd", "gc"}) {
		return errors.New("toolchain receipt is not the exact schema-3 runtime set")
	}
	if err := verifyNativeGCBDReceipts(native, bound, lock, receipt); err != nil {
		return err
	}
	return verifyNativeAgentOpsReceipts(native, bound, receipt)
}

func loadBoundToolchainDocuments(native NativeContext) (map[string]any, map[string]any, map[string]any, error) {
	if native.ToolchainLock != canonicalToolchainLockDigest || native.SuccessorCapability != canonicalBeadsCapabilityDigest || native.BeadsRepresentation != "B-successor-delivery-bead" {
		return nil, nil, nil, errors.New("native context does not bind the repository toolchain and B-successor capability SOT")
	}
	lockPath := filepath.Join(native.RepositoryDir, "deploy", "gc", "toolchain.lock.json")
	if err := fileHasDigest(lockPath, native.ToolchainLock); err != nil {
		return nil, nil, nil, fmt.Errorf("toolchain lock: %w", err)
	}
	capabilityPath := filepath.Join(native.RepositoryDir, "deploy", "gc", "beads-capability-selection.v1.json")
	if err := fileHasDigest(capabilityPath, native.SuccessorCapability); err != nil {
		return nil, nil, nil, fmt.Errorf("successor capability: %w", err)
	}
	if err := fileHasDigest(native.ToolchainReceipt, native.ToolchainReceiptSum); err != nil {
		return nil, nil, nil, fmt.Errorf("toolchain receipt: %w", err)
	}
	lock, err := jsonObject(lockPath)
	if err != nil {
		return nil, nil, nil, err
	}
	capability, err := jsonObject(capabilityPath)
	if err != nil {
		return nil, nil, nil, err
	}
	receipt, err := jsonObject(native.ToolchainReceipt)
	if err != nil {
		return nil, nil, nil, err
	}
	return lock, capability, receipt, nil
}

func verifyNativeGCBDReceipts(native NativeContext, bound map[string]string, lock, receipt map[string]any) error {
	for _, name := range []string{"gc", "bd"} {
		binding := native.Executables[name]
		receiptPath := runtimeReceiptPath(native.ToolchainReceipt, receipt, name)
		if receiptPath != bound[name] || stringAt(receipt, "runtime", name, "sha256") != binding.Digest {
			return errors.New("toolchain receipt does not bind native gc/bd bytes")
		}
		if stringAt(receipt, "pair", name, "source_commit") == "" {
			return errors.New("toolchain receipt lacks native gc/bd source provenance")
		}
	}
	if !lockHasQualifiedPair(lock, stringAt(receipt, "pair", "gc", "source_commit"), stringAt(receipt, "pair", "bd", "source_commit")) {
		return errors.New("toolchain receipt source provenance is not a qualified lock pair")
	}
	return nil
}

func verifyNativeAgentOpsReceipts(native NativeContext, bound map[string]string, receipt map[string]any) error {
	for _, name := range []string{"ao", "agentops-gc-delivery"} {
		receiptPath := runtimeReceiptPath(native.ToolchainReceipt, receipt, name)
		digest := stringAt(receipt, "runtime", name, "sha256")
		if err := fileHasDigest(receiptPath, digest); err != nil {
			return errors.New("toolchain receipt does not bind native AgentOps reducer bytes")
		}
		if name == "agentops-gc-delivery" && (receiptPath != bound[name] || digest != native.Executables[name].Digest) {
			return errors.New("toolchain receipt does not bind native AgentOps reducer bytes")
		}
	}
	if source, tree := stringAt(receipt, "runtime", "ao", "source_commit"), stringAt(receipt, "runtime", "ao", "cli_tree"); len(source) != 40 || len(tree) != 40 || source != stringAt(receipt, "runtime", "agentops-gc-delivery", "source_commit") || tree != stringAt(receipt, "runtime", "agentops-gc-delivery", "cli_tree") {
		return errors.New("toolchain receipt does not bind one AgentOps source and CLI tree")
	}
	return nil
}

func runtimeReceiptPath(receiptPath string, receipt map[string]any, name string) string {
	path := stringAt(receipt, "runtime", name, "path")
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(receiptPath), path)
}

func numberAt(object map[string]any, keys ...string) int {
	var current any = object
	for _, key := range keys {
		mapped, ok := current.(map[string]any)
		if !ok {
			return -1
		}
		current = mapped[key]
	}
	value, ok := current.(float64)
	if !ok || value != float64(int(value)) {
		return -1
	}
	return int(value)
}

func exactObjectKeys(object map[string]any, key string, expected []string) bool {
	mapped, ok := object[key].(map[string]any)
	if !ok || len(mapped) != len(expected) {
		return false
	}
	for _, item := range expected {
		if _, ok := mapped[item]; !ok {
			return false
		}
	}
	return true
}

func fileHasDigest(path, want string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(contents)
	if fmt.Sprintf("%x", sum) != want {
		return errors.New("digest does not match exact bytes")
	}
	return nil
}

func jsonObject(path string) (map[string]any, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(contents, &object); err != nil {
		return nil, err
	}
	return object, nil
}

func stringAt(object map[string]any, keys ...string) string {
	var current any = object
	for _, key := range keys {
		mapped, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = mapped[key]
	}
	value, _ := current.(string)
	return value
}

func lockHasQualifiedPair(lock map[string]any, gc, bd string) bool {
	pairs, ok := lock["accepted_pairs"].([]any)
	if !ok {
		return false
	}
	for _, item := range pairs {
		pair, ok := item.(map[string]any)
		if ok && stringAt(pair, "status") == "qualified" && stringAt(pair, "gc", "source_commit") == gc && stringAt(pair, "bd", "source_commit") == bd {
			return true
		}
	}
	return false
}

func pathsOverlap(first, second string) (bool, error) {
	first, err := filepath.EvalSymlinks(first)
	if err != nil {
		return false, err
	}
	second, err = filepath.EvalSymlinks(second)
	if err != nil {
		return false, err
	}
	for _, pair := range [][2]string{{first, second}, {second, first}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err != nil {
			return false, err
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return true, nil
		}
	}
	return false, nil
}

func (p *NativeProviders) verifyRepository(ctx context.Context) error {
	top, err := p.git(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	want, err := filepath.EvalSymlinks(p.context.RepositoryDir)
	if err != nil {
		return err
	}
	got, err := filepath.EvalSymlinks(top)
	if err != nil {
		return err
	}
	if got != want {
		return errors.New("native context repository directory is not the Git root")
	}
	remoteURL, err := p.git(ctx, "remote", "get-url", p.context.Remote)
	if err != nil {
		return errors.New("native context remote is unavailable")
	}
	if !remoteMatchesRepository(remoteURL, p.context.Repository) {
		return errors.New("native context remote does not match the exact repository")
	}
	return nil
}

func remoteMatchesRepository(remote, repository string) bool {
	for _, prefix := range []string{"https://github.com/", "ssh://git@github.com/", "git@github.com:"} {
		if strings.HasPrefix(remote, prefix) {
			name := strings.TrimSuffix(strings.TrimPrefix(remote, prefix), ".git")
			return name == repository
		}
	}
	return false
}

func (p *NativeProviders) changedPaths(ctx context.Context, parent, candidate string) (map[string]bool, error) {
	output, err := p.git(ctx, "diff-tree", "--no-commit-id", "--name-status", "-r", parent, candidate)
	if err != nil {
		return nil, err
	}
	paths := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 2 || strings.HasPrefix(fields[0], "R") || strings.HasPrefix(fields[0], "C") || !safeRelativePath(fields[1], false) {
			return nil, errors.New("candidate diff contains unsupported rename or path")
		}
		switch fields[0] {
		case "A", "M", "T":
			paths[fields[1]] = false
		case "D":
			paths[fields[1]] = true
		default:
			return nil, errors.New("candidate diff has unsupported status")
		}
	}
	return paths, nil
}

func (p *NativeProviders) verifyEntry(ctx context.Context, commit string, entry ManifestEntry) error {
	output, err := p.git(ctx, "ls-tree", commit, "--", entry.Path)
	if err != nil || output == "" {
		return errors.New("manifest entry does not exist in candidate")
	}
	parts := strings.Fields(output)
	if len(parts) < 3 {
		return errors.New("manifest entry mode differs from candidate")
	}
	mode := parts[0]
	if (entry.Kind == "file" && mode != "100644" && mode != "100755") || (entry.Kind == "symlink" && mode != "120000") || (entry.Executable != (mode == "100755")) {
		return errors.New("manifest entry kind or executable mode differs from candidate")
	}
	bytes, err := runNative(ctx, p.context.RepositoryDir, p.binaries.Git, nil, "show", commit+":"+entry.Path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(bytes)
	if fmt.Sprintf("%x", sum) != entry.Digest {
		return errors.New("manifest entry bytes differ from candidate")
	}
	return nil
}

func (p *NativeProviders) git(ctx context.Context, args ...string) (string, error) {
	output, err := p.gitBytes(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}

func (p *NativeProviders) gitBytes(ctx context.Context, args ...string) ([]byte, error) {
	output, err := runNative(ctx, p.context.RepositoryDir, p.binaries.Git, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("native git %s: %w", args[0], err)
	}
	return output, nil
}

type boundedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	if b.Len()+len(value) > nativeOutputLimit {
		remaining := nativeOutputLimit - b.Len()
		if remaining > 0 {
			_, _ = b.Buffer.Write(value[:remaining])
		}
		b.overflow = true
		return 0, errors.New("native command output exceeded limit")
	}
	return b.Buffer.Write(value)
}

func runNative(ctx context.Context, directory, binary string, input []byte, args ...string) ([]byte, error) {
	return runNativeEnv(ctx, directory, binary, input, nil, args...)
}

func runNativeEnv(ctx context.Context, directory, binary string, input []byte, additions []string, args ...string) ([]byte, error) {
	bounded, cancel := context.WithTimeout(ctx, nativeCommandTimeout)
	defer cancel()
	command := exec.CommandContext(bounded, binary, args...)
	command.Dir = directory
	command.Env = append(os.Environ(), additions...)
	command.Stdin = bytes.NewReader(input)
	var stdout, stderr boundedBuffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if stdout.overflow || stderr.overflow {
		return nil, errors.New("native command output exceeded limit")
	}
	if bounded.Err() != nil {
		return nil, fmt.Errorf("native command deadline: %w", bounded.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

type bdRecord struct {
	ID          string         `json:"id"`
	Status      string         `json:"status"`
	CreatedAt   string         `json:"created_at"`
	ExternalRef string         `json:"external_ref"`
	Metadata    map[string]any `json:"metadata"`
}
type ReadyDelivery struct {
	ID, ReadyAt, RequestPath, RequestDigest string
}

func (p *NativeProviders) bd(ctx context.Context, args ...string) ([]bdRecord, error) {
	output, err := runNative(ctx, p.context.RepositoryDir, p.binaries.Beads, nil, args...)
	if err != nil {
		return nil, err
	}
	var records []bdRecord
	if err := json.Unmarshal(output, &records); err != nil {
		return nil, err
	}
	return records, nil
}
func metadataString(record bdRecord, key string) string {
	value, _ := record.Metadata[key].(string)
	return value
}
func (p *NativeProviders) bead(ctx context.Context, id string) (bdRecord, bool, error) {
	records, err := p.bd(ctx, "list", "--all", "--id", id, "--json")
	if err != nil || len(records) == 0 {
		return bdRecord{}, false, err
	}
	if len(records) != 1 || records[0].ID != id {
		return bdRecord{}, false, errors.New("bd identity response is ambiguous")
	}
	return records[0], true, nil
}
func (p *NativeProviders) Terminal(ctx context.Context, id string) (Terminal, error) {
	record, found, err := p.bead(ctx, id)
	if err != nil || !found {
		return Terminal{}, errors.New("terminal bead is absent")
	}
	var terminal struct {
		Verdict           string `json:"verdict"`
		CertificateDigest string `json:"certificate_digest"`
	}
	if err := json.Unmarshal([]byte(metadataString(record, "gc33.terminal")), &terminal); err != nil {
		return Terminal{}, err
	}
	return Terminal{BeadID: record.ID, Ref: "beads:" + record.ID + "#gc33.terminal", Verdict: terminal.Verdict, CertificateDigest: terminal.CertificateDigest}, nil
}
func (p *NativeProviders) FindDelivery(ctx context.Context, id string) (DeliveryBead, bool, error) {
	record, found, err := p.bead(ctx, id)
	if err != nil || !found {
		return DeliveryBead{}, found, err
	}
	bead, err := deliveryFromBDRecord(record)
	return bead, err == nil, err
}

func deliveryFromBDRecord(record bdRecord) (DeliveryBead, error) {
	encoded := metadataString(record, "gc.delivery.v1")
	if encoded == "" || metadataString(record, "gc.kind") != "delivery" {
		return DeliveryBead{}, errors.New("delivery bead lacks gc.delivery.v1")
	}
	var delivery DeliveryRecord
	if err := decodeStrict([]byte(encoded), &delivery); err != nil {
		return DeliveryBead{}, err
	}
	canonical, err := verdictcheck.CanonicalJSON(delivery)
	if err != nil || string(canonical) != encoded || delivery.SchemaVersion != "gc.delivery.v1" {
		return DeliveryBead{}, errors.New("delivery bead has non-canonical gc.delivery.v1")
	}
	if err := validDeliveryRecord(delivery); err != nil {
		return DeliveryBead{}, err
	}
	return DeliveryBead{ID: record.ID, ExternalRef: record.ExternalRef, Route: metadataString(record, "gc.routed_to"), Record: delivery}, nil
}
func (p *NativeProviders) CreateDelivery(ctx context.Context, expected DeliveryBead) (DeliveryBead, error) {
	if err := validDeliveryRecord(expected.Record); err != nil {
		return DeliveryBead{}, err
	}
	if found, ok, err := p.FindDelivery(ctx, expected.ID); err != nil || ok {
		if err != nil {
			return DeliveryBead{}, err
		}
		return found, nil
	}
	encoded, err := verdictcheck.CanonicalJSON(expected.Record)
	if err != nil {
		return DeliveryBead{}, err
	}
	requestEnvelope, err := p.createRequestEnvelope(expected)
	if err != nil {
		return DeliveryBead{}, err
	}
	metadata, err := verdictcheck.CanonicalJSON(map[string]string{"gc.kind": "delivery", "gc.delivery.v1": string(encoded), "gc.delivery_request": requestEnvelope})
	if err != nil {
		return DeliveryBead{}, err
	}
	// bd v1.1.0 creates the typed metadata atomically with the deterministic
	// external reference; no blank successor is ever observable.
	if _, err := runNative(ctx, p.context.RepositoryDir, p.binaries.Beads, nil, "create", "non-routable delivery successor", "--id", expected.ID, "--force", "--silent", "--external-ref", expected.ExternalRef, "--metadata", string(metadata)); err != nil {
		return DeliveryBead{}, err
	}
	created, ok, err := p.FindDelivery(ctx, expected.ID)
	if err != nil || !ok || created != expected {
		return DeliveryBead{}, errors.New("delivery create was not observable")
	}
	return created, nil
}
func (p *NativeProviders) StoreTransition(ctx context.Context, observed DeliveryBead, next DeliveryRecord) (DeliveryBead, error) {
	if err := validDeliveryRecord(next); err != nil {
		return DeliveryBead{}, err
	}
	current, found, err := p.FindDelivery(ctx, observed.ID)
	if err != nil || !found || current != observed {
		return DeliveryBead{}, errors.New("delivery transition has stale observed delivery.v1")
	}
	if next.SchemaVersion != "gc.delivery.v1" || next.Revision != current.Record.Revision+1 || next.HandoffID != current.Record.HandoffID || next.Epoch.Number != current.Record.Epoch.Number || next.Epoch.BaseRef != current.Record.Epoch.BaseRef || next.Epoch.BaseOID != current.Record.Epoch.BaseOID || next.Epoch.Branch != current.Record.Epoch.Branch {
		return DeliveryBead{}, errors.New("delivery transition changes immutable identity")
	}
	encoded, err := verdictcheck.CanonicalJSON(next)
	if err != nil {
		return DeliveryBead{}, err
	}
	if _, err := runNative(ctx, p.context.RepositoryDir, p.binaries.Beads, nil, "update", observed.ID, "--set-metadata", "gc.delivery.v1="+string(encoded)); err != nil {
		return DeliveryBead{}, err
	}
	stored, found, err := p.FindDelivery(ctx, observed.ID)
	if err != nil || !found || stored.Record != next {
		return DeliveryBead{}, errors.New("delivery transition was not reread exactly")
	}
	return stored, nil
}
func (p *NativeProviders) BaseDescends(ctx context.Context, descendant, ancestor string) (bool, error) {
	if err := p.ensureCommitObject(ctx, descendant); err != nil {
		return false, err
	}
	if err := p.ensureCommitObject(ctx, ancestor); err != nil {
		return false, err
	}
	base, err := p.git(ctx, "merge-base", ancestor, descendant)
	if err != nil {
		return false, err
	}
	return base == ancestor, nil
}
func (p *NativeProviders) ObserveBase(ctx context.Context, ref string) (string, error) {
	oid, err := p.remoteOID(ctx, ref)
	if err != nil {
		return "", err
	}
	if err := p.ensureCommitObject(ctx, oid); err != nil {
		return "", err
	}
	return oid, nil
}
func (p *NativeProviders) PublishRoute(ctx context.Context, id string) error {
	bead, found, err := p.FindDelivery(ctx, id)
	if err != nil || !found || bead.Record.Publication != "published" || !isHex(bead.Record.Committed, 64) {
		return errors.New("route publication lacks committed delivery.v1 envelope")
	}
	record, found, err := p.bead(ctx, id)
	encoded := metadataString(record, "gc.delivery_request")
	if err != nil || !found || encoded == "" {
		return errors.New("route publication lacks its exact immutable request envelope")
	}
	if _, err := runNative(ctx, p.context.RepositoryDir, p.binaries.Beads, nil, "update", id, "--set-metadata", "gc.kind=delivery", "--set-metadata", "gc.routed_to=agentops.delivery"); err != nil {
		return err
	}
	record, found, err = p.bead(ctx, id)
	if err != nil || !found || metadataString(record, "gc.kind") != "delivery" || metadataString(record, "gc.routed_to") != "agentops.delivery" || metadataString(record, "gc.delivery_request") != encoded {
		return errors.New("delivery publication metadata did not bind exact request")
	}
	if _, ok, err := p.FindDelivery(ctx, id); err != nil || !ok {
		return errors.New("route publication did not reread exact delivery.v1")
	}
	return nil
}

func (p *NativeProviders) createRequestEnvelope(expected DeliveryBead) (string, error) {
	request, err := p.requestForDelivery(expected)
	if err != nil {
		return "", err
	}
	if err := p.writeDeliveryRequest(expected.Record, request); err != nil {
		return "", err
	}
	path := requestRefPath(expected.Record)
	fullPath, err := evidencePath(request.Root, path)
	if err != nil {
		return "", err
	}
	bytes, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes)
	ref := deliveryRequestRef{SchemaVersion: "gc.delivery.request-ref.v1", Path: path, Digest: fmt.Sprintf("%x", sum)}
	encoded, err := verdictcheck.CanonicalJSON(ref)
	return string(encoded), err
}

func (p *NativeProviders) requestForDelivery(expected DeliveryBead) (Request, error) {
	if p.requests == nil {
		return Request{}, errors.New("delivery request material was not prevalidated")
	}
	if request, ok := p.requests[expected.ID]; ok {
		return requestForRecord(request, expected.Record), nil
	}
	var source Request
	found := false
	for _, request := range p.requests {
		probe := request
		probe.Target.Epoch = 1
		if makePrepared(probe).HandoffID != expected.Record.HandoffID {
			continue
		}
		if found {
			return Request{}, errors.New("delivery request material is ambiguous for successor")
		}
		source, found = request, true
	}
	if !found {
		return Request{}, errors.New("delivery request material is absent for selected bead")
	}
	return requestForRecord(source, expected.Record), nil
}

func requestForRecord(request Request, record DeliveryRecord) Request {
	request.Target.DeliveryBeadID = ""
	request.Target.SemanticBeadID = record.SemanticBead
	request.Target.SemanticTerminalRef = record.TerminalRef
	request.Target.RigID, request.Target.Repository, request.Target.Remote = record.Rig, record.Repository, record.Remote
	request.Target.Epoch, request.Target.Mode = record.Epoch.Number, record.Mode
	request.Target.Deadline, request.Target.PreparedAt = record.Deadline, record.ReadyAt
	request.Target.BaseRef, request.Target.BaseOID = record.Epoch.BaseRef, record.Epoch.BaseOID
	return request
}

func deliveryRequestFor(record DeliveryRecord, request Request) deliveryRequest {
	prefix := filepath.Join("handoffs", record.HandoffID)
	return deliveryRequest{SchemaVersion: "delivery-request.v1", CertificateRef: filepath.Join(prefix, "certificate.json"), CertificateDigest: request.CertificateDigest, SubjectRef: filepath.Join(prefix, "subject-manifest.json"), SubjectDigest: request.SubjectDigest, NativeRef: filepath.Join(prefix, "native-context.json"), NativeDigest: request.NativeDigest, SemanticBeadID: record.SemanticBead, SemanticTerminalRef: record.TerminalRef, RigID: record.Rig, Repository: record.Repository, Remote: record.Remote, Epoch: record.Epoch.Number, Mode: record.Mode, Deadline: record.Deadline, PreparedAt: record.ReadyAt, CommittedAt: request.Target.CommittedAt, BaseRef: record.Epoch.BaseRef, BaseOID: record.Epoch.BaseOID}
}

func (p *NativeProviders) writeDeliveryRequest(record DeliveryRecord, request Request) error {
	if request.Root == "" || !filepath.IsAbs(request.Root) || len(request.CertificateBytes) == 0 || len(request.SubjectBytes) == 0 || len(request.NativeBytes) == 0 {
		return errors.New("delivery request material lacks exact evidence bytes")
	}
	prefix := filepath.Join("handoffs", record.HandoffID)
	state := markerStore{root: request.Root, prefix: prefix}
	if err := state.matchesBytes("certificate.json", request.CertificateBytes); err != nil {
		return err
	}
	if err := state.writeBytesImmutable("subject-manifest.json", request.SubjectBytes); err != nil {
		return err
	}
	if err := state.writeBytesImmutable("native-context.json", request.NativeBytes); err != nil {
		return err
	}
	wire := deliveryRequestFor(record, request)
	encoded, err := verdictcheck.CanonicalJSON(wire)
	if err != nil {
		return err
	}
	epoch := markerStore{root: request.Root, prefix: filepath.Dir(requestRefPath(record))}
	return epoch.writeBytesImmutable(filepath.Base(requestRefPath(record)), encoded)
}
func (p *NativeProviders) RetireRoute(ctx context.Context, id string) error {
	bead, found, err := p.FindDelivery(ctx, id)
	if err != nil || !found || bead.Route != "agentops.delivery" {
		return errors.New("route retirement lacks an active exact delivery")
	}
	if _, err := runNative(ctx, p.context.RepositoryDir, p.binaries.Beads, nil, "update", id, "--set-metadata", "gc.routed_to="); err != nil {
		return err
	}
	record, found, err := p.bead(ctx, id)
	if err != nil || !found || metadataString(record, "gc.routed_to") != "" {
		return errors.New("route retirement did not reread empty route")
	}
	return nil
}

// RecordSweepFailure is intentionally native-only.  A deterministic bad
// request is visible in the Beads-owned delivery state; cancellation or
// process death never calls this method and therefore records nothing.
func (p *NativeProviders) RecordSweepFailure(ctx context.Context, root, id string, cause error) error {
	bead, found, err := p.FindDelivery(ctx, id)
	if err != nil || !found {
		return errors.New("delivery sweep failure could not reread selected bead")
	}
	if bead.Record.Publication != "published" || bead.Record.State == DeliveryStateStalled || bead.Record.State == DeliveryStateFailed || bead.Record.State == DeliveryStateCancelled || bead.Record.State == DeliveryStateLanded {
		return nil
	}
	if root == "" || !filepath.IsAbs(root) {
		return errors.New("delivery sweep failure has no controller evidence root")
	}
	state := markerStore{root: root, prefix: receiptNamespaceFor(bead.Record)}
	receipt := DeliveryOutcomeReceipt{SchemaVersion: "delivery-outcome-receipt.v1", HandoffID: bead.Record.HandoffID, Epoch: bead.Record.Epoch.Number, State: DeliveryStateStalled, Reason: "sweep_prevalidation_failed"}
	if err := state.writeImmutable("delivery-outcome.json", receipt); err != nil {
		return err
	}
	want := bead.Record
	want.State, want.Current, want.DeliveryOutcome, want.Revision = DeliveryStateStalled, receiptRef("delivery-outcome", state), receipt.Reason, bead.Record.Revision+1
	if err := validDeliveryRecord(want); err != nil {
		return err
	}
	_, err = p.StoreTransition(ctx, bead, want)
	return err
}
func (p *NativeProviders) ReadyDeliveries(ctx context.Context, limit int) ([]ReadyDelivery, error) {
	if limit < 1 || limit > 8 {
		return nil, errors.New("delivery sweep limit is out of bounds")
	}
	const scanLimit = 128
	records, err := p.bd(ctx, "ready", "--metadata-field", "gc.kind=delivery", "--unassigned", "--exclude-type=epic", "--json", "--sort", "oldest", "--limit", fmt.Sprint(scanLimit))
	if err != nil {
		return nil, err
	}
	if len(records) == scanLimit {
		return nil, errors.New("delivery sweep scan is truncated")
	}
	return selectReadyDeliveries(records, limit)
}

type readyDeliveryCandidate struct {
	record   bdRecord
	bead     DeliveryBead
	envelope string
}

func selectReadyDeliveries(records []bdRecord, limit int) ([]ReadyDelivery, error) {
	byHandoff := map[string][]readyDeliveryCandidate{}
	for _, record := range records {
		candidate, err := readyDeliveryCandidateFromRecord(record)
		if err != nil {
			return nil, err
		}
		byHandoff[candidate.bead.Record.HandoffID] = append(byHandoff[candidate.bead.Record.HandoffID], candidate)
	}
	ready := make([]ReadyDelivery, 0, len(byHandoff))
	for _, chain := range byHandoff {
		selected, ok, err := selectReadyDelivery(chain)
		if err != nil {
			return nil, err
		}
		if ok {
			readyAt := selected.bead.Record.ReadyAt
			if _, err := time.Parse(time.RFC3339, readyAt); err != nil {
				return nil, errors.New("ready delivery has no schema ready_at")
			}
			var ref deliveryRequestRef
			if err := decodeStrict([]byte(selected.envelope), &ref); err != nil || ref.SchemaVersion != "gc.delivery.request-ref.v1" || !safeRelativePath(ref.Path, false) || !isHex(ref.Digest, 64) {
				return nil, errors.New("ready delivery has invalid request reference")
			}
			ready = append(ready, ReadyDelivery{ID: selected.record.ID, ReadyAt: readyAt, RequestPath: ref.Path, RequestDigest: ref.Digest})
		}
	}
	sort.SliceStable(ready, func(i, j int) bool {
		if ready[i].ReadyAt != ready[j].ReadyAt {
			return ready[i].ReadyAt < ready[j].ReadyAt
		}
		return ready[i].ID < ready[j].ID
	})
	if len(ready) > limit {
		ready = ready[:limit]
	}
	return ready, nil
}

func readyDeliveryCandidateFromRecord(record bdRecord) (readyDeliveryCandidate, error) {
	bead, err := deliveryFromBDRecord(record)
	if err != nil {
		return readyDeliveryCandidate{}, err
	}
	envelope := metadataString(record, "gc.delivery_request")
	var identity deliveryRequestRef
	if err := decodeStrict([]byte(envelope), &identity); err != nil || identity.SchemaVersion != "gc.delivery.request-ref.v1" || !safeRelativePath(identity.Path, false) || !isHex(identity.Digest, 64) {
		return readyDeliveryCandidate{}, errors.New("ready delivery has invalid request envelope")
	}
	canonical, err := verdictcheck.CanonicalJSON(identity)
	if err != nil || string(canonical) != envelope {
		return readyDeliveryCandidate{}, errors.New("ready delivery request or ready time is non-canonical")
	}
	return readyDeliveryCandidate{record: record, bead: bead, envelope: envelope}, nil
}

func selectReadyDelivery(chain []readyDeliveryCandidate) (readyDeliveryCandidate, bool, error) {
	sort.Slice(chain, func(i, j int) bool { return chain[i].bead.Record.Epoch.Number < chain[j].bead.Record.Epoch.Number })
	if err := validReadyDeliveryChain(chain); err != nil {
		return readyDeliveryCandidate{}, false, err
	}
	leaf := chain[len(chain)-1]
	if leaf.bead.Record.EpochSuccessorID != "" && !recoverableAbsentSuccessor(leaf.bead) {
		return readyDeliveryCandidate{}, false, errors.New("delivery handoff leaf references an absent successor")
	}
	selected := leaf
	if leaf.bead.Record.Publication == "pending" && len(chain) > 1 {
		selected = chain[len(chain)-2]
	}
	return selected, selectableDeliveryState(selected.bead.Record.State), nil
}

func validReadyDeliveryChain(chain []readyDeliveryCandidate) error {
	for index := range chain {
		if chain[index].bead.Record.Epoch.Number != index+1 {
			return errors.New("delivery handoff has a non-contiguous epoch chain")
		}
		if index == 0 {
			if chain[index].bead.Record.Predecessor != "" {
				return errors.New("delivery epoch one has a predecessor")
			}
			continue
		}
		parent, child := chain[index-1].bead, chain[index].bead
		if child.Record.Predecessor != parent.ID || parent.Record.EpochSuccessorID != child.ID {
			return errors.New("delivery handoff successor chain is ambiguous")
		}
	}
	return nil
}

// The predecessor link is persisted before the child is atomically created.
// During that one crash window the linked predecessor remains the sole leaf.
// Any other absent-successor shape is inconsistent and remains fail-closed.
func recoverableAbsentSuccessor(bead DeliveryBead) bool {
	record := bead.Record
	wantID := "delivery-" + record.HandoffID[:20] + fmt.Sprintf("-e%06d", record.Epoch.Number+1)
	epoch, name, ok := deliveryReceiptLocation(record, record.Current)
	return record.EpochSuccessorID == wantID && record.State == DeliveryStateRebaseNeeded && record.Publication == "published" && epoch == record.Epoch.Number && name == "base-move.json" && ok
}

func selectableDeliveryState(state DeliveryState) bool {
	switch state {
	case DeliveryStateLanded, DeliveryStateFailed, DeliveryStateCancelled, DeliveryStateStalled, DeliveryStateRepairWait, DeliveryStateSuccessorRequired:
		return false
	default:
		return true
	}
}
func (p *NativeProviders) FindBranch(ctx context.Context, name string) (Branch, bool, error) {
	head, found, err := p.remoteOIDOptional(ctx, "refs/heads/"+name)
	if err != nil || !found {
		return Branch{}, found, err
	}
	return Branch{Name: name, Head: head}, true, nil
}
func (p *NativeProviders) PrepareBranch(ctx context.Context, planned Branch) (branch Branch, err error) {
	parent, err := p.validateBranchPlan(ctx, planned)
	if err != nil {
		return Branch{}, err
	}
	worktree, err := p.prepareBranchWorktree(ctx, planned)
	if err != nil {
		return Branch{}, err
	}
	defer func() {
		if cleanupErr := p.cleanupWorktree(worktree); err == nil && cleanupErr != nil {
			branch, err = Branch{}, cleanupErr
		}
	}()
	return p.composePreparedBranch(ctx, worktree, planned, parent)
}

func (p *NativeProviders) validateBranchPlan(ctx context.Context, planned Branch) (string, error) {
	if planned.Head != p.candidate || planned.Proof == "" {
		return "", errors.New("epoch composition lacks the verified admitted candidate")
	}
	base, err := p.remoteOID(ctx, planned.BaseRef)
	if err != nil {
		return "", err
	}
	if base != planned.BaseOID {
		return "", errTargetRegression
	}
	if err := p.ensureCommitObject(ctx, base); err != nil {
		return "", err
	}
	if existing, exists, err := p.remoteOIDOptional(ctx, "refs/heads/"+planned.Name); err != nil {
		return "", err
	} else if planned.LeaseOID == "" && exists {
		return "", errors.New("delivery branch appeared during epoch composition")
	} else if planned.LeaseOID != "" && (!exists || existing != planned.LeaseOID) {
		return "", errors.New("delivery branch does not match exact epoch lease")
	}
	parentLine, err := p.git(ctx, "rev-list", "--parents", "-n", "1", planned.Head)
	if err != nil {
		return "", err
	}
	parts := strings.Fields(parentLine)
	if len(parts) != 2 {
		return "", errors.New("epoch candidate must have one parent")
	}
	if collide, err := p.hasPathCollision(ctx, parts[1], planned.Head, planned.BaseOID); err != nil {
		return "", err
	} else if collide {
		return "", errPathCollision
	}
	return parts[1], nil
}

func (p *NativeProviders) prepareBranchWorktree(ctx context.Context, planned Branch) (string, error) {
	worktree, err := nativeWorktreePath(p.context.WorktreeRoot, planned)
	if err != nil {
		return "", err
	}
	// A prior process may have died after worktree creation. When Git still
	// registers this exact plan path but its directory is gone, re-materialize
	// only that registration at the immutable planned base before exact cleanup.
	exactWorktree, registered, err := p.registeredWorktreePath(worktree)
	if err != nil {
		return "", err
	}
	if _, statErr := os.Lstat(worktree); errors.Is(statErr, os.ErrNotExist) && registered {
		if _, err := runNative(ctx, p.context.RepositoryDir, p.binaries.Git, nil, "worktree", "add", "--force", "--detach", exactWorktree, planned.BaseOID); err != nil {
			return "", err
		}
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}
	if err := p.cleanupWorktree(worktree); err != nil {
		return "", err
	}
	if _, err := runNative(ctx, p.context.RepositoryDir, p.binaries.Git, nil, "worktree", "add", "--detach", worktree, planned.BaseOID); err != nil {
		// Preserve the original failure-path guarantee: worktree add can leave a
		// partial registration or directory even when it returns an error.
		_ = p.cleanupWorktree(worktree)
		return "", err
	}
	return worktree, nil
}

func (p *NativeProviders) composePreparedBranch(ctx context.Context, worktree string, planned Branch, parent string) (Branch, error) {
	patch, err := p.gitBytes(ctx, "diff", "--binary", parent, planned.Head)
	if err != nil {
		return Branch{}, err
	}
	if len(bytes.TrimSpace(patch)) == 0 {
		return Branch{}, errZeroDiff
	}
	if _, err := runNative(ctx, worktree, p.binaries.Git, patch, "apply", "--index", "--whitespace=error"); err != nil {
		return Branch{}, err
	}
	if _, err := runNative(ctx, worktree, p.binaries.Git, nil, "diff", "--cached", "--check"); err != nil {
		return Branch{}, err
	}
	changed, err := runNative(ctx, worktree, p.binaries.Git, nil, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return Branch{}, err
	}
	if len(changed) == 0 {
		return Branch{}, errZeroDiff
	}
	snapshot, err := worktreeSnapshot(ctx, worktree, p.binaries.Git)
	if err != nil {
		return Branch{}, err
	}
	for _, gate := range p.context.CheckOnlyGateArgv {
		if _, err := runNative(ctx, worktree, gate[0], nil, gate[1:]...); err != nil {
			return Branch{}, fmt.Errorf("check-only gate: %w", err)
		}
		after, err := worktreeSnapshot(ctx, worktree, p.binaries.Git)
		if err != nil {
			return Branch{}, err
		}
		if after != snapshot {
			return Branch{}, errors.New("check-only gate mutated the isolated worktree")
		}
	}
	tree, err := runNative(ctx, worktree, p.binaries.Git, nil, "write-tree")
	if err != nil {
		return Branch{}, err
	}
	metadata, err := p.gitBytes(ctx, "show", "-s", "--format=%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI", planned.Head)
	if err != nil {
		return Branch{}, err
	}
	parts := strings.Split(strings.TrimSuffix(string(metadata), "\n"), "\x00")
	if len(parts) != 6 || hasEmpty(parts) {
		return Branch{}, errors.New("candidate has incomplete immutable identity metadata")
	}
	env := []string{"GIT_AUTHOR_NAME=" + parts[0], "GIT_AUTHOR_EMAIL=" + parts[1], "GIT_AUTHOR_DATE=" + parts[2], "GIT_COMMITTER_NAME=" + parts[3], "GIT_COMMITTER_EMAIL=" + parts[4], "GIT_COMMITTER_DATE=" + parts[5]}
	epoch, err := runNativeEnv(ctx, worktree, p.binaries.Git, nil, env, "commit-tree", strings.TrimSpace(string(tree)), "-p", planned.BaseOID, "-m", "agentops delivery epoch "+planned.Proof)
	if err != nil {
		return Branch{}, err
	}
	epochOID := strings.TrimSpace(string(epoch))
	if !isHex(epochOID, 40) {
		return Branch{}, errors.New("epoch composition did not return a commit")
	}
	composedTree, err := p.git(ctx, "rev-parse", epochOID+"^{tree}")
	if err != nil || !isHex(composedTree, 40) {
		return Branch{}, errors.New("epoch composition tree is invalid")
	}
	if err := p.verifyComposedManifest(ctx, epochOID); err != nil {
		return Branch{}, err
	}
	return Branch{Name: planned.Name, BaseRef: planned.BaseRef, BaseOID: planned.BaseOID, Head: epochOID, Tree: composedTree, Proof: planned.Proof, LeaseOID: planned.LeaseOID}, nil
}

func nativeWorktreePath(root string, planned Branch) (string, error) {
	if !filepath.IsAbs(root) {
		return "", errors.New("worktree root must be absolute")
	}
	root = filepath.Clean(root)
	plan := strings.Join([]string{planned.Name, planned.BaseRef, planned.BaseOID, planned.Head, planned.Tree, planned.Proof, planned.LeaseOID}, "\x00")
	sum := sha256.Sum256([]byte(plan))
	path := filepath.Join(root, "delivery-epoch-"+fmt.Sprintf("%x", sum[:12]))
	if !isWorktreeChild(root, path) {
		return "", errors.New("worktree path escapes configured root")
	}
	return path, nil
}

func worktreeSnapshot(ctx context.Context, worktree, git string) (string, error) {
	status, err := runNative(ctx, worktree, git, nil, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	tree, err := runNative(ctx, worktree, git, nil, "write-tree")
	if err != nil {
		return "", err
	}
	return string(status) + "\x00" + strings.TrimSpace(string(tree)), nil
}

// PushBranch is the one remote branch effect and is intentionally callable
// only after the reducer persisted the exact EpochReceipt.
func (p *NativeProviders) PushBranch(ctx context.Context, branch Branch) error {
	if existing, found, err := p.remoteOIDOptional(ctx, "refs/heads/"+branch.Name); err != nil {
		return err
	} else if found {
		if existing == branch.Head {
			return nil
		}
		if branch.LeaseOID == "" || existing != branch.LeaseOID {
			return errors.New("delivery branch conflicts with epoch receipt")
		}
	} else if branch.LeaseOID != "" {
		return errors.New("delivery branch is absent for epoch lease")
	}
	lease := strings.Repeat("0", 40)
	if branch.LeaseOID != "" {
		lease = branch.LeaseOID
	}
	if _, err := p.git(ctx, "push", p.context.Remote, branch.Head+":refs/heads/"+branch.Name, "--force-with-lease=refs/heads/"+branch.Name+":"+lease); err != nil {
		return err
	}
	if observed, err := p.remoteOID(ctx, "refs/heads/"+branch.Name); err != nil || observed != branch.Head {
		return errors.New("force-with-lease branch push did not land exact epoch head")
	}
	return nil
}

var (
	errTargetRegression = errors.New("target_regression")
	errPathCollision    = errors.New("path_collision")
	errZeroDiff         = errors.New("zero_diff")
)

func (p *NativeProviders) remoteOID(ctx context.Context, ref string) (string, error) {
	oid, found, err := p.remoteOIDOptional(ctx, ref)
	if err != nil {
		return "", err
	}
	if !found {
		return "", errors.New("remote ref is absent")
	}
	return oid, nil
}

func (p *NativeProviders) remoteOIDOptional(ctx context.Context, ref string) (string, bool, error) {
	output, err := p.git(ctx, "ls-remote", p.context.Remote, ref)
	if err != nil {
		return "", false, err
	}
	line := strings.TrimSpace(output)
	if line == "" {
		return "", false, nil
	}
	fields := strings.Fields(line)
	if len(fields) != 2 || !isHex(fields[0], 40) {
		return "", false, errors.New("remote ref response is invalid")
	}
	return fields[0], true, nil
}

// ensureCommitObject imports only the exact reachable commit object when a
// moving remote exposes an OID newer than the local clone. It writes neither a
// branch/tag ref nor FETCH_HEAD and never touches a worktree.
func (p *NativeProviders) ensureCommitObject(ctx context.Context, oid string) error {
	if !isHex(oid, 40) {
		return errors.New("remote commit object has invalid identity")
	}
	if _, err := p.git(ctx, "cat-file", "-e", oid+"^{commit}"); err == nil {
		return nil
	}
	if _, err := p.git(ctx, "fetch", "--no-tags", "--no-write-fetch-head", p.context.Remote, oid); err != nil {
		return fmt.Errorf("fetch exact remote commit object: %w", err)
	}
	resolved, err := p.git(ctx, "rev-parse", oid+"^{commit}")
	if err != nil || resolved != oid {
		return errors.New("exact remote commit object was not imported")
	}
	return nil
}

func (p *NativeProviders) hasPathCollision(ctx context.Context, parent, candidate, base string) (bool, error) {
	candidatePaths, err := p.changedPaths(ctx, parent, candidate)
	if err != nil {
		return false, err
	}
	targetPaths, err := p.changedPaths(ctx, parent, base)
	if err != nil {
		return false, err
	}
	for left := range candidatePaths {
		for right := range targetPaths {
			if left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/") {
				return true, nil
			}
		}
	}
	return false, nil
}

func (p *NativeProviders) cleanupWorktree(worktree string) error {
	if !isWorktreeChild(p.context.WorktreeRoot, worktree) {
		return errors.New("worktree cleanup escapes configured root")
	}
	exactWorktree, registered, err := p.registeredWorktreePath(worktree)
	if err != nil {
		return err
	}
	_, statErr := os.Lstat(worktree)
	missing := errors.Is(statErr, os.ErrNotExist)
	if missing && !registered {
		return nil
	} else if statErr != nil && !missing {
		return statErr
	}
	if registered {
		if _, err := runNative(context.Background(), p.context.RepositoryDir, p.binaries.Git, nil, "worktree", "remove", "--force", exactWorktree); err != nil && !missing {
			return err
		}
		_, registered, err = p.registeredWorktreePath(worktree)
		if err != nil {
			return err
		}
		if registered || missing {
			if _, err := runNative(context.Background(), p.context.RepositoryDir, p.binaries.Git, nil, "worktree", "remove", "--force", "--force", exactWorktree); err != nil {
				return err
			}
			if _, registered, err = p.registeredWorktreePath(worktree); err != nil {
				return err
			} else if registered {
				return errors.New("exact worktree removal left a registered path")
			}
		}
	}
	return os.RemoveAll(worktree)
}

func isWorktreeChild(root, path string) bool {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil || filepath.Clean(parent) != filepath.Clean(root) {
		return false
	}
	return !sameExistingPath(root, path) && filepath.Clean(path) == filepath.Join(filepath.Dir(path), filepath.Base(path))
}

func (p *NativeProviders) worktreeRegistered(worktree string) (bool, error) {
	_, registered, err := p.registeredWorktreePath(worktree)
	return registered, err
}

func (p *NativeProviders) registeredWorktreePath(worktree string) (string, bool, error) {
	output, err := runNative(context.Background(), p.context.RepositoryDir, p.binaries.Git, nil, "worktree", "list", "--porcelain")
	if err != nil {
		return "", false, err
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			registered := strings.TrimPrefix(line, "worktree ")
			if sameExistingPath(registered, worktree) {
				return registered, true, nil
			}
		}
	}
	return "", false, nil
}

func sameExistingPath(left, right string) bool {
	return canonicalExistingParentPath(left) == canonicalExistingParentPath(right)
}

func canonicalExistingParentPath(path string) string {
	if parent, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		return filepath.Join(filepath.Clean(parent), filepath.Base(path))
	}
	return filepath.Clean(path)
}
func (p *NativeProviders) gh(ctx context.Context, args ...string) ([]byte, error) {
	if p.ghRun != nil {
		return p.ghRun(ctx, args...)
	}
	return runNative(ctx, p.context.RepositoryDir, p.binaries.GH, nil, args...)
}
