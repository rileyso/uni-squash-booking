package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/config"
	"github.com/rileyso/uni-squash-booking/internal/domain"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

func testService(t *testing.T) *Service {
	t.Helper()
	configuration := config.Config{Environment: config.Test, DatabasePath: filepath.Join(t.TempDir(), "test.sqlite"), RecoveryGeneration: "test-generation", Synthetic: true}
	store, err := sqlite.Open(context.Background(), configuration.DatabasePath, configuration.RecoveryGeneration)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	service, err := New(configuration, store)
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 7, 14, 2, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixed }
	today := domain.CivilDateFromTime(fixed, service.location)
	if err := store.LoadSyntheticFixtures(context.Background(), today, service.location); err != nil {
		t.Fatal(err)
	}
	return service
}

func TestDashboardHasFixedBandsAndReducedCapacity(t *testing.T) {
	dashboard, err := testService(t).Dashboard(context.Background(), "2026-07-14", "1080")
	if err != nil {
		t.Fatal(err)
	}
	if len(dashboard.Days) != 7 || len(dashboard.Rows) != 13 || len(dashboard.DesktopSlots) != 13 {
		t.Fatalf("unexpected dimensions: %d days, %d rows", len(dashboard.Days), len(dashboard.Rows))
	}
	if len(dashboard.DateChoices) != 7 || dashboard.DateChoices[0].Today || dashboard.DateChoices[0].Date != "2026-07-13" {
		t.Fatalf("date choices are not a plain Monday-Sunday range: %#v", dashboard.DateChoices)
	}
	sunday := dashboard.Days[6]
	if !dashboard.Days[1].Social || sunday.Social {
		t.Fatal("official social days do not match the supplied schedule")
	}
	if dashboard.Days[1].SocialTime != "4:00pm–6:00pm" || dashboard.Days[4].SocialTime != "3:00pm–6:00pm" {
		t.Fatalf("social times do not match weekly programme: %#v", dashboard.Days)
	}
	if sunday.Slots[8].Band != "Crowded" || sunday.Slots[8].Count != 12 {
		t.Fatalf("unexpected crowded fixture: %#v", sunday.Slots[8])
	}
	if dashboard.Detail == nil || dashboard.Detail.TimeLabel != "18:00" {
		t.Fatalf("interval detail missing: %#v", dashboard.Detail)
	}
	if !dashboard.HasSelectedInterval || dashboard.SelectedMinute != 1080 {
		t.Fatalf("selected interval state missing: selected=%t minute=%d", dashboard.HasSelectedInterval, dashboard.SelectedMinute)
	}
}

func TestDashboardTracksMultipleSelectedIntervals(t *testing.T) {
	dashboard, err := testService(t).Dashboard(context.Background(), "2026-07-14", "960,1080")
	if err != nil {
		t.Fatal(err)
	}
	if !dashboard.HasSelectedInterval || dashboard.SelectedMinute != 1080 || dashboard.SelectedTimeQuery != "960,1080" || dashboard.SelectedTotal != "2 hours" {
		t.Fatalf("multi-selection state missing: selected=%t minute=%d query=%q total=%q", dashboard.HasSelectedInterval, dashboard.SelectedMinute, dashboard.SelectedTimeQuery, dashboard.SelectedTotal)
	}
	if !dashboard.CanConfirmSelection || dashboard.SelectionNotice != "" {
		t.Fatalf("split selection should be confirmable: can=%t notice=%q", dashboard.CanConfirmSelection, dashboard.SelectionNotice)
	}
	var selected int
	for _, slot := range dashboard.DesktopSlots {
		if slot.Selected {
			selected++
		}
	}
	if selected != 2 {
		t.Fatalf("selected desktop slots = %d, want 2", selected)
	}
}

func TestDashboardShowsPastWeekdaySelectionNotice(t *testing.T) {
	dashboard, err := testService(t).Dashboard(context.Background(), "2026-07-13", "960")
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.CanConfirmSelection || dashboard.SelectionNotice != "Week day no longer available." {
		t.Fatalf("past weekday notice mismatch: can=%t notice=%q", dashboard.CanConfirmSelection, dashboard.SelectionNotice)
	}
}

