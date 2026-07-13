# Sydney University Squash Club Attendance Tracker Engineering Plan

Status: APPROVED — SYNTHETIC IMPLEMENTATION AUTHORISED  
Date: 14 July 2026  
Product source: `PRODUCT_DESIGN.md`  
Interface source: `DESIGN.md`

## Objective

Define the smallest reliable Go/SQLite architecture that can test whether members use a forward-looking turnout forecast to choose suitable squash sessions. Synthetic-data milestones M0-M4 are authorised now. Committee decisions gate production capabilities and the real-data pilot in M5, not foundation implementation.

## Implementation authorisation and capability gates

- M0-M4 may be implemented and exercised only with disposable synthetic data.
- Development and test use `APP_ENV=development|test`, bind to loopback, and display an unmistakable synthetic-data banner.
- Production uses `APP_ENV=production` plus a typed approval manifest recording status, approver, and date for public named attendance, public account creation/visitor attendance, PIN-reset identity evidence, and retention/account deletion.
- Missing or invalid production approvals fail closed. Disabled HTTP routes are not registered, and the corresponding application use cases and local commands independently reject the capability.
- Anonymous timetable/aggregate queries never select identity fields. Named attendee details and attendance previews use separate queries and routes.
- Public account launch requires both public-account approval and an approved recovery method. Self-deletion requires an approved retention rule.
- No real member data may be loaded before M5.

## Step 0 Scope Decision

Decision D1: reduce the first implementation release.

### Release 1 includes

- Public-internet read access at a stable URL to the next 14 Sydney calendar dates, conditional on written committee approval
- Venue-wide turnout by 30-minute interval and public display-name details
- Court 1 and Court 2 operational status, light hours, competitions, coaching, closures, official social sessions, and weekly recurrence with dated exceptions
- Self-service permanent accounts, full username plus four-digit PIN sign-in, secure sessions, PIN change, Riley-assisted PIN recovery, and self-service account deletion
- One venue-wide attendance plan per account per day, including add, change, remove, schedule revalidation, reduced-capacity warnings, and schedule-changed warnings
- Riley's protected web administration workflow for maintaining the operational schedule and resetting PINs, plus a deployment-local moderation CLI
- Club location, contact, and WhatsApp information
- Self-reported member/non-member status stored for later aggregate analysis
- Security controls required by the accepted short-PIN model: credential hashing, per-account and network throttling, session revocation, CSRF protection, and audit-safe operational logging

### Deferred until after the turnout hypothesis is tested

- Public `Past plans` and 30-day named-history interface
- Adoption-report and member/non-member aggregate-report interfaces
- Web-based account suspension, reinstatement, impersonation moderation, and administrator-driven permanent deletion; Release 1 provides only the approved deployment-local CLI
- Dated schedule annotations
- General news, automated notifications, Google Calendar integration, and every item already listed as outside the MVP in `PRODUCT_DESIGN.md`

Account self-deletion remains in Release 1 because public upcoming names require a member-controlled removal path. Weekly schedule recurrence and dated exceptions remain because an inaccurate or burdensome schedule would invalidate the turnout forecast.

## What Already Exists

- `go.mod` declares the Go module; there is no application code, migration, test framework, or CI pipeline yet.
- `PRODUCT_DESIGN.md` defines the domain rules, privacy constraints, accepted risks, and committee blockers.
- `DESIGN.md` defines the server-visible interaction states, responsive behavior, accessibility requirements, and later implementation tasks.
- `AGENTS.md` defines the approved non-booking vocabulary and preferred modular-monolith stack.
- `TODOS.md` contains one approved design-validation task for the mobile navigation rail; it does not block architecture work.
- `README.md` still describes a booking application and must be reconciled before implementation documentation is considered trustworthy.

## Boundaries during implementation

- Real member data or public launch before M5.
- A deployment-vendor commitment before operational ownership and budget are known — distribution requirements will still be specified.
- Features deferred by the Release 1 scope decision — they must not shape Release 1 handlers, templates, or administration screens beyond clean data boundaries.
- University SSO, microservices, queues, event streaming, Kubernetes, Redis, or a single-page JavaScript framework — none is justified by a 231-member synchronous club tool.

## Review Coverage

- Domain and package boundaries
- SQLite data model, recurrence representation, and retention boundaries
- Member and administrator authentication and session storage
- Atomic attendance replacement and schedule-conflict handling
- Rate limiting and trusted client-address handling
- Server-rendered/HTMX request and error contracts
- Deployment, backup, migration, observability, and recovery model
- Test architecture and full branch/interaction coverage
- Performance query shape and caching posture

## Approved Architecture Decisions

### D2: SQLite for Release 1 persistence

- Release 1 uses one SQLite database file through Go's `database/sql` API.
- SQLite runs with foreign-key enforcement enabled, WAL journaling, a bounded busy timeout, and short explicit write transactions.
- The application is deployed as one active Go process on one machine. Horizontal application replicas and a database file on network storage are unsupported.
- Database constraints remain authoritative for scalar invariants such as immutable identifiers, unique full usernames/player codes, one attendance plan per account and Sydney date, foreign keys, and valid enum/check values.
- Schedule overlap and continuous-availability rules that SQLite cannot express cleanly are revalidated inside the same write transaction that commits the change.
- Backups must use SQLite's online backup mechanism or a transactionally consistent snapshot; copying a live database/WAL pair ad hoc is not an approved backup procedure.
- Schema and queries avoid unnecessary SQLite-specific behavior so a later PostgreSQL migration remains possible, but Release 1 does not build or maintain dual-database support.
- CSV files are permitted only for explicit export/import tooling in a later phase, never as the authoritative transactional store.

### D3: Small layered modular monolith

```text
cmd/web
   │ composition root and process lifecycle
   ▼
internal/web
   │ HTTP routes, middleware, HTMX/full-page responses, templates, view models
   ▼
internal/app
   │ account, attendance, and schedule use cases; transaction ownership
   ▼
internal/domain
   │ pure value types, invariants, calculations, and typed errors
   ▼
internal/sqlite
     migrations, generated or handwritten queries, concrete transaction adapter
```

- Dependencies flow downward only. `domain` imports no web, database, or infrastructure packages.
- `app` is the only layer that coordinates a use case spanning accounts, attendance, schedules, and a write transaction.
- `web` maps requests into application commands and typed outcomes into full-page or HTMX fragments; it contains no authoritative schedule or attendance rules.
- `sqlite` implements concrete persistence. Do not introduce repository interfaces until a real second implementation or a meaningful test seam requires one.
- The composition root constructs concrete dependencies explicitly; no dependency-injection framework or global database handle is used.
- The application remains one Go binary and one active process.

### D4: Sydney civil dates and minute offsets

- Domain dates use a dedicated value type serialized to strict ISO `YYYY-MM-DD` text and interpreted only in `Australia/Sydney`.
- Attendance and schedule boundaries use integer minutes since local midnight, constrained to `0..1440`, divisible by 30, with `end_minute > start_minute`.
- Weekly rules store an ISO weekday plus local start/end minutes. They do not store a fixed UTC offset.
- The application loads `Australia/Sydney` explicitly and fails startup if timezone data is unavailable; it never relies on the host machine's local timezone.
- The 14-day window and one-plan-per-day invariant use the Sydney civil date derived from the injected application clock.
- Security/session timestamps and operational metadata use UTC Unix seconds with clear column naming such as `created_at_utc` and `expires_at_utc`.
- Tests inject a clock and cover both Sydney daylight-saving boundaries even though normal club sessions are unlikely to occur during the transition hour.
- SQLite tables use `STRICT` mode plus `CHECK` constraints so flexible SQLite typing cannot silently admit malformed dates, minute values, or booleans.

### D5: Weekly series plus dated exceptions

```text
weekly series ──expand for requested dates──┐
                                            ├── precedence resolver ──> court status buckets
one-off events ─────────────────────────────┤
dated cancel/replace exceptions ────────────┘

This occurrence: series + dated exception
This and future: end old series + create replacement series (one transaction)
```

