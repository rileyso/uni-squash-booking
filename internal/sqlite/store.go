// Package sqlite is the concrete SQLite persistence adapter.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	"github.com/rileyso/uni-squash-booking/internal/sqlite/sqlcdb"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

var (
	ErrRecoveryLockdown = errors.New("database is in recovery lockdown")
	ErrGeneration       = errors.New("database recovery generation mismatch")
)

type Store struct {
	writer  *sql.DB
	readers *sql.DB
	queries *sqlcdb.Queries
}

func Open(ctx context.Context, path, generation string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	dsn := dataSourceName(path)
	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open writer: %w", err)
	}
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)

	if err := migrate(ctx, writer); err != nil {
		writer.Close()
		return nil, err
	}
	readers, err := sql.Open("sqlite", dsn)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("open readers: %w", err)
	}
	readers.SetMaxOpenConns(4)
	readers.SetMaxIdleConns(4)
	store := &Store{writer: writer, readers: readers, queries: sqlcdb.New(readers)}
	if err := store.initializeAndVerify(ctx, generation); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}

func dataSourceName(path string) string {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	return "file:" + filepath.ToSlash(path) + "?" + values.Encode()
}

func migrate(ctx context.Context, database *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	if err := goose.UpContext(ctx, database, "migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func (s *Store) initializeAndVerify(ctx context.Context, generation string) error {
	writerQueries := sqlcdb.New(s.writer)
	if _, err := writerQueries.InitializeRecoveryGeneration(ctx, generation); err != nil {
		return fmt.Errorf("initialize recovery generation: %w", err)
	}
	return s.Ready(ctx, generation)
}

func (s *Store) Ready(ctx context.Context, generation string) error {
	metadata, err := s.queries.GetRecoveryMetadata(ctx)
	if err != nil {
		return fmt.Errorf("read recovery metadata: %w", err)
	}
	if metadata.RecoveryLockdown != 0 {
		return ErrRecoveryLockdown
	}
	if metadata.RecoveryGeneration != generation {
		return ErrGeneration
	}
	var integrity string
	if err := s.readers.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&integrity); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("integrity check: %s", integrity)
	}
	return verifyPragmas(ctx, s.readers)
}

func verifyPragmas(ctx context.Context, database *sql.DB) error {
	var foreignKeys int
	if err := database.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		return fmt.Errorf("foreign_keys pragma is not enabled")
	}
	var journalMode string
	if err := database.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil || journalMode != "wal" {
		return fmt.Errorf("journal_mode pragma is not WAL")
	}
	return nil
}

func (s *Store) Close() error {
	return errors.Join(s.readers.Close(), s.writer.Close())
}
