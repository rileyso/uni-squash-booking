package domain

import (
	"errors"
	"testing"
	"time"
)

func TestExpandWeeklyIsBoundedAndInclusive(t *testing.T) {
	location, _ := time.LoadLocation("Australia/Sydney")
	start, _ := ParseCivilDate("2026-07-14")
	through, _ := ParseCivilDate("2026-07-28")
	interval, _ := NewInterval(1080, 1200)
	rule := WeeklyRule{ID: 7, Court: CourtOne, Weekday: time.Tuesday, Interval: interval, Status: StatusOpen, StartsOn: start}
	got := ExpandWeekly(rule, start, through, location)
	if len(got) != 3 || got[0].Date.String() != "2026-07-14" || got[2].Date.String() != "2026-07-28" {
		t.Fatalf("unexpected occurrences: %#v", got)
	}
}

func TestValidateOccupationsRejectsContradiction(t *testing.T) {
	date, _ := ParseCivilDate("2026-07-14")
	a, _ := NewInterval(1080, 1200)
	b, _ := NewInterval(1140, 1260)
	err := ValidateOccupations([]Occurrence{{Date: date, Court: CourtOne, Interval: a, Status: StatusCompetition}, {Date: date, Court: CourtOne, Interval: b, Status: StatusCoaching}})
	if !errors.Is(err, ErrScheduleConflict) {
		t.Fatalf("error = %v", err)
	}
}
