package daemon

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const DefaultTelemetryWindow = 24 * time.Hour

type LedgerTelemetry struct {
	EventCount             int                         `json:"event_count"`
	Window                 string                      `json:"window"`
	PhaseLatency           []PhaseLatencyHistogram     `json:"phase_latency,omitempty"`
	WorkerKindDistribution []WorkerKindDistribution    `json:"worker_kind_distribution,omitempty"`
	FailureRates           []JobTypeFailureRateSummary `json:"failure_rates,omitempty"`
}

type PhaseLatencyHistogram struct {
	PhaseName string `json:"phase_name"`
	Count     int    `json:"count"`
	P50Millis int64  `json:"p50_ms"`
	P99Millis int64  `json:"p99_ms"`
}

type WorkerKindDistribution struct {
	WorkerKind string `json:"worker_kind"`
	Count      int    `json:"count"`
}

type JobTypeFailureRateSummary struct {
	JobType       JobType `json:"job_type"`
	TerminalCount int     `json:"terminal_count"`
	FailedCount   int     `json:"failed_count"`
	FailureRate   float64 `json:"failure_rate"`
}

type telemetryJob struct {
	jobType    JobType
	phaseName  string
	workerKind string
	acceptedAt time.Time
	terminalAt time.Time
	failed     bool
	cancelled  bool
	completed  bool
}

func BuildLedgerTelemetry(events []LedgerEvent, now time.Time, window time.Duration) LedgerTelemetry {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	if window <= 0 {
		window = DefaultTelemetryWindow
	}
	jobs := map[string]*telemetryJob{}
	workerCounts := map[string]int{}
	windowStart := now.Add(-window)

	for _, event := range events {
		occurredAt, hasTime := parseLedgerEventTime(event)
		job := jobs[event.JobID]
		if job == nil {
			job = &telemetryJob{}
			jobs[event.JobID] = job
		}
		applyTelemetryPayload(job, event.Payload)
		switch event.EventType {
		case EventJobAccepted:
			if hasTime {
				job.acceptedAt = occurredAt
				if job.workerKind != "" && !occurredAt.Before(windowStart) && !occurredAt.After(now) {
					workerCounts[job.workerKind]++
				}
			}
		case EventJobCompleted:
			if hasTime {
				job.terminalAt = occurredAt
			}
			job.completed = true
		case EventJobFailed:
			if hasTime {
				job.terminalAt = occurredAt
			}
			job.failed = true
		case EventJobCancelled:
			if hasTime {
				job.terminalAt = occurredAt
			}
			job.cancelled = true
		}
	}

	return LedgerTelemetry{
		EventCount:             len(events),
		Window:                 window.String(),
		PhaseLatency:           buildPhaseLatencyHistograms(jobs),
		WorkerKindDistribution: buildWorkerKindDistribution(workerCounts),
		FailureRates:           buildFailureRates(jobs),
	}
}

func FormatLedgerTelemetrySummary(telemetry LedgerTelemetry) string {
	parts := []string{fmt.Sprintf("events=%d", telemetry.EventCount)}
	if len(telemetry.PhaseLatency) == 0 {
		parts = append(parts, "phase_latency=none")
	} else {
		values := make([]string, 0, len(telemetry.PhaseLatency))
		for _, hist := range telemetry.PhaseLatency {
			values = append(values, fmt.Sprintf("%s n=%d p50=%s p99=%s",
				hist.PhaseName,
				hist.Count,
				time.Duration(hist.P50Millis)*time.Millisecond,
				time.Duration(hist.P99Millis)*time.Millisecond,
			))
		}
		parts = append(parts, "phase_latency="+strings.Join(values, ", "))
	}
	if len(telemetry.WorkerKindDistribution) == 0 {
		parts = append(parts, "worker_kind_24h=none")
	} else {
		values := make([]string, 0, len(telemetry.WorkerKindDistribution))
		for _, dist := range telemetry.WorkerKindDistribution {
			values = append(values, fmt.Sprintf("%s=%d", dist.WorkerKind, dist.Count))
		}
		parts = append(parts, "worker_kind_24h="+strings.Join(values, ", "))
	}
	if len(telemetry.FailureRates) == 0 {
		parts = append(parts, "failure_rate=none")
	} else {
		values := make([]string, 0, len(telemetry.FailureRates))
		for _, rate := range telemetry.FailureRates {
			values = append(values, fmt.Sprintf("%s %d/%d %.1f%%",
				rate.JobType,
				rate.FailedCount,
				rate.TerminalCount,
				rate.FailureRate*100,
			))
		}
		parts = append(parts, "failure_rate="+strings.Join(values, ", "))
	}
	return strings.Join(parts, "; ")
}

