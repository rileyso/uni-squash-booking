# Sydney University Squash Club Attendance Tracker

Status: APPROVED — PENDING COMMITTEE INPUT  
Approved: 14 July 2026  
Date: 14 July 2026  
Product stage: Pre-product MVP  
Club size: Approximately 231 members

Decision update, 16 July 2026: members may hold multiple planned attendance
intervals on the same Sydney date when they intend to attend separated sessions.
Each interval remains venue-wide, independently validated, and continuously
playable. Decision update, 16 July 2026: confirming newly selected cells adds
them to that member's existing intervals for the date. Existing planned cells
cannot be selected again, and adjacent or overlapping intervals are normalised
into one interval in storage and summaries.

Engineering review note: the product vision below remains approved, but the first implementation release was reduced on 14 July 2026. Release 1 is defined in `ENGINEERING_PLAN.md`; explicitly deferred capabilities remain part of the post-validation product scope rather than launch requirements.

Access-model note: engineering decisions D13 and D14 propose a stable public dashboard URL and unrestricted public account creation rather than a token-bearing club-shared link. These changes are conditional on explicit committee approval. Release 1 exposes upcoming named plans only; the 30-day public named-history interface described below is deferred until after the turnout hypothesis is tested.

## Product Summary

Build a shared, turnout-first attendance calendar for the Sydney University Squash Club. Members use it to see when other players intend to attend, choose a session with a suitable turnout, and announce their own expected arrival and departure.

This is not a court-booking system. Court allocation remains first-come-first-served and is governed in person. The application communicates expected venue attendance and operational court status; it never reserves a court.

## Problem Statement

Attendance is currently coordinated through WhatsApp and word of mouth. Messages are time-dependent, get buried, and do not provide a reliable snapshot of who intends to play at a given time.

The resulting failure occurs at both ends:

- Too few people attend, so a member may not find an opponent.
- Too many people attend, reducing each member's playing time across the two courts.

The primary MVP outcome is helping members choose between sessions. Committee evidence for future court-funding requests is a possible secondary benefit, but registrations represent planned attendance rather than verified court usage.

## Primary User

The primary user is a regular social member choosing between official and informal playing days.

The concrete reference user is John:

- John normally plays on the official Tuesday, Friday, and Sunday social days.
- If turnout looks too low, he asks in WhatsApp and may decide not to play.
- If turnout looks too high, he may skip that session and attend another day.
- He needs a persistent, scannable turnout forecast rather than another chat message.

## Other Users

- New or visiting players need the club location, contact details, WhatsApp information, official social days, and an indication of who plans to attend.
- Riley is the sole MVP administrator. Riley maintains the operational schedule, manages recurring events, and resets forgotten PINs.
- Committee members are future consumers of aggregate participation information, but they do not receive separate administration access in the MVP.

## Status Quo

Synthetic implementation schedule supplied 15 July 2026:

- Monday: 16:00–22:00; Pennant training is not given a separate operational
  status in the current synthetic presentation, so the open hours remain green
- Tuesday: 12:00–14:00 and 16:00–22:00; official social 16:00–18:00
- Wednesday: 14:30–22:00
- Thursday: 16:00–22:00
- Friday: 15:00–20:00; official social 15:00–18:00
- Saturday: 09:00–15:00; squads represented as Coaching from 10:00–11:30
- Sunday: 12:00–17:00

The supplied Saturday 10:15–11:15 period is rounded outward to 10:00–11:30 to
preserve the approved 30-minute schedule boundaries. These are synthetic weekly
defaults for both courts, not member reservations.

The dashboard presentation uses two bounded calendar-week pages: the current
Monday–Sunday week, including elapsed days, and the following Monday–Sunday
week. This replaces a rolling visual window but does not expand attendance
registration beyond the approved future horizon. `Today` always returns to the
current Sydney date.

