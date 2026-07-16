package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/domain"
	"github.com/rileyso/uni-squash-booking/internal/sqlite/sqlcdb"
)

type Dashboard struct {
	Days                []Day
	DateChoices         []DateChoice
	Rows                []TimetableRow
	DesktopSlots        []Slot
	SelectedDate        string
	PreviousDate        string
	NextDate            string
	SelectedMinute      int
	SelectedTimeQuery   string
	HasSelectedInterval bool
	SelectedRanges      []SelectedRange
	SelectedTotal       string
	CanConfirmSelection bool
	SelectionNotice     string
	Detail              *IntervalDetail
}

type DateChoice struct {
	Date      string
	DayName   string
	DateShort string
	Today     bool
	Selected  bool
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
	CanAttend       bool
	StartMinute     int
}

type SelectedRange struct {
	StartMinute int
	EndMinute   int
	StartLabel  string
	EndLabel    string
	Duration    string
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
	Date       string
	DayName    string
	DateLabel  string
	DateShort  string
	Today      bool
	Social     bool
	SocialTime string
	Selected   bool
	Slots      []Slot
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
	Selected        bool
}

type CourtState struct {
	Label string
	Class string
	Icon  string
	Note  string
}

func (s *Service) Dashboard(ctx context.Context, requested, requestedMinute string) (Dashboard, error) {
	today := domain.CivilDateFromTime(s.now(), s.location)
	currentWeekStart := mondayOfWeek(today, s.location)
	nextWeekEnd := currentWeekStart.AddDays(13, s.location)
	selected := today
	if requested != "" {
		parsed, err := domain.ParseCivilDate(requested)
		if err == nil && !parsed.Time(s.location).Before(currentWeekStart.Time(s.location)) && !parsed.Time(s.location).After(nextWeekEnd.Time(s.location)) {
			selected = parsed
		}
	}
	selectedMinuteOrder, selectedMinuteSet := parseSelectedMinutes(requestedMinute)
	stripStart := mondayOfWeek(selected, s.location)
	through := stripStart.AddDays(6, s.location)
	data, err := s.store.LoadAnonymousTimetable(ctx, stripStart, through)
	if err != nil {
		return Dashboard{}, err
	}
	dashboard := Dashboard{SelectedDate: selected.String()}
	dashboard.DateChoices = dateChoices(today, selected, s.location)
	if previous := selected.AddDays(-1, s.location); !previous.Time(s.location).Before(currentWeekStart.Time(s.location)) {
		dashboard.PreviousDate = previous.String()
	}
	if next := selected.AddDays(1, s.location); !next.Time(s.location).After(nextWeekEnd.Time(s.location)) {
		dashboard.NextDate = next.String()
	}
	var selectedDay Day
	for date := stripStart; !date.Time(s.location).After(through.Time(s.location)); date = date.AddDays(1, s.location) {
		localTime := date.Time(s.location)
		day := Day{Date: date.String(), DayName: localTime.Format("Mon"), DateLabel: localTime.Format("2 Jan"), DateShort: localTime.Format("02/01"), Today: date == today, Selected: date == selected}
		day.Social = socialOnDate(data.Social, date, s.location)
		day.SocialTime = socialTimeOnDate(data.Social, date, s.location)
		for minute := 600; minute <= 1320; minute += 60 {
			interval, _ := domain.NewInterval(minute, minute+60)
			one := courtState(data.Weekly, data.OneOffs, date, 1, interval, s.location)
			two := courtState(data.Weekly, data.OneOffs, date, 2, interval, s.location)
			if socialOverlaps(data.Social, date, interval, s.location) {
				if one.Class == "open" {
					one.Note = "Social"
				}
				if two.Class == "open" {
					two.Note = "Social"
				}
			}
			count := attendanceCount(data.Attendance, date.String(), interval)
			band, class := turnoutBand(count)
			day.Slots = append(day.Slots, Slot{StartMinute: minute, TimeLabel: minuteLabel(minute), Count: count, Band: band, BandClass: class, CourtOne: one, CourtTwo: two, ReducedCapacity: (one.Class == "open") != (two.Class == "open"), Selected: selectedMinuteSet[minute]})
		}
		dashboard.Days = append(dashboard.Days, day)
		if day.Selected {
			selectedDay = day
		}
	}
	for minute := 600; minute <= 1320; minute += 60 {
		interval, _ := domain.NewInterval(minute, minute+60)
		one := courtState(data.Weekly, data.OneOffs, selected, 1, interval, s.location)
		two := courtState(data.Weekly, data.OneOffs, selected, 2, interval, s.location)
		if socialOverlaps(data.Social, selected, interval, s.location) {
			if one.Class == "open" {
				one.Note = "Social"
			}
			if two.Class == "open" {
				two.Note = "Social"
			}
		}
		count := attendanceCount(data.Attendance, selected.String(), interval)
		band, class := turnoutBand(count)
		dashboard.DesktopSlots = append(dashboard.DesktopSlots, Slot{StartMinute: minute, TimeLabel: hourLabel(minute), Count: count, Band: band, BandClass: class, CourtOne: one, CourtTwo: two, ReducedCapacity: (one.Class == "open") != (two.Class == "open"), Selected: selectedMinuteSet[minute]})
	}
	for slotIndex := 0; len(dashboard.Days) > 0 && slotIndex < len(dashboard.Days[0].Slots); slotIndex++ {
		row := TimetableRow{TimeLabel: dashboard.Days[0].Slots[slotIndex].TimeLabel}
		for _, day := range dashboard.Days {
			row.Cells = append(row.Cells, TimetableCell{Day: day, Slot: day.Slots[slotIndex]})
		}
		dashboard.Rows = append(dashboard.Rows, row)
	}
	if len(selectedMinuteOrder) > 0 && selectedDay.Date != "" {
		validSelected := make([]int, 0, len(selectedMinuteOrder))
		for _, minute := range selectedMinuteOrder {
			detailSlots := selectedDay.Slots
			if minute%60 == 0 {
				detailSlots = dashboard.DesktopSlots
			}
			for _, slot := range detailSlots {
				if slot.StartMinute == minute {
					validSelected = append(validSelected, slot.StartMinute)
					dashboard.Detail = &IntervalDetail{DateLabel: selectedDay.DayName + " " + selectedDay.DateLabel, Date: selectedDay.Date, TimeLabel: slot.TimeLabel, Count: slot.Count, Band: slot.Band, CourtOne: slot.CourtOne, CourtTwo: slot.CourtTwo, ReducedCapacity: slot.ReducedCapacity, CanAttend: slot.CourtOne.Class == "open" || slot.CourtTwo.Class == "open", StartMinute: slot.StartMinute}
					dashboard.SelectedMinute = slot.StartMinute
					dashboard.HasSelectedInterval = true
					break
				}
			}
		}
		sort.Ints(validSelected)
		dashboard.SelectedTimeQuery = minuteQuery(validSelected)
		dashboard.SelectedRanges = selectedRanges(validSelected)
		dashboard.SelectedTotal = selectedTotal(dashboard.SelectedRanges)
		allValid := len(dashboard.SelectedRanges) > 0
		for _, selected := range dashboard.SelectedRanges {
			if err := s.ValidateAttendance(ctx, selectedDay.Date, selected.StartMinute, selected.EndMinute); err != nil {
				allValid = false
				break
			}
		}
		if allValid {
			dashboard.CanConfirmSelection = true
		} else if len(dashboard.SelectedRanges) > 0 {
			if selected.Time(s.location).Before(today.Time(s.location)) {
				dashboard.SelectionNotice = "Week day no longer available."
			} else {
				dashboard.SelectionNotice = "One or more selected ranges are not continuously open for play."
			}
		}
	}
	return dashboard, nil
}

