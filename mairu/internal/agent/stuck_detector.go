package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

type StuckVerdict int

const (
	VerdictOK StuckVerdict = iota
	VerdictNudge
	VerdictStop
)

func (v StuckVerdict) String() string {
	switch v {
	case VerdictNudge:
		return "nudge"
	case VerdictStop:
		return "stop"
	default:
		return "ok"
	}
}

const (
	repeatNudgeThreshold      = 3
	repeatStopThreshold       = 5
	alternatingNudgeThreshold = 6
	alternatingStopThreshold  = 8
	maxWindowSize             = 10
)

type ToolSignature struct {
	Name     string
	ArgsHash string
}

func NewToolSignature(name string, args map[string]any) ToolSignature {
	raw, err := json.Marshal(args)
	if err != nil {
		panic(fmt.Sprintf("stuck_detector: cannot marshal args for tool %q: %v", name, err))
	}
	h := sha256.Sum256(raw)
	return ToolSignature{Name: name, ArgsHash: fmt.Sprintf("%x", h)}
}

type StuckDetector struct {
	history []ToolSignature
}

func NewStuckDetector() *StuckDetector {
	return &StuckDetector{}
}

func (d *StuckDetector) Record(sig ToolSignature) {
	d.history = append(d.history, sig)
	if len(d.history) > maxWindowSize {
		d.history = d.history[len(d.history)-maxWindowSize:]
	}
}

// Reset clears the history. Call this when starting a new user turn.
func (d *StuckDetector) Reset() {
	d.history = d.history[:0]
}

func (d *StuckDetector) RecordBatch(sigs []ToolSignature) {
	for _, s := range sigs {
		d.Record(s)
	}
}

func (d *StuckDetector) Check() StuckVerdict {
	if v := d.checkRepeated(); v != VerdictOK {
		return v
	}
	return d.checkAlternating()
}

func NudgeMessage() string {
	return "WARNING: You appear to be stuck in a loop — you have repeated the same action multiple times. " +
		"Stop and try a fundamentally different approach to solve this problem."
}

func StopMessage() string {
	return "Agent stopped: stuck in a repeated loop after multiple warnings. " +
		"Try rephrasing your request or breaking it into smaller steps."
}

func (d *StuckDetector) checkRepeated() StuckVerdict {
	n := len(d.history)
	if n < repeatNudgeThreshold {
		return VerdictOK
	}

	last := d.history[n-1]
	identicalCount := 0
	for i := n - 1; i >= 0; i-- {
		if d.history[i] == last {
			identicalCount++
		} else {
			break
		}
	}

	if identicalCount >= repeatStopThreshold {
		return VerdictStop
	}
	if identicalCount >= repeatNudgeThreshold {
		return VerdictNudge
	}
	return VerdictOK
}

func (d *StuckDetector) checkAlternating() StuckVerdict {
	n := len(d.history)
	if n < alternatingNudgeThreshold {
		return VerdictOK
	}

	a := d.history[n-2]
	b := d.history[n-1]
	if a == b {
		return VerdictOK
	}

	// Count consecutive (A, B) pairs walking backwards from the end.
	// Pairs occupy indices (n-2, n-1), (n-4, n-3), (n-6, n-5), ...
	pairCount := 0
	for i := n - 1; i >= 1; i -= 2 {
		if d.history[i-1] == a && d.history[i] == b {
			pairCount++
		} else {
			break
		}
	}

	totalAlternating := pairCount * 2
	if totalAlternating >= alternatingStopThreshold {
		return VerdictStop
	}
	if totalAlternating >= alternatingNudgeThreshold {
		return VerdictNudge
	}
	return VerdictOK
}
