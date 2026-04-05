package contextsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (s *AppService) CreateSkill(input SkillCreateInput) (Skill, error) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Description) == "" {
		return Skill{}, fmt.Errorf("name and description are required")
	}
	m := ModerateContent(input.Name+": "+input.Description, s.moderationEnabled)
	input.ModerationStatus = m.Status
	input.ModerationReasons = m.Reasons
	input.ReviewRequired = m.Status == ModerationStatusFlaggedSoft
	if m.Status == ModerationStatusRejectHard {
		return Skill{}, fmt.Errorf("%w: %s", ErrModerationRejected, strings.Join(m.Reasons, ", "))
	}
	if len(input.Metadata) == 0 {
		input.Metadata = json.RawMessage(`{}`)
	}
	out, err := s.repo.CreateSkill(context.Background(), input)
	if err != nil {
		return Skill{}, err
	}
	_ = s.repo.EnqueueOutbox(context.Background(), "skill", out.ID, "upsert", out)
	return out, nil
}

func (s *AppService) ListSkills(project string, limit int) ([]Skill, error) {
	return s.repo.ListSkills(context.Background(), project, limit)
}

func (s *AppService) UpdateSkill(input SkillUpdateInput) (Skill, error) {
	return s.repo.UpdateSkill(context.Background(), input)
}

func (s *AppService) DeleteSkill(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id is required")
	}
	return s.repo.DeleteSkill(context.Background(), id)
}
