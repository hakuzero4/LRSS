package appsvc

import (
	"context"
	"fmt"
	"strings"

	"lrss/internal/model"
	"lrss/internal/service"
)

// BriefingService is the Wails-facing AI briefing API.
type BriefingService struct {
	w *service.BriefingWorker
}

func NewBriefingService(w *service.BriefingWorker) *BriefingService {
	return &BriefingService{w: w}
}

func (s *BriefingService) List() ([]model.Briefing, error) {
	if s == nil || s.w == nil {
		return nil, fmt.Errorf("briefings unavailable")
	}
	return s.w.List(context.Background(), 50)
}

func (s *BriefingService) Get(id string) (model.Briefing, error) {
	if s == nil || s.w == nil {
		return model.Briefing{}, fmt.Errorf("briefings unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Briefing{}, fmt.Errorf("briefing id is required")
	}
	return s.w.Get(context.Background(), id)
}

func (s *BriefingService) SetRead(id string, read bool) error {
	if s == nil || s.w == nil {
		return fmt.Errorf("briefings unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("briefing id is required")
	}
	return s.w.SetRead(context.Background(), id, read)
}

func (s *BriefingService) Delete(id string) error {
	if s == nil || s.w == nil {
		return fmt.Errorf("briefings unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("briefing id is required")
	}
	return s.w.Delete(context.Background(), id)
}

func (s *BriefingService) SetStarred(id string, starred bool) error {
	if s == nil || s.w == nil {
		return fmt.Errorf("briefings unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("briefing id is required")
	}
	return s.w.SetStarred(context.Background(), id, starred)
}

func (s *BriefingService) UnreadCount() (int, error) {
	if s == nil || s.w == nil {
		return 0, fmt.Errorf("briefings unavailable")
	}
	return s.w.UnreadCount(context.Background())
}

// Retry re-generates a failed briefing from its stored source articles.
func (s *BriefingService) Retry(id string) (model.Briefing, error) {
	if s == nil || s.w == nil {
		return model.Briefing{}, fmt.Errorf("briefings unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Briefing{}, fmt.Errorf("briefing id is required")
	}
	if err := s.w.Retry(context.Background(), id); err != nil {
		return model.Briefing{}, err
	}
	return s.w.Get(context.Background(), id)
}
