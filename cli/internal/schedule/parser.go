// Package schedule parses .agents/schedule.yaml into recurring job templates
// consumed by agentopsd. Pre-mortem amendment B4 (DoS protection + schema strictness)
// requires:
//   - strict YAML decoding (unknown fields rejected)
//   - duplicate name rejection
//   - cron validation via daemon.ParseCron (5-field standard, no sub-minute)
//   - job_type pattern enforcement (^[a-z]+\.[a-z]+$)
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

var jobTypePattern = regexp.MustCompile(`^[a-z]+\.[a-z]+$`)

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
		if entry.Name == "" {
			return nil, newErr(path, field+".name", fmt.Errorf("missing name"))
		}
		if _, dup := seen[entry.Name]; dup {
			return nil, newErr(path, field+".name", fmt.Errorf("duplicate name %q", entry.Name))
		}
		seen[entry.Name] = struct{}{}

		if !jobTypePattern.MatchString(entry.JobType) {
			return nil, newErr(path, field+".job_type",
				fmt.Errorf("invalid job_type %q: must match ^[a-z]+\\.[a-z]+$", entry.JobType))
		}

		sched, cronErr := daemon.ParseCron(entry.Cron)
		if cronErr != nil {
			return nil, newErr(path, field+".cron", cronErr)
		}

		// Effective-period floor: measure two consecutive ticks and ensure the gap
		// meets the minimum. ParseCron already rejects 6-field (sub-minute) cron
		// expressions, so this primarily catches operator-tightened minimums.
		now := time.Now()
		first := sched.Next(now)
		second := sched.Next(first)
		if gap := second.Sub(first); gap < minPeriod {
			return nil, newErr(path, field+".cron",
				fmt.Errorf("effective period %s is below minimum %s (set via %s)",
					gap, minPeriod, EnvMinPeriodSeconds))
		}

		var timeout time.Duration
		if entry.Timeout != "" {
			d, perr := time.ParseDuration(entry.Timeout)
			if perr != nil {
				return nil, newErr(path, field+".timeout",
					fmt.Errorf("invalid timeout %q: %w", entry.Timeout, perr))
			}
			timeout = d
		}

		var bp daemon.RecurrenceBackpressure
		if entry.Backpressure != nil {
			if entry.Backpressure.MaxQueueDepth > maxQueueDepth {
				return nil, newErr(path, field+".backpressure.max_queue_depth",
					fmt.Errorf("max_queue_depth %d exceeds ceiling %d (set via %s)",
						entry.Backpressure.MaxQueueDepth, maxQueueDepth, EnvMaxQueueDepthCeiling))
			}
			bp = daemon.RecurrenceBackpressure{
				SkipIfRunning: entry.Backpressure.SkipIfRunning,
				MaxQueueDepth: entry.Backpressure.MaxQueueDepth,
			}
		}

		var payload json.RawMessage
		if !payloadNodeEmpty(entry.Payload) {
			// Re-encode through JSON so the daemon receives canonical bytes.
			var v any
			if err := entry.Payload.Decode(&v); err != nil {
				return nil, newErr(path, field+".payload",
					fmt.Errorf("decode payload: %w", err))
			}
			b, err := json.Marshal(v)
			if err != nil {
				return nil, newErr(path, field+".payload",
					fmt.Errorf("marshal payload: %w", err))
			}
			payload = b
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
			return nil, newErr(path, field+".payload", err)
		}
		out = append(out, template)
	}

	return out, nil
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
