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
	for i := 0; i < 3; i++ {
		d.Record(sig("bash", map[string]any{"command": "ls"}))
	}
	if v := d.Check(); v != VerdictNudge {
		t.Fatalf("expected Nudge after 3 identical calls, got %v", v)
	}
}

func TestStuckDetector_RepeatedAction_Stop(t *testing.T) {
	d := NewStuckDetector()
	for i := 0; i < 5; i++ {
		d.Record(sig("bash", map[string]any{"command": "ls"}))
	}
	if v := d.Check(); v != VerdictStop {
		t.Fatalf("expected Stop after 5 identical calls, got %v", v)
	}
}

func TestStuckDetector_Alternating_Nudge(t *testing.T) {
	d := NewStuckDetector()
	a := sig("read_file", map[string]any{"file_path": "main.go"})
	b := sig("bash", map[string]any{"command": "go build ./..."})
	for i := 0; i < 3; i++ {
		d.Record(a)
		d.Record(b)
	}
	if v := d.Check(); v != VerdictNudge {
		t.Fatalf("expected Nudge after 6 alternating calls, got %v", v)
	}
}

func TestStuckDetector_Alternating_Stop(t *testing.T) {
	d := NewStuckDetector()
	a := sig("read_file", map[string]any{"file_path": "main.go"})
	b := sig("bash", map[string]any{"command": "go build ./..."})
	for i := 0; i < 4; i++ {
		d.Record(a)
		d.Record(b)
	}
	if v := d.Check(); v != VerdictStop {
		t.Fatalf("expected Stop after 8 alternating calls, got %v", v)
	}
}

func TestStuckDetector_MixedCalls_OK(t *testing.T) {
	d := NewStuckDetector()
	d.Record(sig("bash", map[string]any{"command": "ls"}))
	d.Record(sig("read_file", map[string]any{"file_path": "main.go"}))
	d.Record(sig("bash", map[string]any{"command": "cat foo"}))
	if v := d.Check(); v != VerdictOK {
		t.Fatalf("expected OK for mixed calls, got %v", v)
	}
}

func TestStuckDetector_PatternBreaks_AfterNudge(t *testing.T) {
	d := NewStuckDetector()
	for i := 0; i < 3; i++ {
		d.Record(sig("bash", map[string]any{"command": "ls"}))
	}
	if v := d.Check(); v != VerdictNudge {
		t.Fatalf("expected Nudge, got %v", v)
	}
	d.Record(sig("read_file", map[string]any{"file_path": "main.go"}))
	if v := d.Check(); v != VerdictOK {
		t.Fatalf("expected OK after pattern break, got %v", v)
	}
}

func TestStuckDetector_DifferentArgs_NotRepeated(t *testing.T) {
	d := NewStuckDetector()
	d.Record(sig("bash", map[string]any{"command": "ls"}))
	d.Record(sig("bash", map[string]any{"command": "pwd"}))
	d.Record(sig("bash", map[string]any{"command": "cat x"}))
	if v := d.Check(); v != VerdictOK {
		t.Fatalf("expected OK for same tool with different args, got %v", v)
	}
}

func TestStuckDetector_BatchRecord(t *testing.T) {
	d := NewStuckDetector()
	batch := []ToolSignature{
		sig("bash", map[string]any{"command": "ls"}),
		sig("bash", map[string]any{"command": "ls"}),
		sig("bash", map[string]any{"command": "ls"}),
	}
	d.RecordBatch(batch)
	if v := d.Check(); v != VerdictNudge {
		t.Fatalf("expected Nudge after batch of 3 identical, got %v", v)
	}
}

func TestStuckDetector_Reset(t *testing.T) {
	d := NewStuckDetector()
	for i := 0; i < 5; i++ {
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
