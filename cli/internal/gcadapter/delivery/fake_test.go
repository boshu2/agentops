package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FakeProviders is an in-memory conformance boundary for deterministic tests.
// It is intentionally not wired into the optional binary's production mode.
type FakeProviders struct {
	terminal             Terminal
	deliveries           map[string]DeliveryBead
	branches             map[string]Branch
	prs                  map[string]PullRequest
	initiallyNonRoutable bool
	mutations            int
	publishGuard         func() error
	fixtureStatePath     string
	hostedGate           HostedGate
	mergeArm             *MergeArm
	mergeObservation     MergeObservation
	landing              *Landing
	mergeCount           int
	pushCount            int
	prCreateCount        int
	autoMergeGuard       func() error
	baseDescendants      map[string]map[string]bool
	observedBase         string
	prObservation        *PRObservation
	prepareBranchError   error
	calls                []string
}

func NewFakeProviders(terminal Terminal) *FakeProviders {
	return &FakeProviders{terminal: terminal, deliveries: map[string]DeliveryBead{}, branches: map[string]Branch{}, prs: map[string]PullRequest{}, baseDescendants: map[string]map[string]bool{}}
}

// OpenFixtureProviders is an explicitly offline fake boundary. Its state is a
// test fixture, never a production delivery ledger or a Beads replacement.
func OpenFixtureProviders(path string, terminal Terminal) (*FakeProviders, error) {
	provider := NewFakeProviders(terminal)
	provider.fixtureStatePath = path
	bytes, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return provider, nil
	}
	if err != nil {
		return nil, err
	}
	var state fixtureState
	if err := json.Unmarshal(bytes, &state); err != nil {
		return nil, err
	}
	if state.Terminal != terminal {
		return nil, fmt.Errorf("fixture terminal does not match exact invocation")
	}
	provider.deliveries, provider.branches, provider.prs, provider.initiallyNonRoutable = state.Deliveries, state.Branches, state.PRs, state.InitiallyNonRoutable
	provider.hostedGate, provider.mergeArm, provider.mergeObservation, provider.landing, provider.mergeCount = state.HostedGate, state.MergeArm, state.MergeObservation, state.Landing, state.MergeCount
	provider.prCreateCount = state.PRCreateCount
	if provider.deliveries == nil {
		provider.deliveries = map[string]DeliveryBead{}
	}
	if provider.branches == nil {
		provider.branches = map[string]Branch{}
	}
	if provider.prs == nil {
		provider.prs = map[string]PullRequest{}
	}
	return provider, nil
}

type fixtureState struct {
	Terminal             Terminal                `json:"terminal"`
	Deliveries           map[string]DeliveryBead `json:"deliveries"`
	Branches             map[string]Branch       `json:"branches"`
	PRs                  map[string]PullRequest  `json:"prs"`
	InitiallyNonRoutable bool                    `json:"initially_non_routable"`
	HostedGate           HostedGate              `json:"hosted_gate"`
	MergeArm             *MergeArm               `json:"merge_arm"`
	MergeObservation     MergeObservation        `json:"merge_observation"`
	Landing              *Landing                `json:"landing"`
	MergeCount           int                     `json:"merge_count"`
	PRCreateCount        int                     `json:"pr_create_count"`
}

