# Sydney University Squash Club Attendance Tracker

A venue-wide turnout forecast for the Sydney University Squash Club. Members announce when they plan to attend; the application does not reserve either court.

## Proposed stack

- Go
- `net/http`
- HTMX
- Server-rendered HTML templates
- SQLite
- Plain CSS
- A single deployable container with durable storage in production

## Status

Product discovery, interface design, and engineering planning are approved. Synthetic-data implementation is in progress; public identity capabilities and a real-data pilot remain committee-gated.

![Synthetic attendance timetable](docs/images/attendance-timetable.png)

## Local development

The development service is deliberately synthetic-only and binds to loopback. The Makefile uses the Go toolchain on `PATH`, so run it with:

```sh
make run
```

Then open `http://127.0.0.1:18080`. The service creates a disposable database under `data/` and loads obvious synthetic timetable and account fixtures.

For development with automatic backend restarts, run:

```sh
make dev
```

The development watcher restarts the loopback-only synthetic Go service after
changes to Go, HTML, CSS, SQL, JSON, or YAML source files. Refresh the browser
after the restart to request the rebuilt page.

Run all generation, tests, race checks, vetting, and the 80% project-owned coverage gate with:

```sh
make check
```

Expected local tools: Go 1.26.4, `sqlc` 1.31.1, `goose` 3.27.1, and `jq` 1.8.1.

## Synthetic moderation

With the web process stopped, the deployment-local command accepts only an exact full username or four-digit player code:

```sh
go run ./cmd/moderate suspend --identifier 'john#1111'
go run ./cmd/moderate reinstate --identifier '1111'
```

Permanent deletion requires the exact immutable username twice and a new path for a verified, transactionally consistent pre-action snapshot:

```sh
go run ./cmd/moderate delete --identifier 'john#1111' --confirm 'john#1111' --backup data/pre-delete.sqlite
```

Production deletion remains disabled unless the retention-and-deletion approval in the deployment manifest is valid.
