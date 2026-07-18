# Manual validation and hosting-readiness runbook

Use this runbook before deploying the attendance tracker to a third-party host.
All testing before committee approval must use disposable synthetic data.

## Release decision

A candidate may move to a synthetic third-party-host rehearsal only when sections
1–6 pass. It may move to a real-data pilot only when section 7 also passes and
all four production approvals are recorded. A failure involving privacy,
authentication, data loss, court-booking language, keyboard access, or unreadable
mobile layout blocks release.

Record the date, tester, browser/device, result, and a screenshot or short note
for every failed or qualified check. Do not record PINs, cookies, session tokens,
or private member data in evidence.

## 1. Prepare a clean synthetic candidate

From the repository root:

```sh
make check
docker compose build
docker compose up -d
docker compose ps
curl --fail http://127.0.0.1:18080/healthz
curl --fail http://127.0.0.1:18080/readyz
```

Open `http://127.0.0.1:18080`. Confirm the prominent `Synthetic development
data only` marker is visible. If Docker is unavailable, use `make dev` for
browser testing, but leave the container and hosting checks unpassed.

Synthetic credentials:

- Member: `john#1111`, PIN `1111`
- Administrator: `riley`, password `synthetic-admin-password`

Use a private/incognito window when a test needs a clean browser state.

## 2. Browser and responsive-layout matrix

Run the core journey in current Chromium and Firefox on desktop. Also run it in
Safari on iOS and Chrome on Android when those devices are available. Browser
device emulation is acceptable for the first layout pass, but the mobile rail
must also be tried on a real touch device before the pilot.

Test these viewport and zoom combinations:

| View | Required check |
|---|---|
| 1440 × 900 | Persistent desktop sidebar and continuous horizontal timetable |
| 1024 × 768 | No clipped actions or unintended page-level horizontal scrolling |
| 390 × 844 | Selected-day timetable, 44px targets, expandable navigation rail |
| 320 × 568 | Same checks at the narrowest approved width |
| Desktop at 200% zoom | Content reflows; controls and text remain usable |

At 320px and 390px:

- Confirm the seven-column desktop grid is not shown.
- Expand and collapse the navigation rail by touch.
- Confirm `Attendance` and `Club information` are discoverable.
- Confirm the expanded rail overlays content instead of squeezing the timetable.
- Confirm there is no page-level horizontal scrollbar.
- Confirm every interactive target is at least 44 × 44 CSS pixels.
- Repeat with three representative club members. Give no navigation hints;
  record whether each person can find both destinations and return to attendance.

## 3. Member journeys

Complete each journey with the mouse/touch, then repeat the essential path using
only `Tab`, `Shift+Tab`, `Enter`, `Space`, and arrow keys.

- Anonymous: scan turnout, date controls, social markers, status key, Court 1 and
  Court 2 operational rails, and participant detail.
- Create account: acknowledge privacy, create a synthetic member, save the full
  username, sign out, and sign back in.
- Sign-in failure: enter an incorrect PIN and confirm the error is generic and
  does not expose whether an account exists.
- Attendance: select one interval, review it, and confirm it. Verify the wording
  says venue-wide planned attendance and never implies a court reservation.
- Additive attendance: add a separated interval on the same date. Confirm both
  intervals appear and adjacent intervals are combined in the summary.
- Conflict: attempt an invalid or closed interval and confirm the input is
  preserved with an understandable error.
- Your plans: add more attendance and remove attendance. Confirm turnout and the
  personal summary update together.
- Profile: update display name/status, change PIN, and confirm the old PIN fails.
- Account deletion: using a disposable account, confirm the exact username and
  PIN are required and future named plans disappear.

For every state, confirm turnout remains the primary signal and court rails do
not look selectable or reservable. Search visible copy for `book`, `booking`,
`available slot`, and `cancel booking`; none may be member-facing.

## 4. Accessibility and resilience

### Keyboard and focus

- Start at the address bar and traverse the complete page without a pointer.
- Confirm the skip link appears on focus and moves focus to the timetable.
- Confirm focus indicators are clearly visible and focus order follows the page.
- On mobile width, open the rail, use its links, and press `Escape`; focus must
  return to the rail toggle.
- Open sign-in and attendance overlays. Confirm focus remains understandable
  after HTMX content replacement and after closing the overlay.

### Screen reader

Use NVDA with Firefox or VoiceOver with Safari. Confirm:

- Page title, landmarks, headings, navigation names, dates, buttons, errors, and
  status text are announced meaningfully.
- Collapsed mobile navigation links retain their accessible names.
- Court state is conveyed by text/icon context rather than colour alone.
- Planned attendance is not announced as verified attendance or court usage.

### Motion, colour, and zoom

- Enable the operating system's reduced-motion preference and confirm the rail
  has no transition and no essential information depends on animation.
- Inspect the page in grayscale or with colour disabled; states remain
  distinguishable through labels, icons, borders, or patterns.