func (f *FakeProviders) persist() error {
	if f.fixtureStatePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(f.fixtureStatePath), 0o755); err != nil {
		return err
	}
	bytes, err := json.Marshal(fixtureState{Terminal: f.terminal, Deliveries: f.deliveries, Branches: f.branches, PRs: f.prs, InitiallyNonRoutable: f.initiallyNonRoutable, HostedGate: f.hostedGate, MergeArm: f.mergeArm, MergeObservation: f.mergeObservation, Landing: f.landing, MergeCount: f.mergeCount, PRCreateCount: f.prCreateCount})
	if err != nil {
		return err
	}
	return os.WriteFile(f.fixtureStatePath, append(bytes, '\n'), 0o600)
}
func (f *FakeProviders) Clone() *FakeProviders {
	clone := NewFakeProviders(f.terminal)
	clone.hostedGate, clone.mergeObservation = f.hostedGate, f.mergeObservation
	return clone
}
func (f *FakeProviders) Terminal(context.Context, string) (Terminal, error) { return f.terminal, nil }
func (f *FakeProviders) FindDelivery(_ context.Context, id string) (DeliveryBead, bool, error) {
	f.calls = append(f.calls, "find_delivery")
	value, ok := f.deliveries[id]
	return value, ok, nil
}
func (f *FakeProviders) CreateDelivery(_ context.Context, expected DeliveryBead) (DeliveryBead, error) {
	if current, ok := f.deliveries[expected.ID]; ok {
		if current.ID != expected.ID || current.ExternalRef != expected.ExternalRef || current.Record != expected.Record {
			return DeliveryBead{}, fmt.Errorf("delivery exists with different identity")
		}
		return current, nil
	}
	if expected.Route != "" {
		return DeliveryBead{}, fmt.Errorf("delivery must start non-routable")
	}
	f.deliveries[expected.ID] = expected
	f.initiallyNonRoutable = true
	f.mutations++
	if err := f.persist(); err != nil {
		return DeliveryBead{}, err
	}
	return expected, nil
}
func (f *FakeProviders) StoreTransition(_ context.Context, observed DeliveryBead, next DeliveryRecord) (DeliveryBead, error) {
	current, ok := f.deliveries[observed.ID]
	if !ok || current != observed {
		return DeliveryBead{}, fmt.Errorf("delivery transition has stale observed record")
	}
	if next.SchemaVersion != "gc.delivery.v1" || next.Revision != current.Record.Revision+1 || next.HandoffID != current.Record.HandoffID || next.Epoch.Number != current.Record.Epoch.Number || next.Epoch.BaseRef != current.Record.Epoch.BaseRef || next.Epoch.BaseOID != current.Record.Epoch.BaseOID || next.Epoch.Branch != current.Record.Epoch.Branch {
		return DeliveryBead{}, fmt.Errorf("delivery transition changes immutable identity")
	}
	current.Record = next
	f.deliveries[current.ID] = current
	f.mutations++
	if err := f.persist(); err != nil {
		return DeliveryBead{}, err
	}
	return current, nil
}
func (f *FakeProviders) BaseDescends(_ context.Context, descendant, ancestor string) (bool, error) {
	if descendant == ancestor {
		return true, nil
	}
	return f.baseDescendants[descendant][ancestor], nil
}
func (f *FakeProviders) ObserveBase(_ context.Context, _ string) (string, error) {
	f.calls = append(f.calls, "observe_base")
	if f.observedBase != "" {
		return f.observedBase, nil
	}
	if f.landing != nil {
		return f.landing.SHA, nil
	}
	return strings.Repeat("d", 40), nil
}

