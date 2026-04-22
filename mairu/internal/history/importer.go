package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/enekos/mairu/pii-redact/pkg/redact"
)

// BashRepo is the minimal contract the importer needs from
// contextsrv.SQLiteRepository. Having our own interface lets the tests
// drop in a fake without pulling the whole service.
type BashRepo interface {
	InsertBashHistory(ctx context.Context, project string, command string, exitCode int, durationMs int, output string) error
}

type ImportResult struct {
	Parsed           int
	Redacted         int
	Dropped          int
	DuplicateSkipped int
	Stored           int
}

// Import reads r (in the given format), redacts every command, deduplicates
// by content hash of the redacted command, and persists survivors via repo.
// When dryRun is true, no repo calls are made; counters reflect what would
// have been stored.
func Import(ctx context.Context, r io.Reader, format Format, repo BashRepo, redactor *redact.Redactor, project string, dryRun bool) (ImportResult, error) {
	var (
		entries []Entry
		err     error
	)
	switch format {
	case FormatZsh:
		entries, err = ParseZsh(r)
	case FormatBash:
		entries, err = ParseBash(r)
	default:
		return ImportResult{}, fmt.Errorf("unknown format: %d", format)
	}
	if err != nil {
		return ImportResult{}, fmt.Errorf("parse: %w", err)
	}

	res := ImportResult{Parsed: len(entries)}
	seen := make(map[string]struct{}, len(entries))

	for _, e := range entries {
		r := redactor.Redact(e.Command, redact.KindCommand)
		if len(r.Findings) > 0 {
			res.Redacted++
		}
		if r.Dropped {
			res.Dropped++
			continue
		}
		hash := hashCommand(r.Redacted)
		if _, ok := seen[hash]; ok {
			res.DuplicateSkipped++
			continue
		}
		seen[hash] = struct{}{}
		if dryRun {
			res.Stored++
			continue
		}
		if err := repo.InsertBashHistory(ctx, project, r.Redacted, 0, e.DurationMs, ""); err != nil {
			return res, fmt.Errorf("insert: %w", err)
		}
		res.Stored++
	}
	return res, nil
}

func hashCommand(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
