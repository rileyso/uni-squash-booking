package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

type Account struct {
	ID            int64
	FullUsername  string
	DisplayName   string
	PINHash       string
	Status        string
	MemberStatus  string
	MustChangePIN bool
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
	return Account{ID: id, FullUsername: username, DisplayName: displayName, PINHash: pinHash, Status: "active", MemberStatus: memberStatus}, nil
}

func (s *Store) AccountByUsername(ctx context.Context, username string) (Account, error) {
	var account Account
	err := s.readers.QueryRowContext(ctx, `SELECT id, full_username, display_name, pin_hash, account_status, member_status, must_change_pin FROM accounts WHERE lower(full_username) = ?`, username).Scan(&account.ID, &account.FullUsername, &account.DisplayName, &account.PINHash, &account.Status, &account.MemberStatus, &account.MustChangePIN)
	return account, err
}

func (s *Store) CreateMemberSession(ctx context.Context, accountID int64, token, csrf []byte, created, expires time.Time) error {
	hash := sha256.Sum256(token)
	_, err := s.writer.ExecContext(ctx, `INSERT INTO member_sessions (account_id, token_hash, csrf_secret, created_at_utc, expires_at_utc) VALUES (?, ?, ?, ?, ?)`, accountID, hash[:], csrf, created.Unix(), expires.Unix())
	return err
}

func (s *Store) AccountBySession(ctx context.Context, token []byte, now time.Time) (Account, []byte, error) {
	// Keep cleanup bounded so an ordinary request cannot turn accumulated expiry
	// into an unbounded write pause.
	_, _ = s.writer.ExecContext(ctx, `DELETE FROM member_sessions WHERE id IN (SELECT id FROM member_sessions WHERE expires_at_utc <= ? ORDER BY expires_at_utc LIMIT 100)`, now.Unix())
	hash := sha256.Sum256(token)
	var account Account
	var csrf []byte
	err := s.readers.QueryRowContext(ctx, `SELECT a.id, a.full_username, a.display_name, a.pin_hash, a.account_status, a.member_status, a.must_change_pin, s.csrf_secret FROM member_sessions s JOIN accounts a ON a.id = s.account_id WHERE s.token_hash = ? AND s.expires_at_utc > ?`, hash[:], now.Unix()).Scan(&account.ID, &account.FullUsername, &account.DisplayName, &account.PINHash, &account.Status, &account.MemberStatus, &account.MustChangePIN, &csrf)
	return account, csrf, err
}

func (s *Store) UpdateProfileForSession(ctx context.Context, token []byte, expectedPINHash, displayName, memberStatus string, now time.Time) error {
	hash := sha256.Sum256(token)
	result, err := s.writer.ExecContext(ctx, `UPDATE accounts SET display_name = ?, member_status = ? WHERE pin_hash = ? AND account_status = 'active' AND id = (SELECT account_id FROM member_sessions WHERE token_hash = ? AND expires_at_utc > ?)`, displayName, memberStatus, expectedPINHash, hash[:], now.Unix())
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ChangePINForSession(ctx context.Context, token []byte, expectedPINHash, newPINHash string, now time.Time) error {
	hash := sha256.Sum256(token)
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var accountID int64
	if err := transaction.QueryRowContext(ctx, `SELECT a.id FROM member_sessions s JOIN accounts a ON a.id = s.account_id WHERE s.token_hash = ? AND s.expires_at_utc > ? AND a.account_status = 'active' AND a.pin_hash = ?`, hash[:], now.Unix(), expectedPINHash).Scan(&accountID); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `UPDATE accounts SET pin_hash = ?, must_change_pin = 0 WHERE id = ?`, newPINHash, accountID); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM member_sessions WHERE account_id = ? AND token_hash <> ?`, accountID, hash[:]); err != nil {
		return err
	}
	return transaction.Commit()
}

func (s *Store) SetTemporaryPIN(ctx context.Context, accountID int64, pinHash string) error {
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, `UPDATE accounts SET pin_hash = ?, must_change_pin = 1 WHERE id = ?`, pinHash, accountID); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM member_sessions WHERE account_id = ?`, accountID); err != nil {
		return err
	}
	return transaction.Commit()
}

