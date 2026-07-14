# Sydney University Squash Club Attendance Tracker

## Sources of truth

- Read `PRODUCT_DESIGN.md` before product, architecture, design, or implementation work.
- Read `DESIGN.md` before changing any user-facing interface.
- When documents disagree, stop and reconcile them explicitly; do not silently choose booking-oriented language or behaviour.

## Product model

- This is a venue-wide attendance tracker and turnout forecast, not a court-booking application.
- Members announce when they plan to attend. They do not reserve Court 1 or Court 2.
- Court allocation remains first-come-first-served and is governed in person.
- Turnout is the primary timetable signal. Court 1 and Court 2 are secondary operational-status rails and must not look selectable or reservable.
- Always describe registrations and reports as planned attendance, not verified attendance or court usage.
- Preserve the approved identity, privacy, retention, moderation, recurrence, schedule, and reporting rules in `PRODUCT_DESIGN.md`.

## Product language

Use:

- `Add my attendance`
- `Your plans`
- `Change attendance`
- `Remove my attendance`
- `Planned attendance`
- `Open for play`
- `Lights off`
- `Competition`
- `Coaching`
- `Other closure`

Do not use member-facing phrases such as `book court`, `available slot`, `booked slot`, `your bookings`, or `cancel booking`.

## Approved design direction

- The product should feel like a calm, trustworthy, modern university sports service rather than a startup dashboard or consumer fitness application.
- Draw structural inspiration from compact sports timetables without copying YepBooking, Sydney Uni Sport, or University of Sydney branding, assets, colours, or exact composition.
- Desktop uses a persistent restrained sidebar and a seven-day timetable with a shared vertical time axis.
- Mobile uses an expandable navigation rail and an intentional selected-day timetable. Never squeeze the seven-column desktop grid onto a phone.
- The timetable is a continuous grid, not a card-per-day or card-per-interval mosaic.
- Use IBM Plex Sans Condensed, deep navy/slate neutral structure, and restrained status accents as specified in `DESIGN.md`.
- Never rely on colour alone. Pair state colour with text and an icon, plus shape or pattern where required.
- Preserve at least 44px touch targets, visible keyboard focus, semantic landmarks, logical focus order, and WCAG AA text contrast.
- Avoid decorative gradients, excessive shadows, ornamental animation, metric-card dashboards, and official trademarks unless authorised assets are supplied.

## Preferred implementation direction

- Go with standard `net/http` where practical
- Server-rendered HTML templates
- HTMX for focused partial updates
- SQLite for Release 1 through Go's `database/sql`; see `ENGINEERING_PLAN.md`
- Minimal JavaScript for interactions that HTML and HTMX cannot provide accessibly
- A modular monolith and single deployable Go service
- Docker Compose for local development
- The server and SQLite transaction remain authoritative for attendance and schedule conflicts

Do not introduce React, Vue, Next.js, microservices, Kubernetes, or speculative abstraction without an approved architectural reason.

## Current phase and working rules

- Product discovery, visual design, and engineering review are approved.
- Synthetic-data implementation milestones M0-M4 in `ENGINEERING_PLAN.md` are authorised now.
- Use only disposable synthetic data in development and test. Development must bind to loopback and show a prominent synthetic-data marker.
- Committee decisions gate production capabilities and the real-data pilot (M5); they do not block foundation, timetable, account, attendance, administrator, or recovery implementation with synthetic data.
- Capability gates must fail closed at startup and in the application layer. Hiding a control is not an adequate gate.
- Discuss genuinely unresolved architecture decisions one at a time, give two or three concrete options, recommend one, and record the accepted decision before implementation.
- Challenge requests that conflict with the non-booking model instead of silently implementing them.
- Preserve unrelated user changes and do not commit unless explicitly requested.

## Verification expectations once implementation begins

- Add focused tests for domain rules, recurrence boundaries, authentication throttles, and schedule conflicts.
- Verify representative HTMX responses and full-page fallbacks.
- Test the timetable with keyboard-only navigation, a screen reader, 200% zoom, reduced motion, and mobile touch targets.
- Run `gofmt`, `go test ./...`, and `go vet ./...` when those commands exist in the implemented repository.