func parseLedgerEventTime(event LedgerEvent) (time.Time, bool) {
	occurredAt, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if err != nil {
		return time.Time{}, false
	}
	return occurredAt.UTC(), true
}

func applyTelemetryPayload(job *telemetryJob, payload map[string]any) {
	if jobType, ok, err := jobTypeFromPayload(payload); err == nil && ok {
		job.jobType = jobType
	}
	jobPayload := nestedPayload(payload, "job_payload")
	if len(jobPayload) == 0 {
		jobPayload = payload
	}
	if jobType, ok, err := jobTypeFromPayload(jobPayload); err == nil && ok {
		job.jobType = jobType
	}
	if phaseName, ok := stringPayload(jobPayload, "phase_name"); ok {
		job.phaseName = phaseName
	} else if phase, ok := intPayload(jobPayload, "phase"); ok {
		job.phaseName = RPIPhaseName(phase)
	}
	if workerKind, ok := stringPayload(jobPayload, "worker_kind"); ok {
		job.workerKind = workerKind
	} else if workerKind, ok := stringPayload(payload, "worker_kind"); ok {
		job.workerKind = workerKind
	}
}

func buildPhaseLatencyHistograms(jobs map[string]*telemetryJob) []PhaseLatencyHistogram {
	byPhase := map[string][]time.Duration{}
	for _, job := range jobs {
		if job.phaseName == "" || job.acceptedAt.IsZero() || job.terminalAt.IsZero() {
			continue
		}
		latency := job.terminalAt.Sub(job.acceptedAt)
		if latency < 0 {
			continue
		}
		byPhase[job.phaseName] = append(byPhase[job.phaseName], latency)
	}
	phases := make([]string, 0, len(byPhase))
	for phase := range byPhase {
		phases = append(phases, phase)
	}
	sort.Strings(phases)
	out := make([]PhaseLatencyHistogram, 0, len(phases))
	for _, phase := range phases {
		values := byPhase[phase]
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		out = append(out, PhaseLatencyHistogram{
			PhaseName: phase,
			Count:     len(values),
			P50Millis: percentileDuration(values, 0.50).Milliseconds(),
			P99Millis: percentileDuration(values, 0.99).Milliseconds(),
		})
	}
	return out
}

func buildWorkerKindDistribution(counts map[string]int) []WorkerKindDistribution {
	kinds := make([]string, 0, len(counts))
	for kind := range counts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	out := make([]WorkerKindDistribution, 0, len(kinds))
	for _, kind := range kinds {
		out = append(out, WorkerKindDistribution{WorkerKind: kind, Count: counts[kind]})
	}
	return out
}

func buildFailureRates(jobs map[string]*telemetryJob) []JobTypeFailureRateSummary {
	type counts struct {
		terminal int
		failed   int
	}
	byType := map[JobType]counts{}
	for _, job := range jobs {
		if job.jobType == "" || (!job.completed && !job.failed && !job.cancelled) {
			continue
		}
		count := byType[job.jobType]
		count.terminal++
		if job.failed {
			count.failed++
		}
		byType[job.jobType] = count
	}
	jobTypes := make([]string, 0, len(byType))
	for jobType := range byType {
		jobTypes = append(jobTypes, string(jobType))
	}
	sort.Strings(jobTypes)
	out := make([]JobTypeFailureRateSummary, 0, len(jobTypes))
	for _, raw := range jobTypes {
		jobType := JobType(raw)
		count := byType[jobType]
		rate := 0.0
		if count.terminal > 0 {
			rate = float64(count.failed) / float64(count.terminal)
		}
		out = append(out, JobTypeFailureRateSummary{
			JobType:       jobType,
			TerminalCount: count.terminal,
			FailedCount:   count.failed,
			FailureRate:   rate,
		})
	}
	return out
}

func percentileDuration(values []time.Duration, pct float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	if pct <= 0 {
		return values[0]
	}
	index := int(math.Ceil(pct*float64(len(values)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}