func TestDashboardRejectsDateOutsideWindow(t *testing.T) {
	dashboard, err := testService(t).Dashboard(context.Background(), "2026-08-01", "")
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.SelectedDate != "2026-07-14" {
		t.Fatalf("selected date = %s", dashboard.SelectedDate)
	}
}

func TestDateChoicesStayStableUntilDayArrowCrossesWeek(t *testing.T) {
	service := testService(t)
	firstPage, err := service.Dashboard(context.Background(), "2026-07-17", "")
	if err != nil {
		t.Fatal(err)
	}
	if firstPage.DateChoices[0].Date != "2026-07-13" || firstPage.DateChoices[6].Date != "2026-07-19" || firstPage.PreviousDate != "2026-07-16" || firstPage.NextDate != "2026-07-18" {
		t.Fatalf("first date page rolled with selection: %#v", firstPage.DateChoices)
	}
	sunday, err := service.Dashboard(context.Background(), "2026-07-19", "")
	if err != nil {
		t.Fatal(err)
	}
	secondPage, err := service.Dashboard(context.Background(), sunday.NextDate, "")
	if err != nil {
		t.Fatal(err)
	}
	if secondPage.DateChoices[0].Date != "2026-07-20" || secondPage.DateChoices[6].Date != "2026-07-26" || secondPage.PreviousDate != "2026-07-19" {
		t.Fatalf("unexpected second date page: %#v", secondPage.DateChoices)
	}
	pastDay, err := service.Dashboard(context.Background(), "2026-07-13", "")
	if err != nil || pastDay.SelectedDate != "2026-07-13" || pastDay.PreviousDate != "" || pastDay.NextDate != "2026-07-14" {
		t.Fatalf("elapsed current-week day is unavailable: selected=%s err=%v", pastDay.SelectedDate, err)
	}
}

func TestSaturdaySquadsHaveCoachingNote(t *testing.T) {
	dashboard, err := testService(t).Dashboard(context.Background(), "2026-07-18", "")
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.DesktopSlots[0].CourtOne.Note != "Squads" || dashboard.DesktopSlots[0].CourtTwo.Note != "Squads" {
		t.Fatalf("Saturday squads note missing: %#v", dashboard.DesktopSlots[0])
	}
}

func TestSocialSessionsMarkBothOpenCourtCells(t *testing.T) {
	dashboard, err := testService(t).Dashboard(context.Background(), "2026-07-14", "")
	if err != nil {
		t.Fatal(err)
	}
	social := dashboard.DesktopSlots[6]
	if social.TimeLabel != "16:00" || social.CourtOne.Note != "Social" || social.CourtTwo.Note != "Social" {
		t.Fatalf("social notes missing from open court cells: %#v", social)
	}
	beforeSocial := dashboard.DesktopSlots[5]
	if beforeSocial.CourtOne.Note != "" || beforeSocial.CourtTwo.Note != "" {
		t.Fatalf("social note shown before session: %#v", beforeSocial)
	}
}

func TestTurnoutBandBoundaries(t *testing.T) {
	for _, test := range []struct {
		count int
		want  string
	}{{0, "Empty"}, {1, "Players attending"}, {4, "Players attending"}, {5, "Good turnout"}, {9, "Good turnout"}, {10, "Crowded"}} {
		got, _ := turnoutBand(test.count)
		if got != test.want {
			t.Fatalf("turnoutBand(%d) = %q, want %q", test.count, got, test.want)
		}
	}
}

func TestServiceStatusMethods(t *testing.T) {
	service := testService(t)
	if !service.Synthetic() {
		t.Fatal("test service is not synthetic")
	}
	if err := service.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	label := serviceLabelFixture()
	if label == "" {
		t.Fatal("accessible slot label is empty")
	}
}

func serviceLabelFixture() string {
	return (Slot{TimeLabel: "6:00 pm", Count: 5, Band: "Good turnout", CourtOne: statusView("open"), CourtTwo: statusView("coaching")}).AccessibleLabel(Day{DayName: "Tue", DateLabel: "14 Jul"})
}
