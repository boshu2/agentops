package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBeadsRecoveryUsesScopedBrAndTransfersClaim(t *testing.T) {
	tmp := t.TempDir()
	beadsDir := filepath.Join(tmp, "_beads")
	fakeBin := filepath.Join(tmp, "bin")
	statePath := filepath.Join(tmp, "assignee.txt")
	logPath := filepath.Join(tmp, "br-calls.log")
	ledgerPath := filepath.Join(tmp, "ledger.jsonl")

	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads dir: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("agent-a"), 0o644); err != nil {
		t.Fatalf("write initial state: %v", err)
	}

	fakeBR := `#!/usr/bin/env bash
set -euo pipefail
if [ "${BEADS_DIR:-}" != "${EXPECTED_BEADS_DIR}" ]; then
  echo "wrong BEADS_DIR: ${BEADS_DIR:-}" >&2
  exit 45
fi
printf '%s|%s\n' "$BEADS_DIR" "$*" >> "$BR_LOG"
cmd="${1:-}"
shift || true
case "$cmd" in
  list)
    assignee="$(cat "$BR_STATE")"
    printf '[{"id":"ag-recover","status":"in_progress","assignee":"%s","updated_at":"2026-05-20T00:00:00Z"}]\n' "$assignee"
    ;;
  show)
    id="${1:-}"
    assignee="$(cat "$BR_STATE")"
    printf '{"id":"%s","status":"in_progress","assignee":"%s","updated_at":"2026-05-20T00:00:00Z"}\n' "$id" "$assignee"
    ;;
  update)
    id="${1:-}"
    shift || true
    actor=""
    while [ "$#" -gt 0 ]; do
      case "$1" in
        --actor)
          shift
          actor="${1:-}"
          ;;
      esac
      shift || true
    done
    if [ "$id" != "ag-recover" ]; then
      echo "unexpected id: $id" >&2
      exit 46
    fi
    if [ -z "$actor" ]; then
      echo "missing actor" >&2
      exit 47
    fi
    printf '%s' "$actor" > "$BR_STATE"
    ;;
  *)
    echo "unexpected br command: $cmd" >&2
    exit 48
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(fakeBin, "br"), []byte(fakeBR), 0o755); err != nil {
		t.Fatalf("write fake br: %v", err)
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BEADS_DIR", beadsDir)
	t.Setenv("EXPECTED_BEADS_DIR", beadsDir)
	t.Setenv("BR_STATE", statePath)
	t.Setenv("BR_LOG", logPath)

	prevThreshold := beadsStaleThresholdHours
	prevStaleJSON := beadsStaleJSON
	prevStaleNow := beadsStaleNowOverride
	prevAgent := beadsResumeAgentID
	prevLedger := beadsResumeLedgerPath
	prevResumeJSON := beadsResumeJSON
	prevResumeNow := beadsResumeNowOverride
	defer func() {
		beadsStaleThresholdHours = prevThreshold
		beadsStaleJSON = prevStaleJSON
		beadsStaleNowOverride = prevStaleNow
		beadsResumeAgentID = prevAgent
		beadsResumeLedgerPath = prevLedger
		beadsResumeJSON = prevResumeJSON
		beadsResumeNowOverride = prevResumeNow
	}()

	beadsStaleThresholdHours = 4
	beadsStaleJSON = true
	beadsStaleNowOverride = "2026-05-20T12:00:00Z"

	staleBuf := &bytes.Buffer{}
	legacyBeadsStaleCommand.SetOut(staleBuf)
	defer legacyBeadsStaleCommand.SetOut(nil)
	if err := executeBeadsStale(legacyBeadsStaleCommand, nil); err != nil {
		t.Fatalf("executeBeadsStale: %v", err)
	}
	var staleEvents []staleEvent
	if err := json.Unmarshal(staleBuf.Bytes(), &staleEvents); err != nil {
		t.Fatalf("stale output is not JSON: %v\n%s", err, staleBuf.String())
	}
	if len(staleEvents) != 1 || staleEvents[0].BeadID != "ag-recover" {
		t.Fatalf("stale events = %+v, want single ag-recover event", staleEvents)
	}
	if staleEvents[0].OriginalClaimant.ID != "agent-a" {
		t.Fatalf("original claimant = %q, want agent-a", staleEvents[0].OriginalClaimant.ID)
	}

	beadsResumeAgentID = "agent-b"
	beadsResumeLedgerPath = ledgerPath
	beadsResumeJSON = true
	beadsResumeNowOverride = "2026-05-20T12:30:00Z"

	resumeBuf := &bytes.Buffer{}
	legacyBeadsResumeCommand.SetOut(resumeBuf)
	defer legacyBeadsResumeCommand.SetOut(nil)
	if err := executeBeadsResume(legacyBeadsResumeCommand, []string{"ag-recover"}); err != nil {
		t.Fatalf("executeBeadsResume: %v", err)
	}
	assignee, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if string(assignee) != "agent-b" {
		t.Fatalf("assignee after resume = %q, want agent-b", string(assignee))
	}
	if !strings.Contains(resumeBuf.String(), `"new_claimant":{"id":"agent-b"}`) {
		t.Fatalf("resume JSON missing new claimant agent-b: %s", resumeBuf.String())
	}

	logRaw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read br call log: %v", err)
	}
	log := string(logRaw)
	for _, want := range []string{
		beadsDir + "|list --status in_progress --json --limit 500",
		beadsDir + "|show ag-recover --json",
		beadsDir + "|update ag-recover --claim --actor agent-b",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("br call log missing %q:\n%s", want, log)
		}
	}
	if _, err := beadsResumeShowFunc(context.Background(), "ag-recover"); err != nil {
		t.Fatalf("posterior br show after transfer: %v", err)
	}
}
