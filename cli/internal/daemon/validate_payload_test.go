package daemon

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

// TestSubmitJob_RejectsMalformedPayloadBeforeAppend is the L2 contract for
// soc-qra05: a payload whose fields carry the wrong JSON type for a known
// JobType's schema must be rejected BEFORE the ledger append, so the malformed
// payload never contaminates the ledger.
func TestSubmitJob_RejectsMalformedPayloadBeforeAppend(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute, MaxAttempts: 3})

	// start_phase is an int field in RPIRunJobSpec; a string there is a shape
	// violation that would make the event unparseable on replay.
	_, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-malformed",
		JobID:     "job-malformed",
		JobType:   JobTypeRPIRun,
		Actor:     "ao",
		Payload:   map[string]any{"goal": "x", "start_phase": "not-a-number"},
	}, QueueMutationOptions{})
	if err == nil {
		t.Fatalf("SubmitJob with malformed payload returned nil error, want rejection")
	}
	if !errors.Is(err, ErrInvalidJobPayload) {
		t.Fatalf("error = %v, want errors.Is(ErrInvalidJobPayload)", err)
	}

	// The ledger must be untouched: no event was appended for the rejected
	// submission.
	events := readTestQueueEvents(t, queue)
	if len(events) != 0 {
		t.Fatalf("rejected SubmitJob appended %d events, want 0: %+v", len(events), events)
	}
}

// TestSubmitJob_AcceptsValidPayloadAndProcesses pins that a structurally-valid
// (and historically-minimal) payload still appends exactly one accepted event
// and remains claimable — i.e. the new guard preserves existing valid behavior.
func TestSubmitJob_AcceptsValidPayloadAndProcesses(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute, MaxAttempts: 3})

	submitted, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-valid",
		JobID:     "job-valid",
		JobType:   JobTypeRPIRun,
		Actor:     "ao",
		Payload:   map[string]any{"goal": "ship daemon"},
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("SubmitJob with valid payload returned error: %v", err)
	}
	if submitted.Status != JobStatusQueued {
		t.Fatalf("submitted status = %q, want %q", submitted.Status, JobStatusQueued)
	}

	events := readTestQueueEvents(t, queue)
	if len(events) != 1 {
		t.Fatalf("valid SubmitJob appended %d events, want 1: %+v", len(events), events)
	}
	if events[0].EventType != EventJobAccepted {
		t.Fatalf("appended event type = %q, want %q", events[0].EventType, EventJobAccepted)
	}

	// And it is still processable end-to-end: it can be claimed.
	claim, err := queue.ClaimNext("worker-1", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("ClaimNext after valid submit: %v", err)
	}
	if claim.Job.JobID != "job-valid" || claim.Job.Status != JobStatusRunning {
		t.Fatalf("claim = %#v, want running job-valid", claim.Job)
	}
}

// TestMutationSubmitMalformedPayloadReturnsBadRequest is the HTTP-path sibling:
// a malformed payload for a known JobType returns 400 and leaves the ledger
// empty. Mirrors TestMutationSubmitInvalidJobTypeReturnsBadRequest.
func TestMutationSubmitMalformedPayloadReturnsBadRequest(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)

	resp := postJob(t, router,
		`{"request_id":"req-bad","job_id":"job-bad","job_type":"rpi.run","payload":{"goal":"x","start_phase":"not-a-number"}}`,
		"secret-token", "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("malformed payload status = %d body=%s, want 400", resp.Code, resp.Body.String())
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("malformed payload wrote %d events, want 0", len(events))
	}
}

// TestMutationSubmitValidPayloadStillAppends pins that the HTTP path keeps
// accepting a valid payload after the guard: 202 and exactly one accepted event.
func TestMutationSubmitValidPayloadStillAppends(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)

	resp := postJob(t, router,
		`{"request_id":"req-ok","job_id":"job-ok","job_type":"rpi.run","payload":{"goal":"daemon"}}`,
		"secret-token", "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("valid payload status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	var body SubmitJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}
	if !body.Accepted || body.JobID != "job-ok" {
		t.Fatalf("submit response = %#v, want accepted job-ok", body)
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 || events[0].EventType != EventJobAccepted {
		t.Fatalf("ledger events = %#v, want one accepted event", events)
	}
}

// TestValidateJobPayload_ShapeMatrix locks the structural-vs-semantic boundary:
// minimal/empty payloads pass (no behavior change), type-mismatched fields are
// rejected, and JobTypes without a typed schema pass through.
func TestValidateJobPayload_ShapeMatrix(t *testing.T) {
	cases := []struct {
		name    string
		jobType JobType
		payload map[string]any
		wantErr bool
	}{
		{"rpi.run empty payload", JobTypeRPIRun, nil, false},
		{"rpi.run minimal goal", JobTypeRPIRun, map[string]any{"goal": "x"}, false},
		{"rpi.run bad start_phase", JobTypeRPIRun, map[string]any{"start_phase": "abc"}, true},
		{"rpi.run object goal", JobTypeRPIRun, map[string]any{"goal": map[string]any{"k": 1}}, true},
		{"dream.run bad max_iterations", JobTypeDreamRun, map[string]any{"max_iterations": "lots"}, true},
		{"dream.run minimal", JobTypeDreamRun, map[string]any{"dream_run_id": "d1"}, false},
		{"skill.invoke bad args element", JobTypeSkillInvoke, map[string]any{"args": []any{1, 2}}, true},
		{"skill.invoke string args", JobTypeSkillInvoke, map[string]any{"skill_name": "evolve", "args": "a b c"}, false},
		{"unknown-schema type passes", JobTypeEvalSuite, map[string]any{"anything": []any{1, "x", true}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJobPayload(tc.jobType, tc.payload)
			if tc.wantErr && err == nil {
				t.Fatalf("validateJobPayload(%s) = nil, want error", tc.jobType)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateJobPayload(%s) = %v, want nil", tc.jobType, err)
			}
			if tc.wantErr && !errors.Is(err, ErrInvalidJobPayload) {
				t.Fatalf("validateJobPayload(%s) error = %v, want errors.Is(ErrInvalidJobPayload)", tc.jobType, err)
			}
		})
	}
}
