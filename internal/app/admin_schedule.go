package app

import (
	"context"
	"strings"

	"github.com/rileyso/uni-squash-booking/internal/domain"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

func (s *Service) AdminSchedule(ctx context.Context) ([]sqlite.AdminScheduleEntry, error) {
	return s.store.ListAdminSchedule(ctx)
}

func validScheduleInput(court, weekday, start, end int, kind, title string) bool {
	validKind := kind == "open" || kind == "lights_off" || kind == "competition" || kind == "coaching" || kind == "closure"
	return (court == 1 || court == 2) && weekday >= 1 && weekday <= 7 && start >= 0 && end <= 1440 && start%30 == 0 && end%30 == 0 && end > start && validKind && len(strings.TrimSpace(title)) <= 80
}

func (s *Service) CreateWeeklySchedule(ctx context.Context, court int, kind, title string, weekday, start, end int, effective string) error {
	if !validScheduleInput(court, weekday, start, end, kind, title) {
		return ErrInvalidInput
	}
	if _, err := domain.ParseCivilDate(effective); err != nil {
		return ErrInvalidInput
	}
	return s.store.CreateWeeklySchedule(ctx, court, kind, strings.TrimSpace(title), weekday, start, end, effective)
}

func (s *Service) CreateDatedSchedule(ctx context.Context, court int, kind, title, date string, start, end int) error {
	if !validScheduleInput(court, 1, start, end, kind, title) {
		return ErrInvalidInput
	}
	if _, err := domain.ParseCivilDate(date); err != nil {
		return ErrInvalidInput
	}
	return s.store.CreateDatedSchedule(ctx, court, kind, strings.TrimSpace(title), date, start, end)
}

func (s *Service) CreateSocialSession(ctx context.Context, weekday, start, end int, effective string) error {
	if weekday < 1 || weekday > 7 || start < 0 || end > 1440 || start%30 != 0 || end%30 != 0 || end <= start {
		return ErrInvalidInput
	}
	if _, err := domain.ParseCivilDate(effective); err != nil {
		return ErrInvalidInput
	}
	return s.store.CreateSocialSession(ctx, weekday, start, end, effective)
}

func (s *Service) DeleteScheduleEntry(ctx context.Context, entryType string, id int64) error {
	if id <= 0 {
		return ErrInvalidInput
	}
	return s.store.DeleteScheduleEntry(ctx, entryType, id)
}

func (s *Service) ReplaceWeeklySchedule(ctx context.Context, id int64, scope, occurrence, kind, title string, court, weekday, start, end int, confirmDeleteExceptions bool) error {
	if id <= 0 || !validScheduleInput(court, weekday, start, end, kind, title) {
		return ErrInvalidInput
	}
	date, err := domain.ParseCivilDate(occurrence)
	if err != nil {
		return ErrInvalidInput
	}
	switch scope {
	case "occurrence":
		return s.store.ReplaceWeeklyOccurrence(ctx, id, occurrence, court, kind, start, end)
	case "future":
		oldEnd := date.AddDays(-1, s.location).String()
		return s.store.ReplaceWeeklyFuture(ctx, id, occurrence, oldEnd, court, kind, strings.TrimSpace(title), weekday, start, end, confirmDeleteExceptions)
	default:
		return ErrInvalidInput
	}
}