- Weekly series store schedule kind, court target, weekday, local start/end minutes, effective start date, and optional end date.
- One-off events store an explicit Sydney date and interval.
- A dated exception identifies one series occurrence and either cancels it or supplies replacement values for that date.
- Expansion is bounded to the requested timetable or validation range; Release 1 never materializes an open-ended future calendar.
- Precedence is a pure domain function: closures/lights-off override occupations, occupations override open-for-play, and contradictory coaching/competition on the same court is rejected.
- `This and future` follows D33: preview later exceptions, require explicit destructive confirmation, then split the series, create the replacement, and delete those exceptions in one SQLite transaction.
- Reads use a deterministic order and return an explicit conflict error if persisted data violates assumptions; they never silently pick whichever overlapping row was read last.
- The series expansion and precedence resolver should carry an inline ASCII diagram in `internal/domain` because stale recurrence logic would mislead both implementation and tests.

### D6: Argon2id PIN hashes without a pepper

- Member PINs and temporary PINs are never stored as plaintext or reversible ciphertext.
- Each account stores an encoded Argon2id hash containing a unique random salt, algorithm identifier, version, and calibrated work parameters.
- Work parameters are benchmarked on the production-sized host, kept below the service's request timeout, and upgradeable on a later successful sign-in.
- Verification uses constant-time comparison after hashing. Unknown usernames execute a calibrated dummy hash so response timing does not disclose account existence.
- Cheap per-source and per-account throttling occurs before Argon2 work to limit CPU denial of service. Generic responses do not reveal whether an account exists, is locked, or used a wrong PIN.
- Temporary PINs use the same hashing path and carry a `must_change_pin` state; Riley never reads an existing PIN.
- Accepted risk: because there are only 10,000 possible PINs and no server-held pepper, theft of the SQLite database or a backup permits practical offline enumeration. File permissions, encrypted backup storage, access control, and prompt incident-driven credential resets are required operational controls, not substitutes for hashing.

### D7: Opaque server-side member sessions

- Successful sign-in creates a random 256-bit session token using `crypto/rand`; the browser receives the token once and SQLite stores only a cryptographic hash.
- The cookie is `HttpOnly`, `Secure`, `SameSite=Lax`, scoped narrowly to the application, and has a seven-day absolute expiry enforced by the server.
- Each device has an independent session row. Explicit sign-out deletes that session server-side before expiring the browser cookie.
- A member PIN change verifies the current PIN, commits the new hash, preserves only the current session, and deletes every other member session in one transaction.
- Riley's reset, account deletion, or security-sensitive account state change deletes all member sessions in the same transaction as the state change.
- Session lookup is an indexed SQLite read. Expired rows are rejected regardless of cookie expiry and removed opportunistically in bounded batches.
- Authentication middleware exposes an optional principal to member-facing reads. Mutation handlers require a principal and still enforce ownership in the application use case.
- CSRF protection is required independently of `SameSite`: every state-changing form carries a session-bound token and rejects missing or invalid tokens.
- Session identifiers, raw cookies, PINs, and CSRF tokens are redacted from logs and error reports.

### D8: Last-write-wins attendance replacement

- SQLite enforces one attendance plan per account and Sydney civil date with a unique constraint.
- Add/change uses an atomic upsert keyed by account and date. Remove deletes the currently stored plan for that owned account/date even if another tab rendered an older value.
- Every mutation runs in a short write transaction that rechecks the authenticated account, date window, interval shape, and continuous current schedule availability before writing.
- Schedule revalidation never trusts a timetable revision or form field supplied by the browser. If the interval is no longer valid, no write occurs and the approved preserved-input conflict UI is returned.
- The response always re-renders the authoritative interval and `Your plans` state after commit; it never assumes that submitted values became final without reading the result.
- Duplicate rapid submissions are safe because the unique key and upsert make the final row well-formed, though the most recently committed valid request wins.
- Accepted risk: two stale tabs can silently replace or remove a newer plan. Release 1 does not carry an optimistic plan version or present a lost-update conflict.
- A process-wide mutex is not part of correctness. Database transactions and constraints remain authoritative so a future storage migration does not inherit hidden in-memory locking assumptions.

### D9: Deployment-secret administrator credential

- Riley's fixed administrator username and encoded Argon2id password hash are provided through deployment secrets, never committed to the repository or stored in SQLite backups.
- The administrator password is a long credential, not a four-digit PIN, and uses independently calibrated hashing and stricter throttling.
- Successful administrator authentication creates a separate opaque server-side admin session with a short absolute lifetime. Member sessions can never acquire administrator authority.
- Administrator cookies use a distinct name and narrow path where practical, with `HttpOnly`, `Secure`, and `SameSite=Strict`; every mutation also requires CSRF validation.
- Rotation replaces the deployment-secret hash and restarts the single service. Startup validates that a non-default hash is configured outside local development and applies D32 before readiness.
- No administrator password change, recovery, multiple-admin, or role-assignment UI is built in Release 1.
- Admin authentication and schedule/PIN-reset operations receive security event logs without credential, token, or sensitive member data.

### D10: Bounded in-memory throttling

- Member sign-in, administrator sign-in, account creation, and reset-sensitive operations use separate bounded in-memory throttle stores with explicit window and lock-expiry rules.
- Keys combine the relevant account identifier where known with a normalized client network source. Account creation also uses a random signed anonymous device cookie to enforce the approved browser/device limit.
- The store has maximum entry counts, periodic expiry, and deterministic eviction so arbitrary source identifiers cannot exhaust process memory.
- Cheap throttle checks run before Argon2 verification. A generic response hides account existence, lock state, and whether the account or source limit fired.
- Client addresses come from `RemoteAddr` unless the immediate reverse proxy is explicitly trusted. Forwarding headers from arbitrary clients are ignored.
- Tests use an injected clock and cover window rollover, lock expiry, bounded eviction, account/network combinations, unknown accounts, and concurrent access under the race detector.
- Accepted risk: every throttle and lock resets when the process restarts. A deliberate attacker able to trigger or wait for restarts can bypass accumulated limits.
- This choice reinforces the one-active-process constraint. A second process would enforce independent counters and is unsupported until throttling moves to shared storage.

### D11: Single container, durable local volume, off-host backups

```text
GitHub Actions ──build/test──> versioned Linux container image
                                      │
                                      ▼
                         one active application instance
                                      │
                         local persistent volume (SQLite)
                                      │ consistent online snapshot
                                      ▼
                         encrypted off-host backup storage
```

- Production runs exactly one active container instance with the SQLite database, WAL, and temporary database files on a durable local volume supported by the hosting provider.
- SQLite files never live on NFS, shared network storage, an image layer, or an ephemeral filesystem.
- Templates, migrations, timezone data, and static assets are embedded into the versioned Go binary/container; member and configuration secrets remain external.
- CI runs formatting, vetting, unit/integration tests, migration tests, race-sensitive tests, and a container smoke test before publishing an immutable Linux image.
- Deployment follows D34: stop the old process, create and verify the preflight backup, apply forward migrations under an exclusive startup lock, start and health-check the new image, and retain the previous image for rollback only after restoring the preflight database.
- A scheduled backup uses the driver's online-backup support or `VACUUM INTO`, verifies integrity, encrypts the result, and copies it off-host. Retention and destination depend on committee/university policy.
- CI restore tests into a clean environment are required before launch. The accepted D26 decision does not require a human off-host restore drill, leaving operator access and recovery readiness as explicit risks.
- Readiness fails if the volume is unavailable, migrations are incomplete, required secrets are missing, or SQLite integrity checks fail. Liveness reports process health without leaking database details.
- Ephemeral storage is allowed only for local development, CI, and disposable demonstrations containing no real member data.

### D12: Derive schedule-change warnings on read

