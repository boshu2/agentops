package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
}

func NewFakeProviders(terminal Terminal) *FakeProviders {
	return &FakeProviders{terminal: terminal, deliveries: map[string]DeliveryBead{}, branches: map[string]Branch{}, prs: map[string]PullRequest{}}
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
}

func (f *FakeProviders) persist() error {
	if f.fixtureStatePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(f.fixtureStatePath), 0o755); err != nil {
		return err
	}
	bytes, err := json.Marshal(fixtureState{Terminal: f.terminal, Deliveries: f.deliveries, Branches: f.branches, PRs: f.prs, InitiallyNonRoutable: f.initiallyNonRoutable})
	if err != nil {
		return err
	}
	return os.WriteFile(f.fixtureStatePath, append(bytes, '\n'), 0o600)
}
func (f *FakeProviders) Clone() *FakeProviders                              { return NewFakeProviders(f.terminal) }
func (f *FakeProviders) Terminal(context.Context, string) (Terminal, error) { return f.terminal, nil }
func (f *FakeProviders) FindDelivery(_ context.Context, id string) (DeliveryBead, bool, error) {
	value, ok := f.deliveries[id]
	return value, ok, nil
}
func (f *FakeProviders) CreateDelivery(_ context.Context, expected DeliveryBead) (DeliveryBead, error) {
	if current, ok := f.deliveries[expected.ID]; ok {
		if current != expected {
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
func (f *FakeProviders) PublishDelivery(_ context.Context, id string) error {
	if f.publishGuard != nil {
		if err := f.publishGuard(); err != nil {
			return err
		}
	}
	value, ok := f.deliveries[id]
	if !ok {
		return fmt.Errorf("delivery does not exist")
	}
	value.Route = "agentops.delivery"
	f.deliveries[id] = value
	f.mutations++
	if err := f.persist(); err != nil {
		return err
	}
	return nil
}
func (f *FakeProviders) FindBranch(_ context.Context, name string) (Branch, bool, error) {
	value, ok := f.branches[name]
	return value, ok, nil
}
func (f *FakeProviders) PrepareBranch(_ context.Context, expected Branch) (Branch, error) {
	if current, ok := f.branches[expected.Name]; ok {
		if current != expected {
			return Branch{}, fmt.Errorf("branch exists with different identity")
		}
		return current, nil
	}
	f.branches[expected.Name] = expected
	f.mutations++
	if err := f.persist(); err != nil {
		return Branch{}, err
	}
	return expected, nil
}
func (f *FakeProviders) FindPR(_ context.Context, id string) (PullRequest, bool, error) {
	value, ok := f.prs[id]
	return value, ok, nil
}
func (f *FakeProviders) CreatePR(_ context.Context, expected PullRequest) (PullRequest, error) {
	if current, ok := f.prs[expected.ID]; ok {
		if current != expected {
			return PullRequest{}, fmt.Errorf("PR exists with different identity")
		}
		return current, nil
	}
	f.prs[expected.ID] = expected
	f.mutations++
	if err := f.persist(); err != nil {
		return PullRequest{}, err
	}
	return expected, nil
}
func (f *FakeProviders) DeliveryCount() int                        { return len(f.deliveries) }
func (f *FakeProviders) BranchCount() int                          { return len(f.branches) }
func (f *FakeProviders) PRCount() int                              { return len(f.prs) }
func (f *FakeProviders) OnlyDeliveryWasInitiallyNonRoutable() bool { return f.initiallyNonRoutable }
func (f *FakeProviders) MutationCount() int                        { return f.mutations }
func (f *FakeProviders) SetPublishGuard(guard func() error)        { f.publishGuard = guard }
func (f *FakeProviders) PutDelivery(value DeliveryBead)            { f.deliveries[value.ID] = value }
func (f *FakeProviders) PutBranch(value Branch)                    { f.branches[value.Name] = value }
func (f *FakeProviders) PutPR(value PullRequest)                   { f.prs[value.ID] = value }
