package overnight

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// externalWatchlistGeneratorName is the read-side generator that emits
// candidates from a checked-in operator-managed watchlist of external
// sources (repos, packages, URLs). All emitted candidates are held for
// human review (status="proposed", requires=["human-review"]) per RFC 0001
// Proposal 2; the queue selector enforces the hold via
// rpi.IsQueueItemHeldForReview.
const externalWatchlistGeneratorName = "external-watchlist"

// externalWatchlistSourceEpic is the per-generator source_epic stamped on
// the resulting sidecar.
const externalWatchlistSourceEpic = "external-watchlist"

// defaultWatchlistStaleAfter is the per-entry staleness threshold when a
// watchlist entry doesn't override stale_after. One week balances "noisy
// re-emit every run" against "operator forgets to look".
const defaultWatchlistStaleAfter = 168 * time.Hour

// externalWatchlistRelativePath is the operator-managed watchlist file
// location, relative to the run cwd. PROGRAM.md permits this path under
// "Mutable Scope" when Dream is the active command.
const externalWatchlistRelativePath = ".agents/dream/external-watchlist.yaml"

// WatchlistEntry is one operator-curated source to watch for changes.
//
// last_seen_at is bumped manually by the operator after they review the
// upstream source. stale_after is optional; when zero or omitted, the
// generator falls back to defaultWatchlistStaleAfter.
type WatchlistEntry struct {
	ID         string        `yaml:"id"`
	Title      string        `yaml:"title"`
	Type       string        `yaml:"type"`
	Severity   string        `yaml:"severity"`
	Source     string        `yaml:"source"`
	LastSeen   time.Time     `yaml:"last_seen_at"`
	StaleAfter time.Duration `yaml:"stale_after"`
}

// Watchlist is the top-level YAML schema operators write under
// .agents/dream/external-watchlist.yaml.
type Watchlist struct {
	Entries []WatchlistEntry `yaml:"entries"`
}

// runExternalWatchlistGenerator reads the operator-managed watchlist file
// and emits a sidecar candidate for every entry past its stale-after
// window. All emitted candidates carry status="proposed" and
// requires=["human-review"]; the queue selector holds them.
func runExternalWatchlistGenerator(ctx context.Context, opts RunLoopOptions) FindingGeneratorSidecar {
	started := time.Now()
	watchlistPath := filepath.Join(opts.Cwd, externalWatchlistRelativePath)
	return runExternalWatchlistGeneratorAt(ctx, opts, watchlistPath, time.Now(), started)
}

// runExternalWatchlistGeneratorAt is the testable seam: tests inject the
// watchlist path, the "now" time used for staleness comparison, and the
// "started" stamp for sidecar bookkeeping.
func runExternalWatchlistGeneratorAt(
	ctx context.Context,
	opts RunLoopOptions,
	watchlistPath string,
	now time.Time,
	started time.Time,
) FindingGeneratorSidecar {
	if err := ctx.Err(); err != nil {
		return buildFailedFindingGeneratorSidecar(opts, externalWatchlistGeneratorName,
			externalWatchlistSourceEpic, started, time.Now(), err)
	}
	data, err := os.ReadFile(watchlistPath)
	if os.IsNotExist(err) {
		// Soft success: operator has not curated a watchlist yet.
		return newFindingGeneratorSidecar(opts, externalWatchlistGeneratorName,
			externalWatchlistSourceEpic, "completed", started, time.Now(), nil, "")
	}
	if err != nil {
		return buildFailedFindingGeneratorSidecar(opts, externalWatchlistGeneratorName,
			externalWatchlistSourceEpic, started, time.Now(), err)
	}
	var watchlist Watchlist
	if err := yaml.Unmarshal(data, &watchlist); err != nil {
		return buildFailedFindingGeneratorSidecar(opts, externalWatchlistGeneratorName,
			externalWatchlistSourceEpic, started, time.Now(),
			fmt.Errorf("parse watchlist: %w", err))
	}
	candidates := make([]FindingGeneratorCandidate, 0, len(watchlist.Entries))
	for _, entry := range watchlist.Entries {
		if err := ctx.Err(); err != nil {
			return buildFailedFindingGeneratorSidecar(opts, externalWatchlistGeneratorName,
				externalWatchlistSourceEpic, started, time.Now(), err)
		}
		if !watchlistEntryIsStale(entry, now) {
			continue
		}
		candidates = append(candidates, watchlistEntryCandidate(entry))
	}
	return newFindingGeneratorSidecar(opts, externalWatchlistGeneratorName,
		externalWatchlistSourceEpic, "completed", started, time.Now(), candidates, "")
}

// watchlistEntryIsStale returns true when the entry's last_seen_at is
// older than its stale_after window (or the default when unset). Entries
// with a zero last_seen_at are treated as not-yet-reviewed and never
// emitted — operators must seed the timestamp before the entry is active.
func watchlistEntryIsStale(entry WatchlistEntry, now time.Time) bool {
	if entry.LastSeen.IsZero() {
		return false
	}
	threshold := entry.StaleAfter
	if threshold <= 0 {
		threshold = defaultWatchlistStaleAfter
	}
	return now.Sub(entry.LastSeen) >= threshold
}

// watchlistEntryCandidate converts one stale watchlist entry into a
// sidecar candidate. The candidate is always held for human review.
func watchlistEntryCandidate(entry WatchlistEntry) FindingGeneratorCandidate {
	description := fmt.Sprintf("External source %q has not been reviewed since %s",
		entry.Source, entry.LastSeen.UTC().Format(time.RFC3339))
	return FindingGeneratorCandidate{
		ID:          entry.ID,
		Title:       entry.Title,
		Type:        normalizeGeneratorCandidateType(entry.Type),
		Severity:    normalizeGeneratorCandidateSeverity(entry.Severity),
		Source:      "evolve-generator",
		Description: description,
		DedupKey:    "external-watchlist|" + normalizeDedupComponent(entry.ID+"|"+entry.Title),
		Status:      "proposed",
		Requires:    []string{"human-review"},
	}
}