- Attendance plans store only the member's stated Sydney date and interval; they do not store a mutable `schedule_changed` flag.
- Timetable, interval-detail, confirmation, and `Your plans` queries expand the current authoritative schedule for the relevant dates and evaluate every plan against the resulting 30-minute venue-availability buckets.
- A plan is valid only when at least one court is open throughout every covered bucket. Reduced capacity and invalid sub-intervals are derived in the same pure domain calculation.
- Restoring schedule availability automatically removes the warning on the next authoritative render without an attendance-row update.
- Riley's schedule transaction validates schedule invariants but never rewrites or silently removes member plans.
- The 14-day window and club size keep this calculation bounded. Tests include closures added, removed, shortened, and switched between courts at interval boundaries.

### D13: Public stable dashboard URL, conditional on committee approval

- Anonymous visitors can open the attendance dashboard at a stable URL without a club token or access cookie.
- This supersedes the shared-link distribution barrier for read access. It does not turn `robots.txt`, `noindex`, or `nofollow` into access control.
- Pages containing public display names and planned intervals send restrictive indexing, framing, referrer, and cache headers; application logs avoid query data and rendered personal information.
- Anonymous scraping and discovery remain possible. The interface must state plainly that upcoming display names and plans are public on the internet, not merely visible to club-link recipients.
- Committee approval for public named attendance becomes a hard implementation and launch blocker. Approval of a forwardable shared link alone is insufficient for this expanded exposure.
- If the committee rejects public-internet names, this decision must be revisited rather than silently falling back to a token URL.
- Account creation and mutation access are a separate architecture decision; public read access does not automatically authorise unrestricted public writes.

### D14: Fully public self-service account creation

- Anyone who discovers the public dashboard may create an account without a club invite code, verified membership, or managed bot challenge.
- Release 1 relies only on the approved bounded in-memory device/network creation limits, input validation, unique database constraints, and generic failure responses.
- Account and player-code allocation is transactional. Random code collisions retry within a fixed bound; exhaustion or repeated collision returns a generic temporary-unavailable response without partially creating an account.
- Account creation records self-reported member status but never presents it as verified identity or membership.
- The privacy notice must explain public display names and plans before creation and confirmation. Creation cannot be completed without explicit acknowledgement.
- Accepted risks: distributed sources and service restarts can bypass throttles; fake or impersonating accounts can pollute turnout; automated creation can consume the finite 10,000 player-code namespace.
- Written committee approval must cover unrestricted public account creation, not merely creation by people who received a club link.
- This decision creates a dependency on a minimum abuse-response workflow; deferring all moderation would leave Riley unable to restore forecast trust after public abuse.

### D15: Deployment-local moderation command

- The same versioned Go binary exposes an operator-only command group for exact account lookup, suspension, reinstatement, and permanent deletion.
- Commands run only with local deployment access and direct SQLite access; they are never exposed as public HTTP routes or hidden web endpoints.
- Targets require the immutable full username or player code. Display-name-only targeting is rejected because display names are not unique.
- Suspension transactionally changes account state, revokes all sessions, removes future plans from the forecast, and leaves past data subject to the eventual retention decision.
- Reinstatement restores sign-in eligibility but never recreates removed future plans.
- Permanent deletion requires the exact immutable username to be repeated in an explicit confirmation argument, creates a pre-action consistent backup, then applies the approved anonymisation/removal transaction.
- Every operation emits a redacted structured security log containing action, target account ID, outcome, and UTC time. Logs never contain PINs, session tokens, or attendance names beyond the exact operator-supplied identifier.
- CLI integration tests use temporary SQLite databases and cover wrong target, already-suspended, reinstatement, deletion, transaction failure, and backup failure. A failed pre-delete backup blocks deletion.
- Accepted operational burden: Riley needs deployment-shell access or help from whoever operates the service during an abuse incident.

### D31: Restore generation and privacy lockdown

- The deployment holds a random recovery generation outside SQLite. SQLite stores its current generation plus a durable `recovery_lockdown` flag.
- A missing generation, a mismatch, or an asserted lockdown makes readiness fail before ordinary routes are served. Only the deployment-local recovery command may operate in that state.
- Disaster restore runs with the application stopped. It rotates the deployment generation, restores and verifies the snapshot, sets lockdown, removes every account, profile, credential, session, identity link, and future attendance plan, and retains only committee-approved anonymous aggregates.
- Members recreate accounts after recovery. This intentionally chooses privacy continuity over account continuity; an old backup must never republish deleted identities or credentials.
- The long deployment-held administrator credential remains the recovery authority. Recovery revokes all stored administrator sessions, and Riley authenticates afresh before the local command can clear lockdown after integrity, migration, and scrub verification.
- CI restores snapshots from before identity deletion and credential changes and proves the restored service cannot become normally ready until identity scrubbing and explicit recovery clearance succeed.

### D32: Revoke admin sessions when the configured credential changes

- The application derives a one-way non-secret fingerprint/version from the configured encoded administrator hash and stores only that fingerprint in SQLite.
- On startup, a changed fingerprint atomically updates SQLite and deletes every administrator session before readiness succeeds.
- An unchanged fingerprint does not revoke sessions merely because the application restarted.
- The fingerprint cannot be used to verify or reconstruct the administrator password and is never treated as the credential itself.
- A database transaction failure or ambiguous partial rotation keeps the service unready.
- Tests cover first startup, unchanged restart, successful rotation, old-session rejection, database failure, and restoring a snapshot containing the prior fingerprint and sessions.

### D33: Confirmed deletion of future exceptions on series split

- Before a `This and future` edit, the application queries every exception on or after the split date and presents Riley with the exact count and affected Sydney dates.
- The confirmation form binds the series ID, split date, proposed replacement, and a server-issued preview revision so a stale confirmation cannot delete exceptions added after preview.
- On confirmation, one writer transaction revalidates the preview revision, ends the original series, creates the replacement series, and deletes every original-series exception on or after the split date.
- Cancel, revision mismatch, invalid replacement, or any database failure leaves the original series and all exceptions unchanged.
- Deleted exceptions are not copied or automatically recreated. The completion state tells Riley they were removed and must be re-added deliberately where still needed.
- Tests cover zero/one/many future exceptions, an exception on the split date, concurrent exception creation, stale confirmation, rollback after each write step, and the completion summary.

## Architecture Review Summary

- State: greenfield innovation with a deliberately boring single-process architecture.
- Blast radius is contained to one Go service, one local SQLite database, and one backup destination.
- No queues, caches, microservices, ORMs, distributed locks, background workers, or multi-instance coordination are introduced.
- The main residual risks are public named exposure, unrestricted account creation, restart-reset throttles, last-write-wins attendance changes, low-entropy PINs without a pepper, and single-operator recovery.
- Architecture decisions are internally compatible only under the one-active-process deployment constraint.

## Code Quality Decisions

### D16: Explicit SQL with `sqlc` generation

- Schema migrations and named queries are reviewed as explicit SQL; Release 1 does not introduce an ORM or query builder.
- `sqlc` generates the `database/sql` parameter, result, and row-scanning code into `internal/sqlite`.
- Generated files are committed and carry generated-code headers. They are never edited by hand.
- CI regenerates queries and fails when the worktree changes, preventing stale generated code from merging.
- `internal/sqlite` wraps generated query methods only where transaction composition, SQLite error translation, or domain-type conversion is required; it does not duplicate simple generated CRUD methods.
- `internal/app` owns transaction boundaries and passes the transaction-bound generated query set to persistence operations. Generated database models do not escape into `internal/domain` or web view models.
- Queries used to construct the 14-day timetable are range-oriented and return the necessary rows in bounded batches; template rendering never performs database calls.
- SQL query tests run against a real temporary SQLite database with the current migrations rather than mocking generated methods.

### D17: Pure-Go SQLite driver with pinned transitive runtime