- Court use is first-come-first-served and governed in person.
- The application does not replace or integrate with that governing process.
- The two courts may be used seven days a week when the lights are on.
- Tuesday, Friday, and Sunday are the recurring official social days.
- Members currently coordinate through WhatsApp and word of mouth.
- When official sessions are crowded, members may move to quieter non-social days.

## Core Product Hypothesis

If planned attendance is visible in a persistent 14-day calendar, regular members will use it to choose sessions with a suitable turnout instead of relying on buried WhatsApp messages.

The application cannot guarantee less crowding. The MVP tests whether members consult the forecast and contribute their plans often enough for it to become useful.

## MVP Scope

### Member-facing dashboard

- One shared-link dashboard covering the next 14 days.
- All seven days are visible, not only official social days.
- Official Tuesday, Friday, and Sunday sessions are clearly marked.
- The primary visual signal is venue-wide expected turnout by 30-minute interval.
- Court 1 and Court 2 appear as secondary operational status lanes, not reservable resources.
- Upcoming attendee names and their expected attendance intervals are visible to anyone with the club link.
- Past named attendance remains visible through the club link for 30 days.
- A separate `Past plans` view shows the previous 30 days; the upcoming dashboard does not mix historical and future navigation.
- Location, club contact details, and WhatsApp joining information are easy to find.
- Operational notices may explain closures, schedule changes, competitions, or coaching. The MVP does not include a general-purpose news feed.

### Turnout labels

The fixed MVP turnout bands are:

| Planned attendees | Label |
|---:|---|
| 0 | Empty |
| 1-4 | Players attending |
| 5-9 | Good turnout |
| 10+ | Crowded |

These labels describe concurrent venue-wide planned attendance, not people assigned to a specific court.

The bands remain fixed in the MVP even when only one court is open. In that case the dashboard displays a prominent `Reduced capacity: one court open` warning beside the turnout label. Capacity-aware turnout thresholds are deferred.

### Court-status vocabulary

Member-facing language must avoid `available` and `booked`, because those terms imply reservations. Each court may instead show:

- Open for play
- Lights off
- Competition
- Coaching
- Other closure

Colour must not be the only status indicator. Every status also requires a text label and distinguishable shape or pattern.

### Lightweight permanent accounts

- Account creation is self-service for anyone with the club-shared link.
- An account consists of a display name, a system-generated unique four-digit player code, and a four-digit PIN.
- The full username combines a handle-style name and player code, for example `johnDoe#1234`.
- Player codes range from `0000` through `9999`; leading zeroes are significant. Generation retries until it finds an unused code.
- The player code distinguishes accounts and supports recovery when names are similar.
- Display names are not unique. Two people may use the same name because their full usernames differ.
- The shared attendance calendar shows only the display name. The full username appears only in sign-in, the member's profile, and Riley's administration tools.
- The full username and generated player code are immutable after account creation. Members may update their separate public display name and self-reported membership status after entering their PIN without changing how they sign in.
- Signed-in members may change their PIN after entering the current PIN. The current device remains signed in and every other member session is revoked.
- Member status is self-reported during account creation.
- Member/non-member status is visible only in Riley's committee totals, never beside public attendee names.
- Riley can reset forgotten PINs from the password-protected administration page.
- A PIN reset requires the committee-approved identity check. The system generates a temporary PIN for Riley to deliver privately through WhatsApp or another approved channel, revokes all existing member sessions, and requires the member to choose a new PIN at the next sign-in.
- A four-digit PIN is accepted only as low-assurance authentication for this low-risk club tool. Five failed attempts lock the account for 15 minutes; the error does not reveal whether the player code exists, and Riley can clear the lock.
- Account creation is limited to three user-submitted creation attempts per browser/device per hour and twenty per network address per hour. Internal retries caused by random player-code collisions do not count. A limited user sees a generic retry-later message.
- Login protection must combine per-account and network-source throttling so one source cannot lock many accounts rapidly.
- A successful member sign-in lasts seven days on that device. Members can sign out explicitly and may hold sessions on more than one personal device.
- Sign-in requires the full username and PIN; the player code alone is not a sign-in identifier. The interface may remember the full username for convenience but never displays or remembers the PIN in application content.
- Members may delete their own account after entering their PIN. Deletion removes all future plans and strips their name and account identifier from past records; anonymous planned-attendance counts remain.

