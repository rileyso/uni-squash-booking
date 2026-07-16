package sqlite

import (
	"context"
	"database/sql"
	"errors"
)

type AdminScheduleEntry struct {
	ID        int64
	Type      string
	Court     int
	Kind      string
	Title     string
	Weekday   int
	Date      string
	Start     int
	End       int
	Effective string
}

func (s *Store) ListAdminSchedule(ctx context.Context) ([]AdminScheduleEntry, error) {
	rows, err := s.readers.QueryContext(ctx, `
SELECT id, 'weekly', court, kind, title, iso_weekday, '', start_minute, end_minute, effective_start_date FROM weekly_series
UNION ALL
SELECT id, 'dated', court, kind, title, 0, event_date, start_minute, end_minute, event_date FROM one_off_events
UNION ALL
SELECT id, 'social', 0, 'social', '', iso_weekday, '', start_minute, end_minute, effective_start_date FROM social_sessions
ORDER BY 2, 10, 6, 8, 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []AdminScheduleEntry
	for rows.Next() {
		var entry AdminScheduleEntry
		if err := rows.Scan(&entry.ID, &entry.Type, &entry.Court, &entry.Kind, &entry.Title, &entry.Weekday, &entry.Date, &entry.Start, &entry.End, &entry.Effective); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Store) CreateWeeklySchedule(ctx context.Context, court int, kind, title string, weekday, start, end int, effective string) error {
	_, err := s.writer.ExecContext(ctx, `INSERT INTO weekly_series (court, kind, title, iso_weekday, start_minute, end_minute, effective_start_date) VALUES (?, ?, ?, ?, ?, ?, ?)`, court, kind, title, weekday, start, end, effective)
	return err
}

func (s *Store) CreateDatedSchedule(ctx context.Context, court int, kind, title, date string, start, end int) error {
	_, err := s.writer.ExecContext(ctx, `INSERT INTO one_off_events (event_date, court, kind, title, start_minute, end_minute) VALUES (?, ?, ?, ?, ?, ?)`, date, court, kind, title, start, end)
	return err
}

func (s *Store) CreateSocialSession(ctx context.Context, weekday, start, end int, effective string) error {
	_, err := s.writer.ExecContext(ctx, `INSERT INTO social_sessions (iso_weekday, start_minute, end_minute, effective_start_date) VALUES (?, ?, ?, ?)`, weekday, start, end, effective)
	return err
}

func (s *Store) DeleteScheduleEntry(ctx context.Context, entryType string, id int64) error {
	table := map[string]string{"weekly": "weekly_series", "dated": "one_off_events", "social": "social_sessions"}[entryType]
	if table == "" {
		return sql.ErrNoRows
	}
	result, err := s.writer.ExecContext(ctx, `DELETE FROM `+table+` WHERE id = ?`, id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ReplaceWeeklyOccurrence(ctx context.Context, id int64, date string, court int, kind string, start, end int) error {
	result, err := s.writer.ExecContext(ctx, "INSERT INTO schedule_exceptions (weekly_series_id, occurrence_date, cancelled, replacement_court, replacement_kind, replacement_start_minute, replacement_end_minute) SELECT id, ?, 0, ?, ?, ?, ? FROM weekly_series WHERE id = ? ON CONFLICT(weekly_series_id, occurrence_date) DO UPDATE SET cancelled = 0, replacement_court = excluded.replacement_court, replacement_kind = excluded.replacement_kind, replacement_start_minute = excluded.replacement_start_minute, replacement_end_minute = excluded.replacement_end_minute", date, court, kind, start, end, id)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ReplaceWeeklyFuture(ctx context.Context, id int64, occurrenceDate, oldEndDate string, court int, kind, title string, weekday, start, end int, confirmDeleteExceptions bool) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var futureExceptions int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM schedule_exceptions WHERE weekly_series_id = ? AND occurrence_date >= ?", id, occurrenceDate).Scan(&futureExceptions); err != nil {
		return err
	}
	if futureExceptions > 0 && !confirmDeleteExceptions {
		return errors.New("future exceptions require explicit deletion confirmation")
	}
	result, err := tx.ExecContext(ctx, "UPDATE weekly_series SET effective_end_date = ? WHERE id = ? AND effective_start_date < ? AND (effective_end_date IS NULL OR effective_end_date >= ?)", oldEndDate, id, occurrenceDate, occurrenceDate)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO weekly_series (court, kind, title, iso_weekday, start_minute, end_minute, effective_start_date) VALUES (?, ?, ?, ?, ?, ?, ?)", court, kind, title, weekday, start, end, occurrenceDate); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM schedule_exceptions WHERE weekly_series_id = ? AND occurrence_date >= ?", id, occurrenceDate); err != nil {
		return err
	}
	return tx.Commit()
}
