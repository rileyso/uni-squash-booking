package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/domain"
)

func TestOpenMigratesAndVerifiesPragmas(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.sqlite")
	store, err := Open(ctx, path, "test-generation")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Ready(ctx, "test-generation"); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx, "other-generation"); !errors.Is(err, ErrGeneration) {
		t.Fatalf("generation mismatch error = %v", err)
	}
}

func TestSyntheticFixturesFeedIdentityFreeTimetable(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"), "test-generation")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	location, _ := time.LoadLocation("Australia/Sydney")
	today, _ := domain.ParseCivilDate("2026-07-14")
	if err := store.LoadSyntheticFixtures(ctx, today, location, "synthetic-disabled"); err != nil {
		t.Fatal(err)
	}
	// Loading twice for the same date is deliberately idempotent.
	if err := store.LoadSyntheticFixtures(ctx, today, location, "synthetic-disabled"); err != nil {
		t.Fatal(err)
	}
	data, err := store.LoadAnonymousTimetable(ctx, today, today.AddDays(6, location))
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Weekly) != 18 || len(data.Social) != 2 || len(data.OneOffs) != 0 || len(data.Attendance) != 17 {
		t.Fatalf("unexpected fixture dimensions: weekly=%d social=%d oneOffs=%d attendance=%d", len(data.Weekly), len(data.Social), len(data.OneOffs), len(data.Attendance))
	}
}

func TestStrictSchemaRejectsInvalidInterval(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"), "test-generation")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.writer.ExecContext(ctx, `INSERT INTO weekly_series (court, kind, iso_weekday, start_minute, end_minute, effective_start_date) VALUES (1, 'open', 2, 1, 60, '2026-07-14')`)
	if err == nil {
		t.Fatal("invalid interval was accepted")
	}
}

func TestRecoveryLockdownFailsReadiness(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"), "test-generation")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.writer.ExecContext(ctx, "UPDATE recovery_metadata SET recovery_lockdown = 1 WHERE singleton = 1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx, "test-generation"); !errors.Is(err, ErrRecoveryLockdown) {
		t.Fatalf("lockdown readiness error = %v", err)
	}
}
