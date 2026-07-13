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
	OneOffs    []sqlcdb.OneOffEvent
	Social     []sqlcdb.SocialSession
	Attendance []sqlcdb.ListAnonymousAttendanceIntervalsRow
}

func (s *Store) LoadAnonymousTimetable(ctx context.Context, from, through domain.CivilDate) (AnonymousTimetableData, error) {
	parameters := sqlcdb.ListWeeklySeriesParams{ThroughDate: through.String(), FromDate: sql.NullString{String: from.String(), Valid: true}}
	weekly, err := s.queries.ListWeeklySeries(ctx, parameters)
	if err != nil {
		return AnonymousTimetableData{}, fmt.Errorf("list weekly schedule: %w", err)
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
	return AnonymousTimetableData{Weekly: weekly, OneOffs: oneOffs, Social: social, Attendance: attendance}, nil
}

func (s *Store) LoadSyntheticFixtures(ctx context.Context, today domain.CivilDate, location *time.Location) error {
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin synthetic fixtures: %w", err)
	}
	defer transaction.Rollback()
	var loadedFor string
	err = transaction.QueryRowContext(ctx, "SELECT loaded_for_date FROM synthetic_fixture_metadata WHERE singleton = 1").Scan(&loadedFor)
	if err == nil && loadedFor == today.String() {
		return transaction.Commit()
	}
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read synthetic fixture marker: %w", err)
	}
	for _, table := range []string{"attendance_plans", "member_sessions", "accounts", "schedule_exceptions", "one_off_events", "weekly_series", "social_sessions", "synthetic_fixture_metadata"} {
		if _, err := transaction.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("clear synthetic %s: %w", table, err)
		}
	}
	for weekday := 1; weekday <= 7; weekday++ {
		for court := 1; court <= 2; court++ {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO weekly_series (court, kind, title, iso_weekday, start_minute, end_minute, effective_start_date) VALUES (?, 'open', 'Lights on', ?, 1020, 1320, ?)`, court, weekday, today.String()); err != nil {
				return fmt.Errorf("insert synthetic light hours: %w", err)
			}
		}
	}
	for _, weekday := range []int{2, 5, 7} {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO social_sessions (iso_weekday, start_minute, end_minute, effective_start_date) VALUES (?, 1080, 1260, ?)`, weekday, today.String()); err != nil {
			return fmt.Errorf("insert synthetic social session: %w", err)
		}
	}
	competitionDate := today.AddDays(5, location)
	if _, err := transaction.ExecContext(ctx, `INSERT INTO one_off_events (event_date, court, kind, title, start_minute, end_minute) VALUES (?, 2, 'competition', 'Synthetic competition', 1080, 1200)`, competitionDate.String()); err != nil {
		return fmt.Errorf("insert synthetic competition: %w", err)
	}
	coachingDate := today.AddDays(2, location)
	if _, err := transaction.ExecContext(ctx, `INSERT INTO one_off_events (event_date, court, kind, title, start_minute, end_minute) VALUES (?, 1, 'coaching', 'Synthetic coaching', 1140, 1260)`, coachingDate.String()); err != nil {
		return fmt.Errorf("insert synthetic coaching: %w", err)
	}
	for member := 1; member <= 12; member++ {
		code := fmt.Sprintf("%04d", member)
		result, err := transaction.ExecContext(ctx, `INSERT INTO accounts (player_code, full_username, display_name, pin_hash, member_status, created_at_utc) VALUES (?, ?, ?, 'synthetic-disabled', 'member', 0)`, code, "synthetic"+code+"#"+code, fmt.Sprintf("Player %d", member))
		if err != nil {
			return fmt.Errorf("insert synthetic account: %w", err)
		}
		accountID, _ := result.LastInsertId()
		if _, err := transaction.ExecContext(ctx, `INSERT INTO attendance_plans (account_id, attendance_date, start_minute, end_minute, created_at_utc, updated_at_utc) VALUES (?, ?, 1080, 1200, 0, 0)`, accountID, today.AddDays(5, location).String()); err != nil {
			return fmt.Errorf("insert synthetic attendance: %w", err)
		}
		if member <= 5 {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO attendance_plans (account_id, attendance_date, start_minute, end_minute, created_at_utc, updated_at_utc) VALUES (?, ?, 1080, 1200, 0, 0)`, accountID, today.AddDays(1, location).String()); err != nil {
				return fmt.Errorf("insert sparse synthetic attendance: %w", err)
			}
		}
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO synthetic_fixture_metadata (singleton, loaded_for_date) VALUES (1, ?)`, today.String()); err != nil {
		return fmt.Errorf("mark synthetic fixtures: %w", err)
	}
	return transaction.Commit()
}
