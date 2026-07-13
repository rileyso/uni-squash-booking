-- +goose Up
CREATE TABLE recovery_metadata (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    recovery_generation TEXT NOT NULL DEFAULT '',
    recovery_lockdown INTEGER NOT NULL DEFAULT 0 CHECK (recovery_lockdown IN (0, 1)),
    admin_credential_fingerprint TEXT NOT NULL DEFAULT ''
) STRICT;
INSERT INTO recovery_metadata (singleton) VALUES (1);

CREATE TABLE accounts (
    id INTEGER PRIMARY KEY,
    player_code TEXT NOT NULL UNIQUE CHECK (length(player_code) = 4),
    full_username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    pin_hash TEXT NOT NULL,
    member_status TEXT NOT NULL CHECK (member_status IN ('member', 'visitor')),
    account_status TEXT NOT NULL DEFAULT 'active' CHECK (account_status IN ('active', 'suspended')),
    must_change_pin INTEGER NOT NULL DEFAULT 0 CHECK (must_change_pin IN (0, 1)),
    created_at_utc INTEGER NOT NULL
) STRICT;

CREATE TABLE member_sessions (
    id INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL UNIQUE,
    csrf_secret BLOB NOT NULL,
    created_at_utc INTEGER NOT NULL,
    expires_at_utc INTEGER NOT NULL CHECK (expires_at_utc > created_at_utc)
) STRICT;
CREATE INDEX member_sessions_account_id ON member_sessions(account_id);

CREATE TABLE admin_sessions (
    id INTEGER PRIMARY KEY,
    token_hash BLOB NOT NULL UNIQUE,
    csrf_secret BLOB NOT NULL,
    created_at_utc INTEGER NOT NULL,
    expires_at_utc INTEGER NOT NULL CHECK (expires_at_utc > created_at_utc)
) STRICT;

CREATE TABLE weekly_series (
    id INTEGER PRIMARY KEY,
    court INTEGER NOT NULL CHECK (court IN (1, 2)),
    kind TEXT NOT NULL CHECK (kind IN ('open', 'lights_off', 'competition', 'coaching', 'closure')),
    title TEXT NOT NULL DEFAULT '',
    iso_weekday INTEGER NOT NULL CHECK (iso_weekday BETWEEN 1 AND 7),
    start_minute INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1410 AND start_minute % 30 = 0),
    end_minute INTEGER NOT NULL CHECK (end_minute BETWEEN 30 AND 1440 AND end_minute % 30 = 0 AND end_minute > start_minute),
    effective_start_date TEXT NOT NULL CHECK (length(effective_start_date) = 10),
    effective_end_date TEXT CHECK (effective_end_date IS NULL OR length(effective_end_date) = 10)
) STRICT;

CREATE TABLE schedule_exceptions (
    id INTEGER PRIMARY KEY,
    weekly_series_id INTEGER NOT NULL REFERENCES weekly_series(id) ON DELETE CASCADE,
    occurrence_date TEXT NOT NULL CHECK (length(occurrence_date) = 10),
    cancelled INTEGER NOT NULL DEFAULT 0 CHECK (cancelled IN (0, 1)),
    replacement_court INTEGER CHECK (replacement_court IS NULL OR replacement_court IN (1, 2)),
    replacement_kind TEXT CHECK (replacement_kind IS NULL OR replacement_kind IN ('open', 'lights_off', 'competition', 'coaching', 'closure')),
    replacement_start_minute INTEGER CHECK (replacement_start_minute IS NULL OR (replacement_start_minute BETWEEN 0 AND 1410 AND replacement_start_minute % 30 = 0)),
    replacement_end_minute INTEGER CHECK (replacement_end_minute IS NULL OR (replacement_end_minute BETWEEN 30 AND 1440 AND replacement_end_minute % 30 = 0)),
    UNIQUE (weekly_series_id, occurrence_date)
) STRICT;

CREATE TABLE one_off_events (
    id INTEGER PRIMARY KEY,
    event_date TEXT NOT NULL CHECK (length(event_date) = 10),
    court INTEGER NOT NULL CHECK (court IN (1, 2)),
    kind TEXT NOT NULL CHECK (kind IN ('open', 'lights_off', 'competition', 'coaching', 'closure')),
    title TEXT NOT NULL DEFAULT '',
    start_minute INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1410 AND start_minute % 30 = 0),
    end_minute INTEGER NOT NULL CHECK (end_minute BETWEEN 30 AND 1440 AND end_minute % 30 = 0 AND end_minute > start_minute)
) STRICT;

CREATE TABLE attendance_plans (
    id INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    attendance_date TEXT NOT NULL CHECK (length(attendance_date) = 10),
    start_minute INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1410 AND start_minute % 30 = 0),
    end_minute INTEGER NOT NULL CHECK (end_minute BETWEEN 30 AND 1440 AND end_minute % 30 = 0 AND end_minute > start_minute),
    created_at_utc INTEGER NOT NULL,
    updated_at_utc INTEGER NOT NULL,
    UNIQUE (account_id, attendance_date)
) STRICT;
CREATE INDEX attendance_plans_date_interval ON attendance_plans(attendance_date, start_minute, end_minute);

-- +goose Down
SELECT 1;
