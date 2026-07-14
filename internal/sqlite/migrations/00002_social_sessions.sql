-- +goose Up
CREATE TABLE social_sessions (
    id INTEGER PRIMARY KEY,
    iso_weekday INTEGER NOT NULL CHECK (iso_weekday BETWEEN 1 AND 7),
    start_minute INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1410 AND start_minute % 30 = 0),
    end_minute INTEGER NOT NULL CHECK (end_minute BETWEEN 30 AND 1440 AND end_minute % 30 = 0 AND end_minute > start_minute),
    effective_start_date TEXT NOT NULL CHECK (length(effective_start_date) = 10),
    effective_end_date TEXT CHECK (effective_end_date IS NULL OR length(effective_end_date) = 10)
) STRICT;

CREATE TABLE synthetic_fixture_metadata (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    loaded_for_date TEXT NOT NULL CHECK (length(loaded_for_date) = 10)
) STRICT;

-- +goose Down
SELECT 1;