### Registering attendance

- A signed-in member chooses a day and one or more arrival/departure intervals; the interface also shows the resulting expected duration for each interval and the total selected duration.
- Time is selected in 30-minute increments.
- An interval has a 30-minute minimum, cannot cross midnight, and cannot extend beyond that day's final playable light-off time.
- Attendance uses half-open time intervals `[arrival, departure)`: a member leaving at 19:00 does not count in the 19:00-19:30 turnout bucket.
- Attendance is venue-wide; the member does not select Court 1 or Court 2.
- Registration is allowed whenever at least one court is open during each full selected interval. If only one court is open, the confirmation repeats the reduced-capacity warning.
- Continuous venue availability is sufficient: the particular open court may change at a 30-minute boundary as long as at least one court remains open in every selected bucket.
- Each account may hold multiple non-overlapping attendance intervals per day when the member plans to attend separated sessions.
- Members may add to their own intervals or remove their planned attendance for
  the date. Existing planned cells cannot be selected again; adjacent additions
  are combined in summaries.
- Attendance can be registered no more than 14 days ahead.
- The 14-day window contains today plus the next 13 calendar dates in the `Australia/Sydney` timezone.
- A member may remove an entry throughout its 30-day named-history period.
- Removing an individual plan deletes it from named history and all anonymous turnout totals. It is not retained as a hidden retraction.
- A successful registration updates the turnout forecast and the member's `Your plans` summary.

### Administration

- Riley is the only MVP administrator.
- Administration uses a password-protected page and a temporary authenticated browser session; a secret URL alone is insufficient.
- Riley can manage light hours, official social sessions, court occupations, coaching, competitions, other closures, and dated schedule annotations.
- Events support weekly recurrence and individual-date exceptions for holidays or cancellations.
- If a later schedule edit makes an existing attendance plan partly or fully unplayable, the plan is retained and marked with a prominent `Schedule changed` warning. The system does not silently remove a member's stated intention.
- Riley uses the existing WhatsApp group for urgent schedule changes because the MVP sends no automated notifications.
- Riley can inspect planned-attendance totals, including private member/non-member aggregates.
- Riley can view an admin-only adoption report with distinct accounts registering attendance per Sydney calendar week and registration counts by day.
- Riley can find an account by display name, full username, or player code and reset its PIN.
- Riley can suspend or delete abusive, duplicate, or impersonating accounts. Suspension blocks access and new attendance and immediately hides the account's future plans from the turnout forecast. Past plans remain under the normal 30-day rule unless Riley deletes the account.
- Reinstatement restores account access but does not restore hidden future plans; the member must register attendance again.
- Permanently deleting an account releases its four-digit player code for future random allocation because all past contributions have already been detached from that identity.
- Account merging and transfer of attendance history are not supported in the MVP.
- There is no backup administrator in the MVP; Riley's availability is an accepted operational risk.

## Member Workflow

1. Open the club-shared link.
2. Scan the next 14 days and identify empty, active, good-turnout, or crowded periods.
3. Check official-session markers, light hours, and court-specific occupations.
4. Create a lightweight account or unlock an existing account with full username and PIN.
5. Choose an arrival time and duration in 30-minute increments.
6. Review the resulting intervals and privacy notice.
7. Confirm attendance.
8. See the forecast and `Your plans` update immediately.
9. Later, remove or replace the intervals if plans change.

## New-Player Workflow

1. Receive the club-shared link.
2. Find the venue, contact, official social-day, and WhatsApp information without creating an account.
3. Scan upcoming turnout and identify a suitable session.
4. Create an account if they decide to announce attendance.
5. Self-report whether they are a current club member.

