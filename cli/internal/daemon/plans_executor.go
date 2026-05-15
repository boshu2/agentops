package daemon

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql" // register `mysql` driver for sql.Open
)

// PlansBdSource is the input edge of PlansProjectionExecutor. Production wires
// this to the shared bushido Dolt server via dbBdSource (database/sql); tests
// inject a fakePlansBdSource for L1 BDD coverage.
type PlansBdSource interface {
	// QueryEpics returns the bd-side epic state for project_id (filtered to
	// rows whose ID begins with issuePrefix when non-empty). Implementations
	// must respect ctx cancellation.
	QueryEpics(ctx context.Context, projectID, issuePrefix string) ([]PlansProjectionEntry, error)
}

// PlansProjectionExecutorOptions configures a PlansProjectionExecutor.
type PlansProjectionExecutorOptions struct {
	Store    *Store
	BdSource PlansBdSource
	Now      func() time.Time
}

// PlansProjectionExecutor is the JobExecutor (supervisor.go:18) for
// plans.projection jobs. Mirrors the RPIJobExecutor shape: thin wrapper that
// the supervisor invokes with a claimed job; rebuild/write/validate mechanics
// live in plans_projection.go.
type PlansProjectionExecutor struct {
	store  *Store
	source PlansBdSource
	now    func() time.Time
}

// NewPlansProjectionExecutor builds an executor from explicit dependencies.
// Returns an error when the store or bd source is nil — mirrors
// NewRPIJobExecutor's required-field contract.
func NewPlansProjectionExecutor(opts PlansProjectionExecutorOptions) (*PlansProjectionExecutor, error) {
	if opts.Store == nil {
		return nil, errors.New("plans.projection executor: store is required")
	}
	if opts.BdSource == nil {
		return nil, errors.New("plans.projection executor: bd source is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &PlansProjectionExecutor{store: opts.Store, source: opts.BdSource, now: now}, nil
}

// JobTypes reports the daemon job types this executor handles.
func (e *PlansProjectionExecutor) JobTypes() []JobType {
	return []JobType{JobTypePlansProjection}
}

// RunJob executes one claimed plans.projection job. The supervisor wraps this
// call with claim/heartbeat/terminal-record bookkeeping. The executor:
//  1. parses the spec from claim payload,
//  2. queries bd for the project's epic set,
//  3. builds a DaemonPlansProjection in memory,
//  4. writes the manifest snapshot atomically (tmp + os.Rename), and
//  5. returns artifacts mapping the snapshot path and entry count.
//
// RunJob requires a non-nil ctx; callers passing nil will panic on first use.
func (e *PlansProjectionExecutor) RunJob(ctx context.Context, claim QueueLease) (JobExecutionResult, error) {
	if claim.Job.JobType != JobTypePlansProjection {
		return JobExecutionResult{}, fmt.Errorf("plans.projection executor does not support job type %s", claim.Job.JobType)
	}
	spec, err := PlansProjectionJobSpecFromPayload(claim.Job.Payload)
	if err != nil {
		return JobExecutionResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return JobExecutionResult{}, err
	}
	rows, err := e.source.QueryEpics(ctx, spec.ProjectID, spec.IssuePrefix)
	if err != nil {
		return JobExecutionResult{}, fmt.Errorf("plans.projection bd query: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return JobExecutionResult{}, err
	}
	rebuiltAt := e.now().UTC().Format(time.RFC3339Nano)
	projection := DaemonPlansProjection{
		SchemaVersion: DaemonPlansProjectionSchemaVersion,
		ProjectID:     spec.ProjectID,
		IssuePrefix:   spec.IssuePrefix,
		Entries:       enrichPlansEntryChecksums(rows),
		RebuiltAt:     rebuiltAt,
	}
	if err := ValidateDaemonPlansProjection(projection); err != nil {
		return JobExecutionResult{}, err
	}
	snapshotPath, err := WriteDaemonPlansProjection(spec.OutputDir, projection)
	if err != nil {
		return JobExecutionResult{}, err
	}
	artifacts := map[string]string{
		"manifest_jsonl": snapshotPath,
		"manifest_count": strconv.Itoa(len(projection.Entries)),
		"rebuilt_at":     rebuiltAt,
		"trigger":        string(spec.RefreshTrigger),
	}
	return JobExecutionResult{Artifacts: artifacts}, nil
}

// enrichPlansEntryChecksums fills in a deterministic checksum on entries that
// arrive without one. The checksum is a short SHA-256 hex prefix over the
// (BeadsID, Title, Status, Priority, IssueType, UpdatedAt) tuple — enough to
// detect drift across rebuilds without paying the full hash cost.
func enrichPlansEntryChecksums(entries []PlansProjectionEntry) []PlansProjectionEntry {
	out := make([]PlansProjectionEntry, len(entries))
	for i, entry := range entries {
		if entry.Checksum == "" {
			h := sha256.New()
			fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s",
				entry.BeadsID, entry.Title, entry.Status, entry.Priority,
				entry.IssueType, entry.UpdatedAt.Format(time.RFC3339Nano))
			entry.Checksum = hex.EncodeToString(h.Sum(nil))[:16]
		}
		out[i] = entry
	}
	return out
}

// dbBdSource is the production PlansBdSource backed by the shared bushido
// Dolt server. Connection params come from the caller (typically loaded from
// .beads/metadata.json by the agentopsd CLI). The shared db has no auth
// (root + empty password) and is firewalled to the tailnet.
type dbBdSource struct {
	db *sql.DB
}

// NewDbBdSource opens a connection pool for plans.projection bd queries. The
// caller owns Close(). dsn examples:
//   - "root:@tcp(127.0.0.1:3306)/bushido"        (from bushido itself)
//   - "root:@tcp(100.109.17.108:3306)/bushido"   (from Mac via tailnet)
func NewDbBdSource(dsn string) (*dbBdSource, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("plans.projection db open: %w", err)
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	return &dbBdSource{db: db}, nil
}

// Close releases the underlying connection pool.
func (s *dbBdSource) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// QueryEpics implements PlansBdSource.QueryEpics by reading the `issues`
// table on the shared bushido database. Filters to type='epic' and (when
// issuePrefix is set) the prefix-config recorded by bd.
func (s *dbBdSource) QueryEpics(ctx context.Context, projectID, issuePrefix string) ([]PlansProjectionEntry, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("plans.projection db source: not initialised")
	}
	const query = `SELECT id, title, status, priority, issue_type, updated_at
		FROM issues
		WHERE _project_id = ?
		  AND issue_type = 'epic'
		  AND (? = '' OR id LIKE CONCAT(?, '-%'))`
	rows, err := s.db.QueryContext(ctx, query, projectID, issuePrefix, issuePrefix)
	if err != nil {
		return nil, fmt.Errorf("plans.projection db query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []PlansProjectionEntry
	for rows.Next() {
		var entry PlansProjectionEntry
		var priority sql.NullString
		if err := rows.Scan(&entry.BeadsID, &entry.Title, &entry.Status, &priority, &entry.IssueType, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("plans.projection db scan: %w", err)
		}
		if priority.Valid {
			entry.Priority = priority.String
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("plans.projection db rows: %w", err)
	}
	return out, nil
}

// compile-time assertion that PlansProjectionExecutor satisfies JobExecutor.
var _ JobExecutor = (*PlansProjectionExecutor)(nil)
