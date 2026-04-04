package contextsrv

import (
	"context"
)

func (s *AppService) ListModerationQueue(limit int) ([]ModerationEvent, error) {
	return s.repo.ListModerationQueue(context.Background(), limit)
}

func (s *AppService) ReviewModeration(input ModerationReviewInput) error {
	return s.repo.ReviewModeration(context.Background(), input)
}