## Past-Plans Workflow

1. Open `Past plans` from the shared dashboard.
2. Browse the previous 30 Sydney calendar dates by day.
3. See display names, planned arrival and departure, and a snapshot of the court-status context shown for that date.
4. A signed-in member may remove their own historical plan during the 30-day visibility period.
5. The view labels all records as planned attendance and does not claim that a person arrived or played.
6. Contributions anonymised through account deletion remain in aggregate totals only and do not appear as `Anonymous` rows in the named list.

## Administrator Workflow

1. Sign in to the protected administration page.
2. Maintain normal light hours for Court 1 and Court 2.
3. Create weekly official social sessions, competitions, or coaching events.
4. Add individual exceptions, closures, and dated schedule annotations.
5. Preview the member-facing result, then save the change. The MVP does not have a separate draft-and-publish workflow.
6. Review planned-attendance and member/non-member totals.
7. Search by display name if needed, then select the exact account by full username or player code and complete the approved identity check before resetting its PIN.
8. Deliver the temporary PIN privately; the member sets a new PIN at next sign-in and all older sessions remain revoked.

## Interaction and Content Requirements

- The first screen answers three questions quickly: `When can I play?`, `How many people plan to be there?`, and `Are either courts occupied?`
- Turnout is visually dominant; court operations are secondary context.
- The interface must never present an attendance action as `Book court` or a registration as a `booking`.
- Confirmation states repeat the date, arrival, departure, and venue-wide nature of the plan.
- Cancellation language should be `Remove my attendance` rather than `Cancel court`.
- Conflict states explain when light hours or a court event changed while a member was confirming.
- Plans affected by a later closure or light-hours change remain visible with a prominent warning that identifies what changed and asks the member to review or remove the plan.
- Schedule-change warnings are recalculated from the current authoritative schedule. A warning identifies the affected 30-minute sub-intervals and disappears if Riley restores valid availability.
- Members may remove an affected plan or replace it with a currently valid interval; the application never allows replacement with an interval that is already invalid.
- Empty periods invite the member to be the first to announce attendance without implying that a court has been reserved.
- Loading and failure states preserve the selected date and entered interval where safe, so members do not have to start again.
- The weekly desktop view may be dense, but mobile must prioritise one selected day at a time with obvious previous/next-day controls.

## Data, Privacy, and Trust

- The club-shared link is a distribution mechanism, not true access control; it may be forwarded.
- Account creation and attendance confirmation must explain that display names and planned intervals are visible to anyone who receives the link.
- Named attendance history remains visible for 30 days.
- Members can remove their own visible attendance throughout that period.
- Removing an individual attendance plan deletes that plan and its aggregate contribution entirely.
- Deleting an account removes its future plans and anonymises its past contributions immediately, even when those records are less than 30 days old.
- The final treatment of records after 30 days must be confirmed with the committee: delete them entirely or retain anonymous aggregates.
- Reports must label registrations as `planned attendance`, never `attendance` or `court usage` without independent verification.
- PINs are credentials and must not be displayed or stored as readable values.
- Riley needs a documented process for identity checks before resetting a PIN; the unique player code alone may be known by other people.

## Recurring Events and Schedule Rules

- Tuesday, Friday, and Sunday are weekly official social sessions.
- These days are the initial recurring defaults. Riley may change the authoritative schedule when the club programme changes.
- Court light hours may differ by day and court.
- Competition, coaching, lights-off, and closure events may apply to one court or both.
- Coaching and competition occupations cannot overlap on the same court. Administration rejects the conflict instead of publishing multiple contradictory statuses.
- Lights-off and closure rules take precedence over coaching, competition, and open-for-play status. If a new closure conflicts with an existing event, Riley sees the affected event and must confirm the override before saving.
- Weekly events support an optional end date and per-date exceptions.
- Editing a weekly event offers two scopes only: `This occurrence`, which creates a dated exception, and `This and future`, which ends the old series before that date and starts a replacement series.
- Past occurrences are never rewritten by a recurrence edit. Existing exceptions before the split remain with the old series; future exceptions must be reviewed against the replacement before Riley confirms the change.
- `Past plans` uses a stored daily court-status snapshot rather than reconstructing history from the latest recurring rules.
- The snapshot becomes final at midnight after that date in `Australia/Sydney`. Riley cannot rewrite it retrospectively; later corrections appear only as clearly labelled explanatory notes.
- Manually maintained events are authoritative in the MVP.
- Google Calendar overlay or synchronisation is a future capability, not an MVP dependency.

