package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/domain"
	"github.com/rileyso/uni-squash-booking/internal/sqlite/sqlcdb"
)

type Dashboard struct {
	Days         []Day
	Rows         []TimetableRow
	SelectedDate string
	PreviousDate string
	NextDate     string
	Detail       *IntervalDetail
}

type IntervalDetail struct {
	DateLabel       string
	Date            string
	TimeLabel       string
	Count           int
	Band            string
	CourtOne        CourtState
	CourtTwo        CourtState
	ReducedCapacity bool
}

type TimetableRow struct {
	TimeLabel string
	Cells     []TimetableCell
}

type TimetableCell struct {
	Day  Day
	Slot Slot
}

type Day struct {
	Date      string
	DayName   string
	DateLabel string
	Social    bool
	Selected  bool
	Slots     []Slot
}

type Slot struct {
	StartMinute     int
	TimeLabel       string
	Count           int
	Band            string
	BandClass       string
	CourtOne        CourtState
	CourtTwo        CourtState
	ReducedCapacity bool
}

type CourtState struct {
	Label string
	Class string
	Icon  string
}

func (s *Service) Dashboard(ctx context.Context, requested, requestedMinute string) (Dashboard, error) {
	today := domain.CivilDateFromTime(s.now(), s.location)
	selected := today
	if requested != "" {
		parsed, err := domain.ParseCivilDate(requested)
		if err == nil && !parsed.Time(s.location).Before(today.Time(s.location)) && !parsed.Time(s.location).After(today.AddDays(13, s.location).Time(s.location)) {
			selected = parsed
		}
	}
	through := selected.AddDays(6, s.location)
	windowEnd := today.AddDays(13, s.location)
	if through.Time(s.location).After(windowEnd.Time(s.location)) {
		through = windowEnd
	}
	data, err := s.store.LoadAnonymousTimetable(ctx, selected, through)
	if err != nil {
		return Dashboard{}, err
	}
	dashboard := Dashboard{SelectedDate: selected.String()}
	if selected.Time(s.location).After(today.Time(s.location)) {
		dashboard.PreviousDate = selected.AddDays(-1, s.location).String()
	}
	if selected.Time(s.location).Before(windowEnd.Time(s.location)) {
		dashboard.NextDate = selected.AddDays(1, s.location).String()
	}
	for date := selected; !date.Time(s.location).After(through.Time(s.location)); date = date.AddDays(1, s.location) {
		localTime := date.Time(s.location)
		day := Day{Date: date.String(), DayName: localTime.Format("Mon"), DateLabel: localTime.Format("2 Jan"), Selected: date == selected}
		day.Social = socialOnDate(data.Social, date, s.location)
		for minute := 1020; minute < 1320; minute += 30 {
			interval, _ := domain.NewInterval(minute, minute+30)
			one := courtState(data.Weekly, data.OneOffs, date, 1, interval, s.location)
			two := courtState(data.Weekly, data.OneOffs, date, 2, interval, s.location)
			count := attendanceCount(data.Attendance, date.String(), interval)
			band, class := turnoutBand(count)
			day.Slots = append(day.Slots, Slot{StartMinute: minute, TimeLabel: minuteLabel(minute), Count: count, Band: band, BandClass: class, CourtOne: one, CourtTwo: two, ReducedCapacity: (one.Class == "open") != (two.Class == "open")})
		}
		dashboard.Days = append(dashboard.Days, day)
	}
	for slotIndex := 0; len(dashboard.Days) > 0 && slotIndex < len(dashboard.Days[0].Slots); slotIndex++ {
		row := TimetableRow{TimeLabel: dashboard.Days[0].Slots[slotIndex].TimeLabel}
		for _, day := range dashboard.Days {
			row.Cells = append(row.Cells, TimetableCell{Day: day, Slot: day.Slots[slotIndex]})
		}
		dashboard.Rows = append(dashboard.Rows, row)
	}
	if minute, err := strconv.Atoi(requestedMinute); err == nil && len(dashboard.Days) > 0 {
		for _, slot := range dashboard.Days[0].Slots {
			if slot.StartMinute == minute {
				dashboard.Detail = &IntervalDetail{DateLabel: dashboard.Days[0].DayName + " " + dashboard.Days[0].DateLabel, Date: dashboard.Days[0].Date, TimeLabel: slot.TimeLabel, Count: slot.Count, Band: slot.Band, CourtOne: slot.CourtOne, CourtTwo: slot.CourtTwo, ReducedCapacity: slot.ReducedCapacity}
				break
			}
		}
	}
	return dashboard, nil
}

