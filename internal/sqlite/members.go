package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Account struct {
	ID           int64
	FullUsername string
	DisplayName  string
	PINHash      string
	Status       string
}

type AttendancePlan struct {
	Date        string
	StartMinute int
	EndMinute   int
}

func (s *Store) CreateAccount(ctx context.Context, code, username, displayName, pinHash, memberStatus string, now time.Time) (Account, error) {
	result, err := s.writer.ExecContext(ctx, `INSERT INTO accounts (player_code, full_username, display_name, pin_hash, member_status, created_at_utc) VALUES (?, ?, ?, ?, ?, ?)`, code, username, displayName, pinHash, memberStatus, now.Unix())
	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}
	id, _ := result.LastInsertId()
	return Account{ID: id, FullUsername: username, DisplayName: displayName, PINHash: pinHash, Status: "active"}, nil
}

func (s *Store) AccountByUsername(ctx context.Context, username string) (Account, error) {
	var account Account
	err := s.readers.QueryRowContext(ctx, `SELECT id, full_username, display_name, pin_hash, account_status FROM accounts WHERE full_username = ?`, username).Scan(&account.ID, &account.FullUsername, &account.DisplayName, &account.PINHash, &account.Status)
	return account, err
}

func (s *Store) CreateMemberSession(ctx context.Context, accountID int64, token, csrf []byte, created, expires time.Time) error {
	hash := sha256.Sum256(token)
	_, err := s.writer.ExecContext(ctx, `INSERT INTO member_sessions (account_id, token_hash, csrf_secret, created_at_utc, expires_at_utc) VALUES (?, ?, ?, ?, ?)`, accountID, hash[:], csrf, created.Unix(), expires.Unix())
	return err
}

func (s *Store) AccountBySession(ctx context.Context, token []byte, now time.Time) (Account, []byte, error) {
	hash := sha256.Sum256(token)
	var account Account
	var csrf []byte
	err := s.readers.QueryRowContext(ctx, `SELECT a.id, a.full_username, a.display_name, a.pin_hash, a.account_status, s.csrf_secret FROM member_sessions s JOIN accounts a ON a.id = s.account_id WHERE s.token_hash = ? AND s.expires_at_utc > ?`, hash[:], now.Unix()).Scan(&account.ID, &account.FullUsername, &account.DisplayName, &account.PINHash, &account.Status, &csrf)
	return account, csrf, err
}

func (s *Store) DeleteMemberSession(ctx context.Context, token []byte) error {
	hash := sha256.Sum256(token)
	_, err := s.writer.ExecContext(ctx, `DELETE FROM member_sessions WHERE token_hash = ?`, hash[:])
	return err
}

func (s *Store) PlansForAccount(ctx context.Context, accountID int64, from, through string) ([]AttendancePlan, error) {
	rows, err := s.readers.QueryContext(ctx, `SELECT attendance_date, start_minute, end_minute FROM attendance_plans WHERE account_id = ? AND attendance_date BETWEEN ? AND ? ORDER BY attendance_date, start_minute, end_minute`, accountID, from, through)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var plans []AttendancePlan
	for rows.Next() {
		var p AttendancePlan
		if err := rows.Scan(&p.Date, &p.StartMinute, &p.EndMinute); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (s *Store) PlanForAccountDate(ctx context.Context, accountID int64, date string) (AttendancePlan, error) {
	var p AttendancePlan
	err := s.readers.QueryRowContext(ctx, `SELECT attendance_date, start_minute, end_minute FROM attendance_plans WHERE account_id = ? AND attendance_date = ? ORDER BY start_minute, end_minute LIMIT 1`, accountID, date).Scan(&p.Date, &p.StartMinute, &p.EndMinute)
	return p, err
}

func (s *Store) UpsertAttendance(ctx context.Context, accountID int64, date string, start, end int, now time.Time) error {
	return s.ReplaceAttendance(ctx, accountID, date, []AttendancePlan{{Date: date, StartMinute: start, EndMinute: end}}, now)
}

func (s *Store) ReplaceAttendance(ctx context.Context, accountID int64, date string, plans []AttendancePlan, now time.Time) error {
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, `DELETE FROM attendance_plans WHERE account_id = ? AND attendance_date = ?`, accountID, date); err != nil {
		return err
	}
	for _, plan := range plans {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO attendance_plans (account_id, attendance_date, start_minute, end_minute, created_at_utc, updated_at_utc) VALUES (?, ?, ?, ?, ?, ?)`, accountID, date, plan.StartMinute, plan.EndMinute, now.Unix(), now.Unix()); err != nil {
			return err
		}
	}
	err = transaction.Commit()
	return err
}

func (s *Store) RemoveAttendance(ctx context.Context, accountID int64, date string) error {
	result, err := s.writer.ExecContext(ctx, `DELETE FROM attendance_plans WHERE account_id = ? AND attendance_date = ?`, accountID, date)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func IsNotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }
