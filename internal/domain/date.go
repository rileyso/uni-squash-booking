// Package domain contains the attendance tracker's pure business rules.
package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidDate     = errors.New("invalid Sydney civil date")
	ErrInvalidInterval = errors.New("invalid 30-minute interval")
)

// CivilDate is a calendar date interpreted in Australia/Sydney.
type CivilDate struct {
	year  int
	month time.Month
	day   int
}

func ParseCivilDate(value string) (CivilDate, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return CivilDate{}, ErrInvalidDate
	}
	return CivilDate{year: parsed.Year(), month: parsed.Month(), day: parsed.Day()}, nil
}

func CivilDateFromTime(value time.Time, location *time.Location) CivilDate {
	local := value.In(location)
	return CivilDate{year: local.Year(), month: local.Month(), day: local.Day()}
}

func (d CivilDate) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.year, d.month, d.day)
}

func (d CivilDate) Time(location *time.Location) time.Time {
	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, location)
}

func (d CivilDate) AddDays(days int, location *time.Location) CivilDate {
	return CivilDateFromTime(d.Time(location).AddDate(0, 0, days), location)
}

func (d CivilDate) Weekday(location *time.Location) time.Weekday {
	return d.Time(location).Weekday()
}

type Interval struct {
	StartMinute int
	EndMinute   int
}

func NewInterval(startMinute, endMinute int) (Interval, error) {
	if startMinute < 0 || endMinute > 1440 || endMinute <= startMinute || startMinute%30 != 0 || endMinute%30 != 0 {
		return Interval{}, ErrInvalidInterval
	}
	return Interval{StartMinute: startMinute, EndMinute: endMinute}, nil
}

func (i Interval) Contains(other Interval) bool {
	return i.StartMinute <= other.StartMinute && i.EndMinute >= other.EndMinute
}

func (i Interval) Overlaps(other Interval) bool {
	return i.StartMinute < other.EndMinute && other.StartMinute < i.EndMinute
}