- `modernc.org/sqlite` is the Release 1 `database/sql` driver; the application does not require CGO or a system SQLite library.
- `go.mod` pins both the driver and the exact `modernc.org/libc` version required by that driver. Automated dependency updates must update and test them together.
- CI builds and runs database integration tests on the same Linux architecture used by production, plus the developers' supported local architecture where practical.
- A startup database check verifies required pragmas and compile-time behavior rather than assuming DSN options were applied.
- Driver-specific imports, error codes, backup calls, and DSN construction remain confined to `internal/sqlite`; application and domain packages depend only on project-owned types.
- The container smoke test proves that the binary starts and accesses a migrated SQLite database without a C runtime toolchain or external SQLite shared library.

### D18: Embedded forward-only `goose` SQL migrations

- Numbered, immutable SQL migrations live beside the SQLite adapter and are embedded into the versioned binary.
- Startup takes the approved consistent preflight backup, acquires the exclusive migration/startup lock, and applies pending `goose` migrations before readiness can succeed.
- Migration files contain explicit `Up` changes. Production recovery restores a tested pre-migration backup instead of relying on lossy or untested `Down` migrations.
- Each migration is transactional unless SQLite requires otherwise. A non-transactional migration needs an inline rationale, an idempotent recovery procedure, and a dedicated failure-path test.
- Applied migration state is stored in the database. Missing historical files, duplicate versions, a schema newer than the binary, or a failed migration are fatal startup conditions.
- CI migrates an empty database and representative prior-version fixtures to current, runs foreign-key and integrity checks, and verifies that `sqlc` generation matches the resulting schema.
- Existing migration files are never edited after release; corrections use a new forward migration.

### D34: Bounded-downtime migration and rollback

- A deployment acquires the operator/deployment lock and stops the old application before touching the local SQLite volume; Release 1 does not attempt zero-downtime migration.
- It creates and verifies the consistent preflight SQLite backup, then applies embedded forward migrations and starts exactly one new container.
- Readiness, schema version, integrity, recovery generation, lockdown state, and a bounded smoke transaction must pass before deployment is declared successful.
- On migration or health-check failure, the new process stops, the preflight snapshot is restored, integrity and recovery-lockdown state are verified, and only then is the prior immutable image started.
- The prior image never opens a database migrated by a newer image. Down migrations and simultaneous old/new processes against the same file are unsupported.
- The maintenance response communicates temporary unavailability without exposing schema, backup, or filesystem details.
- CI rehearses successful upgrade and failure at backup, migration, startup, readiness, restore lockdown, identity scrub, and prior-image restart boundaries.

### D35: Deployment evidence for volume durability

- Application startup checks only that configured paths exist, have required permissions, support SQLite operations, and pass database integrity/recovery-generation checks; it never claims to detect provider durability.
- Before launch, the selected deployment manifest or infrastructure configuration is reviewed to confirm a provider-backed persistent local volume, explicit mount path, retention/deletion policy, and one-instance attachment semantics.
- A host rehearsal writes synthetic state, replaces the application container, restarts the service/host through the provider-supported path, and proves the same database generation and state remain available and valid.
- The rehearsal also proves that rebuilding or replacing only the container image does not initialize an empty database over the mounted volume.
- Evidence records the provider, volume identifier, mount, commands/actions, before/after checksums or application records, date, and operator; secrets and real member data are excluded.
- A writable ephemeral development/CI path may pass runtime probes but cannot satisfy the production deployment gate without manifest evidence and the host rehearsal.

### D19: Progressive HTML with paired page and fragment renderers

- Every member and administrator workflow has a functional server-rendered HTML form and link path without JavaScript.
- Full-page and HTMX handlers call the same application use case, authorization checks, validator, and transaction; HTMX never becomes a parallel business API.
- Successful ordinary mutations use Post/Redirect/Get. The equivalent HTMX mutation returns a named authoritative fragment or issues an HTMX redirect when the whole page context has changed.
- Renderers are explicit: a page template owns document structure and composes fragment templates; handlers never slice HTML strings or conditionally emit half a document from one ambiguous template.
- Responses that vary by `HX-Request` send the appropriate `Vary` header and restrictive cache headers for personalised content.
- Unsupported or malformed HTMX headers fall back safely to the ordinary HTML path. Business decisions never depend on a client-asserted HTMX header.
- Fragment contracts are documented beside routes by target element, swap strategy, success status, validation status, conflict status, and focus destination.
- Go HTTP/template tests exercise the full-page and HTMX response contracts for attendance, removal, sign-in, and administrator schedule changes. The release checklist exercises the no-JavaScript baseline and enhanced HTMX flows in real browsers, including DOM replacement and focus.

### D20: Small sentinel-error vocabulary

- `internal/app` exposes a closed, documented set of sentinel categories checked with `errors.Is`, including invalid input, unauthenticated, forbidden, not found, conflict, throttled, and temporarily unavailable.
- Expected validation failures return a separate safe result structure containing field identifiers and presentation-safe messages; handlers do not parse error text.
- Wrapped internal causes retain context with `%w` for logs and diagnostics but never cross into rendered member or administrator messages.
- A central web mapper owns HTTP status, full-page state, HTMX fragment state, and retry headers for each sentinel. Individual handlers do not invent mappings.
- SQLite constraint and busy errors are translated once inside `internal/sqlite` into the narrow application categories the caller can act upon; driver errors do not escape to templates.
- Unknown errors map to a generic server error, receive a correlation identifier, and are logged once at the outer HTTP boundary to avoid duplicate stack-like noise.
- Tests table-drive every sentinel mapping and verify that wrapped errors still match while internal SQL, paths, credentials, and submitted PINs never appear in responses.

### D21: Plain standard-library logging with stable fields

- Release 1 uses the standard `log` package and does not add `slog` or a third-party logging framework.
- Operational and security events follow a documented single-line `key=value` convention with fixed event names, UTC timestamps, severity text, and a request or operation correlation identifier.
- Request middleware creates or validates a bounded internal correlation ID; it does not reflect an arbitrary untrusted header into logs.
- A project-owned redaction helper is the only path for security-sensitive event logging. PINs, password material, cookies, session and CSRF tokens, raw form bodies, and public attendance names are never logged.
- Errors are logged once at the HTTP, startup, backup, migration, or CLI boundary that owns the outcome. Lower layers wrap and return errors without logging them again.
- Log destinations, rotation, retention, and access controls belong to the container platform and deployment runbook.
- Accepted tradeoff: plain text is less reliably machine-queryable than structured JSON. Exact field spelling and escaping therefore become tested compatibility requirements for security events.

## Code Quality Review Summary

- SQL remains explicit and compile-checked, while generated database types stay confined to persistence.
- The project uses a pure-Go, pinned SQLite stack and embedded forward-only migrations.
- Full-page and HTMX paths share application logic but render explicit response shapes.
- Expected failures use a small sentinel vocabulary with safe validation results and centralized HTTP mapping.
- Logging deliberately stays minimal; stable field conventions and redaction tests compensate partially for the lack of structured JSON.
- No ORM, DI framework, JSON browser API, generic repository layer, or third-party logging framework is introduced.

## Test Review Decisions

### D22: Go HTTP and template tests without browser automation

- Release 1 uses Go tests only; it does not add Playwright, Node.js, browser binaries, or a cross-browser CI job.
- `httptest` exercises complete routes through middleware, authentication, CSRF, application use cases, templates, and a migrated temporary SQLite database.
- Full-page and HTMX requests are separate table cases asserting status, headers, stable semantic HTML markers, validation text, preserved input, and authoritative response content.
- Template tests parse and render every page and fragment with representative states. Assertions prefer roles, labels, headings, form actions, and project-owned stable element IDs over complete HTML snapshots.
- A documented manual release checklist covers JavaScript-enabled HTMX swaps, mobile viewport navigation, focus movement, keyboard use, reduced motion, and the no-JavaScript baseline in supported browsers.
- CI cannot claim browser, responsive-layout, DOM-swap, or focus-management coverage. Those remain accepted manual-verification risks until a real browser suite is added.

### D23: Example tests plus native Go fuzzing for schedule rules