func (f *FakeProviders) Calls() []string {
	return append([]string(nil), f.calls...)
}
func (f *FakeProviders) PublishRoute(_ context.Context, id string) error {
	if f.publishGuard != nil {
		if err := f.publishGuard(); err != nil {
			return err
		}
	}
	value, ok := f.deliveries[id]
	if !ok {
		return fmt.Errorf("delivery does not exist")
	}
	if value.Record.Publication != "published" || len(value.Record.Committed) != 64 {
		return fmt.Errorf("route publication lacks committed delivery.v1 envelope")
	}
	value.Route = "agentops.delivery"
	f.deliveries[id] = value
	f.mutations++
	if err := f.persist(); err != nil {
		return err
	}
	return nil
}
func (f *FakeProviders) RetireRoute(_ context.Context, id string) error {
	value, ok := f.deliveries[id]
	if !ok || value.Route != "agentops.delivery" {
		return fmt.Errorf("delivery route is not active")
	}
	value.Route = ""
	f.deliveries[id] = value
	f.mutations++
	return f.persist()
}
func (f *FakeProviders) FindBranch(_ context.Context, name string) (Branch, bool, error) {
	value, ok := f.branches[name]
	return value, ok, nil
}
func (f *FakeProviders) PrepareBranch(_ context.Context, expected Branch) (Branch, error) {
	if f.prepareBranchError != nil {
		return Branch{}, f.prepareBranchError
	}
	actual := expected
	actual.Head = identifier("fake-epoch-head", expected.Head, expected.BaseOID, expected.Proof)[:40]
	actual.Tree = identifier("fake-epoch-tree", actual.Head, expected.BaseOID, expected.Proof)[:40]
	return actual, nil
}
func (f *FakeProviders) PushBranch(_ context.Context, expected Branch) error {
	if current, ok := f.branches[expected.Name]; ok {
		if current.Head == expected.Head {
			return nil
		}
		if expected.LeaseOID == "" || current.Head != expected.LeaseOID {
			return fmt.Errorf("branch exists with different identity")
		}
	}
	f.branches[expected.Name] = expected
	f.mutations++
	f.pushCount++
	return f.persist()
}
func (f *FakeProviders) ObservePR(_ context.Context, intent PRIntent) (PRObservation, error) {
	if f.prObservation != nil {
		return *f.prObservation, nil
	}
	actual, found := f.prs[intent.PRID]
	if !found {
		return PRObservation{State: "absent"}, nil
	}
	actual.ID, actual.Repository, actual.Branch, actual.BaseRef, actual.EffectID = intent.PRID, intent.Repository, intent.Branch, intent.BaseRef, intent.EffectID
	return PRObservation{State: "open", BaseOID: intent.BaseOID, Head: intent.ExpectedHead, PR: actual}, nil
}
func (f *FakeProviders) CreatePR(_ context.Context, intent PRIntent) (PRObservation, error) {
	return f.createPRIntent(intent)
}
func (f *FakeProviders) createPRIntent(intent PRIntent) (PRObservation, error) {
	if actual, found := f.prs[intent.PRID]; found {
		return PRObservation{State: "open", BaseOID: intent.BaseOID, Head: intent.ExpectedHead, PR: actual}, fmt.Errorf("PR already exists")
	}
	actual := PullRequest{ID: intent.PRID, Repository: intent.Repository, Branch: intent.Branch, BaseRef: intent.BaseRef, EffectID: intent.EffectID, NodeID: "PR_" + intent.PRID, Number: "1", URL: "https://example.invalid/" + intent.PRID}
	f.prs[actual.ID], f.mutations, f.prCreateCount = actual, f.mutations+1, f.prCreateCount+1
	if err := f.persist(); err != nil {
		return PRObservation{}, err
	}
	return PRObservation{State: "open", BaseOID: intent.BaseOID, Head: intent.ExpectedHead, PR: actual}, nil
}
func (f *FakeProviders) HostedGate(_ context.Context, pr PullRequest) (HostedGate, error) {
	if f.hostedGate.Head != "" {
		return f.hostedGate, nil
	}
	branch := f.branches[pr.Branch]
	required := HostedCheck{AppID: "1234", Context: "required / test"}
	passed := required
	passed.Status, passed.Conclusion = "COMPLETED", "SUCCESS"
	return HostedGate{Repository: "boshu2/agentops", BaseRef: pr.BaseRef, BaseOID: branch.BaseOID, Head: branch.Head, PRState: "OPEN", MergeState: "CLEAN", Strict: true, ProtectionDigest: strings.Repeat("b", 64), RequiredChecks: []HostedCheck{required}, Checks: []HostedCheck{passed}}, nil
}
func (f *FakeProviders) ObserveMerge(_ context.Context, arm MergeArm) (MergeObservation, error) {
	if f.mergeArm == nil {
		return MergeObservation{State: "absent"}, nil
	}
	if *f.mergeArm != arm {
		return MergeObservation{}, fmt.Errorf("merge arm identity conflict")
	}
	if f.mergeObservation.State != "" {
		return f.mergeObservation, nil
	}
	if f.landing != nil {
		return MergeObservation{State: "landed"}, nil
	}
	return MergeObservation{State: "armed"}, nil
}
func (f *FakeProviders) ArmMerge(_ context.Context, arm MergeArm) (MergeArm, error) {
	if f.mergeArm != nil {
		if *f.mergeArm != arm {
			return MergeArm{}, fmt.Errorf("merge arm identity conflict")
		}
		return *f.mergeArm, nil
	}
	if f.autoMergeGuard != nil {
		if err := f.autoMergeGuard(); err != nil {
			return MergeArm{}, err
		}
	}
	f.mergeArm = &arm
	pr, ok := f.prs[arm.PRID]
	if !ok {
		return MergeArm{}, fmt.Errorf("auto-merge PR does not exist")
	}
	branch, ok := f.branches[pr.Branch]
	if !ok || branch.Tree == "" {
		return MergeArm{}, fmt.Errorf("auto-merge branch has no epoch tree")
	}
	landing := Landing{PRID: pr.ID, Head: branch.Head, SHA: strings.Repeat("c", 40), Tree: branch.Tree, Parents: []string{arm.BaseOID}}
	f.landing, f.mergeCount = &landing, f.mergeCount+1
	f.mutations++
	if err := f.persist(); err != nil {
		return MergeArm{}, err
	}
	return arm, nil
}
func (f *FakeProviders) Landing(_ context.Context, pr PullRequest) (Landing, bool, error) {
	if f.mergeObservation.State == "unknown" || f.mergeObservation.State == "refused" {
		return Landing{}, false, nil
	}
	if f.landing == nil {
		return Landing{}, false, nil
	}
	if f.landing.PRID != pr.ID {
		return Landing{}, false, fmt.Errorf("landing PR identity conflict")
	}
	return *f.landing, true, nil
}
func (f *FakeProviders) DeliveryCount() int                         { return len(f.deliveries) }
func (f *FakeProviders) BranchCount() int                           { return len(f.branches) }
func (f *FakeProviders) PRCount() int                               { return len(f.prs) }
func (f *FakeProviders) OnlyDeliveryWasInitiallyNonRoutable() bool  { return f.initiallyNonRoutable }
func (f *FakeProviders) MutationCount() int                         { return f.mutations }
func (f *FakeProviders) MergeCount() int                            { return f.mergeCount }
func (f *FakeProviders) AutoMergeCount() int                        { return f.mergeCount }
func (f *FakeProviders) PushCount() int                             { return f.pushCount }
func (f *FakeProviders) PRCreateCount() int                         { return f.prCreateCount }
func (f *FakeProviders) SetPublishGuard(guard func() error)         { f.publishGuard = guard }
func (f *FakeProviders) SetHostedGate(gate HostedGate)              { f.hostedGate = gate }
func (f *FakeProviders) SetMergeObservation(value MergeObservation) { f.mergeObservation = value }
func (f *FakeProviders) SetAutoMergeGuard(guard func() error)       { f.autoMergeGuard = guard }
func (f *FakeProviders) SetLanding(value Landing)                   { f.landing = &value }
func (f *FakeProviders) AllowBaseDescendant(descendant, ancestor string) {
	if f.baseDescendants[descendant] == nil {
		f.baseDescendants[descendant] = map[string]bool{}
	}
	f.baseDescendants[descendant][ancestor] = true
}
func (f *FakeProviders) SetObservedBase(base string) { f.observedBase = base }
func (f *FakeProviders) SetPRObservation(value PRObservation) {
	f.prObservation = &value
}
func (f *FakeProviders) SetPrepareBranchError(value error) { f.prepareBranchError = value }
func (f *FakeProviders) SetPrepareBranchFailure(reason string) {
	switch reason {
	case "target_regression":
		f.prepareBranchError = errTargetRegression
	case "path_collision":
		f.prepareBranchError = errPathCollision
	case "zero_diff":
		f.prepareBranchError = errZeroDiff
	default:
		f.prepareBranchError = fmt.Errorf("unknown fake prepare failure %q", reason)
	}
}
func (f *FakeProviders) Delivery(epoch int) (DeliveryBead, bool) {
	for _, bead := range f.deliveries {
		if bead.Record.Epoch.Number == epoch {
			return bead, true
		}
	}
	return DeliveryBead{}, false
}
func (f *FakeProviders) PutDelivery(value DeliveryBead) { f.deliveries[value.ID] = value }
func (f *FakeProviders) PutBranch(value Branch)         { f.branches[value.Name] = value }
func (f *FakeProviders) PutPR(value PullRequest)        { f.prs[value.ID] = value }
