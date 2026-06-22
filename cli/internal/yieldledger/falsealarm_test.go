package yieldledger

import "testing"

func TestDetectFalseAlarms_SameSHAReversalIsAlarm(t *testing.T) {
	root := t.TempDir()
	const run = "r-fa"
	// REFUTED at attempt 1, then CONFIRMED at attempt 2 on the SAME head_sha — the
	// membrane reversed itself on unchanged code = a cry-wolf false alarm.
	gv(t, root, run, "age-cry", DispositionRefuted, "samesha1", 1, nil)
	gv(t, root, run, "age-cry", DispositionConfirmed, "samesha1", 2, nil)

	l, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := DetectFalseAlarms(l, run)
	if len(got) != 1 {
		t.Fatalf("DetectFalseAlarms = %d, want 1: %+v", len(got), got)
	}
	if got[0].BeadID != "age-cry" || got[0].HeadSHA != "samesha1" {
		t.Fatalf("alarm = %+v, want age-cry @ samesha1", got[0])
	}
}

func TestDetectFalseAlarms_DifferentSHAIsReworkNotAlarm(t *testing.T) {
	root := t.TempDir()
	const run = "r-rework"
	// REFUTED at sha A, then CONFIRMED at a DIFFERENT sha B — the author FIXED the
	// work. The membrane was right; this is rework, NOT a false alarm.
	gv(t, root, run, "age-fixed", DispositionRefuted, "shaaaaa1", 1, nil)
	gv(t, root, run, "age-fixed", DispositionConfirmed, "shbbbbb2", 2, nil)

	l, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := DetectFalseAlarms(l, run); len(got) != 0 {
		t.Fatalf("DetectFalseAlarms = %d, want 0 (different sha is rework, not cry-wolf): %+v", len(got), got)
	}
}

func TestDetectFalseAlarms_EscapeIsNotFalseAlarm(t *testing.T) {
	root := t.TempDir()
	const run = "r-esc"
	// CONFIRMED then REFUTED is an ESCAPE (the other side), never a false alarm.
	gv(t, root, run, "age-escapee", DispositionConfirmed, "hhhhhh1", 1, nil)
	gv(t, root, run, "age-escapee", DispositionRefuted, "hhhhhh2", 2, nil)
	// A lone REFUTED with no later confirm is neither.
	gv(t, root, run, "age-stuck", DispositionRefuted, "hhhhhh3", 1, nil)

	l, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := DetectFalseAlarms(l, run); len(got) != 0 {
		t.Fatalf("DetectFalseAlarms = %d, want 0 (escape + lone-refute are not alarms): %+v", len(got), got)
	}
}
