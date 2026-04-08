package enricher

import (
	"context"
	"math"
	"os/exec"
	"strings"
	"time"
)

// ChangeVelocityEnricher computes churn signals for a file based on git commit frequency.
// It writes enrichment_churn_score (0.0–1.0), enrichment_churn_label, and
// enrichment_total_commits into fc.Metadata.
type ChangeVelocityEnricher struct {
	LookbackDays int // how far back to analyze; 0 defaults to 180
}

func (e *ChangeVelocityEnricher) Name() string { return "change_velocity" }

func (e *ChangeVelocityEnricher) Enrich(ctx context.Context, fc *FileContext) error {
	lookback := e.LookbackDays
	if lookback <= 0 {
		lookback = 180
	}

	timestamps, err := gitCommitTimestamps(ctx, fc.WatchDir, fc.RelPath)
	if err != nil || len(timestamps) == 0 {
		return nil // no git history — skip silently
	}

	now := time.Now()
	cutoff := now.AddDate(0, 0, -lookback)

	var recentCount int
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			recentCount++
		}
	}

	// Churn score: normalized commits-per-day in the lookback window, capped at 1.0.
	// A file changing once per day = 1.0; once per month ≈ 0.03.
	daysInWindow := float64(lookback)
	score := math.Min(float64(recentCount)/daysInWindow, 1.0)

	label := "stable"
	if score >= 0.5 {
		label = "volatile"
	} else if score >= 0.1 {
		label = "moderate"
	}

	fc.Metadata["enrichment_churn_score"] = score
	fc.Metadata["enrichment_churn_label"] = label
	fc.Metadata["enrichment_total_commits"] = len(timestamps)
	fc.Metadata["enrichment_recent_commits"] = recentCount

	return nil
}

// gitCommitTimestamps returns author-date timestamps for all commits touching relPath.
func gitCommitTimestamps(ctx context.Context, repoDir, relPath string) ([]time.Time, error) {
	cmd := exec.CommandContext(ctx, "git", "log", "--follow", "--format=%aI", "--", relPath)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var timestamps []time.Time
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, line)
		if err != nil {
			continue
		}
		timestamps = append(timestamps, t)
	}
	return timestamps, nil
}
