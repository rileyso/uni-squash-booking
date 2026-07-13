package domain

import (
	"errors"
	"testing"
	"time"
)

func TestCivilDateUsesSydneyCalendarDay(t *testing.T) {
	location, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		t.Fatal(err)
	}
	instant := time.Date(2026, 7, 13, 15, 30, 0, 0, time.UTC)
	if got := CivilDateFromTime(instant, location).String(); got != "2026-07-14" {
		t.Fatalf("got %q", got)
	}
}

func TestParseCivilDateRejectsNonCanonicalValues(t *testing.T) {
	for _, value := range []string{"", "2026-2-03", "2026-02-30", "14-07-2026"} {
		if _, err := ParseCivilDate(value); !errors.Is(err, ErrInvalidDate) {
			t.Fatalf("ParseCivilDate(%q) error = %v", value, err)
		}
	}
}

func TestIntervalBoundaries(t *testing.T) {
	valid, err := NewInterval(1050, 1170)
	if err != nil {
		t.Fatal(err)
	}
	inside, _ := NewInterval(1080, 1140)
	adjacent, _ := NewInterval(1170, 1200)
	if !valid.Contains(inside) || valid.Overlaps(adjacent) {
		t.Fatal("interval containment or overlap boundary is wrong")
	}
	for _, tc := range [][2]int{{-30, 30}, {0, 0}, {1, 30}, {0, 1470}} {
		if _, err := NewInterval(tc[0], tc[1]); !errors.Is(err, ErrInvalidInterval) {
			t.Fatalf("NewInterval(%d, %d) error = %v", tc[0], tc[1], err)
		}
	}
}
