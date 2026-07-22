package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

func TestNativeProviderBindsEntireReducerReceiptAndGateExecutable(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "repository")
	worktrees := filepath.Join(root, "worktrees")
	beads := filepath.Join(repository, ".beads")
	for _, directory := range []string{filepath.Join(repository, "deploy", "gc"), worktrees, beads, filepath.Join(root, "bin")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sourceRoot := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	for _, name := range []string{"toolchain.lock.json", "beads-capability-selection.v1.json"} {
		raw, err := os.ReadFile(filepath.Join(sourceRoot, "deploy", "gc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repository, "deploy", "gc", name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	paths := map[string]string{}
	bindings := map[string]ExecutableBinding{}
	for _, name := range []string{"gc", "bd", "ao", "git", "gh", "bash", "agentops-gc-delivery"} {
		path := filepath.Join(root, "bin", name)
		raw := []byte("#!/bin/sh\n# " + name + "\nexit 0\n")
		if err := os.WriteFile(path, raw, 0o700); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		paths[name] = path
		bindings[name] = ExecutableBinding{Path: path, Digest: fmt.Sprintf("%x", sum)}
	}
	lockRaw, err := os.ReadFile(filepath.Join(repository, "deploy", "gc", "toolchain.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lock map[string]any
	if err := json.Unmarshal(lockRaw, &lock); err != nil {
		t.Fatal(err)
	}
	pair := lock["accepted_pairs"].([]any)[0]
	receipt := map[string]any{
		"schema_version": 3,
		"pair":           pair,
		"runtime": map[string]any{
			"gc":                   map[string]any{"path": "bin/gc", "sha256": bindings["gc"].Digest},
			"bd":                   map[string]any{"path": "bin/bd", "sha256": bindings["bd"].Digest},
			"ao":                   map[string]any{"path": "bin/ao", "sha256": bindings["ao"].Digest, "source_commit": strings.Repeat("a", 40), "cli_tree": strings.Repeat("b", 40)},
			"agentops-gc-delivery": map[string]any{"path": "bin/agentops-gc-delivery", "sha256": bindings["agentops-gc-delivery"].Digest, "source_commit": strings.Repeat("a", 40), "cli_tree": strings.Repeat("b", 40)},
		},
	}
	receiptPath := filepath.Join(root, "toolchain.json")
	writeReceipt := func() string {
		t.Helper()
		raw, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, '\n')
		if err := os.WriteFile(receiptPath, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		return fmt.Sprintf("%x", sum)
	}
	native := NativeContext{
		SchemaVersion: "gc-delivery-native-context.v1", RigID: "agentops", Repository: "boshu2/agentops",
		RepositoryDir: repository, WorktreeRoot: worktrees, BeadsDir: beads, Remote: "origin", BaseRef: "main",
		SuccessorCapability: canonicalBeadsCapabilityDigest, ToolchainLock: canonicalToolchainLockDigest,
		ToolchainReceipt: receiptPath, ToolchainReceiptSum: writeReceipt(), BeadsRepresentation: "B-successor-delivery-bead",
		Executables:       map[string]ExecutableBinding{"gc": bindings["gc"], "bd": bindings["bd"], "git": bindings["git"], "gh": bindings["gh"], "bash": bindings["bash"], "agentops-gc-delivery": bindings["agentops-gc-delivery"]},
		CheckOnlyGateArgv: [][]string{{paths["bash"], "scripts/check-gc-executor.sh"}},
	}
	binaries := NativeBinaries{GC: paths["gc"], Beads: paths["bd"], Git: paths["git"], GH: paths["gh"], Bash: paths["bash"], Delivery: paths["agentops-gc-delivery"]}
	if _, err := NewNativeProviders(binaries, native); err != nil {
		t.Fatalf("valid provider identity: %v", err)
	}

	receipt["runtime"].(map[string]any)["ao"].(map[string]any)["sha256"] = strings.Repeat("0", 64)
	native.ToolchainReceiptSum = writeReceipt()
	if _, err := NewNativeProviders(binaries, native); err == nil || !strings.Contains(err.Error(), "AgentOps reducer bytes") {
		t.Fatalf("tampered ao receipt = %v", err)
	}
	receipt["runtime"].(map[string]any)["ao"].(map[string]any)["sha256"] = bindings["ao"].Digest
	receipt["runtime"].(map[string]any)["extra"] = map[string]any{}
	native.ToolchainReceiptSum = writeReceipt()
	if _, err := NewNativeProviders(binaries, native); err == nil || !strings.Contains(err.Error(), "exact schema-3 runtime") {
		t.Fatalf("expanded runtime receipt = %v", err)
	}

	delete(receipt["runtime"].(map[string]any), "extra")
	native.ToolchainReceiptSum = writeReceipt()
	native.CheckOnlyGateArgv = [][]string{{paths["git"], "status"}}
	raw, err := json.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if _, err := DecodeExactNativeContext(raw, fmt.Sprintf("%x", sum)); err == nil || !strings.Contains(err.Error(), "bound bash") {
		t.Fatalf("unbound gate executable = %v", err)
	}
}

func TestSelectReadyDeliveriesChoosesOneLeafAcrossRouteTransfer(t *testing.T) {
	handoff := strings.Repeat("a", 64)
	parentID, childID := "delivery-"+handoff[:20]+"-e000001", "delivery-"+handoff[:20]+"-e000002"
	baseRecord := func(epoch int) DeliveryRecord {
		return DeliveryRecord{
			SchemaVersion: "gc.delivery.v1", Revision: 4, HandoffID: handoff,
			Epoch: DeliveryEpoch{Number: epoch, BaseRef: "main", BaseOID: strings.Repeat(string(rune('a'+epoch)), 40), Branch: "gc/delivery/" + handoff[:20]},
			State: DeliveryStateRebaseNeeded, Current: ReceiptRef{Path: fmt.Sprintf("handoffs/%s/epochs/%06d/base-move.json", handoff, epoch), Digest: strings.Repeat("b", 64)},
			Publication: "published", ReadyAt: "2026-07-21T00:00:00Z", Deadline: "2026-07-22T00:00:00Z",
			SemanticBead: "semantic", TerminalRef: "beads:semantic#terminal", Certificate: strings.Repeat("c", 64), Committed: strings.Repeat("d", 64),
			Mode: "auto", Rig: "rig", Repository: "boshu2/agentops", Remote: "origin", Candidate: strings.Repeat("e", 40), Manifest: strings.Repeat("f", 64),
		}
	}
	parent := baseRecord(1)
	parent.Epoch.Head, parent.Epoch.Tree, parent.EpochSuccessorID = strings.Repeat("1", 40), strings.Repeat("2", 40), childID
	child := baseRecord(2)
	child.Revision, child.State, child.Publication, child.Current = 1, DeliveryStateQueued, "pending", ReceiptRef{}
	child.Predecessor, child.PredecessorReceiptDigest, child.Epoch.LeaseOID = parentID, parent.Current.Digest, parent.Epoch.Head
	child.Committed = parent.Committed

	parentRecord := readyBDRecord(t, parentID, "2026-07-21T00:00:00Z", "agentops.delivery", parent)
	childRecord := readyBDRecord(t, childID, "2026-07-21T00:00:01Z", "", child)
	ready, err := selectReadyDeliveries([]bdRecord{childRecord, parentRecord}, 2)
	if err != nil || len(ready) != 1 || ready[0].ID != parentID {
		t.Fatalf("pending child selection = %#v, %v", ready, err)
	}
	if ready[0].ReadyAt != parent.ReadyAt {
		t.Fatalf("ready ordering time = %q, want schema ready_at %q", ready[0].ReadyAt, parent.ReadyAt)
	}

	child.Revision, child.Publication = 2, "published"
	child.Current = ReceiptRef{Path: fmt.Sprintf("handoffs/%s/epochs/000002/activation.json", handoff), Digest: strings.Repeat("3", 64)}
	childRecord = readyBDRecord(t, childID, "2026-07-21T00:00:01Z", "", child)
	ready, err = selectReadyDeliveries([]bdRecord{parentRecord, childRecord}, 2)
	if err != nil || len(ready) != 1 || ready[0].ID != childID {
		t.Fatalf("published unrouted leaf selection = %#v, %v", ready, err)
	}
	childRecord.Metadata["gc.routed_to"] = "agentops.delivery"
	ready, err = selectReadyDeliveries([]bdRecord{parentRecord, childRecord}, 2)
	if err != nil || len(ready) != 1 || ready[0].ID != childID {
		t.Fatalf("dual-route observation selected more than leaf: %#v, %v", ready, err)
	}
}

func TestSelectReadyDeliveriesRecoversLinkedPredecessorBeforeChildCreation(t *testing.T) {
	handoff := strings.Repeat("a", 64)
	parentID, childID := "delivery-"+handoff[:20]+"-e000001", "delivery-"+handoff[:20]+"-e000002"
	parent := DeliveryRecord{
		SchemaVersion: "gc.delivery.v1", Revision: 5, HandoffID: handoff,
		Epoch: DeliveryEpoch{Number: 1, BaseRef: "main", BaseOID: strings.Repeat("a", 40), Branch: "gc/delivery/" + handoff[:20], Head: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40)},
		State: DeliveryStateRebaseNeeded, Current: ReceiptRef{Path: fmt.Sprintf("handoffs/%s/epochs/000001/base-move.json", handoff), Digest: strings.Repeat("d", 64)},
		Publication: "published", ReadyAt: "2026-07-21T00:00:00Z", Deadline: "2026-07-22T00:00:00Z",
		SemanticBead: "semantic", TerminalRef: "beads:semantic#terminal", Certificate: strings.Repeat("e", 64), Committed: strings.Repeat("f", 64),
		Mode: "auto", Rig: "rig", Repository: "boshu2/agentops", Remote: "origin", Candidate: strings.Repeat("1", 40), Manifest: strings.Repeat("2", 64),
		EpochSuccessorID: childID,
	}
	ready, err := selectReadyDeliveries([]bdRecord{readyBDRecord(t, parentID, "2026-07-21T00:00:00Z", "agentops.delivery", parent)}, 1)
	if err != nil || len(ready) != 1 || ready[0].ID != parentID {
		t.Fatalf("crash-window recovery selected %#v, %v", ready, err)
	}
}

func TestDeliveryStatusScansBlockedNonRoutableAndSuccessorChains(t *testing.T) {
	handoff := strings.Repeat("a", 64)
	parentID, childID := "delivery-"+handoff[:20]+"-e000001", "delivery-"+handoff[:20]+"-e000002"
	parent := statusRecord(handoff, 1, "2026-07-20T01:00:00Z")
	parent.State, parent.Current, parent.Epoch.Head, parent.Epoch.Tree, parent.EpochSuccessorID = DeliveryStateRebaseNeeded, ReceiptRef{Path: "handoffs/" + handoff + "/epochs/000001/base-move.json", Digest: strings.Repeat("b", 64)}, strings.Repeat("1", 40), strings.Repeat("2", 40), childID
	parent.Publication, parent.Committed = "published", strings.Repeat("f", 64)
	child := statusRecord(handoff, 2, "2026-07-21T01:00:00Z")
	child.Predecessor, child.PredecessorReceiptDigest, child.Epoch.LeaseOID, child.Committed = parentID, parent.Current.Digest, parent.Epoch.Head, parent.Committed

	parentBead := readyBDRecord(t, parentID, "2026-07-22T01:00:00Z", "agentops.delivery", parent)
	parentBead.Status = "blocked"
	childBead := readyBDRecord(t, childID, "2026-07-22T02:00:00Z", "", child)
	childBead.Status = "open" // pending successor is deliberately non-routable.
	status, err := deliveryStatusFromRecords([]bdRecord{childBead, parentBead}, time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if status.ObservedAt != "2026-07-22T03:00:00Z" || len(status.WIP) != 1 {
		t.Fatalf("status = %#v", status)
	}
	got := status.WIP[0]
	if got.DeliveryBeadID != parentID || got.BeadStatus != "blocked" || got.Route != "agentops.delivery" {
		t.Fatalf("status WIP = %#v", got)
	}
	if got.ReadyAt != parent.ReadyAt {
		t.Fatalf("status ready_at = %q, want minimum schema time %q", got.ReadyAt, parent.ReadyAt)
	}
	if got.AgeSeconds != 50*60*60 {
		t.Fatalf("status age_seconds = %d, want caller-observed 180000", got.AgeSeconds)
	}
}

func TestDeliveryStatusRejectsInvalidCanonicalRecordAndObservation(t *testing.T) {
	handoff := strings.Repeat("a", 64)
	record := readyBDRecord(t, "delivery-"+handoff[:20]+"-e000001", "2026-07-22T01:00:00Z", "", statusRecord(handoff, 1, "2026-07-21T01:00:00Z"))
	record.Status = "open"
	record.Metadata["gc.delivery_request"] = "{}"
	if _, err := deliveryStatusFromRecords([]bdRecord{record}, time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("status accepted invalid request envelope")
	}
	if _, err := deliveryStatusFromRecords(nil, time.Time{}); err == nil {
		t.Fatal("status accepted caller-unattested observation time")
	}
}

func statusRecord(handoff string, epoch int, readyAt string) DeliveryRecord {
	return DeliveryRecord{
		SchemaVersion: "gc.delivery.v1", Revision: 1, HandoffID: handoff,
		Epoch: DeliveryEpoch{Number: epoch, BaseRef: "main", BaseOID: strings.Repeat("a", 40), Branch: "gc/delivery/" + handoff[:20]},
		State: DeliveryStateQueued, Publication: "pending", ReadyAt: readyAt, Deadline: "2026-07-23T00:00:00Z",
		SemanticBead: "semantic", TerminalRef: "beads:semantic#terminal", Certificate: strings.Repeat("c", 64),
		Mode: "auto", Rig: "rig", Repository: "boshu2/agentops", Remote: "origin", Candidate: strings.Repeat("d", 40), Manifest: strings.Repeat("e", 64),
	}
}

func readyBDRecord(t *testing.T, id, createdAt, route string, record DeliveryRecord) bdRecord {
	t.Helper()
	encoded, err := verdictcheck.CanonicalJSON(record)
	if err != nil {
		t.Fatal(err)
	}
	request, err := verdictcheck.CanonicalJSON(deliveryRequestRef{SchemaVersion: "gc.delivery.request-ref.v1", Path: fmt.Sprintf("handoffs/%s/epochs/%06d/delivery-request.json", record.HandoffID, record.Epoch.Number), Digest: strings.Repeat("1", 64)})
	if err != nil {
		t.Fatal(err)
	}
	return bdRecord{ID: id, CreatedAt: createdAt, ExternalRef: "handoff:" + record.HandoffID + fmt.Sprintf(":epoch:%d", record.Epoch.Number), Metadata: map[string]any{"gc.kind": "delivery", "gc.routed_to": route, "gc.delivery.v1": string(encoded), "gc.delivery_request": string(request)}}
}

func TestPrepareBranchComposesEpochAndForceLeasePushes(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	remote, repo, worktrees := filepath.Join(root, "remote.git"), filepath.Join(root, "repo"), filepath.Join(root, "worktrees")
	mustGit(t, root, git, "init", "--bare", remote)
	mustGit(t, root, git, "init", "-b", "main", repo)
	mustGit(t, repo, git, "config", "user.name", "delivery test")
	mustGit(t, repo, git, "config", "user.email", "delivery@example.invalid")
	mustWrite(t, filepath.Join(repo, "base.txt"), "base\n")
	mustGit(t, repo, git, "add", "base.txt")
	mustGit(t, repo, git, "commit", "-m", "base")
	parent := mustGit(t, repo, git, "rev-parse", "HEAD")
	mustGit(t, repo, git, "remote", "add", "origin", remote)
	mustGit(t, repo, git, "push", "origin", "main")

	// The target moves without touching the candidate's path.
	mustWrite(t, filepath.Join(repo, "target.txt"), "moved target\n")
	mustGit(t, repo, git, "add", "target.txt")
	mustGit(t, repo, git, "commit", "-m", "target moved")
	base := mustGit(t, repo, git, "rev-parse", "HEAD")
	mustGit(t, repo, git, "push", "origin", "main")
	mustGit(t, repo, git, "checkout", "-b", "candidate", parent)
	mustWrite(t, filepath.Join(repo, "candidate.txt"), "candidate\n")
	mustGit(t, repo, git, "add", "candidate.txt")
	mustGit(t, repo, git, "commit", "-m", "candidate")
	candidate := mustGit(t, repo, git, "rev-parse", "HEAD")
	if err := os.Mkdir(worktrees, 0o755); err != nil {
		t.Fatal(err)
	}

	contents := []byte("candidate\n")
	sum := sha256.Sum256(contents)
	manifest := SubjectManifest{CanonicalManifestDigest: strings.Repeat("a", 64), Entries: []ManifestEntry{{Path: "candidate.txt", Kind: "file", Digest: fmt.Sprintf("%x", sum)}}}
	p := &NativeProviders{binaries: NativeBinaries{Git: git}, context: NativeContext{RepositoryDir: repo, WorktreeRoot: worktrees, Remote: "origin", BaseRef: "main", CheckOnlyGateArgv: [][]string{{git, "status", "--porcelain"}}}, manifest: manifest, candidate: candidate}
	branch, err := p.PrepareBranch(context.Background(), Branch{Name: "delivery/epoch", BaseRef: "main", BaseOID: base, Head: candidate, Proof: strings.Repeat("b", 64)})
	if err != nil {
		t.Fatalf("PrepareBranch: %v", err)
	}
	if branch.Head == candidate || branch.BaseOID != base {
		t.Fatalf("branch = %#v; expected separately composed epoch on base", branch)
	}
	if got := mustGit(t, repo, git, "rev-list", "--parents", "-n", "1", branch.Head); got != branch.Head+" "+base {
		t.Fatalf("epoch parents = %q, want %q", got, branch.Head+" "+base)
	}
	if got := mustGit(t, repo, git, "ls-remote", "origin", "refs/heads/delivery/epoch"); got != "" {
		t.Fatalf("composition pushed before receipt fence: %q", got)
	}
	if err := p.PushBranch(context.Background(), branch); err != nil {
		t.Fatalf("PushBranch: %v", err)
	}
	if got := mustGit(t, repo, git, "ls-remote", "origin", "refs/heads/delivery/epoch"); !strings.HasPrefix(got, branch.Head+"\t") {
		t.Fatalf("remote branch = %q, expected %s", got, branch.Head)
	}
	entries, err := os.ReadDir(worktrees)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("ephemeral worktree leaked: %#v", entries)
	}
}

func TestNativePrepareBranchLeaseContracts(t *testing.T) {
	t.Run("epoch_one_requires_absent_remote_and_retains_zero_lease", func(t *testing.T) {
		fixture := newNativeEpochFixture(t, "target.txt")
		branch, err := fixture.provider.PrepareBranch(context.Background(), fixture.plan)
		if err != nil {
			t.Fatal(err)
		}
		if branch.LeaseOID != "" {
			t.Fatalf("epoch one lease = %q", branch.LeaseOID)
		}
	})
	t.Run("epoch_one_rejects_existing_remote", func(t *testing.T) {
		fixture := newNativeEpochFixture(t, "target.txt")
		pushRemoteRef(t, fixture, fixture.base, fixture.plan.Name)
		if _, err := fixture.provider.PrepareBranch(context.Background(), fixture.plan); err == nil {
			t.Fatal("epoch one accepted an existing remote branch")
		}
	})
	t.Run("epoch_successor_requires_exact_remote_lease", func(t *testing.T) {
		for name, scenario := range map[string]struct {
			remote string
			wantOK bool
		}{
			"exact":       {remote: "base", wantOK: true},
			"absent":      {wantOK: false},
			"conflicting": {remote: "parent", wantOK: false},
		} {
			t.Run(name, func(t *testing.T) {
				fixture := newNativeEpochFixture(t, "target.txt")
				plan := fixture.plan
				plan.LeaseOID = fixture.base
				if scenario.remote != "" {
					ref := fixture.base
					if scenario.remote == "parent" {
						ref = fixture.parent
					}
					pushRemoteRef(t, fixture, ref, plan.Name)
				}
				branch, err := fixture.provider.PrepareBranch(context.Background(), plan)
				if scenario.wantOK {
					if err != nil || branch.LeaseOID != plan.LeaseOID {
						t.Fatalf("PrepareBranch = %#v, %v", branch, err)
					}
					return
				}
				if err == nil {
					t.Fatalf("epoch successor accepted %s remote state", name)
				}
			})
		}
	})
}

func TestNativePushBranchForceWithLeaseContracts(t *testing.T) {
	t.Run("epoch_one_uses_absent_remote_and_adopts_exact_head_without_main_read", func(t *testing.T) {
		fixture := newNativeEpochFixture(t, "target.txt")
		branch := fixture.plan
		branch.BaseOID = fixture.parent // main has already moved to fixture.base.
		if err := fixture.provider.PushBranch(context.Background(), branch); err != nil {
			t.Fatal(err)
		}
		if got := remoteRef(t, fixture, branch.Name); got != branch.Head {
			t.Fatalf("remote branch = %s, want %s", got, branch.Head)
		}
		if err := fixture.provider.PushBranch(context.Background(), branch); err != nil {
			t.Fatalf("exact head adoption: %v", err)
		}
	})
	t.Run("epoch_successor_requires_exact_lease", func(t *testing.T) {
		for name, scenario := range map[string]struct {
			remote string
			wantOK bool
		}{
			"exact":       {remote: "base", wantOK: true},
			"absent":      {wantOK: false},
			"conflicting": {remote: "parent", wantOK: false},
		} {
			t.Run(name, func(t *testing.T) {
				fixture := newNativeEpochFixture(t, "target.txt")
				branch := fixture.plan
				branch.LeaseOID, branch.BaseOID = fixture.base, fixture.parent
				if scenario.remote != "" {
					ref := fixture.base
					if scenario.remote == "parent" {
						ref = fixture.parent
					}
					pushRemoteRef(t, fixture, ref, branch.Name)
				}
				err := fixture.provider.PushBranch(context.Background(), branch)
				if scenario.wantOK {
					if err != nil || remoteRef(t, fixture, branch.Name) != branch.Head {
						t.Fatalf("PushBranch = %v", err)
					}
					return
				}
				if err == nil {
					t.Fatalf("epoch successor push accepted %s remote state", name)
				}
			})
		}
	})
}

func TestNativeFindBranchObservesOnlyRemoteHead(t *testing.T) {
	fixture := newNativeEpochFixture(t, "target.txt")
	pushRemoteRef(t, fixture, fixture.base, fixture.plan.Name)
	mustGit(t, fixture.repo, fixture.git, "checkout", "main")
	mustWrite(t, filepath.Join(fixture.repo, "later.txt"), "later\n")
	mustGit(t, fixture.repo, fixture.git, "add", "later.txt")
	mustGit(t, fixture.repo, fixture.git, "commit", "-m", "main moved again")
	mustGit(t, fixture.repo, fixture.git, "push", "origin", "main")
	branch, found, err := fixture.provider.FindBranch(context.Background(), fixture.plan.Name)
	if err != nil || !found {
		t.Fatalf("FindBranch = %#v, found=%t, err=%v", branch, found, err)
	}
	if branch.Name != fixture.plan.Name || branch.Head != fixture.base || branch.BaseOID != "" || branch.BaseRef != "" {
		t.Fatalf("branch observation leaked main identity: %#v", branch)
	}
}

func TestNativeObserveBaseImportsExactMovingCommitWithoutRefsFetchHeadOrWorktree(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	remote, writer, observer := filepath.Join(root, "remote.git"), filepath.Join(root, "writer"), filepath.Join(root, "observer")
	mustGit(t, root, git, "init", "--bare", remote)
	mustGit(t, root, git, "init", "-b", "main", writer)
	mustGit(t, writer, git, "config", "user.name", "delivery test")
	mustGit(t, writer, git, "config", "user.email", "delivery@example.invalid")
	mustWrite(t, filepath.Join(writer, "base.txt"), "base\n")
	mustGit(t, writer, git, "add", "base.txt")
	mustGit(t, writer, git, "commit", "-m", "base")
	base := mustGit(t, writer, git, "rev-parse", "HEAD")
	mustGit(t, writer, git, "remote", "add", "origin", remote)
	mustGit(t, writer, git, "push", "origin", "main")
	mustGit(t, remote, git, "symbolic-ref", "HEAD", "refs/heads/main")
	mustGit(t, root, git, "clone", remote, observer)
	mustWrite(t, filepath.Join(writer, "later.txt"), "later\n")
	mustGit(t, writer, git, "add", "later.txt")
	mustGit(t, writer, git, "commit", "-m", "main moved")
	moved := mustGit(t, writer, git, "rev-parse", "HEAD")
	mustGit(t, writer, git, "push", "origin", "main")

	missingProbe := exec.Command(git, "cat-file", "-e", moved+"^{commit}")
	missingProbe.Dir = observer
	if missingProbe.Run() == nil {
		t.Fatal("observer already contained the moving commit")
	}
	fetchHead := filepath.Join(observer, ".git", "FETCH_HEAD")
	beforeFetchHead, beforeErr := os.ReadFile(fetchHead)
	provider := &NativeProviders{binaries: NativeBinaries{Git: git}, context: NativeContext{RepositoryDir: observer, Remote: "origin"}}
	observed, err := provider.ObserveBase(context.Background(), "main")
	if err != nil || observed != moved {
		t.Fatalf("ObserveBase = %q, %v; want %s", observed, err, moved)
	}
	if got := mustGit(t, observer, git, "rev-parse", "main"); got != base {
		t.Fatalf("local main moved to %s, want %s", got, base)
	}
	if got := mustGit(t, observer, git, "rev-parse", "refs/remotes/origin/main"); got != base {
		t.Fatalf("remote tracking ref moved to %s, want %s", got, base)
	}
	afterFetchHead, afterErr := os.ReadFile(fetchHead)
	beforeMissing, afterMissing := errors.Is(beforeErr, os.ErrNotExist), errors.Is(afterErr, os.ErrNotExist)
	if beforeMissing != afterMissing || (beforeErr != nil && !beforeMissing) || (afterErr != nil && !afterMissing) {
		t.Fatalf("FETCH_HEAD existence changed: before=%v after=%v", beforeErr, afterErr)
	}
	if beforeErr == nil && string(beforeFetchHead) != string(afterFetchHead) {
		t.Fatal("exact object import rewrote FETCH_HEAD")
	}
	if descends, err := provider.BaseDescends(context.Background(), moved, base); err != nil || !descends {
		t.Fatalf("moving base ancestry = %t, %v", descends, err)
	}
}

func TestNativeWorktreePathAndStaleRecovery(t *testing.T) {
	fixture := newNativeEpochFixture(t, "target.txt")
	path, err := nativeWorktreePath(fixture.worktrees, fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	withLease := fixture.plan
	withLease.LeaseOID = fixture.base
	other, err := nativeWorktreePath(fixture.worktrees, withLease)
	if err != nil || path == other || filepath.Dir(path) != fixture.worktrees || filepath.Dir(other) != fixture.worktrees {
		t.Fatalf("worktree paths = %q, %q, %v", path, other, err)
	}
	for _, stale := range []string{"registered", "registered_missing", "unregistered"} {
		t.Run(stale, func(t *testing.T) {
			fixture := newNativeEpochFixture(t, "target.txt")
			path, err := nativeWorktreePath(fixture.worktrees, fixture.plan)
			if err != nil {
				t.Fatal(err)
			}
			if stale == "registered" || stale == "registered_missing" {
				mustGit(t, fixture.repo, fixture.git, "worktree", "add", "--detach", path, fixture.base)
				if stale == "registered_missing" {
					if err := os.RemoveAll(path); err != nil {
						t.Fatal(err)
					}
					registered, err := fixture.provider.worktreeRegistered(path)
					if err != nil || !registered {
						t.Fatalf("missing worktree registration = %t, %v", registered, err)
					}
				}
			} else {
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
				mustWrite(t, filepath.Join(path, "stale.txt"), "stale\n")
			}
			if _, err := fixture.provider.PrepareBranch(context.Background(), fixture.plan); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("stale worktree leaked at %q: %v", path, err)
			}
			if registered, err := fixture.provider.worktreeRegistered(path); err != nil || registered {
				t.Fatalf("stale worktree registration leaked: registered=%t err=%v", registered, err)
			}
		})
	}
}

func TestNativePrepareBranchRejectsOneParentPrefixCollisionAndZeroDiff(t *testing.T) {
	t.Run("one_parent", func(t *testing.T) {
		fixture := newNativeEpochFixture(t, "target.txt")
		mustGit(t, fixture.repo, fixture.git, "checkout", "-b", "other", fixture.parent)
		mustWrite(t, filepath.Join(fixture.repo, "other.txt"), "other\n")
		mustGit(t, fixture.repo, fixture.git, "add", "other.txt")
		mustGit(t, fixture.repo, fixture.git, "commit", "-m", "other")
		mustGit(t, fixture.repo, fixture.git, "checkout", "candidate")
		mustGit(t, fixture.repo, fixture.git, "merge", "--no-ff", "other", "-m", "merge candidate")
		fixture.plan.Head = mustGit(t, fixture.repo, fixture.git, "rev-parse", "HEAD")
		fixture.provider.candidate = fixture.plan.Head
		if _, err := fixture.provider.PrepareBranch(context.Background(), fixture.plan); err == nil {
			t.Fatal("merge candidate was accepted")
		}
	})
	t.Run("file_directory_prefix_collision", func(t *testing.T) {
		fixture := newNativeEpochFixture(t, "shared")
		mustGit(t, fixture.repo, fixture.git, "checkout", "candidate")
		if err := os.Mkdir(filepath.Join(fixture.repo, "shared"), 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(fixture.repo, "shared", "child.txt"), "child\n")
		mustGit(t, fixture.repo, fixture.git, "add", "shared/child.txt")
		mustGit(t, fixture.repo, fixture.git, "commit", "-m", "candidate prefix")
		fixture.plan.Head = mustGit(t, fixture.repo, fixture.git, "rev-parse", "HEAD")
		fixture.provider.candidate = fixture.plan.Head
		if _, err := fixture.provider.PrepareBranch(context.Background(), fixture.plan); !errors.Is(err, errPathCollision) {
			t.Fatalf("prefix collision error = %v", err)
		}
	})
	t.Run("zero_diff", func(t *testing.T) {
		fixture := newNativeEpochFixture(t, "target.txt")
		mustGit(t, fixture.repo, fixture.git, "checkout", "candidate")
		mustGit(t, fixture.repo, fixture.git, "commit", "--allow-empty", "-m", "empty candidate")
		fixture.plan.Head = mustGit(t, fixture.repo, fixture.git, "rev-parse", "HEAD")
		fixture.provider.candidate = fixture.plan.Head
		if _, err := fixture.provider.PrepareBranch(context.Background(), fixture.plan); !errors.Is(err, errZeroDiff) {
			t.Fatalf("zero diff error = %v", err)
		}
	})
}

type nativeEpochFixture struct {
	git, repo, remote, worktrees string
	provider                     *NativeProviders
	plan                         Branch
	parent, base, candidate      string
}

func newNativeEpochFixture(t *testing.T, targetPath string) nativeEpochFixture {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	remote, repo, worktrees := filepath.Join(root, "remote.git"), filepath.Join(root, "repo"), filepath.Join(root, "worktrees")
	mustGit(t, root, git, "init", "--bare", remote)
	mustGit(t, root, git, "init", "-b", "main", repo)
	mustGit(t, repo, git, "config", "user.name", "delivery test")
	mustGit(t, repo, git, "config", "user.email", "delivery@example.invalid")
	mustWrite(t, filepath.Join(repo, "base.txt"), "base\n")
	mustGit(t, repo, git, "add", "base.txt")
	mustGit(t, repo, git, "commit", "-m", "base")
	parent := mustGit(t, repo, git, "rev-parse", "HEAD")
	mustGit(t, repo, git, "remote", "add", "origin", remote)
	mustGit(t, repo, git, "push", "origin", "main")
	mustWrite(t, filepath.Join(repo, targetPath), "moved target\n")
	mustGit(t, repo, git, "add", targetPath)
	mustGit(t, repo, git, "commit", "-m", "target moved")
	base := mustGit(t, repo, git, "rev-parse", "HEAD")
	mustGit(t, repo, git, "push", "origin", "main")
	mustGit(t, repo, git, "checkout", "-b", "candidate", parent)
	mustWrite(t, filepath.Join(repo, "candidate.txt"), "candidate\n")
	mustGit(t, repo, git, "add", "candidate.txt")
	mustGit(t, repo, git, "commit", "-m", "candidate")
	candidate := mustGit(t, repo, git, "rev-parse", "HEAD")
	if err := os.Mkdir(worktrees, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := []byte("candidate\n")
	sum := sha256.Sum256(contents)
	manifest := SubjectManifest{CanonicalManifestDigest: strings.Repeat("a", 64), Entries: []ManifestEntry{{Path: "candidate.txt", Kind: "file", Digest: fmt.Sprintf("%x", sum)}}}
	provider := &NativeProviders{binaries: NativeBinaries{Git: git}, context: NativeContext{RepositoryDir: repo, WorktreeRoot: worktrees, Remote: "origin", BaseRef: "main", CheckOnlyGateArgv: [][]string{{git, "status", "--porcelain"}}}, manifest: manifest, candidate: candidate}
	return nativeEpochFixture{git: git, repo: repo, remote: remote, worktrees: worktrees, provider: provider, plan: Branch{Name: "delivery/epoch", BaseRef: "main", BaseOID: base, Head: candidate, Proof: strings.Repeat("b", 64)}, parent: parent, base: base, candidate: candidate}
}

func pushRemoteRef(t *testing.T, fixture nativeEpochFixture, oid, branch string) {
	t.Helper()
	mustGit(t, fixture.repo, fixture.git, "push", "origin", oid+":refs/heads/"+branch)
}

func remoteRef(t *testing.T, fixture nativeEpochFixture, branch string) string {
	t.Helper()
	output := mustGit(t, fixture.repo, fixture.git, "ls-remote", "origin", "refs/heads/"+branch)
	fields := strings.Fields(output)
	if len(fields) != 2 {
		t.Fatalf("remote ref %q = %q", branch, output)
	}
	return fields[0]
}

func mustGit(t *testing.T, dir, git string, args ...string) string {
	t.Helper()
	command := exec.Command(git, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func mustWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}