- Table-driven unit tests are the executable specification for civil dates, 30-minute intervals, recurrence expansion, dated exceptions, precedence, series splitting, and schedule-change warnings.
- Native Go fuzz targets exercise pure domain parsers and resolvers; Release 1 does not add a property-testing dependency.
- Required properties include deterministic expansion, bounded output, valid ordered intervals, no output outside the requested date range, stable precedence independent of input row order, and round-trip validity for accepted civil dates.
- Fuzz inputs have explicit size and range limits so malformed series cannot create unbounded allocations or test hangs.
- Any discovered failure is reduced and committed to the seed corpus with a named table regression explaining the business rule it protects.
- Pull-request CI runs the committed corpus and ordinary fuzz target execution for a fixed short duration; longer fuzz campaigns may run manually before schedule-engine releases.
- Fuzzing complements rather than replaces named examples for Sydney daylight-saving boundaries, midnight edges, split-series transactions, and all approved status combinations.

### D24: Mandatory race and SQLite contention coverage

- Pull-request CI runs `go test -race ./...` on Linux in addition to the ordinary test suite.
- Integration tests use separate connections to a real temporary file-backed SQLite database configured with the production pragmas; an in-memory database is not a substitute for WAL and lock behaviour.
- Concurrent cases cover attendance upserts and removal, finite player-code allocation, session revocation during mutation, account creation limits, throttle-map access, and Riley changing a schedule while a member submits a plan.
- Assertions permit every documented last-write-wins ordering but require database constraints, ownership, availability validation, session revocation, and bounded completion to hold in all outcomes.
- Busy handling has a fixed timeout/retry budget and returns the approved temporary-unavailable category when exhausted; tests do not spin or sleep without a deadline.
- Contention tests synchronize goroutines with barriers or channels rather than timing guesses, and repeat critical cases enough to reveal races without making CI probabilistic.
- A race finding, lock timeout, leaked goroutine, or invariant violation fails CI; tests do not skip these failures on the production architecture.

### D25: Repository-wide 80% line-coverage gate

- Pull-request CI fails when aggregate statement coverage for project-owned Go packages falls below 80%.
- Generated `sqlc` files, generated mocks if any are later approved, and the trivial `cmd/web` composition entry point are excluded through a documented deterministic package/file list; application, domain, web, and SQLite adapter logic are never excluded.
- CI publishes the text coverage summary and retains the profile long enough for reviewers to inspect low-coverage files.
- The percentage is supplementary: every critical path and failure mode named in the coverage matrix still requires a direct test even when aggregate coverage exceeds 80%.
- Tests that execute lines without checking business outcomes do not satisfy review. Assertions must cover state, response, security boundary, or invariant.
- Raising the threshold is allowed after evidence; lowering it requires an explicit plan decision with the uncovered risk identified.

### D26: Automated recovery verification without operator drills

- CI creates a representative prior-version file-backed database, writes accounts, schedules, sessions, and attendance, then produces a consistent backup through the production backup adapter.
- The test migrates the original, injects a post-backup failure, restores the snapshot into a clean service instance, reapplies only the expected migrations, and verifies integrity plus domain invariants.
- Cases include WAL activity, backup destination failure, corrupt/truncated snapshots, schema newer than the binary, migration failure, and insufficient restore permissions.
- A backup is accepted only after SQLite integrity and foreign-key checks and application-level reads succeed; file creation alone is not success.
- Restored instances remain unready until the recovery generation is reconciled, lockdown is durable, all identities and sessions are scrubbed, and Riley explicitly clears lockdown after verification.
- Recovery tests use synthetic credentials and attendance data and ensure backup or diagnostic output does not leak secrets.
- Release 1 does not require Riley to perform a manual off-host restore drill before launch or periodically afterward.
- Accepted risk: CI proves the program path but not operator access, encryption-key availability, off-host retrieval, platform permissions, elapsed recovery time, or Riley's ability to execute the runbook during a real incident.

## Test Coverage Diagram

```text
                                      automated in Go
┌────────────────────────────────────────────────────────────────────────────┐
│ HTTP contract tests (`httptest`)                                           │
│                                                                            │
│ request ─> request ID ─> auth/session ─> CSRF ─> throttle ─> web handler   │
│                                                           │                │
│                         ┌─────────────────────────────────┴──────────────┐ │
│                         │ full page / redirect   HTMX named fragment    │ │
│                         └──────────────────┬─────────────────────────────┘ │
│                                            ▼                               │
│                                    application use case                    │
│                                    │ validate/authorise                    │
│                                    │ transaction                           │
│                    ┌───────────────┴────────────────┐                      │
│                    ▼                                ▼                      │
│            pure domain rules              real file SQLite + WAL           │
│            table tests + fuzz             migration/query/backup tests     │
│                    │                                │                      │
│                    └──────────── authoritative result ────────> template    │
│                                                                            │
│ concurrent mutation paths ─> barriers ─> `go test -race` ─> invariants    │
└────────────────────────────────────────────────────────────────────────────┘

                                      manual only
┌────────────────────────────────────────────────────────────────────────────┐
│ real browser: HTMX DOM swaps, focus, mobile layout, keyboard, reduced      │
│ motion, visual accessibility, and no-JavaScript usability                  │
└────────────────────────────────────────────────────────────────────────────┘
```

## Critical-Path Test Matrix

| Area | Required success evidence | Required failure evidence |
|---|---|---|
| Accounts | unique code allocation, sign-in, PIN change | collision exhaustion, wrong PIN, throttling, suspension |
| Sessions/security | per-device session, CSRF acceptance, admin separation | expiry, revocation race, invalid CSRF, cookie/header leakage |
| Attendance | add, replace, remove, current-user visibility | closed interval, stale schedule, invalid duration, SQLite busy |
| Schedule | weekly expansion, exceptions, split future series | overlap, precedence conflict, DST/date boundary, invalid exception |
| Rendering | full pages and matching HTMX fragments | preserved validation input, conflict, empty/loading/error states |
| Administration | schedule edit, PIN reset, CLI suspension/reinstatement | wrong target, backup failure, unauthorized web mutation |
| Data lifecycle | migration, consistent backup, clean restore | corrupt backup, failed migration, newer schema, permission failure |
| Operations | startup/readiness, redacted correlation logs | missing secret/volume/timezone, integrity failure, log leakage |

## Test Review Summary

- Tests remain primarily Go-native: table tests, fuzzing, real-SQLite integration, `httptest`, and race detection.
- CI enforces 80% aggregate coverage while the matrix independently requires direct evidence for risky workflows.
- Browser behaviour and operational off-host recovery are not automated or manually gated in Release 1; both are explicit residual risks.
- No database mocks, in-memory SQLite substitutes for persistence behaviour, snapshot-only HTML assertions, or timing-based concurrency tests are accepted.

## Performance Review Decisions

### D27: Fixed-query timetable assembly in memory

- A 14-day dashboard load executes a small fixed set of range queries for applicable weekly series, dated exceptions and one-offs, attendance plans, and account display data.
- Query count is independent of days, courts, 30-minute buckets, or number of rendered members; templates and per-slot helpers never access SQLite.
- `internal/domain` expands recurrence, resolves precedence, derives plan warnings, and groups the complete result into immutable render models in one bounded pass.
- SQL selects only required columns and filters by effective/effected date ranges. Account joins return only public display fields required for the dashboard.
- The implementation records query-count assertions in integration tests and adds `EXPLAIN QUERY PLAN` checks for the principal range queries once schema and indexes exist.
- Persisted 30-minute slot materialisation, per-cell queries, and background read-model rebuild jobs are outside Release 1.
- The 14-day horizon, two courts, and 231-member scale bound memory use; the renderer still rejects unexpectedly oversized result sets with an operational error instead of allocating without limit.

### D28: No application-level timetable cache