func socialOnDate(rows []sqlcdb.SocialSession, date domain.CivilDate, location *time.Location) bool {
	iso := isoWeekday(date.Weekday(location))
	for _, row := range rows {
		if int64(iso) == row.IsoWeekday && date.String() >= row.EffectiveStartDate && (!row.EffectiveEndDate.Valid || date.String() <= row.EffectiveEndDate.String) {
			return true
		}
	}
	return false
}

func courtState(weekly []sqlcdb.WeeklySeries, oneOffs []sqlcdb.OneOffEvent, date domain.CivilDate, court int64, interval domain.Interval, location *time.Location) CourtState {
	status := "lights_off"
	priority := 1
	for _, row := range weekly {
		if row.Court != court || row.IsoWeekday != int64(isoWeekday(date.Weekday(location))) || date.String() < row.EffectiveStartDate || (row.EffectiveEndDate.Valid && date.String() > row.EffectiveEndDate.String) || !overlaps(row.StartMinute, row.EndMinute, interval) {
			continue
		}
		if candidate := statusPriority(row.Kind); candidate >= priority {
			status, priority = row.Kind, candidate
		}
	}
	for _, row := range oneOffs {
		if row.Court == court && row.EventDate == date.String() && overlaps(row.StartMinute, row.EndMinute, interval) {
			if candidate := statusPriority(row.Kind); candidate >= priority {
				status, priority = row.Kind, candidate
			}
		}
	}
	return statusView(status)
}

func statusPriority(status string) int {
	switch status {
	case "closure", "lights_off":
		return 4
	case "competition", "coaching":
		return 3
	case "open":
		return 2
	default:
		return 0
	}
}

func statusView(status string) CourtState {
	switch status {
	case "open":
		return CourtState{Label: "Open for play", Class: "open", Icon: "✓"}
	case "competition":
		return CourtState{Label: "Competition", Class: "competition", Icon: "◆"}
	case "coaching":
		return CourtState{Label: "Coaching", Class: "coaching", Icon: "◆"}
	case "closure":
		return CourtState{Label: "Other closure", Class: "closed", Icon: "×"}
	default:
		return CourtState{Label: "Lights off", Class: "closed", Icon: "×"}
	}
}

func attendanceCount(rows []sqlcdb.ListAnonymousAttendanceIntervalsRow, date string, interval domain.Interval) int {
	count := 0
	for _, row := range rows {
		if row.AttendanceDate == date && int(row.StartMinute) < interval.EndMinute && int(row.EndMinute) > interval.StartMinute {
			count++
		}
	}
	return count
}

func turnoutBand(count int) (string, string) {
	switch {
	case count == 0:
		return "Empty", "empty"
	case count <= 4:
		return "Players attending", "players"
	case count <= 9:
		return "Good turnout", "good"
	default:
		return "Crowded", "crowded"
	}
}

func overlaps(start, end int64, interval domain.Interval) bool {
	return int(start) < interval.EndMinute && int(end) > interval.StartMinute
}
func isoWeekday(day time.Weekday) int {
	if day == time.Sunday {
		return 7
	}
	return int(day)
}
func minuteLabel(minute int) string {
	return time.Date(2000, 1, 1, minute/60, minute%60, 0, 0, time.UTC).Format("3:04 pm")
}

func (s Slot) AccessibleLabel(day Day) string {
	return fmt.Sprintf("%s %s, %s, %d planned attendees, %s. Court 1: %s. Court 2: %s.", day.DayName, day.DateLabel, s.TimeLabel, s.Count, s.Band, s.CourtOne.Label, s.CourtTwo.Label)
}
