package gates

import (
	"fmt"
	"sort"
	"sync"
)

// Registry is a collection of checks keyed by ID. Use NewRegistry for an
// isolated instance (tests) or the package-level Default via Register (the
// init()-registration path used by per-check files).
type Registry struct {
	mu     sync.RWMutex
	checks map[string]Check
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{checks: map[string]Check{}}
}

// Add registers c. It returns an error if c is malformed (validate) or its ID
// is already registered.
func (r *Registry) Add(c Check) error {
	if err := c.validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.checks[c.ID]; dup {
		return fmt.Errorf("gates: duplicate check ID %q", c.ID)
	}
	r.checks[c.ID] = c
	return nil
}

// All returns every registered check, sorted by ID so callers get a
// deterministic order regardless of registration/iteration order.
func (r *Registry) All() []Check {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Check, 0, len(r.checks))
	for _, c := range r.checks {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get returns the check with the given ID and whether it was found.
func (r *Registry) Get(id string) (Check, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.checks[id]
	return c, ok
}

// Len returns the number of registered checks.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.checks)
}

// validate enforces the structural invariants of a Check.
func (c Check) validate() error {
	if c.ID == "" {
		return fmt.Errorf("gates: check has empty ID")
	}
	if c.Tiers == 0 {
		return fmt.Errorf("gates: check %q has no tiers", c.ID)
	}
	hasBacking := c.Backing != ""
	hasRun := c.Run != nil
	if hasBacking == hasRun {
		return fmt.Errorf("gates: check %q must set exactly one of Backing or Run", c.ID)
	}
	return nil
}

// Default is the package-level registry that init()-registered checks populate.
var Default = NewRegistry()

// Register adds c to the Default registry, panicking on error. It is intended
// for use from init() in per-check files: a registration failure is a
// programmer error that must break the build/tests, not a runtime condition.
func Register(c Check) {
	if err := Default.Add(c); err != nil {
		panic(err)
	}
}