- Every dashboard and interval-detail request reads the current committed SQLite state through the bounded query path.
- Release 1 does not add an in-memory result cache, pre-rendered HTML cache, distributed cache, or background refresh worker.
- Successful attendance and schedule mutations are visible to the next request without explicit invalidation or time-to-live delay.
- HTTP responses containing attendance names, current-user state, or CSRF material use restrictive cache headers and are not stored by shared intermediaries.
- SQLite's own page cache and the operating system filesystem cache are permitted implementation details; the application does not treat them as correctness dependencies.
- If measurement later shows a read bottleneck, the team must first inspect query plans, indexes, selected columns, and rendering allocations before proposing a cache.

### D29: Informational performance benchmarks without hard limits

- CI runs repeatable benchmarks against a seeded file-backed SQLite database containing 231 accounts and a deliberately crowded 14-day timetable.
- Benchmarks report dashboard assembly/render duration and allocations, ordinary attendance mutation duration, authentication duration, response bytes, query count, and a 20-request concurrent smoke result.
- Results are retained or printed in a form reviewers can compare with the main branch, but no absolute latency, allocation, response-size, or concurrency threshold automatically fails Release 1.
- Benchmark fixtures, warm-up, clock, SQLite pragmas, driver version, and runner architecture are recorded so changes remain interpretable.
- Argon2 benchmarks are reported separately from ordinary request work because security calibration intentionally consumes CPU and should not be “optimised” by silently weakening parameters.
- A functional failure, data invariant violation, timeout, unbounded allocation, or query-count regression still fails tests; only numeric performance changes remain advisory.
- Accepted risk: a materially slower but functionally correct release can pass CI if reviewers overlook or accept the reported regression.

### D30: One writer connection and a four-connection read pool

- The SQLite adapter opens two `database/sql` handles to the same local WAL database: a writer limited to one open/idle connection and a read handle limited to four.
- Every mutation and read-modify-write use case runs entirely through the single writer handle so application transactions never span pools.
- Pure dashboard, detail, and account lookups use the read pool. Read queries are bounded and carry request deadlines.
- Required connection-local pragmas are applied through the driver DSN or connection hook and verified for both handles; startup fails if foreign keys, busy timeout, journal mode, or other approved settings differ.
- The writer serializes application writes before SQLite locking. A bounded busy timeout still handles external maintenance/backup contention and returns temporary-unavailable on exhaustion.
- Graceful shutdown stops new requests, waits within a deadline for active transactions, closes the read handle, then the writer, without deleting WAL sidecar files manually.
- Integration and contention tests use this exact pool construction rather than a test-only connection arrangement.

## Performance Review Summary

- Dashboard cost is bounded by fixed range queries and one in-memory expansion pass, not by per-cell database access.
- No application cache, materialized slot table, pre-render worker, or default unbounded connection pool is introduced.
- SQLite writes are serialized through one connection while four read connections permit concurrent WAL reads.
- CI reports representative performance trends but does not enforce numeric latency, allocation, or response-size budgets.
- Query-count regressions, timeouts, unbounded allocations, and correctness failures remain hard failures even though numeric speed changes are advisory.

## Final System Diagram

```text
anonymous browser                         signed-in member / Riley
        │                                           │
        └──────────────────────┬────────────────────┘
                               ▼
                    one Go `net/http` service
             ┌─────────────────────────────────┐
             │ middleware                      │
             │ request ID · headers · sessions │
             │ CSRF · throttle · recovery      │
             └───────────────┬─────────────────┘
                             ▼
             full-page HTML / HTMX web adapter
                             │
                             ▼
                 application use-case layer
          authorise · validate · own transactions
                    │                  │
                    ▼                  ▼
        pure schedule/domain rules   SQLite adapter
                                     │
                         ┌───────────┴───────────┐
                         ▼                       ▼
                  one writer connection   four read connections
                         └───────────┬───────────┘
                                     ▼
                       local durable SQLite + WAL
                                     │
                        consistent encrypted backup
                                     ▼
                              off-host storage

deployment-local operator ──> same binary moderation CLI ──> writer connection
```

The trust boundary is the application use-case layer: browser fields and headers are requests, never authority. SQLite constraints and transactions remain authoritative for stored state; pure domain rules remain authoritative for recurrence, precedence, and interval validity.

## Failure Modes and Recovery

| Failure | Detection | User-visible behaviour | Recovery / containment |
|---|---|---|---|
| SQLite volume missing or read-only | startup open/write probe | service never becomes ready | restore volume permissions or attach the correct durable volume |
| Corrupt database or foreign-key failure | startup/backup integrity checks | generic unavailable response; readiness fails | preserve evidence, restore the last verified backup, rerun forward migrations |
| WAL/write contention exceeds budget | translated SQLite busy error | preserved form plus temporary-unavailable retry state | short bounded retry; inspect long transactions or external file access |
| Migration fails | `goose` startup result and schema version check | bounded maintenance/unavailable response | stop new image, restore and verify the preflight backup under recovery lockdown, then start prior image |
| Recovery generation mismatches or lockdown is set | startup generation comparison | ordinary service remains unready | run the local identity-scrubbing recovery procedure, verify it, then clear lockdown with fresh administrator authentication |
| Schedule changes during attendance submission | transaction revalidation | conflict explanation with submitted date/time preserved | member reviews current timetable and resubmits |
| Two tabs change the same attendance plan | documented last-write-wins outcome | latest committed valid state is rendered | no automatic merge; member changes again if needed |
| Session is revoked during mutation | transactional ownership/session check | sign-in required; mutation does not commit | sign in again or complete Riley reset flow |
| Throttle state is lost on restart | process lifecycle event | accumulated limits disappear | accepted Release 1 risk; investigate persistent/shared limits after abuse evidence |
| Public fake/impersonating account | member report or operator observation | polluted turnout until action | exact-identifier deployment CLI suspension/deletion; revoke sessions and future plans |
| Argon2 or request flood exhausts CPU | latency/error rate and throttle events | generic throttled/unavailable state | cheap bounded throttle before hashing; operator containment and parameter review |
| Timezone data unavailable | explicit Sydney location startup check | service unready | ship embedded timezone data and redeploy |
| HTMX response fails or JavaScript is absent | ordinary browser navigation remains available | full-page form/redirect path | retry with full page; manual browser checklist before release |
| Operator cannot retrieve a real backup or deployment generation | only discovered during an incident under D26 | service remains unready; extended outage/data loss possible | recover the snapshot and deployment secrets; add a human restore drill when operational assurance is prioritised |

## Implementation Worktree Lanes

```text
Lane A — foundation/data     E1 ─> E2 ─> E3 ───────────────┐
Lane B — schedule/read       E3 ─> E4 ─> E7 ────────────┐  │
Lane C — identity/write      E3 ─> E5 ─> E8 ─> E9 ──────┼──┤
Lane D — interface/admin     E2 ─> E6 ────────┬─────────┘  │
Lane E — operations/tests    E3 ─> E10 ─> E11 ┴─> E12 <────┘
```

Separate worktrees may be used after E3 establishes package and schema contracts. No lane may edit generated SQL, shared migrations, route names, or public view-model contracts without first coordinating the owning lane. Integration remains sequential at the dependency joins shown above.

## Implementation Tasks

## Implementation milestones

Implementation progress on 14 July 2026:

| Milestone | Status | Evidence / remaining boundary |
|---|---|---|
| M0 | Complete | Official Go archive checksum verified; Go 1.26.4, `sqlc` 1.31.1, `goose` 3.27.1, and `jq` 1.8.1 verified; governance reconciled |
| M1 | Complete | Runnable modular service, typed gates, strict SQLite migrations, generation/lockdown readiness, CI, race/vet, and 82.6% eligible coverage |
| M2 | Code complete; validation pending | Synthetic fixtures, anonymous batch read model and interval disclosure, desktop/mobile timetable, turnout bands, social markers, and court rails implemented; manual accessibility/browser validation remains |
| M3 | Not started | Account and attendance identity routes remain deliberately unregistered |
| M4 | Not started | Recovery detection exists; administration, lifecycle, scrub command, backups, and deployment are not implemented |
| M5 | Committee-gated | No real data or production capability has been enabled |

