package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreRequiresLockdownScrubAndExplicitClearance(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	original := filepath.Join(directory, "original.sqlite")
	store, err := Open(ctx, original, "old-generation")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.writer.ExecContext(ctx, "INSERT INTO accounts (player_code, full_username, display_name, pin_hash, member_status, created_at_utc) VALUES ('1234', 'restore#1234', 'Restore Person', 'hash', 'member', 0)"); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(directory, "backup.sqlite")
	if err := store.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	store.Close()
	restored := filepath.Join(directory, "restored.sqlite")
	if err := Restore(ctx, backup, restored, "new-generation"); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, restored, "new-generation"); !errors.Is(err, ErrRecoveryLockdown) {
		t.Fatalf("restored database opened normally: %v", err)
	}
	if err := ClearRecoveryLockdown(ctx, restored, "new-generation"); err == nil {
		t.Fatal("lockdown cleared before identity scrub")
	}
	if err := ScrubRestoredIdentities(ctx, restored, "new-generation"); err != nil {
		t.Fatal(err)
	}
	if err := ClearRecoveryLockdown(ctx, restored, "new-generation"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, restored, "new-generation")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var accounts int
	if err := reopened.readers.QueryRowContext(ctx, "SELECT count(*) FROM accounts").Scan(&accounts); err != nil || accounts != 0 {
		t.Fatalf("accounts=%d error=%v", accounts, err)
	}
}

func TestRestoreRejectsCorruptSource(t *testing.T) {
	directory := t.TempDir()
	corrupt := filepath.Join(directory, "corrupt.sqlite")
	if err := os.WriteFile(corrupt, []byte("not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(context.Background(), corrupt, filepath.Join(directory, "restored.sqlite"), "generation"); err == nil {
		t.Fatal("corrupt restore source accepted")
	}
}
