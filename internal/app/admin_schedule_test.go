package app

import (
	"context"
	"errors"
	"testing"
)

func TestAdminScheduleCreateListAndDelete(t *testing.T) {
	service := testService(t)
	ctx := context.Background()

	if err := service.CreateWeeklySchedule(ctx, 2, "coaching", "Development squad", 3, 1200, 1260, "2026-07-15"); err != nil {
		t.Fatal(err)
	}
	if err := service.CreateDatedSchedule(ctx, 1, "competition", "Synthetic fixture", "2026-07-19", 600, 660); err != nil {
		t.Fatal(err)
	}
	if err := service.CreateSocialSession(ctx, 5, 1110, 1200, "2026-07-17"); err != nil {
		t.Fatal(err)
	}

	entries, err := service.AdminSchedule(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var createdID int64
	for _, entry := range entries {
		if entry.Type == "dated" && entry.Title == "Synthetic fixture" {
			createdID = entry.ID
		}
	}
	if createdID == 0 {
		t.Fatal("dated schedule entry was not listed")
	}
	if err := service.DeleteScheduleEntry(ctx, "dated", createdID); err != nil {
		t.Fatal(err)
	}
}

func TestAdminScheduleRejectsInvalidBoundariesAndVocabulary(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	for _, err := range []error{
		service.CreateWeeklySchedule(ctx, 1, "available", "", 2, 1080, 1140, "2026-07-14"),
		service.CreateWeeklySchedule(ctx, 1, "open", "", 2, 1085, 1140, "2026-07-14"),
		service.CreateDatedSchedule(ctx, 3, "open", "", "2026-07-14", 1080, 1140),
		service.CreateSocialSession(ctx, 2, 1200, 1200, "2026-07-14"),
	} {
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("error = %v, want ErrInvalidInput", err)
		}
	}
}