Schedule content has four distinct roles:

- `Light-hours template`: normal open intervals for one court on a weekday.
- `Court occupation`: competition or coaching with start, end, court target, title, and optional note; it changes that court's status.
- `Closure override`: lights-off or other closure with start, end, court target, and reason; it overrides normal hours and occupations after confirmation.
- `Schedule annotation`: schedule-related informational text with a required affected date or date range; it appears on those timetable dates but never changes court status by itself.

Light hours, court occupations, and closure overrides must start and end on 30-minute boundaries. Schedule annotations target whole dates and do not create partial availability buckets.

## Explicitly Not in the MVP

- Court reservations or enforcement of the in-person court-governance process
- University single sign-on
- Email-based accounts or formal membership verification
- Payments, credits, or membership entitlements
- Automated attendance notifications
- Google Calendar API integration
- General-purpose club news publishing
- Multiple administrator roles or audit attribution
- Account merging or attendance transfer between accounts
- Verified check-in or proof of actual court usage
- Court assignment for individual attendees

## Future-Compatible Capabilities

- Google Calendar event overlay or synchronisation
- Multiple administrators and change audit history
- Stronger authentication or university identity integration
- Anonymous long-term trend reporting
- Verified on-site check-in if the committee needs actual-usage evidence
- Configurable turnout bands and peak/off-peak rules
- Capacity-aware turnout bands based on the number of open courts
- Optional WhatsApp sharing shortcuts, without automated messages

## Success Criteria

Primary success criterion:

- At the end of week four, at least 20 distinct active accounts have a non-deleted attendance plan scheduled during that Monday-Sunday period.

Measurement rule:

- Week four is the fourth complete Monday-Sunday period after launch in the `Australia/Sydney` timezone.
- An account counts once when, at the end of that Sunday, it is active and has at least one non-deleted attendance plan whose planned date falls within week four.
- Removed plans do not count. Changing a qualifying plan does not create a second count.
- Riley's report shows the total qualifying accounts and, separately, the subset that currently self-report as club members.
- Account creation alone is excluded, and the metric does not claim that one account is a verified person.

Secondary indicators:

- Registrations cover more than one official social day and at least one non-social day.
- Members consult the forecast before choosing a session.
- Members report fewer unexpected turnout levels.
- Riley can keep light hours and event occupations accurate without the administration becoming burdensome.

Secondary measurement uses a short WhatsApp poll after week four rather than an in-app survey. Riley asks whether members checked the dashboard before choosing a session, whether it reduced unexpected turnout, and what prevented them from using it. These responses are qualitative learning, not part of the primary adoption threshold.

Account creation alone does not count as successful adoption; an active account must have a qualifying plan at the reporting cutoff.

## Approaches Considered

### A. Minimal one-off attendance board

Use independent name-and-PIN registrations without permanent accounts. This is the fastest way to test demand and stores less personal data, but it makes repeat use and recovery awkward.

### B. Lightweight club attendance hub

Add permanent self-service accounts, personal plans, Riley-managed recovery, operational court overlays, and 30-day named history. This reduces repeat friction but increases privacy and administration responsibility.

### C. Turnout forecast

Make the empty/active/good/crowded forecast the primary calendar signal and keep Court 1 and Court 2 status secondary. This answers the member's real decision better than a booking-style grid and adapts more naturally to mobile.

