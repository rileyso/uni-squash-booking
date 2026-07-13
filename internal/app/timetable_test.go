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
	if len(dashboard.Days) != 7 || len(dashboard.Rows) != 10 {
		t.Fatalf("unexpected dimensions: %d days, %d rows", len(dashboard.Days), len(dashboard.Rows))
	}
	sunday := dashboard.Days[5]
	if !sunday.Social {
		t.Fatal("Sunday is not marked as an official social day")
	}
	if sunday.Slots[2].Band != "Crowded" || sunday.Slots[2].Count != 12 || !sunday.Slots[2].ReducedCapacity {
		t.Fatalf("unexpected crowded fixture: %#v", sunday.Slots[2])
	}
	if dashboard.Detail == nil || dashboard.Detail.TimeLabel != "6:00 pm" {
		t.Fatalf("interval detail missing: %#v", dashboard.Detail)
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
