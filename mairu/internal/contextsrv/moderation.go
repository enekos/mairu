package contextsrv

import (
	"strings"

	"mairu/internal/core"
)

type ModerationResult struct {
	Status  string
	Reasons []string
}

func ModerateContent(content string, moderationEnabled bool) ModerationResult {
	if !moderationEnabled {
		return ModerationResult{Status: ModerationStatusClean}
	}

	text := strings.TrimSpace(content)
	if text == "" {
		return ModerationResult{Status: ModerationStatusClean}
	}

	res := core.ScanContent(text)
	if res.Safe {
		return ModerationResult{Status: ModerationStatusClean}
	}

	var hasHard bool
	var reasons []string

	for _, w := range res.Warnings {
		reasons = append(reasons, w)
		if strings.Contains(w, "Private Key") || strings.Contains(w, "Possible exposed credential") {
			hasHard = true
		}
	}

	if hasHard {
		return ModerationResult{Status: ModerationStatusRejectHard, Reasons: reasons}
	}

	return ModerationResult{Status: ModerationStatusFlaggedSoft, Reasons: reasons}
}
