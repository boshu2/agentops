// Package schedule parses .agents/schedule.yaml into recurring job templates
// consumed by agentopsd. Pre-mortem amendment B4 (DoS protection + schema strictness)
// requires:
//   - strict YAML decoding (unknown fields rejected)
//   - duplicate name rejection
//   - cron validation via daemon.ParseCron (5-field standard, no sub-minute)
//   - job_type pattern enforcement (^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$)
//   - effective-period floor (AGENTOPS_SCHEDULE_MIN_PERIOD_SECONDS, default 60s)
//   - max_queue_depth ceiling (AGENTOPS_SCHEDULE_MAX_QUEUE_DEPTH_CEILING, default 1000)
package schedule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/daemon"
)

const (
	// EnvMinPeriodSeconds overrides the minimum effective cron period in seconds.
	EnvMinPeriodSeconds = "AGENTOPS_SCHEDULE_MIN_PERIOD_SECONDS"
	// EnvMaxQueueDepthCeiling overrides the maximum permissible max_queue_depth.
	EnvMaxQueueDepthCeiling = "AGENTOPS_SCHEDULE_MAX_QUEUE_DEPTH_CEILING"

	defaultMinPeriodSeconds     = 60
	defaultMaxQueueDepthCeiling = 1000
)

var jobTypePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$`)

// fileShape mirrors the top-level structure of .agents/schedule.yaml. yaml.v3 with
// KnownFields(true) rejects any unknown top-level fields.
type fileShape struct {
	Schedules []scheduleEntry `yaml:"schedules"`
}

// scheduleEntry mirrors a single schedule item. Unknown fields per-entry are also
// rejected by KnownFields(true).
type scheduleEntry struct {
	Name         string             `yaml:"name"`
	Cron         string             `yaml:"cron"`
	JobType      string             `yaml:"job_type"`
	Payload      yaml.Node          `yaml:"payload,omitempty"`
	Timeout      string             `yaml:"timeout,omitempty"`
	Backpressure *backpressureEntry `yaml:"backpressure,omitempty"`
}

type backpressureEntry struct {
	SkipIfRunning bool `yaml:"skip_if_running,omitempty"`
	MaxQueueDepth int  `yaml:"max_queue_depth,omitempty"`
}

// LoadError is returned for any validation failure. It carries the file path and
// (when applicable) the offending field name so operators can act on the message.
type LoadError struct {
	Path  string
	Field string
	Err   error
}

func (e *LoadError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("schedule: %s: %s: %v", e.Path, e.Field, e.Err)
	}
	return fmt.Sprintf("schedule: %s: %v", e.Path, e.Err)
}

func (e *LoadError) Unwrap() error { return e.Err }

func newErr(path, field string, err error) error {
	return &LoadError{Path: path, Field: field, Err: err}
}

// Load reads the schedule file at path, validates it, and returns the parsed
// recurring job templates. All validation per amendment B4 is enforced here.
func Load(path string) ([]daemon.RecurringJobTemplate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, newErr(path, "", fmt.Errorf("read: %w", err))
	}

	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)

	var shape fileShape
	if err := dec.Decode(&shape); err != nil {
		return nil, newErr(path, "", fmt.Errorf("decode: %w", err))
	}

	minPeriod, err := envDurationSeconds(EnvMinPeriodSeconds, defaultMinPeriodSeconds)
	if err != nil {
		return nil, newErr(path, EnvMinPeriodSeconds, err)
	}
	maxQueueDepth, err := envInt(EnvMaxQueueDepthCeiling, defaultMaxQueueDepthCeiling)
	if err != nil {
		return nil, newErr(path, EnvMaxQueueDepthCeiling, err)
	}

	seen := make(map[string]struct{}, len(shape.Schedules))
	out := make([]daemon.RecurringJobTemplate, 0, len(shape.Schedules))

	for i, entry := range shape.Schedules {
		field := fmt.Sprintf("schedules[%d]", i)
		template, err := parseScheduleEntry(path, field, entry, seen, minPeriod, maxQueueDepth)
		if err != nil {
			return nil, err
		}
		out = append(out, template)
	}

	return out, nil
}

// parseScheduleEntry validates a single schedule entry and returns its compiled
// RecurringJobTemplate. seen is updated to track the entry's name so duplicate
// names within the same file fail. Extracted from Load to keep cyclomatic
// complexity below the cli/internal/ ceiling of 18.
func parseScheduleEntry(
	path, field string,
	entry scheduleEntry,
	seen map[string]struct{},
	minPeriod time.Duration,
	maxQueueDepth int,
) (daemon.RecurringJobTemplate, error) {
	if entry.Name == "" {
		return daemon.RecurringJobTemplate{}, newErr(path, field+".name", fmt.Errorf("missing name"))
	}
	if _, dup := seen[entry.Name]; dup {
		return daemon.RecurringJobTemplate{}, newErr(path, field+".name", fmt.Errorf("duplicate name %q", entry.Name))
	}
	seen[entry.Name] = struct{}{}

	if !jobTypePattern.MatchString(entry.JobType) {
		return daemon.RecurringJobTemplate{}, newErr(path, field+".job_type",
			fmt.Errorf("invalid job_type %q: must match ^[a-z][a-z0-9-]*\\.[a-z][a-z0-9-]*$", entry.JobType))
	}

	if err := validateScheduleEntryCron(path, field, entry.Cron, minPeriod); err != nil {
		return daemon.RecurringJobTemplate{}, err
	}

	timeout, err := parseScheduleEntryTimeout(path, field, entry.Timeout)
	if err != nil {
		return daemon.RecurringJobTemplate{}, err
	}

	bp, err := buildScheduleEntryBackpressure(path, field, entry.Backpressure, maxQueueDepth)
	if err != nil {
		return daemon.RecurringJobTemplate{}, err
	}

	payload, err := encodeScheduleEntryPayload(path, field, entry.Payload)
	if err != nil {
		return daemon.RecurringJobTemplate{}, err
	}

	template := daemon.RecurringJobTemplate{
		Name:         entry.Name,
		Cron:         entry.Cron,
		JobType:      daemon.JobType(entry.JobType),
		Payload:      payload,
		Timeout:      timeout,
		Backpressure: bp,
	}
	if err := daemon.ValidateRecurringJobTemplatePayload(template); err != nil {
		return daemon.RecurringJobTemplate{}, newErr(path, field+".payload", err)
	}
	return template, nil
}

// validateScheduleEntryCron parses the cron expression and ensures the
// effective period between consecutive ticks meets the configured minimum.
func validateScheduleEntryCron(path, field, cron string, minPeriod time.Duration) error {
	sched, cronErr := daemon.ParseCron(cron)
	if cronErr != nil {
		return newErr(path, field+".cron", cronErr)
	}
	now := time.Now()
	first := sched.Next(now)
	second := sched.Next(first)
	if gap := second.Sub(first); gap < minPeriod {
		return newErr(path, field+".cron",
			fmt.Errorf("effective period %s is below minimum %s (set via %s)",
				gap, minPeriod, EnvMinPeriodSeconds))
	}
	return nil
}

func parseScheduleEntryTimeout(path, field, raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	d, perr := time.ParseDuration(raw)
	if perr != nil {
		return 0, newErr(path, field+".timeout",
			fmt.Errorf("invalid timeout %q: %w", raw, perr))
	}
	return d, nil
}

func buildScheduleEntryBackpressure(
	path, field string,
	in *backpressureEntry,
	maxQueueDepth int,
) (daemon.RecurrenceBackpressure, error) {
	if in == nil {
		return daemon.RecurrenceBackpressure{}, nil
	}
	if in.MaxQueueDepth > maxQueueDepth {
		return daemon.RecurrenceBackpressure{}, newErr(path, field+".backpressure.max_queue_depth",
			fmt.Errorf("max_queue_depth %d exceeds ceiling %d (set via %s)",
				in.MaxQueueDepth, maxQueueDepth, EnvMaxQueueDepthCeiling))
	}
	return daemon.RecurrenceBackpressure{
		SkipIfRunning: in.SkipIfRunning,
		MaxQueueDepth: in.MaxQueueDepth,
	}, nil
}

func encodeScheduleEntryPayload(path, field string, node yaml.Node) (json.RawMessage, error) {
	if payloadNodeEmpty(node) {
		return nil, nil
	}
	var v any
	if err := node.Decode(&v); err != nil {
		return nil, newErr(path, field+".payload",
			fmt.Errorf("decode payload: %w", err))
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, newErr(path, field+".payload",
			fmt.Errorf("marshal payload: %w", err))
	}
	return b, nil
}

// payloadNodeEmpty returns true when the yaml.Node has no content (key absent in
// the source). yaml.v3 leaves the node Kind as 0 in that case.
func payloadNodeEmpty(n yaml.Node) bool {
	return n.Kind == 0
}

func envInt(key string, def int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, raw, err)
	}
	if v <= 0 {
		return 0, fmt.Errorf("invalid %s=%q: must be > 0", key, raw)
	}
	return v, nil
}

func envDurationSeconds(key string, defSeconds int) (time.Duration, error) {
	v, err := envInt(key, defSeconds)
	if err != nil {
		return 0, err
	}
	return time.Duration(v) * time.Second, nil
}
