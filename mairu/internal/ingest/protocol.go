package ingest

import (
	"bufio"
	"encoding/json"
	"io"
	"time"
)

// Record represents a single shell command execution captured for ingestion.
//
// Output is populated only by opt-in capture paths (e.g. `mairu capture`).
// The shell-hook client never sets it — metadata-only capture is the
// default for the always-on real-time path because command output is the
// highest-leakage surface in shell history.
type Record struct {
	Command    string    `json:"command"`
	ExitCode   int       `json:"exit_code,omitempty"`
	DurationMs int       `json:"duration_ms,omitempty"`
	Cwd        string    `json:"cwd,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Project    string    `json:"project,omitempty"`
	Output     string    `json:"output,omitempty"`
}

// Encode writes rec as a single JSON object on one line, terminated by '\n'.
func Encode(w io.Writer, rec Record) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}

// Decode reads a single line from r and unmarshals it into a Record.
// Returns io.EOF cleanly when the stream is exhausted.
func Decode(r *bufio.Reader) (Record, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		if err == io.EOF && len(line) == 0 {
			return Record{}, io.EOF
		}
		if err != io.EOF {
			return Record{}, err
		}
		// Partial last line with no trailing newline — still try to parse.
	}
	var rec Record
	if jsonErr := json.Unmarshal(line, &rec); jsonErr != nil {
		return Record{}, jsonErr
	}
	return rec, nil
}