func (s *Store) DeleteAccountForSession(ctx context.Context, token []byte, expectedPINHash, exactUsername, today string, now time.Time) error {
	hash := sha256.Sum256(token)
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var accountID int64
	if err := transaction.QueryRowContext(ctx, `SELECT a.id FROM member_sessions s JOIN accounts a ON a.id = s.account_id WHERE s.token_hash = ? AND s.expires_at_utc > ? AND a.account_status = 'active' AND a.must_change_pin = 0 AND a.pin_hash = ? AND a.full_username = ?`, hash[:], now.Unix(), expectedPINHash, exactUsername).Scan(&accountID); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM attendance_plans WHERE account_id = ? AND attendance_date >= ?`, accountID, today); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `UPDATE attendance_plans SET account_id = NULL WHERE account_id = ?`, accountID); err != nil {
		return err
	}
	result, err := transaction.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, accountID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return sql.ErrNoRows
	}
	return transaction.Commit()
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

func (s *Store) ParticipantNames(ctx context.Context, date string, start, end int) ([]string, error) {
	rows, err := s.readers.QueryContext(ctx, `SELECT DISTINCT a.display_name FROM attendance_plans p JOIN accounts a ON a.id = p.account_id WHERE p.attendance_date = ? AND p.start_minute < ? AND p.end_minute > ? AND a.account_status = 'active' ORDER BY a.display_name COLLATE NOCASE, a.id LIMIT 250`, date, end, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
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
	plans, err = mergedAttendance(ctx, transaction, accountID, date, plans)
	if err != nil {
		return err
	}
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

// ReplaceAttendanceForSession makes session/account authorization and the
// attendance replacement one authoritative writer transaction. Revocation or
// suspension racing this mutation therefore wins before or after the whole
// operation, never between authorization and the write.
func (s *Store) ReplaceAttendanceForSession(ctx context.Context, token []byte, date string, plans []AttendancePlan, now time.Time) error {
	hash := sha256.Sum256(token)
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var accountID int64
	err = transaction.QueryRowContext(ctx, `SELECT a.id FROM member_sessions s JOIN accounts a ON a.id = s.account_id WHERE s.token_hash = ? AND s.expires_at_utc > ? AND a.account_status = 'active' AND a.must_change_pin = 0`, hash[:], now.Unix()).Scan(&accountID)
	if err != nil {
		return err
	}
	plans, err = mergedAttendance(ctx, transaction, accountID, date, plans)
	if err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM attendance_plans WHERE account_id = ? AND attendance_date = ?`, accountID, date); err != nil {
		return err
	}
	for _, plan := range plans {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO attendance_plans (account_id, attendance_date, start_minute, end_minute, created_at_utc, updated_at_utc) VALUES (?, ?, ?, ?, ?, ?)`, accountID, date, plan.StartMinute, plan.EndMinute, now.Unix(), now.Unix()); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

type queryRower interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func mergedAttendance(ctx context.Context, query queryRower, accountID int64, date string, additions []AttendancePlan) ([]AttendancePlan, error) {
	rows, err := query.QueryContext(ctx, `SELECT attendance_date, start_minute, end_minute FROM attendance_plans WHERE account_id = ? AND attendance_date = ?`, accountID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := append([]AttendancePlan(nil), additions...)
	for rows.Next() {
		var plan AttendancePlan
		if err := rows.Scan(&plan.Date, &plan.StartMinute, &plan.EndMinute); err != nil {
			return nil, err
		}
		all = append(all, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].StartMinute == all[j].StartMinute {
			return all[i].EndMinute < all[j].EndMinute
		}
		return all[i].StartMinute < all[j].StartMinute
	})
	merged := make([]AttendancePlan, 0, len(all))
	for _, plan := range all {
		plan.Date = date
		if len(merged) == 0 || plan.StartMinute > merged[len(merged)-1].EndMinute {
			merged = append(merged, plan)
			continue
		}
		if plan.EndMinute > merged[len(merged)-1].EndMinute {
			merged[len(merged)-1].EndMinute = plan.EndMinute
		}
	}
	return merged, nil
}

func (s *Store) RemoveAttendanceForSession(ctx context.Context, token []byte, date string, now time.Time) error {
	hash := sha256.Sum256(token)
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var accountID int64
	if err := transaction.QueryRowContext(ctx, `SELECT a.id FROM member_sessions s JOIN accounts a ON a.id = s.account_id WHERE s.token_hash = ? AND s.expires_at_utc > ? AND a.account_status = 'active' AND a.must_change_pin = 0`, hash[:], now.Unix()).Scan(&accountID); err != nil {
		return err
	}
	result, err := transaction.ExecContext(ctx, `DELETE FROM attendance_plans WHERE account_id = ? AND attendance_date = ?`, accountID, date)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return sql.ErrNoRows
	}
	return transaction.Commit()
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