- At 200% zoom, confirm no text or action is clipped and no two-dimensional page
  scrolling is needed to complete a journey.

### JavaScript and HTMX fallback

- Disable JavaScript in browser developer tools and reload.
- Repeat sign-in, add/review/confirm attendance, remove attendance, profile, and
  administrator form submissions.
- Confirm ordinary full pages and redirects complete the same authoritative
  operations without blank fragments or lost form input.
- Re-enable JavaScript, block one HTMX request, and confirm retrying by ordinary
  navigation remains possible.

## 5. Administrator and recovery journeys

- Open `/admin`, sign in with the synthetic administrator, and confirm member
  sessions cannot access administrator pages.
- Create/edit a synthetic weekly schedule item, an occurrence exception, and a
  future-scope change. Verify the public timetable reflects each change.
- Create a schedule conflict with an existing plan. Confirm the plan is retained
  and receives a schedule-changed warning rather than being silently removed.
- Reset a disposable member PIN, confirm existing sessions are revoked, and
  confirm the temporary PIN forces replacement before attendance changes.
- With the web process stopped, exercise suspend and reinstate against an exact
  synthetic username. Confirm future plans disappear on suspension.

Run the documented backup/recovery commands in `README.md` against copied,
disposable database paths. Confirm a restored database remains unready until
identity scrub and generation-matched clearance are complete. Never perform
this exercise against the only copy of a database.

## 6. Container and synthetic-host rehearsal

The selected host must support exactly one active container, a persistent local
volume suitable for SQLite/WAL files, HTTPS, external secrets, health checks,
logs, and deployment-shell access for moderation/recovery. Do not use an
ephemeral filesystem or horizontal autoscaling.

On a third-party host, initially deploy with synthetic configuration only:

- Mount the persistent volume at `/data` and set `DATABASE_PATH=/data/squash.sqlite`.
- Use a unique, externally stored `RECOVERY_GENERATION`.
- Keep one instance and one attached volume.
- Require HTTPS and verify secure response headers and cookie behaviour.
- Confirm `/healthz` and `/readyz` drive the platform health check.
- Create synthetic attendance, restart the process, replace the container image,
  and perform the provider-supported host restart. The same data and recovery
  generation must survive every operation.
- Confirm replacing the image never creates an empty database over the mounted
  volume.
- Confirm logs contain request/correlation information but no PINs, passwords,
  cookies, tokens, raw forms, or attendance names.
- Create and retrieve a verified off-host backup. Record who owns the backup key,
  retention, alert response, and recovery shell access.
- Roll back once to the previous immutable image after taking a verified
  preflight backup.

Do not expose the synthetic rehearsal as a real club service. Keep the synthetic
marker visible and distribute its URL only to testers.

## 7. Production go/no-go gate

Before setting `APP_ENV=production`, obtain and record all four decisions in a
deployment copy of the approval manifest:

- Public display names and upcoming planned-attendance intervals.
- Public account creation and visitor/non-member attendance.
- The identity evidence Riley must require before resetting a PIN.
- Retention and deletion of expired plans and deleted-account contributions.

Also record approved light hours, social sessions, occupations/closures, club
copy, host owner, durable-volume policy, backup owner/key/retention, alert
recipient, and Riley's deployment-shell support path.

Production secrets must be generated outside the repository and supplied by the
host's secret manager:

- `RECOVERY_GENERATION`
- `DEVICE_COOKIE_SECRET` (at least 32 characters)
- `ADMIN_USERNAME`
- `ADMIN_PASSWORD_HASH`
- `APPROVAL_MANIFEST` path

The production candidate passes only if startup fails when any required secret
or approval file is absent, disabled capabilities have no registered route, the
durability rehearsal passed, and sections 1–6 have no blocking failures.

## Evidence summary

Copy this table into the release issue or test record:

| Gate | Tester/date | Result | Evidence or defect |
|---|---|---|---|
| Automated `make check` | | ☐ Pass ☐ Fail | |
| Container build/health | | ☐ Pass ☐ Fail | |
| Desktop browsers | | ☐ Pass ☐ Fail | |
| 320px and 390px layouts | | ☐ Pass ☐ Fail | |
| Three-member rail test | | ☐ Pass ☐ Fail | |
| Keyboard and focus | | ☐ Pass ☐ Fail | |
| Screen reader | | ☐ Pass ☐ Fail | |
| 200% zoom/reduced motion | | ☐ Pass ☐ Fail | |
| No-JavaScript fallback | | ☐ Pass ☐ Fail | |
| Member journeys/privacy copy | | ☐ Pass ☐ Fail | |
| Administrator lifecycle | | ☐ Pass ☐ Fail | |
| Backup/recovery | | ☐ Pass ☐ Fail | |
| Hosted volume persistence | | ☐ Pass ☐ Fail | |
| Production approvals/config | | ☐ Pass ☐ Fail | |
