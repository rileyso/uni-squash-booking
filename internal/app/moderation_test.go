package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

func TestModerationSuspensionRevokesSessionsAndFuturePlans(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	created, err := service.CreateAccount(ctx, "Moderated Member", "2468", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SaveAttendance(ctx, created.Member, "2026-07-15", 960, 1020); err != nil {
		t.Fatal(err)
	}

	account, err := service.SuspendAccount(ctx, created.Member.FullUsername)
	if err != nil {
		t.Fatal(err)
	}
	if account.Status != "suspended" {
		t.Fatalf("status = %q", account.Status)
	}
	if _, _, err := service.MemberForToken(ctx, created.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("session survived: %v", err)
	}
	if plans, err := service.Plans(ctx, created.Member); err != nil || len(plans) != 0 {
		t.Fatalf("future plans = %#v, error = %v", plans, err)
	}
	playerCode := created.Member.FullUsername[strings.LastIndex(created.Member.FullUsername, "#")+1:]
	if _, err := service.SuspendAccount(ctx, playerCode); err != nil {
		t.Fatalf("idempotent suspension: %v", err)
	}
	if _, err := service.ReinstateAccount(ctx, playerCode); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SignIn(ctx, created.Member.FullUsername, "2468"); err != nil {
		t.Fatalf("sign in after reinstatement: %v", err)
	}
	if _, err := service.ReinstateAccount(ctx, created.Member.FullUsername); !errors.Is(err, sqlite.ErrAccountState) {
		t.Fatalf("second reinstatement error = %v", err)
	}
}

func TestPermanentDeletionRequiresSuccessfulBackupAndExactConfirmation(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	created, err := service.CreateAccount(ctx, "Delete Target", "2468", "visitor", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PermanentlyDeleteAccount(ctx, created.Member.FullUsername, "wrong", filepath.Join(t.TempDir(), "wrong.sqlite")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("confirmation error = %v", err)
	}

	existing := filepath.Join(t.TempDir(), "existing.sqlite")
	if err := os.WriteFile(existing, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PermanentlyDeleteAccount(ctx, created.Member.FullUsername, created.Member.FullUsername, existing); err == nil {
		t.Fatal("deletion proceeded after backup failure")
	}
	if _, err := service.SignIn(ctx, created.Member.FullUsername, "2468"); err != nil {
		t.Fatal("backup failure deleted the account")
	}

	backup := filepath.Join(t.TempDir(), "pre-delete.sqlite")
	account, err := service.PermanentlyDeleteAccount(ctx, created.Member.FullUsername, created.Member.FullUsername, backup)
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != created.Member.ID {
		t.Fatalf("deleted account ID = %d", account.ID)
	}
	if info, err := os.Stat(backup); err != nil || info.Size() == 0 {
		t.Fatalf("backup info=%v error=%v", info, err)
	}
	if _, err := service.SignIn(ctx, created.Member.FullUsername, "2468"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("deleted account sign in error = %v", err)
	}
}
