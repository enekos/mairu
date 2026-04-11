package contextsrv

import (
	"context"
	"fmt"
)

func (s *AppService) ApplyBashHistoryFeedback(id string, reward int) (BashHistory, error) {
	if s.repo == nil {
		return BashHistory{}, fmt.Errorf("repository not configured")
	}

	// 1. Get current bash history to read OldImportance
	h, err := s.repo.GetBashHistory(context.Background(), id)
	if err != nil {
		return BashHistory{}, err
	}

	// 2. TD Learning update
	// NewImportance = OldImportance + alpha * (Reward - OldImportance)
	const alpha = 0.5
	oldImportance := float64(h.Importance)
	targetReward := float64(reward)

	newImportanceFloat := oldImportance + alpha*(targetReward-oldImportance)
	newImportance := int(newImportanceFloat + 0.5) // Round to nearest int

	// 3. Clamp between 1 and 10
	if newImportance < 1 {
		newImportance = 1
	}
	if newImportance > 10 {
		newImportance = 10
	}

	// 4. Bump feedback_count so implicit decay knows this retrieval was rewarded.
	_ = s.repo.IncrementBashHistoryFeedbackCount(context.Background(), id)
	h.FeedbackCount++ // Update local struct so UpdateBashHistory doesn't overwrite it

	// 5. Update the history if importance changed
	if newImportance != h.Importance {
		h.Importance = newImportance
		if err := s.repo.UpdateBashHistory(context.Background(), h); err != nil {
			return BashHistory{}, err
		}
	}

	return h, nil
}
