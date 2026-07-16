package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/domain"
	"github.com/rileyso/uni-squash-booking/internal/sqlite/sqlcdb"
)

type AnonymousTimetableData struct {
	Weekly     []sqlcdb.WeeklySeries
	Exceptions []ScheduleException
	OneOffs    []sqlcdb.OneOffEvent
	Social     []sqlcdb.SocialSession
	Attendance []sqlcdb.ListAnonymousAttendanceIntervalsRow
}

type ScheduleException struct {
	WeeklySeriesID int64
	OccurrenceDate string
	Cancelled      bool
	Court          sql.NullInt64
	Kind           sql.NullString
	Start          sql.NullInt64
	End            sql.NullInt64
}

func (s *Store) LoadAnonymousTimetable(ctx context.Context, from, through domain.CivilDate) (AnonymousTimetableData, error) {
	parameters := sqlcdb.ListWeeklySeriesParams{ThroughDate: through.String(), FromDate: sql.NullString{String: from.String(), Valid: true}}
	weekly, err := s.queries.ListWeeklySeries(ctx, parameters)
	if err != nil {
		return AnonymousTimetableData{}, fmt.Errorf("list weekly schedule: %w", err)
	}
	rows, err := s.readers.QueryContext(ctx, "SELECT weekly_series_id, occurrence_date, cancelled, replacement_court, replacement_kind, replacement_start_minute, replacement_end_minute FROM schedule_exceptions WHERE occurrence_date BETWEEN ? AND ? ORDER BY occurrence_date, weekly_series_id", from.String(), through.String())
	if err != nil {
		return AnonymousTimetableData{}, fmt.Errorf("list schedule exceptions: %w", err)
	}
	var exceptions []ScheduleException
	for rows.Next() {
		var exception ScheduleException
		if err := rows.Scan(&exception.WeeklySeriesID, &exception.OccurrenceDate, &exception.Cancelled, &exception.Court, &exception.Kind, &exception.Start, &exception.End); err != nil {
			rows.Close()
			return AnonymousTimetableData{}, err
		}
		exceptions = append(exceptions, exception)
	}
	if err := rows.Close(); err != nil {
		return AnonymousTimetableData{}, err
	}
	oneOffs, err := s.queries.ListOneOffEvents(ctx, sqlcdb.ListOneOffEventsParams{FromDate: from.String(), ThroughDate: through.String()})
	if err != nil {
		return AnonymousTimetableData{}, fmt.Errorf("list one-off schedule: %w", err)
	}
	social, err := s.queries.ListSocialSessions(ctx, sqlcdb.ListSocialSessionsParams{ThroughDate: through.String(), FromDate: sql.NullString{String: from.String(), Valid: true}})
	if err != nil {
		return AnonymousTimetableData{}, fmt.Errorf("list social sessions: %w", err)
	}
	attendance, err := s.queries.ListAnonymousAttendanceIntervals(ctx, sqlcdb.ListAnonymousAttendanceIntervalsParams{FromDate: from.String(), ThroughDate: through.String()})
	if err != nil {
		return AnonymousTimetableData{}, fmt.Errorf("list anonymous attendance: %w", err)
	}
	return AnonymousTimetableData{Weekly: weekly, Exceptions: exceptions, OneOffs: oneOffs, Social: social, Attendance: attendance}, nil
}

