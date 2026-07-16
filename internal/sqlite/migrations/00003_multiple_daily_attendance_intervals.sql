-- +goose Up
CREATE TABLE attendance_plans_new (
    id INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    attendance_date TEXT NOT NULL CHECK (length(attendance_date) = 10),
    start_minute INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1410 AND start_minute % 30 = 0),
    end_minute INTEGER NOT NULL CHECK (end_minute BETWEEN 30 AND 1440 AND end_minute % 30 = 0 AND end_minute > start_minute),
    created_at_utc INTEGER NOT NULL,
    updated_at_utc INTEGER NOT NULL,
    UNIQUE (account_id, attendance_date, start_minute, end_minute)
) STRICT;

INSERT INTO attendance_plans_new (id, account_id, attendance_date, start_minute, end_minute, created_at_utc, updated_at_utc)
SELECT id, account_id, attendance_date, start_minute, end_minute, created_at_utc, updated_at_utc
FROM attendance_plans;

DROP TABLE attendance_plans;
ALTER TABLE attendance_plans_new RENAME TO attendance_plans;
CREATE INDEX attendance_plans_date_interval ON attendance_plans(attendance_date, start_minute, end_minute);
CREATE INDEX attendance_plans_account_date ON attendance_plans(account_id, attendance_date, start_minute);

-- +goose Down
SELECT 1;
