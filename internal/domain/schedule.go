package domain

import (
	"errors"
	"sort"
	"time"
)

var ErrScheduleConflict = errors.New("schedule contains contradictory occupations")

type Court int

const (
	CourtOne Court = 1
	CourtTwo Court = 2
)

type CourtStatus string

const (
	StatusOpen        CourtStatus = "open"
	StatusLightsOff   CourtStatus = "lights_off"
	StatusCompetition CourtStatus = "competition"
	StatusCoaching    CourtStatus = "coaching"
	StatusClosure     CourtStatus = "closure"
)

type WeeklyRule struct {
	ID       int64
	Court    Court
	Weekday  time.Weekday
	Interval Interval
	Status   CourtStatus
	StartsOn CivilDate
	EndsOn   *CivilDate
}

type Occurrence struct {
	RuleID   int64
	Date     CivilDate
	Court    Court
	Interval Interval
	Status   CourtStatus
}

// ExpandWeekly keeps recurrence bounded to an explicit inclusive date range.
//
// weekly series --expand--> occurrences --sort--> deterministic court timeline
func ExpandWeekly(rule WeeklyRule, from, through CivilDate, location *time.Location) []Occurrence {
	var occurrences []Occurrence
	for date := from; !date.Time(location).After(through.Time(location)); date = date.AddDays(1, location) {
		if date.Time(location).Before(rule.StartsOn.Time(location)) || date.Weekday(location) != rule.Weekday {
			continue
		}
		if rule.EndsOn != nil && date.Time(location).After(rule.EndsOn.Time(location)) {
			continue
		}
		occurrences = append(occurrences, Occurrence{RuleID: rule.ID, Date: date, Court: rule.Court, Interval: rule.Interval, Status: rule.Status})
	}
	sort.Slice(occurrences, func(i, j int) bool {
		if occurrences[i].Date.String() != occurrences[j].Date.String() {
			return occurrences[i].Date.String() < occurrences[j].Date.String()
		}
		if occurrences[i].Court != occurrences[j].Court {
			return occurrences[i].Court < occurrences[j].Court
		}
		return occurrences[i].Interval.StartMinute < occurrences[j].Interval.StartMinute
	})
	return occurrences
}

func ValidateOccupations(occurrences []Occurrence) error {
	for i := range occurrences {
		for j := i + 1; j < len(occurrences); j++ {
			if occurrences[i].Date == occurrences[j].Date && occurrences[i].Court == occurrences[j].Court && occurrences[i].Interval.Overlaps(occurrences[j].Interval) && occurrences[i].Status != occurrences[j].Status {
				return ErrScheduleConflict
			}
		}
	}
	return nil
}