func (s *Store) LoadSyntheticFixtures(ctx context.Context, today domain.CivilDate, location *time.Location, trialPINHash string) error {
	weekday := int(today.Weekday(location))
	if weekday == 0 {
		weekday = 7
	}
	effectiveStart := today.AddDays(1-weekday, location).String()
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin synthetic fixtures: %w", err)
	}
	defer transaction.Rollback()
	var loadedFor string
	err = transaction.QueryRowContext(ctx, "SELECT loaded_for_date FROM synthetic_fixture_metadata WHERE singleton = 1").Scan(&loadedFor)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read synthetic fixture marker: %w", err)
	}
	for _, table := range []string{"attendance_plans", "member_sessions", "accounts", "schedule_exceptions", "one_off_events", "weekly_series", "social_sessions", "synthetic_fixture_metadata"} {
		if _, err := transaction.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("clear synthetic %s: %w", table, err)
		}
	}
	openHours := []struct {
		weekday int
		start   int
		end     int
	}{
		{1, 960, 1320},
		{2, 720, 840},
		{2, 960, 1320},
		{3, 870, 1320},
		{4, 960, 1320},
		{5, 900, 1200},
		{6, 540, 900},
		{7, 720, 1020},
	}
	for _, hours := range openHours {
		for court := 1; court <= 2; court++ {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO weekly_series (court, kind, title, iso_weekday, start_minute, end_minute, effective_start_date) VALUES (?, 'open', 'Lights on', ?, ?, ?, ?)`, court, hours.weekday, hours.start, hours.end, effectiveStart); err != nil {
				return fmt.Errorf("insert synthetic light hours: %w", err)
			}
		}
	}
	for _, session := range []struct {
		weekday int
		start   int
		end     int
	}{{2, 960, 1080}, {5, 900, 1080}} {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO social_sessions (iso_weekday, start_minute, end_minute, effective_start_date) VALUES (?, ?, ?, ?)`, session.weekday, session.start, session.end, effectiveStart); err != nil {
			return fmt.Errorf("insert synthetic social session: %w", err)
		}
	}
	occupations := []struct {
		weekday int
		kind    string
		title   string
		start   int
		end     int
	}{
		{6, "coaching", "Squads", 600, 690},
	}
	for _, occupation := range occupations {
		for court := 1; court <= 2; court++ {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO weekly_series (court, kind, title, iso_weekday, start_minute, end_minute, effective_start_date) VALUES (?, ?, ?, ?, ?, ?, ?)`, court, occupation.kind, occupation.title, occupation.weekday, occupation.start, occupation.end, effectiveStart); err != nil {
				return fmt.Errorf("insert synthetic court occupation: %w", err)
			}
		}
	}
	if trialPINHash == "" {
		trialPINHash = "synthetic-disabled"
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO accounts (player_code, full_username, display_name, pin_hash, member_status, created_at_utc) VALUES ('1111', 'john#1111', 'John', ?, 'member', 0)`, trialPINHash); err != nil {
		return fmt.Errorf("insert synthetic trial account: %w", err)
	}
	dummyNames := []string{"Alex Chen", "Maya Singh", "Sam Taylor", "Jordan Lee", "Priya Nair", "Noah Williams", "Sofia Martinez", "Ethan Brown", "Chloe Nguyen", "Liam Wilson", "Ava Thompson", "Ben Davis"}
	accountIDs := make([]int64, 0, len(dummyNames))
	for member, displayName := range dummyNames {
		memberNumber := member + 1
		code := fmt.Sprintf("%04d", memberNumber)
		result, err := transaction.ExecContext(ctx, `INSERT INTO accounts (player_code, full_username, display_name, pin_hash, member_status, created_at_utc) VALUES (?, ?, ?, 'synthetic-disabled', 'member', 0)`, code, "synthetic"+code+"#"+code, displayName)
		if err != nil {
			return fmt.Errorf("insert synthetic account: %w", err)
		}
		accountID, _ := result.LastInsertId()
		accountIDs = append(accountIDs, accountID)
		if _, err := transaction.ExecContext(ctx, `INSERT INTO attendance_plans (account_id, attendance_date, start_minute, end_minute, created_at_utc, updated_at_utc) VALUES (?, ?, 1080, 1200, 0, 0)`, accountID, today.AddDays(5, location).String()); err != nil {
			return fmt.Errorf("insert synthetic attendance: %w", err)
		}
		if memberNumber <= 5 {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO attendance_plans (account_id, attendance_date, start_minute, end_minute, created_at_utc, updated_at_utc) VALUES (?, ?, 1080, 1200, 0, 0)`, accountID, today.AddDays(1, location).String()); err != nil {
				return fmt.Errorf("insert sparse synthetic attendance: %w", err)
			}
		}
	}
	for offset := 0; offset < 14; offset++ {
		if offset == 1 || offset == 5 {
			continue
		}
		date := today.AddDays(offset, location)
		start := map[time.Weekday]int{time.Monday: 960, time.Tuesday: 960, time.Wednesday: 900, time.Thursday: 960, time.Friday: 900, time.Saturday: 720, time.Sunday: 720}[date.Weekday(location)]
		for attendee := 0; attendee < 3; attendee++ {
			accountID := accountIDs[(offset*2+attendee)%len(accountIDs)]
			if _, err := transaction.ExecContext(ctx, `INSERT INTO attendance_plans (account_id, attendance_date, start_minute, end_minute, created_at_utc, updated_at_utc) VALUES (?, ?, ?, ?, 0, 0)`, accountID, date.String(), start, start+120); err != nil {
				return fmt.Errorf("insert distributed synthetic attendance: %w", err)
			}
		}
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO synthetic_fixture_metadata (singleton, loaded_for_date) VALUES (1, ?)`, today.String()); err != nil {
		return fmt.Errorf("mark synthetic fixtures: %w", err)
	}
	return transaction.Commit()
}