## Recommended Approach

Combine B and C: build a lightweight club attendance hub whose information hierarchy is centred on turnout forecasting.

The application should borrow the compact scanability of sports timetables without copying YepBooking branding, assets, or its booking mental model. Permanent accounts make repeated attendance registration practical; the forecast-first presentation keeps the product focused on John's actual decision.

## Premises Accepted

1. The MVP tests whether visibility changes member decisions; it cannot guarantee reduced crowding.
2. Registrations are stated intentions, not verified attendance or court reservations.
3. Riley's manual operational schedule is authoritative in the MVP.
4. A shared link can be forwarded, so named history is potentially visible beyond the intended club audience.
5. Four-digit PINs provide low-assurance access only and require attempt limiting plus administrator recovery.

## Accepted MVP Risks

- The chosen MVP is broader than the narrowest turnout experiment because it includes permanent accounts, public 30-day named history, recurrence management, private membership aggregates, moderation, dated schedule annotations, and adoption reporting. These were retained through explicit discovery decisions.
- Four-digit player codes and PINs are enumerable and allow some targeted lockout risk even with layered throttling. The club accepts this low-assurance model for the MVP; stronger credentials are the first security upgrade if abuse occurs.
- Deleted player codes may later be reused after all linked history has been anonymised. A former owner could recognise the reused code, but cannot sign in without the new immutable username and PIN.
- Riley is a single point of operational failure for schedule maintenance, moderation, and recovery.
- Public named history is weakly connected to the forward-looking turnout hypothesis and creates privacy obligations; it remains because the product owner explicitly selected it.

## Information Required From the Committee

### Implementation decisions

These four answers materially affect data, access, or product rules and must be resolved before technical planning is approved:

1. Approval to display member names and 30-day planned-attendance history through a forwardable shared link.
2. Whether records older than 30 days must be deleted or may be anonymised for long-term totals.
3. Whether visitors and non-members may create accounts and announce attendance under existing club or university rules.
4. The identity evidence Riley must require before resetting a PIN.

Release 1 clarification: `ENGINEERING_PLAN.md` supersedes items 1 and 3 for the first implementation. The committee must approve public-internet upcoming display names and intervals plus unrestricted public account creation/visitor attendance. Public 30-day named history is deferred. The retention answer remains required for expired plans, anonymous success evidence, and account deletion.

### Launch content and configuration

These answers do not block architecture work but are required before launch:

1. Exact light-on and light-off times for each court and day of the week.
2. Exact start and end times for Tuesday, Friday, and Sunday official social sessions.
3. Current recurring competition and coaching schedules, including which court each occupies.
4. Known university closures, term breaks, public-holiday rules, and schedule exceptions.
5. Approved club location wording, directions, contact details, and WhatsApp invitation process.
6. The preferred channel for distributing and periodically rotating the club-shared link.

### Post-launch and operational decisions

1. Who will take over administration if the single-administrator assumption becomes unacceptable.
2. Whether the fixed turnout bands match actual experience after the initial trial.

## The Assignment

Riley should take this document to the club committee and obtain written answers to the four launch decisions above. The approved `ENGINEERING_PLAN.md` authorises synthetic-data milestones M0-M4 in parallel; real member data, affected production capabilities, and the M5 pilot remain gated until the answers confirm the plan or are recorded as accepted launch risks.

## What I Noticed During Discovery

- The initial language was `booking app`, but the in-person system already governs courts. The actual product emerged when the problem became `WhatsApp messages can get drowned out`.
- The concrete user behaviour mattered: John will skip a session when turnout is too low or too high. That makes turnout, not court inventory, the primary interface.
- The plan deliberately accepted more identity scope over time: one-off attendance became permanent accounts, Riley-managed recovery, and 30-day named history. Those choices improve repeat use while increasing privacy responsibility.
- The most important visual subtraction is removing the implied promise that clicking a time reserves a court.
