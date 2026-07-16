package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"time"
)

type AdminAccountResult struct {
	ID           int64
	PlayerCode   string
	FullUsername string
	DisplayName  string
	Status       string
}

func (s *Store) ApplyAdminCredentialFingerprint(ctx context.Context, fingerprint string) error {
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var current string
	if err := transaction.QueryRowContext(ctx, `SELECT admin_credential_fingerprint FROM recovery_metadata WHERE singleton = 1`).Scan(&current); err != nil {
		return err
	}
	if current != fingerprint {
		if _, err := transaction.ExecContext(ctx, `UPDATE recovery_metadata SET admin_credential_fingerprint = ? WHERE singleton = 1`, fingerprint); err != nil {
			return err
		}
		if _, err := transaction.ExecContext(ctx, `DELETE FROM admin_sessions`); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func (s *Store) CreateAdminSession(ctx context.Context, token, csrf []byte, created, expires time.Time) error {
	hash := sha256.Sum256(token)
	_, err := s.writer.ExecContext(ctx, `INSERT INTO admin_sessions (token_hash, csrf_secret, created_at_utc, expires_at_utc) VALUES (?, ?, ?, ?)`, hash[:], csrf, created.Unix(), expires.Unix())
	return err
}

func (s *Store) AdminSession(ctx context.Context, token []byte, now time.Time) ([]byte, error) {
	hash := sha256.Sum256(token)
	var csrf []byte
	err := s.readers.QueryRowContext(ctx, `SELECT csrf_secret FROM admin_sessions WHERE token_hash = ? AND expires_at_utc > ?`, hash[:], now.Unix()).Scan(&csrf)
	return csrf, err
}

func (s *Store) DeleteAdminSession(ctx context.Context, token []byte) error {
	hash := sha256.Sum256(token)
	_, err := s.writer.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token_hash = ?`, hash[:])
	return err
}

func (s *Store) SearchAccounts(ctx context.Context, query string) ([]AdminAccountResult, error) {
	like := "%" + query + "%"
	rows, err := s.readers.QueryContext(ctx, `SELECT id, player_code, full_username, display_name, account_status FROM accounts WHERE display_name LIKE ? COLLATE NOCASE OR lower(full_username) = lower(?) OR player_code = ? ORDER BY display_name COLLATE NOCASE, id LIMIT 50`, like, query, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []AdminAccountResult
	for rows.Next() {
		var result AdminAccountResult
		if err := rows.Scan(&result.ID, &result.PlayerCode, &result.FullUsername, &result.DisplayName, &result.Status); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func (s *Store) ResetMemberPIN(ctx context.Context, accountID int64, pinHash string) error {
	transaction, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	result, err := transaction.ExecContext(ctx, `UPDATE accounts SET pin_hash = ?, must_change_pin = 1 WHERE id = ?`, pinHash, accountID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return sql.ErrNoRows
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM member_sessions WHERE account_id = ?`, accountID); err != nil {
		return err
	}
	return transaction.Commit()
}