func parseSelectedMinutes(value string) ([]int, map[int]bool) {
	selected := map[int]bool{}
	var ordered []int
	for _, part := range strings.Split(value, ",") {
		minute, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || selected[minute] {
			continue
		}
		selected[minute] = true
		ordered = append(ordered, minute)
	}
	return ordered, selected
}

func minuteQuery(minutes []int) string {
	parts := make([]string, 0, len(minutes))
	for _, minute := range minutes {
		parts = append(parts, strconv.Itoa(minute))
	}
	return strings.Join(parts, ",")
}

func selectedRanges(minutes []int) []SelectedRange {
	if len(minutes) == 0 {
		return nil
	}
	var ranges []SelectedRange
	start := minutes[0]
	end := start + 60
	for _, minute := range minutes[1:] {
		if minute == end {
			end = minute + 60
			continue
		}
		ranges = append(ranges, selectedRange(start, end))
		start = minute
		end = minute + 60
	}
	ranges = append(ranges, selectedRange(start, end))
	return ranges
}

func selectedRange(start, end int) SelectedRange {
	return SelectedRange{StartMinute: start, EndMinute: end, StartLabel: minuteLabel(start), EndLabel: minuteLabel(end), Duration: durationLabel(end - start)}
}

func selectedTotal(ranges []SelectedRange) string {
	total := 0
	for _, selected := range ranges {
		total += selected.EndMinute - selected.StartMinute
	}
	return durationLabel(total)
}

