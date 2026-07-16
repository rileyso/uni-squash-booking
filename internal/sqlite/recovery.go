package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Restore(ctx context.Context, source, destination, generation string) error {
	if source == "" || destination == "" || generation == "" {
		return errors.New("source, destination, and recovery generation are required")
	}
	if _, err := os.Stat(destination); err == nil {
		return errors.New("restore destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := verifyDatabaseFile(ctx, source); err != nil {
		return fmt.Errorf("verify restore source: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	temporary := destination + ".restore"
	if err := copyExclusive(source, temporary); err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(temporary)
		}
	}()
	database, err := sql.Open("sqlite", dataSourceName(temporary))
	if err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, "UPDATE recovery_metadata SET recovery_generation = ?, recovery_lockdown = 1 WHERE singleton = 1", generation); err != nil {
		database.Close()
		return fmt.Errorf("lock restored database: %w", err)
	}
	if err := database.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, destination); err != nil {
		return err
	}
	keep = true
	return nil
}

func copyExclusive(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = out.Close()
		if !ok {
			_ = os.Remove(destination)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}

func verifyDatabaseFile(ctx context.Context, path string) error {
	database, err := sql.Open("sqlite", dataSourceName(path))
	if err != nil {
		return err
	}
	defer database.Close()
	var integrity string
	if err := database.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return fmt.Errorf("integrity result=%q: %w", integrity, err)
	}
	rows, err := database.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return errors.New("foreign-key violations found")
	}
	return nil
}

func ScrubRestoredIdentities(ctx context.Context, path, generation string) error {
	database, err := sql.Open("sqlite", dataSourceName(path))
	if err != nil {
		return err
	}
	defer database.Close()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var stored string
	var lockdown int
	if err := tx.QueryRowContext(ctx, "SELECT recovery_generation, recovery_lockdown FROM recovery_metadata WHERE singleton = 1").Scan(&stored, &lockdown); err != nil {
		return err
	}
	if stored != generation || lockdown != 1 {
		return ErrGeneration
	}
	for _, statement := range []string{"DELETE FROM member_sessions", "DELETE FROM admin_sessions", "UPDATE attendance_plans SET account_id = NULL", "DELETE FROM accounts"} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ClearRecoveryLockdown(ctx context.Context, path, generation string) error {
	database, err := sql.Open("sqlite", dataSourceName(path))
	if err != nil {
		return err
	}
	defer database.Close()
	result, err := database.ExecContext(ctx, "UPDATE recovery_metadata SET recovery_lockdown = 0 WHERE singleton = 1 AND recovery_generation = ? AND recovery_lockdown = 1 AND NOT EXISTS (SELECT 1 FROM accounts) AND NOT EXISTS (SELECT 1 FROM member_sessions) AND NOT EXISTS (SELECT 1 FROM admin_sessions)", generation)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return errors.New("recovery clearance requires matching generation, lockdown, and completed identity scrub")
	}
	return verifyDatabaseFile(ctx, path)
}