### M0 — Planning and toolchain baseline

- Reconcile repository governance with synthetic implementation authorisation.
- Install checksum-verified Go 1.26.4 under `~/.local/go`; use a user-local `GOBIN` for pinned development tools.
- Pin `sqlc` 1.31.1 and `goose` 3.27.1, and keep unrelated prompt artifacts out of implementation commits.
- Exit when versions are reproducible, the documentation authorises M1, and only intended planning files are staged for any planning commit.

### M1 — Green foundation

- Establish the composition root and `web -> app -> domain + sqlite` package graph. `sqlite` converts to domain types; `domain` imports neither HTTP nor persistence.
- Add typed environment/capability configuration, SQLite migrations and access, pure civil-time/recurrence foundations, readiness/liveness, and CI.
- Register no member-data routes and load no real data.
- Exit on migration/pragmas/capability/domain tests plus formatting, vet, race, and the applicable critical-path matrix.

### M2 — Synthetic schedule and timetable

- Add an explicit development/test fixture loader with sparse, crowded, closure, exception, and one-court examples.
- Implement the anonymous timetable, aggregate interval detail, court status rails, club information, desktop week, and mobile selected-day presentation.
- Anonymous queries never select identities; named member routes do not exist in this milestone.
- Exit on fixed-query/read-model tests, route privacy tests, template tests, and the manual responsive/accessibility checks applicable to this slice.

### M3 — Gated accounts and attendance

- Implement accounts, PINs, sessions, CSRF, throttling, add/change/remove attendance, and current-user plans.
- Keep anonymous and named read models/routes separate. Enable identity capabilities only in disposable development/test; production remains disabled until approvals are valid.
- Exit when disabled routes are absent, application/CLI gates reject direct calls, identity queries cannot run on anonymous paths, and the security/attendance test matrix passes.

### M4 — Administration, lifecycle, and recovery

- Implement Riley's schedule administration, recurrence scopes, gated PIN reset/self-delete, moderation CLI, snapshots, recovery generation/lockdown, identity scrub, and rollback paths.
- A restored snapshot never becomes normally ready until scrub and explicit recovery clearance have succeeded.
- Exit on administrator, recurrence, lifecycle, backup, restore-lockdown, and container smoke tests.

### M5 — Durable deployment and real-data pilot

- Select a single-instance host with durable local storage, encrypted off-host backup ownership, alerting, and an operator recovery path.
- Load committee-approved club configuration and real content; enable only capabilities covered by the typed approval manifest.
- Complete the manual accessibility/privacy/navigation checklist, run a four-week pilot, and measure the approved target of 20 distinct active accounts in week four.
- M5 requires written decisions for public named attendance, public account creation/visitor attendance, PIN-reset evidence, and retention/account deletion.

Every milestone must pass its applicable named critical-path tests even if aggregate coverage exceeds the existing 80% gate. M0 and M1 are sequential. After M1, schedule/read, identity primitives, and operations work may use coordinated worktrees, but shared migrations, route contracts, and generated SQL remain serial integration points.

### E1 — Reconcile project contracts and launch configuration

- **Files:** `README.md`, `AGENTS.md`, `PRODUCT_DESIGN.md`, `DESIGN.md`, new configuration/runbook documentation.
- Replace remaining booking and shared-link assumptions with the approved attendance/public-URL model, without erasing deferred product history. Maintain the reviewed Release 1 interface matrix in `DESIGN.md`.
- Define typed configuration for database path, trusted proxy, public origin, admin hash, backup destination, Sydney timezone, and club content; validate all required production values at startup.
- **Verify:** repository terminology search; configuration table review; committee blockers map exactly to configuration or explicit implementation gates.
- **Depends on:** none for synthetic M0-M4; committee answers gate only the corresponding production capabilities and M5.

### E2 — Establish the modular-monolith skeleton

- **Files:** `cmd/web`, `internal/web`, `internal/app`, `internal/domain`, `internal/sqlite`, embedded templates/static/timezone assets.
- Build the explicit composition root, process lifecycle, graceful shutdown, health/readiness endpoints, standard middleware order, sentinel mapper, and redacted logging convention.
- **Verify:** package dependency test/static check, startup smoke test, missing-secret/volume/timezone failure tests, graceful shutdown test.
- **Depends on:** E1.

### E3 — Define and migrate the SQLite schema

- Add strict forward-only `goose` migrations for accounts, member/admin sessions, attendance plans, weekly series, exceptions, one-offs, and migration/security metadata.
- Add database constraints and indexes for immutable unique identifiers, one plan per account/date, foreign keys, state enums, dates, and 30-minute intervals.
- Configure pinned `modernc.org/sqlite`, one writer/four readers, required pragmas, `sqlc` generation, and stale-generation CI detection.
- **Verify:** empty/prior fixture migrations, integrity/foreign-key checks, query generation clean diff, principal `EXPLAIN QUERY PLAN` assertions.
- **Depends on:** E2.

### E4 — Implement pure civil-time and schedule rules

- Implement Sydney civil dates, intervals, weekly expansion, dated exceptions, one-offs, precedence, series splitting, and derived schedule-change warnings.
- Keep recurrence independent of HTTP and SQLite row types; include the approved recurrence diagram with the code.
- **Verify:** named table tests, DST and boundary cases, deterministic fuzz properties, split-series transaction integration tests.
- **Depends on:** E3 domain/storage contracts.

### E5 — Implement member and administrator security

- Implement public account creation, random finite player-code allocation, Argon2id member PIN/admin password verification, opaque hashed sessions, PIN change/reset, CSRF, and bounded in-memory throttles.
- Apply explicit privacy acknowledgement, generic authentication failures, session revocation rules, trusted-proxy handling, and cookie/header policy.
- **Verify:** security matrix including timing-path dummy hash, collision exhaustion, restart-reset throttle behaviour, fixation/revocation, CSRF, cookie flags, and log redaction.
- **Depends on:** E3.

### E6 — Build the accessible interface shell and state components

- Implement the approved desktop timetable shell, mobile selected-day navigation, semantic status keys, public club information, account forms, confirmation/removal, and every Release 1 state in the authoritative `DESIGN.md` interface matrix.
- Use ordinary HTML as the baseline and explicit paired HTMX fragments; do not create selectable/reservable court cells.
- **Verify:** template parse/render cases, semantic marker assertions, HTML validator where available, keyboard/screen-reader/zoom/reduced-motion/mobile manual checklist.
- **Depends on:** E2 and stable view-model contracts from E4/E5.

### E7 — Build the public timetable read model

- Implement fixed range queries and bounded in-memory expansion for the next 14 Sydney dates, venue turnout, court status rails, attendance details, current-user plans, and derived warnings.
- Apply public-name privacy headers and restrictive cache policy; expose no username, player code, membership flag, PIN, or session information.
- **Verify:** fixed query-count tests across sparse/crowded fixtures, response privacy assertions, empty/occupation/closure/reduced-capacity cases, informational benchmarks.
- **Depends on:** E3, E4, E6 contracts.

### E8 — Implement attendance mutations

- Add/change/remove one owned venue-wide daily plan using short writer transactions, continuous availability checks, last-write-wins upsert, authoritative reread, full-page PRG, and equivalent HTMX fragments.
- **Verify:** ownership, CSRF, invalid interval/date, schedule race, double-submit, concurrent tabs, SQLite busy, session revocation, and no-JavaScript route tests.
- **Depends on:** E4, E5, E6, E7.

### E9 — Implement Riley operations and account lifecycle

- Build schedule administration, occurrence/scope edits, PIN reset with forced change, member self-delete, and exact-identifier local CLI suspension/reinstatement/permanent deletion.
- Gate PIN reset and permanent deletion on their committee approvals. Permanent deletion also blocks if its consistent pre-action backup fails.
- **Verify:** administrator authorization/CSRF, recurrence scope cases, reset session revocation, wrong CLI target, idempotent suspension, reinstatement, deletion rollback, and redacted audit lines.
- **Depends on:** E3–E8.

