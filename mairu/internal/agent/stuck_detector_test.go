package agent

import (
	"testing"
)

func sig(name string, args map[string]any) ToolSignature {
	return NewToolSignature(name, args)
}

func TestStuckDetector_RepeatedAction_OK(t *testing.T) {
	d := NewStuckDetector()
	d.Record(sig("bash", map[string]any{"command": "ls"}))
	d.Record(sig("bash", map[string]any{"command": "ls"}))
	if v := d.Check(); v != VerdictOK {
		t.Fatalf("expected OK after 2 identical calls, got %v", v)
	}
}

func TestStuckDetector_RepeatedAction_Nudge(t *testing.T) {
	d := NewStuckDetector()
	for i := 0; i < 4; i++ {
		d.Record(sig("bash", map[string]any{"command": "ls"}))
	}
	if v := d.Check(); v != VerdictNudge {
		t.Fatalf("expected Nudge after 4 identical calls, got %v", v)
	}
}

func TestStuckDetector_RepeatedAction_NudgeAgain(t *testing.T) {
	d := NewStuckDetector()
	for i := 0; i < 5; i++ {
		d.Record(sig("bash", map[string]any{"command": "ls"}))
	}
	if v := d.Check(); v != VerdictNudge {
		t.Fatalf("expected Nudge at count=5, got %v", v)
	}
}

func TestStuckDetector_RepeatedAction_Stop(t *testing.T) {
	d := NewStuckDetector()
	for i := 0; i < 8; i++ {
		d.Record(sig("bash", map[string]any{"command": "ls"}))
	}
	if v := d.Check(); v != VerdictStop {
		t.Fatalf("expected Stop after 8 identical calls, got %v", v)
	}
}

func TestStuckDetector_Alternating_Nudge(t *testing.T) {
	d := NewStuckDetector()
	a := sig("read_file", map[string]any{"file_path": "main.go"})
	b := sig("bash", map[string]any{"command": "go build ./..."})
	// Nudge threshold is 5 total items
	d.Record(a)
	d.Record(b)
	d.Record(a)
	d.Record(b)
	d.Record(a)
	d.Record(b)
	
	if v := d.Check(); v != VerdictNudge {
		t.Fatalf("expected Nudge after 5 alternating calls, got %v", v)
	}
}

func TestStuckDetector_Alternating_Stop(t *testing.T) {
	d := NewStuckDetector()
	a := sig("read_file", map[string]any{"file_path": "main.go"})
	b := sig("bash", map[string]any{"command": "go build ./..."})
	// Stop threshold is 10 total items
	for i := 0; i < 5; i++ {
		d.Record(a)
		d.Record(b)
	}
	if v := d.Check(); v != VerdictStop {
		t.Fatalf("expected Stop after 10 alternating calls, got %v", v)
	}
}

func TestStuckDetector_MixedCalls_OK(t *testing.T) {
	d := NewStuckDetector()
	a := sig("read_file", map[string]any{"file_path": "main.go"})
	b := sig("bash", map[string]any{"command": "go build ./..."})
	c := sig("bash", map[string]any{"command": "ls"})

	for i := 0; i < 3; i++ {
		d.Record(a)
		d.Record(b)
		d.Record(c)
	}
	if v := d.Check(); v != VerdictOK {
		t.Fatalf("expected OK for mixed pattern without high repetition, got %v", v)
	}
}

func TestStuckDetector_PatternBreaks_AfterNudge(t *testing.T) {
	d := NewStuckDetector()
	for i := 0; i < 4; i++ {
		d.Record(sig("bash", map[string]any{"command": "ls"}))
	}
	if v := d.Check(); v != VerdictNudge {
		t.Fatalf("expected Nudge, got %v", v)
	}

	// Pattern breaks
	d.Record(sig("bash", map[string]any{"command": "echo 'fixed'"}))
	if v := d.Check(); v != VerdictOK {
		t.Fatalf("expected OK after breaking pattern, got %v", v)
	}
}

func TestStuckDetector_DifferentArgs_NotRepeated(t *testing.T) {
	d := NewStuckDetector()
	d.Record(sig("read_file", map[string]any{"file_path": "file1.go"}))
	d.Record(sig("read_file", map[string]any{"file_path": "file2.go"}))
	d.Record(sig("read_file", map[string]any{"file_path": "file3.go"}))
	d.Record(sig("read_file", map[string]any{"file_path": "file4.go"}))
	d.Record(sig("read_file", map[string]any{"file_path": "file5.go"}))

	if v := d.Check(); v != VerdictOK {
		t.Fatalf("expected OK for different arguments, got %v", v)
	}
}

func TestStuckDetector_BatchRecord(t *testing.T) {
	d := NewStuckDetector()
	sig1 := sig("bash", map[string]any{"command": "pwd"})
	sigs := []ToolSignature{sig1, sig1, sig1, sig1}

	d.RecordBatch(sigs)
	if v := d.Check(); v != VerdictNudge {
		t.Fatalf("expected Nudge after batch of 4 identical, got %v", v)
	}
}

func TestStuckDetector_Reset(t *testing.T) {
	d := NewStuckDetector()
	for i := 0; i < 8; i++ {
		d.Record(sig("bash", map[string]any{"command": "ls"}))
	}
	if v := d.Check(); v != VerdictStop {
		t.Fatalf("expected Stop before reset, got %v", v)
	}

	d.Reset()
	if v := d.Check(); v != VerdictOK {
		t.Fatalf("expected OK after reset, got %v", v)
	}
}
