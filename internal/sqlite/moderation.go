package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrAccountState = errors.New("account is not in the required state")

type ModerationAccount struct {
	ID           int64
	FullUsername string
	Status       string
}

func lookupModerationAccount(ctx context.Context, transaction *sql.Tx, identifier string) (ModerationAccount, error) {
	var account ModerationAccount
	err := transaction.QueryRowContext(ctx, `SELECT id, full_username, account_status FROM accounts WHERE full_username = ? OR player_code = ?`, identifier, identifier).Scan(&account.ID, &account.FullUsername, &account.Status)
	return account, err
}

func (s *Store) SuspendAccount(ctx context.Context, identifier, today string) (ModerationAccount, error) {
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return ModerationAccount{}, err
	}
	defer transaction.Rollback()
	account, err := lookupModerationAccount(ctx, transaction, strings.TrimSpace(identifier))
	if err != nil {
		return ModerationAccount{}, err
	}
	if _, err = transaction.ExecContext(ctx, `UPDATE accounts SET account_status = 'suspended' WHERE id = ?`, account.ID); err != nil {
		return ModerationAccount{}, err
	}
	if _, err = transaction.ExecContext(ctx, `DELETE FROM member_sessions WHERE account_id = ?`, account.ID); err != nil {
		return ModerationAccount{}, err
	}
	if _, err = transaction.ExecContext(ctx, `DELETE FROM attendance_plans WHERE account_id = ? AND attendance_date >= ?`, account.ID, today); err != nil {
		return ModerationAccount{}, err
	}
	if err = transaction.Commit(); err != nil {
		return ModerationAccount{}, err
	}
	account.Status = "suspended"
	return account, nil
}

func (s *Store) ReinstateAccount(ctx context.Context, identifier string) (ModerationAccount, error) {
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return ModerationAccount{}, err
	}
	defer transaction.Rollback()
	account, err := lookupModerationAccount(ctx, transaction, strings.TrimSpace(identifier))
	if err != nil {
		return ModerationAccount{}, err
	}
	if account.Status != "suspended" {
		return ModerationAccount{}, ErrAccountState
	}
	if _, err = transaction.ExecContext(ctx, `UPDATE accounts SET account_status = 'active' WHERE id = ? AND account_status = 'suspended'`, account.ID); err != nil {
		return ModerationAccount{}, err
	}
	if err = transaction.Commit(); err != nil {
		return ModerationAccount{}, err
	}
	account.Status = "active"
	return account, nil
}

func (s *Store) PermanentlyDeleteAccount(ctx context.Context, exactUsername string, now time.Time) (ModerationAccount, error) {
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return ModerationAccount{}, err
	}
	defer transaction.Rollback()
	account, err := lookupModerationAccount(ctx, transaction, strings.TrimSpace(exactUsername))
	if err != nil {
		return ModerationAccount{}, err
	}
	if account.FullUsername != exactUsername {
		return ModerationAccount{}, sql.ErrNoRows
	}
	today := now.Format("2006-01-02")
	if _, err = transaction.ExecContext(ctx, `DELETE FROM attendance_plans WHERE account_id = ? AND attendance_date >= ?`, account.ID, today); err != nil {
		return ModerationAccount{}, err
	}
	if _, err = transaction.ExecContext(ctx, `UPDATE attendance_plans SET account_id = NULL, updated_at_utc = ? WHERE account_id = ?`, now.Unix(), account.ID); err != nil {
		return ModerationAccount{}, err
	}
	if _, err = transaction.ExecContext(ctx, `DELETE FROM accounts WHERE id = ? AND full_username = ?`, account.ID, exactUsername); err != nil {
		return ModerationAccount{}, err
	}
	if err = transaction.Commit(); err != nil {
		return ModerationAccount{}, err
	}
	return account, nil
}

// Backup creates a transactionally consistent SQLite snapshot and accepts it
// only after integrity and foreign-key verification through a separate handle.
func (s *Store) Backup(ctx context.Context, destination string) error {
	destination = filepath.Clean(destination)
	if destination == "." || destination == "" {
		return errors.New("backup destination is required")
	}
	if _, err := os.Stat(destination); err == nil {
		return errors.New("backup destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	if _, err := s.writer.ExecContext(ctx, `VACUUM INTO ?`, destination); err != nil {
		return fmt.Errorf("create consistent backup: %w", err)
	}
	verified, err := sql.Open("sqlite", dataSourceName(destination))
	if err != nil {
		os.Remove(destination)
		return err
	}
	defer verified.Close()
	var integrity string
	if err := verified.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		os.Remove(destination)
		return fmt.Errorf("verify backup integrity: result=%q error=%v", integrity, err)
	}
	rows, err := verified.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		os.Remove(destination)
		return fmt.Errorf("verify backup foreign keys: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		os.Remove(destination)
		return errors.New("verify backup foreign keys: violations found")
	}
	return nil
}