func durationLabel(minutes int) string {
	if minutes == 30 {
		return "30 minutes"
	}
	if minutes%60 == 0 {
		hours := minutes / 60
		if hours == 1 {
			return "1 hour"
		}
		return strconv.Itoa(hours) + " hours"
	}
	return strconv.Itoa(minutes/60) + "½ hours"
}

func dateChoices(today, selected domain.CivilDate, location *time.Location) []DateChoice {
	weekStart := mondayOfWeek(selected, location)
	var choices []DateChoice
	for offset := 0; offset < 7; offset++ {
		choice := dateChoice(weekStart.AddDays(offset, location), today, selected, location)
		choice.Today = false
		choices = append(choices, choice)
	}
	return choices
}

func mondayOfWeek(date domain.CivilDate, location *time.Location) domain.CivilDate {
	return date.AddDays(1-isoWeekday(date.Weekday(location)), location)
}

func dateChoice(date, today, selected domain.CivilDate, location *time.Location) DateChoice {
	localTime := date.Time(location)
	return DateChoice{Date: date.String(), DayName: localTime.Format("Mon"), DateShort: localTime.Format("02/01"), Today: date == today, Selected: date == selected}
}

func socialOnDate(rows []sqlcdb.SocialSession, date domain.CivilDate, location *time.Location) bool {
	return socialTimeOnDate(rows, date, location) != ""
}

func socialTimeOnDate(rows []sqlcdb.SocialSession, date domain.CivilDate, location *time.Location) string {
	iso := isoWeekday(date.Weekday(location))
	for _, row := range rows {
		if int64(iso) == row.IsoWeekday && date.String() >= row.EffectiveStartDate && (!row.EffectiveEndDate.Valid || date.String() <= row.EffectiveEndDate.String) {
			return compactMinuteLabel(int(row.StartMinute)) + "–" + compactMinuteLabel(int(row.EndMinute))
		}
	}
	return ""
}

func socialOverlaps(rows []sqlcdb.SocialSession, date domain.CivilDate, interval domain.Interval, location *time.Location) bool {
	iso := isoWeekday(date.Weekday(location))
	for _, row := range rows {
		if int64(iso) == row.IsoWeekday && date.String() >= row.EffectiveStartDate && (!row.EffectiveEndDate.Valid || date.String() <= row.EffectiveEndDate.String) && overlaps(row.StartMinute, row.EndMinute, interval) {
			return true
		}
	}
	return false
}

func courtState(weekly []sqlcdb.WeeklySeries, oneOffs []sqlcdb.OneOffEvent, date domain.CivilDate, court int64, interval domain.Interval, location *time.Location) CourtState {
	status := "lights_off"
	title := ""
	priority := 1
	for _, row := range weekly {
		if row.Court != court || row.IsoWeekday != int64(isoWeekday(date.Weekday(location))) || date.String() < row.EffectiveStartDate || (row.EffectiveEndDate.Valid && date.String() > row.EffectiveEndDate.String) || !overlaps(row.StartMinute, row.EndMinute, interval) {
			continue
		}
		if candidate := statusPriority(row.Kind); candidate >= priority {
			status, title, priority = row.Kind, row.Title, candidate
		}
	}
	for _, row := range oneOffs {
		if row.Court == court && row.EventDate == date.String() && overlaps(row.StartMinute, row.EndMinute, interval) {
			if candidate := statusPriority(row.Kind); candidate >= priority {
				status, title, priority = row.Kind, row.Title, candidate
			}
		}
	}
	view := statusView(status)
	if status == "coaching" && title == "Squads" {
		view.Note = "Squads"
	}
	return view
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

func hourLabel(minute int) string {
	return time.Date(2000, 1, 1, minute/60, minute%60, 0, 0, time.UTC).Format("15:04")
}

func compactMinuteLabel(minute int) string {
	return time.Date(2000, 1, 1, minute/60, minute%60, 0, 0, time.UTC).Format("3:04pm")
}

func (s Slot) AccessibleLabel(day Day) string {
	return fmt.Sprintf("%s %s, %s, %d planned attendees, %s. Court 1: %s. Court 2: %s.", day.DayName, day.DateLabel, s.TimeLabel, s.Count, s.Band, s.CourtOne.Label, s.CourtTwo.Label)
}
