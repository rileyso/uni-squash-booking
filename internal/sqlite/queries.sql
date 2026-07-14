-- name: GetRecoveryMetadata :one
SELECT recovery_generation, recovery_lockdown, admin_credential_fingerprint
FROM recovery_metadata
WHERE singleton = 1;

-- name: InitializeRecoveryGeneration :execrows
UPDATE recovery_metadata
SET recovery_generation = ?
WHERE singleton = 1 AND recovery_generation = '' AND recovery_lockdown = 0;

-- name: CountAccounts :one
SELECT count(*) FROM accounts;

-- name: ListWeeklySeries :many
SELECT id, court, kind, title, iso_weekday, start_minute, end_minute,
       effective_start_date, effective_end_date
FROM weekly_series
WHERE effective_start_date <= sqlc.arg(through_date)
  AND (effective_end_date IS NULL OR effective_end_date >= sqlc.arg(from_date))
ORDER BY iso_weekday, start_minute, court, id;

-- name: ListOneOffEvents :many
SELECT id, event_date, court, kind, title, start_minute, end_minute
FROM one_off_events
WHERE event_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(through_date)
ORDER BY event_date, start_minute, court, id;

-- name: ListSocialSessions :many
SELECT id, iso_weekday, start_minute, end_minute, effective_start_date, effective_end_date
FROM social_sessions
WHERE effective_start_date <= sqlc.arg(through_date)
  AND (effective_end_date IS NULL OR effective_end_date >= sqlc.arg(from_date))
ORDER BY iso_weekday, start_minute, id;

-- name: ListAnonymousAttendanceIntervals :many
SELECT attendance_date, start_minute, end_minute
FROM attendance_plans
WHERE attendance_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(through_date)
ORDER BY attendance_date, start_minute, end_minute;