### E10 — Implement backup, migration, and deployment operations

- Produce the versioned single-instance container, durable-volume contract, startup migration lock, consistent encrypted off-host backup adapter, recovery-generation/lockdown controls, identity-scrubbing restore runbook, and immutable-image CI pipeline.
- **Verify:** container smoke test without CGO/system SQLite, synthetic backup/migrate/failure/restore-lockdown test, corrupt/newer-schema/permission/generation-mismatch cases, deployment-manifest durability inspection, and D35 container/host persistence rehearsal.
- **Depends on:** E3 and deployment owner/vendor input.

### E11 — Complete CI quality gates

- Run formatting, vet, `sqlc` clean generation, migrations, `go test`, fixed fuzz corpus, `go test -race`, 80% eligible-package coverage, container smoke, and advisory performance benchmarks.
- Publish coverage and benchmark output; keep browser automation and fixed performance budgets out of Release 1 as approved.
- **Verify:** deliberately break each gate once in a branch or fixture and confirm CI fails for the intended reason.
- **Depends on:** E3–E10 incrementally.

### E12 — Conduct release validation

- Execute the approved mobile-rail member test in `TODOS.md`, representative crowded-session review, manual browser/accessibility checklist, privacy/content review, and production configuration rehearsal.
- Confirm every committee blocker and launch value has an owner and written answer; do not describe planned attendance as verified usage or court reservation.
- **Verify:** signed release checklist with links to CI results, manual evidence, configured schedules/content, known accepted risks, and rollback contact.
- **Depends on:** E1–E11.

## Committee and Operational Inputs Still Required

### Production-capability and M5 launch gates

1. Written approval to expose public display names and upcoming attendance intervals on a discoverable public-internet URL.
2. Written approval for unrestricted public account creation and self-reported visitor/non-member attendance, including the accepted impersonation and finite-code risks.
3. The identity evidence Riley must require before issuing a temporary PIN.
4. The retention rule for expired attendance plans and account deletion: delete completely or retain approved anonymous aggregates.

### Required launch configuration

1. Exact light hours for each court and weekday.
2. Exact Tuesday, Friday, and Sunday social-session times.
3. Current recurring competition/coaching occupations and court assignments.
4. Closures, term breaks, public-holiday rules, and known exceptions.
5. Approved location, directions, contact, WhatsApp, privacy, and public-account wording.
6. Production host, durable-volume owner, backup destination/key owner, retention period, alert recipient, and Riley's deployment-shell support path.

### Explicitly accepted follow-up risks

- No automated browser tests; interaction and responsive behaviour depend on the manual release checklist.
- No human off-host restore drill; real operator recovery readiness is unproven.
- Plain-text logs are less queryable than JSON.
- In-memory throttles reset on restart and do not support multiple processes.
- Four-digit PINs without a pepper permit offline enumeration after database theft.
- Public creation permits distributed abuse and impersonation; moderation requires deployment access.
- Last-write-wins permits stale tabs to replace or remove a newer plan.
- Performance regressions are reported but do not fail numeric budgets.

## Decision Index

| Section | Decisions |
|---|---|
| Scope | D1 reduced Release 1 |
| Architecture | D2 SQLite; D3 layered monolith; D4 civil time; D5 recurrence; D6 Argon2id/no pepper; D7 opaque sessions; D8 last-write-wins; D9 deployment admin secret; D10 memory throttles; D11 single durable container; D12 derived warnings; D13 public URL; D14 public creation; D15 local moderation CLI; D31 restore generation/privacy lockdown; D32 admin rotation revocation; D33 destructive series split |
| Code quality | D16 `sqlc`; D17 `modernc`; D18 embedded `goose`; D19 progressive HTML; D20 sentinel errors; D21 plain logs; D34 bounded-downtime migration; D35 durability evidence |
| Tests | D22 Go-only HTTP/template tests; D23 fuzzing; D24 race/contention; D25 80% coverage; D26 CI-only recovery |
| Performance | D27 batch read model; D28 no app cache; D29 advisory benchmarks; D30 one writer/four readers |

## Sources Consulted

- [SQLite appropriate-use guidance](https://www.sqlite.org/whentouse.html), [WAL](https://www.sqlite.org/wal.html), [foreign keys](https://www.sqlite.org/foreignkeys.html), and [backup API](https://www.sqlite.org/backup.html)
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html) and [Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [`sqlc` SQLite guide](https://docs.sqlc.dev/en/stable/tutorials/getting-started-sqlite.html), [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite), and [`goose` embedded migrations](https://github.com/pressly/goose#embedded-sql-migrations)
- [Go timezone embedding](https://pkg.go.dev/time/tzdata)
- [Playwright CI](https://playwright.dev/docs/ci) and [locator guidance](https://playwright.dev/docs/locators), considered but not selected for Release 1

## Outside Review Resolution

An independent reviewer assessed the complete draft for contradictions, security/data-loss risks, unverifiable claims, and missing dependencies.

| Finding | Resolution |
|---|---|
| Full username identity contradiction | Dismissed after source check: the immutable full username and mutable public display name are already separate, and sign-in consistently uses full username + PIN |
| Backup could resurrect deleted/security-sensitive state | Replaced the second authoritative journal with D31 recovery-generation lockdown and mandatory identity scrubbing |
| Admin password rotation left sessions valid | Approved D32 fingerprint-triggered revocation |
| Future exceptions were undefined after series split | Approved D33 explicit preview and confirmed deletion |
| Migration rollback conflicted with one-instance storage | Approved D34 bounded-downtime backup/migrate/restore sequence |
| Deferred design states could leak into Release 1 | Added authoritative Release 1 interface matrix and deferred annotations in `DESIGN.md` |
| Ephemeral-volume rejection was not verifiable | Approved D35 manifest evidence plus host persistence rehearsal |
| D19 overstated browser automation | Corrected D19 to HTTP contracts plus manual real-browser validation |

## Review Dashboard

| Review area | Result |
|---|---|
| Scope challenge | Complete — Release 1 reduced |
| Architecture | Complete — 17 decisions |
| Code quality | Complete — 8 decisions |
| Test strategy | Complete — 5 decisions, diagram and critical-path matrix |
| Performance | Complete — 4 decisions |
| Outside review | Complete — 1 dismissed with evidence, 7 approved corrections |
| Implementation tasks | Complete — E1 through E12 with dependencies and verification |
| Failure modes | Complete — detection, member behaviour, and recovery documented |
| Production code | Authorised for synthetic milestones M0-M4 |
| Existing TODOs | 1 approved mobile-navigation validation task; no new TODO proposed |

## Review Artifacts

- Test plan: `/home/riso7312/.gstack/projects/rileyso-uni-squash-booking/test-plan-20260713T161227Z.md`
- Review log: `/home/riso7312/.gstack/projects/rileyso-uni-squash-booking/review-log-20260713T161227Z.md`
- Structured JSONL log: skipped because `jq` is unavailable; the Markdown review log records the same review summary.

## GSTACK REVIEW REPORT

**Review type:** Engineering plan review  
**Target:** `PRODUCT_DESIGN.md` + `DESIGN.md`  
**Status:** APPROVED — synthetic milestones M0-M4 authorised; M5 remains committee-gated  
**Decisions recorded:** 35  
**Implementation tasks:** 12  
**Outside review:** complete  
**Production code written:** none

**UNRESOLVED DECISIONS:**
- No engineering-review decisions remain unresolved.
- Committee approval for public-internet display names and upcoming intervals is outstanding.
- Committee approval for unrestricted public account creation and visitor/non-member attendance is outstanding.
- PIN-reset identity evidence is outstanding.
- Expired-plan and account-deletion retention treatment is outstanding.

Do not enable real-data production capabilities or begin M5 until the corresponding decisions are answered in writing or explicitly accepted as launch risks.
