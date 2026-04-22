// Package history backfills the bash_history store from shell history files.
// It recognises zsh extended-history and bash history (with or without
// HISTTIMEFORMAT timestamps), redacts entries through pii-redact, and
// persists survivors via the contextsrv repository.
package history

import (
	"bufio"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Format int

const (
	FormatBash Format = iota
	FormatZsh
)

type Entry struct {
	Command    string
	Timestamp  time.Time // zero if unknown
	DurationMs int       // 0 if unknown
}

// DetectFormat picks a parser based on the history file path. Zsh stores in
// ~/.zsh_history or a custom $HISTFILE — we match by name fragment rather
// than exact equality so renamed/backup copies still route correctly.
func DetectFormat(path string) Format {
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(base, "zsh_history") || strings.HasSuffix(base, ".zhistory") || base == ".zhistory" {
		return FormatZsh
	}
	return FormatBash
}

// ParseZsh reads zsh extended-history format:
//
//	: <unix_ts>:<elapsed_secs>;<command>
//
// Multi-line commands continue with a trailing `\` on the prior raw line;
// that line is concatenated (keeping the `\n` in the command) until a line
// without a trailing `\` is seen. Malformed lines are silently skipped —
// history files are user-owned and sometimes contain partial writes.
func ParseZsh(r io.Reader) ([]Entry, error) {
	scanner := bufio.NewScanner(r)
	// History files can carry long one-liners (pasted commands, JSON bodies).
	// Default 64KB buffer is too small; bump to 1MB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var out []Entry
	var pending *Entry
	for scanner.Scan() {
		line := scanner.Text()
		if pending != nil {
			// Continuation of a previous multi-line command.
			if strings.HasSuffix(pending.Command, `\`) {
				pending.Command = strings.TrimSuffix(pending.Command, `\`) + "\n" + line
				if !strings.HasSuffix(line, `\`) {
					out = append(out, *pending)
					pending = nil
				}
				continue
			}
		}
		e, ok := parseZshLine(line)
		if !ok {
			continue
		}
		if strings.HasSuffix(e.Command, `\`) {
			pending = &e
			continue
		}
		out = append(out, e)
	}
	if pending != nil {
		out = append(out, *pending)
	}
	return out, scanner.Err()
}

func parseZshLine(line string) (Entry, bool) {
	if !strings.HasPrefix(line, ": ") {
		return Entry{}, false
	}
	rest := line[2:]
	colon := strings.IndexByte(rest, ':')
	if colon < 1 {
		return Entry{}, false
	}
	semi := strings.IndexByte(rest, ';')
	if semi < colon+2 {
		return Entry{}, false
	}
	ts, err := strconv.ParseInt(rest[:colon], 10, 64)
	if err != nil {
		return Entry{}, false
	}
	elapsed, err := strconv.ParseInt(rest[colon+1:semi], 10, 64)
	if err != nil {
		return Entry{}, false
	}
	cmd := rest[semi+1:]
	if cmd == "" {
		return Entry{}, false
	}
	return Entry{
		Command:    cmd,
		Timestamp:  time.Unix(ts, 0).UTC(),
		DurationMs: int(elapsed * 1000),
	}, true
}

// ParseBash reads bash history. With HISTTIMEFORMAT set, each command is
// preceded by a line `#<unix_ts>`. Without it, the file is just commands,
// one per line. We tolerate both in the same file.
func ParseBash(r io.Reader) ([]Entry, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var out []Entry
	var pendingTs time.Time
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			if ts, ok := parseBashTimestamp(line); ok {
				pendingTs = ts
				continue
			}
			// A `#` line that isn't a valid timestamp is treated as a
			// command — users do run shell commands with leading `#`.
		}
		if line == "" {
			continue
		}
		e := Entry{Command: line}
		if !pendingTs.IsZero() {
			e.Timestamp = pendingTs
			pendingTs = time.Time{}
		}
		out = append(out, e)
	}
	return out, scanner.Err()
}

func parseBashTimestamp(line string) (time.Time, bool) {
	if len(line) < 2 {
		return time.Time{}, false
	}
	ts, err := strconv.ParseInt(line[1:], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(ts, 0).UTC(), true
}
