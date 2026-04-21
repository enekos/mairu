// Package redact implements the two redaction modes: structured JSON and
// regex-only line mode.
package redact

import (
	"bufio"
	"io"

	"github.com/join-com/pii-redact/internal/patterns"
)

// Lines streams stdin to stdout, applying content-regex redaction to each
// line. This is the fallback mode for gcloud --format="value(...)" output
// where there is no structure to walk.
//
// Stats are accumulated across all lines and returned when the input is
// exhausted.
func Lines(in io.Reader, out io.Writer, set *patterns.Set) (patterns.Stats, error) {
	total := patterns.Stats{}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	bw := bufio.NewWriter(out)
	defer bw.Flush()

	for scanner.Scan() {
		redacted, stats := set.Redact(scanner.Text())
		for k, v := range stats {
			total[k] += v
		}
		if _, err := bw.WriteString(redacted); err != nil {
			return total, err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return total, err
		}
	}
	if err := scanner.Err(); err != nil {
		return total, err
	}
	return total, nil
}
