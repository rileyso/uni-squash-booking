# Sydney University Squash Club Attendance Tracker Design

Status: APPROVED — PENDING COMMITTEE INPUT  
Date: 14 July 2026  
Product source: `PRODUCT_DESIGN.md`

## Design Objective

Create a fast, practical, accessible attendance forecast for a university sports club. Members should understand expected turnout and court operating conditions within seconds, without mistaking the interface for a court-booking system.

## Release 1 Interface Matrix

`ENGINEERING_PLAN.md` D1 reduced the first implementation after this broader design was approved. This matrix is authoritative for Release 1; sections marked post-validation preserve approved future direction but do not authorise Release 1 routes, navigation, templates, queries, or administration controls.

| Surface / destination | Release 1 status |
|---|---|
| Public 14-day attendance timetable and interval details | Build |
| Club location, contact, and WhatsApp information | Build |
| Create account, sign in/out, profile/PIN change, self-delete | Build |
| `Your plans`, add/change/remove attendance, confirmation and conflicts | Build |
| Riley sign-in, schedule/light/event recurrence, exceptions, PIN reset | Build |
| Loading, empty, success, conflict, and unavailable states for built surfaces | Build |
| Public `Past plans` and 30-day named history | Deferred until after turnout validation |
| Dated schedule annotations/notices | Deferred until after turnout validation |
| Adoption and member/non-member report interfaces | Deferred until after turnout validation |
| Web suspension/reinstatement/deletion UI | Deferred; Release 1 has a deployment-local CLI only |

## Reference Boundary

The interface may take structural inspiration from the compact timetable layout of the Australia Badminton Development Centre YepBooking schedule: dense time information, direct date controls, sports-category clarity, and visible location/contact context.

It must not copy YepBooking branding, assets, colours, text, logo, or exact composition. It must also reject the reference product's reservation mental model because this application does not book courts.

## Product Language

Use:

- Add my attendance
- Your plans
- Remove my attendance
- Planned attendance
- Open for play
- Lights off
- Competition
- Coaching
- Other closure

Do not use:

- Book court
- Booking confirmation
- Your bookings
- Cancel booking
- Available slot
- Booked slot

## Information Hierarchy

Every main-dashboard viewport answers these questions in order:

1. How busy is the venue expected to be?
2. When can I play, and are one or both courts operational?
3. What have I already planned, and how do I add or change attendance?

Location, contact, WhatsApp, and account controls remain discoverable but do not compete with the timetable. `Past plans` becomes discoverable only in the post-validation interface.

## Approved Desktop Weekly Structure

### Superseding desktop timetable layout — 14 July 2026

This desktop layout supersedes the seven-day-column structure below. The
earlier structure remains as historical context only.

- Desktop shows one selected date in a stable-width horizontal timetable.
  Narrowing the window scrolls the timetable instead of compressing its cells.
- A nine-element date bar sits above it: one calendar-icon/selected-date cell
  followed by eight consecutive date choices inside the 14-day window. Today is
  labelled `Today`; the selected date uses text, border, and background emphasis.
  Official-social badges do not appear inside this compact desktop date bar.
- Time runs horizontally from 10 am through 10 pm at one-hour points. The server
  retains authoritative 30-minute intervals for attendance and validation.
- Each time header shows venue-wide turnout using only a people icon and exact
  count visually. Its accessible name states that these are planned attendees.
- Court 1 and Court 2 are the only two body rows. They are non-interactive
  operational-status rows and must not resemble selectable or reservable courts.
- Mobile retains the selected-day vertical timetable with 30-minute intervals.
- The dashboard does not include a separate `How the timetable works` legend
  bar. The non-reservation statement remains above the calendar, while status
  meaning is communicated inline through text, icons, and accessible names.
- The desktop date bar distributes all nine elements across the available main
  width without its own scrollbar. Its leading selected-date cell has no icon.
- The desktop date bar uses 70% of the available main width and an approximately
  45px height. Mobile retains a full-width strip with 44px minimum targets.
- The screenshot-approved date-bar treatment supersedes those dimensions: a
  shrink-wrapped rounded rail contains a previous/date/next pill and eight date
  pills. It uses the approved white, border, navy, and pale-blue selection
  palette rather than the screenshot's dark and amber colours. Date logic and
  accessible labels remain application-owned.
- Date pills use the calendar's compact `0.82rem` IBM Plex typography at medium
  weight. The selected date is a solid navy pill with white text and no inset
  underline.
- Date pills, arrows, and main-content interactive controls use the existing
  `info` blue hover text without changing their container or pill borders. The
  navy selected pill remains white for sufficient contrast.
- Each week page contains only seven normal Monday–Sunday date pills, including
  today's ordinary weekday/date label and elapsed days on the current page.
  Selecting a date does not roll the visible choices. Previous/next arrows move
  one day at a time; crossing a Monday/Sunday boundary changes the visible week.
  Navigation is bounded to those two weeks.
- Previous/next controls use compact long-shaft SVG arrows with angled heads,
  matching the approved screenshot shape while inheriting accessible state and
  hover colours.
- The Court 1/Court 2 desktop calendar borrows only the public reference's
  Alexandria grid dimensions: `65px × 32px` cells with `64px × 31px` inner
  status content. Our one label plus thirteen hourly columns total `910px`; the
  table is centered and scrolls rather than shrinking. Interactive turnout
  counters retain the product-required 44px target.
- The bordered timetable container shrink-wraps the desktop calendar instead of
  stretching across unused main-content width. Its left edge aligns with the
  date navigation bar, and it remains capped at the viewport width so narrow
  layouts can scroll safely.
- Desktop hourly headers show the numeric turnout counter without a people icon.
  Counters use small plain text without a filled background or status underline.
  Visible hourly labels use 24-hour values with minutes, such as `13:00`.
  On pointer devices, a desktop counter appears when the matching Court 1 or
  Court 2 cell is hovered, or when the counter is keyboard-focused. It remains
  visible where hover is absent.
  The revealed count uses a compact bordered tooltip anchored to the matching
  Court 2 cell, with its pointer aimed at that cell's bottom edge. Zero-attendance
  tooltips explicitly show `0`. The tooltip is flipped below the Court 2 row,
  widened to 64px, and reduced to 22px high; its top pointer targets the bottom
  of Court 2. The time header is 32px high.
  Synthetic Saturday coaching cells include the medium-weight inline label
  `Squads`; this wording is derived only from the `Squads` event title.
  Coaching cells omit the special-event diamond because the compact label is the
  clearer marker. The desktop timetable container permits visible overflow so
  attendance tooltips can overlap the main canvas without creating an internal
  scrollbar.
- A compact key below the timetable pairs red with `Lights off`, purple with
  `Squads`, and green with the approved `Open for play` wording. The date-control
  pill has no calendar logo. Monday Pennant training has no separate purple
  operational overlay in the current synthetic presentation.
  Key swatches are small solid circles rather than outlined squares.
- A bordered seven-column `Social` schedule sits below the status key and follows
  the selected Monday–Sunday week. Its neutral date headings receive no selected
  highlight. Tuesday displays `4:00pm–6:00pm`; Friday displays
  `3:00pm–6:00pm`, derived from authoritative social-session rows.
- Narrow desktop windows with a mouse or trackpad retain the horizontal calendar
  instead of hiding it at the mobile breakpoint. Non-hover touch devices retain
  the selected-day mobile timetable.
  Court cells omit visible `Open for play` and `Lights off` text to reduce row
  height; their status symbols remain visible and full labels remain available
  programmatically. Other surfaces retain the approved full status vocabulary.
  Desktop open/closed cells omit tick and cross symbols. Closed cells use a
  subtle hatch so operational state is not communicated by colour alone;
  special-event symbols may remain. Desktop date-bar type is 30% smaller than
  the surrounding interface, while mobile retains the normal readable size.
- The dashboard heading names the venue as `Manning Squash Courts (A24)`; planned
  attendance remains the meaning of its turnout counts and accessible labels.

Decision: seven day columns with time running vertically.

```text
+------------+--------------------------------------------------------------------+
| Club       | 14-day controls                         Account                     |
| Attendance +--------------------------------------------------------------------+
| Club info  | Attendance does not reserve a court     Your plans                 |
| Account    +------+--------+--------+--------+--------+--------+--------+---------+
|            | Time | Mon 14 | Tue 15 | Wed 16 | Thu 17 | Fri 18 | Sat 19 | Sun 20 |
|            |      |        | Social |        |        | Social |        | Social |
|            +------+--------+--------+--------+--------+--------+--------+--------+
|            |17:00 | turnout count, band, and compact member previews            |
|            |      | aligned Court 1 and Court 2 operational-status rails        |
|            |17:30 | selected detail opens without breaking grid alignment       |
|            |18:00 | current user's interval has `Your plan` plus an outline     |
|            | ...  |                                                            |
|            +------+------------------------------------------------------------+
|            |                         Add my attendance                           |
+------------+--------------------------------------------------------------------+
```

### Desktop rules

- The time axis and day headers remain visible while scrolling.
- Turnout occupies most of each day column.
- Court 1 and Court 2 use slim status rails and must never look clickable for reservation.
- Official social days use a text marker in the header, not colour alone.
- Up to three display-name initial tokens preview attendance without expanding a row; the complete names appear in the detail disclosure.
- `Your plans` uses a persistent outline, label, or personal marker in addition to colour.
- A single primary `Add my attendance` action remains visible without covering timetable content.
- Empty, reduced-capacity, changed-schedule, and court-occupation states must preserve the same grid geometry.

### Approved attendance-detail disclosure

- Selecting or focusing a turnout interval opens an anchored detail popover on desktop.
- On mobile, the same action opens a bottom sheet with a visible heading, close control, and focus trap.
- The detail surface lists public display names and their planned intervals, repeats the turnout label, and includes `Add my attendance` when the selected interval is eligible.
- The triggering interval remains visibly selected while details are open.
- Escape closes the desktop popover; close, back, or downward dismissal closes the mobile sheet without changing attendance.
- On close, keyboard focus returns to the triggering interval.
- Complete attendee names are never permanently repeated inside the dense timetable; only the approved initial-token preview appears there.

## Responsive Direction

- Desktop weekly structure is approved.
- The desktop grid must never be scaled or horizontally squeezed into a phone viewport.

### Approved mobile date navigation

- Mobile shows one selected day at a time.
- A horizontally scrollable strip exposes seven adjacent dates, including text markers for official social days.
- Persistent previous-day and next-day buttons flank or sit immediately beside the strip.
- Selecting a date replaces the timetable below while preserving the page position and focus context.
- Dates use at least 44px touch targets and expose selected/today/social state through text or shape as well as colour.
- Swipe may supplement the controls but is never the only navigation method.
- Moving beyond the visible seven dates shifts the strip within the approved 14-day window.

### Approved mobile selected-day structure

- The selected day uses a vertical time axis with 30-minute intervals.
- Venue-wide turnout is the dominant band for each interval.
- Two aligned compact rails beneath or immediately beside each turnout band show Court 1 and Court 2 status for the same interval.
- Court rails carry a short text label or accessible abbreviation plus pattern/icon; they do not look tappable for reservation.
- Time, turnout, and court status remain visually aligned so members never have to compare separate tabs.
- Repetitive card-per-timeslot and separate turnout/court tabs are rejected.

## Accessibility Baseline

- Body text contrast is at least 4.5:1.
- Interactive controls have visible keyboard focus.
- Touch targets are at least 44px by 44px.
- Colour is always paired with text and a distinguishable shape, pattern, or icon.
- Timetable reading order, headers, and interval relationships must be available to screen readers.
- Pointer hover is never the only way to reveal attendance or status information.

## Approved Add-Attendance Interaction

- Desktop opens a right-side panel while the weekly timetable remains visible and the chosen interval stays highlighted.
- Mobile opens a full-screen sheet rather than a cramped modal.
- Step 1 selects date, 30-minute arrival, and departure between 10:00 am and 10:00 pm, constrained by current venue availability and a one-hour minimum. An accessible two-handle visual range is synchronised with explicit arrival and departure fields, and the resulting duration is shown in text.
- Step 2 presents a short review containing date, arrival, departure, venue-wide wording, current turnout, reduced-capacity or schedule warnings, public-name notice, and the primary `Confirm attendance` action.
- `Back` returns to the editable fields without losing values. `Close` exits without creating a plan and returns focus to the trigger.
- Confirmation success closes the panel or sheet, updates the timetable and `Your plans`, and announces the result through an accessible live region.
- The interface never labels this flow booking or court reservation.

## Approved Remove-Attendance Interaction

- `Remove my attendance` opens a confirmation dialog on desktop and a confirmation sheet on mobile.
- The confirmation repeats the date, arrival, departure, and the fact that the plan will be removed from the public forecast and applicable totals.
- The destructive action is labelled `Remove attendance`; the safe action is `Keep my plan`.
- A signed-in member does not re-enter their PIN.
- Successful removal is permanent, closes the confirmation surface, updates the timetable and `Your plans`, and announces success through an accessible live region.
- If removal fails, the plan remains visibly present and the confirmation surface explains that nothing changed and offers `Try again`.

## Approved Current-User Visibility

- Every interval covered by the signed-in member's plan has a persistent high-contrast outline and visible `Your plan` marker in addition to any colour treatment.
- A compact `Your plans` summary above the timetable lists the member's upcoming date, arrival, departure, and `Change` / `Remove` actions.
- The summary collapses to one concise row per plan and never becomes a dashboard-card mosaic.
- If the member has no upcoming plan, the summary becomes a quiet orientation line with `Add my attendance`, not an empty decorative container.
- Personal markers remain distinguishable in Empty, Players attending, Good turnout, Crowded, and reduced-capacity states.

## Approved Empty States

- A playable interval with zero planned attendees keeps the same grid dimensions and displays `Empty` through text plus its status treatment.
- Opening that interval's detail popover or sheet explains `No one has announced attendance yet` and offers the primary `Be the first to attend` action.
- Empty intervals never contain repeated inline plus buttons.
- Non-playable intervals use their actual court-status context and never invite attendance when no court remains open for the full interval.
- A member with no personal plans sees a quiet `No upcoming plans` line and one `Add my attendance` action.
- A day with no playable hours explains why, such as `Lights off all day`, and points to the next playable date rather than showing a blank grid.

## Approved Loading and Updating States

- Initial loading uses a restrained skeleton that matches the timetable headers, time axis, turnout bands, and court rails so the page does not jump when data arrives.
- The skeleton is labelled for assistive technology with a concise `Loading attendance timetable` status; decorative skeleton shapes are hidden from screen readers.
- Date or week changes keep the current timetable visible while showing a subtle `Updating…` status and setting the affected region to `aria-busy="true"`.
- Only controls affected by an in-flight request are temporarily disabled. Global navigation and cancellation of an open panel remain available.
- Updating never replaces known attendance with a blank grid or a full-page spinner.

## Approved Conflict States

- The server revalidates the selected interval when the member confirms attendance; the browser's earlier timetable state is never treated as authoritative.
- If a schedule change makes the interval invalid, the confirmation panel or sheet remains open and preserves the member's date, arrival, and duration choices.
- A prominent inline conflict message explains exactly what changed, identifies the affected time or court status, and states that attendance was not recorded.
- The affected timetable intervals refresh behind the confirmation surface without moving focus or discarding input.
- The member can adjust to another valid time in the same flow or close it and return to the refreshed timetable.
- Conflict messaging uses direct language such as `The courts are now unavailable from 7:00 pm` and never implies that another member took a reservable slot.

## Approved Refresh Error State

- If the initial timetable request or a later refresh fails, the timetable is replaced by a full-region error state rather than continuing to display potentially stale attendance.
- The error state says that current attendance could not be loaded, makes clear that no data or attendance change was submitted, and provides a prominent `Try again` action.
- Attendance creation, change, removal, and interval-detail actions remain unavailable until a successful refresh restores authoritative data.
- The global page header, contact information, WhatsApp information, account controls, and date context remain visible; this is not a blank browser page.
- Keyboard focus moves to the error heading after a failed user-initiated refresh and returns to the refreshed timetable context after recovery.
- Technical diagnostics are not exposed to members. A short reference code may be shown for support.

## Approved Administrator Blocking Interaction

- After password authentication, Riley enters an explicit `Manage schedule` mode; member attendance controls are hidden or disabled in this mode to prevent role confusion.
- Riley begins an event by selecting a continuous time range directly on Court 1, Court 2, or both within the timetable.
- Pointer users may drag across intervals. Keyboard users focus a start interval, invoke `Select range`, extend the range with arrow keys, and confirm it without requiring a pointer.
- The selected court, date, start, and end remain highlighted while a right-side panel opens for status, public title, recurrence, and effective dates. Dated notice fields are added only with the deferred annotation surface.
- Status choices use the approved operational vocabulary: `Open for play`, `Lights off`, `Competition`, `Coaching`, and `Other closure`.
- The panel provides a timetable preview before saving. `Cancel` discards the draft and restores focus to the starting interval.
- Overlap validation occurs before save; a conflict is explained in the panel and the draft values remain intact.
- Successful save updates the timetable and notice context, announces the result, and retains Riley's current date and scroll position.

### Approved recurrence preview

- A weekly event is reviewed as a compact natural-language summary, such as `Every Tuesday, 6:00–8:00 pm · Both courts · Until 30 September`, rather than a list of generated dates.
- The summary appears immediately above the save action and updates as recurrence fields change.
- The interface still validates generated occurrences before saving. If an overlap or exception needs attention, the panel identifies the affected date or dates and blocks or requests the approved override; the compact summary never hides a known conflict.
- An event without an end date is labelled `Continues until changed` rather than implying a finite schedule.

### Approved recurring-event edit scope

- Selecting edit or delete on a recurring occurrence first opens a dedicated scope dialog.
- The dialog offers exactly the product-approved scopes: `This occurrence` and `This and future`.
- Neither scope is preselected. Each option includes a short explanation of its effect, including that past occurrences will not change.
- Riley must choose a scope before continuing to the editor or deletion confirmation; closing the dialog makes no change and returns focus to the event.
- `This occurrence` creates or removes a dated exception. `This and future` shows the effective date and warns that the existing series will end before a replacement begins.
- If future exceptions need review against a replacement series, the final confirmation identifies them before save.

## Approved First-Visit Orientation

- The dashboard places a short statement directly above the timetable: `See when people plan to play. Attendance does not reserve a court.`
- A persistent compact legend explains the four turnout bands and every court operational status through colour plus text, shape, pattern, or icon.
- The legend is available without opening a modal on desktop and through a clearly labelled expandable region on narrow screens.
- First-time visitors are not forced through a walkthrough, coach mark sequence, or dismissible marketing panel.
- A concise `How it works` disclosure explains first-come-first-served court use, public display names, official social days, and how to add or remove attendance.
- Club location, contact, and WhatsApp information remain easy to find from the dashboard for new or visiting players.

## Approved Timetable Time Window

- Desktop uses one shared vertical time range for the displayed week, beginning at the earliest court light-on time and ending at the latest light-off time across those seven days.
- Every day and both court rails share the same 30-minute row boundaries, allowing direct horizontal comparison.
- Time outside a given court's operating hours remains visible within that shared range and is labelled `Lights off`; it is not presented as empty attendance or selectable time.
- Hours outside the shared weekly range are omitted. The MVP does not render a full 24-hour grid.
- Mobile uses the same authoritative range for the selected day but may initially position the viewport at the first playable interval; earlier lights-off rows remain reachable.

## Approved Typography Direction

- The interface uses IBM Plex Sans Condensed as its single type family for headings, timetable labels, controls, and explanatory text.
- Hierarchy comes from size, weight, case, spacing, and placement rather than an additional display typeface.
- Timetable times and numeric turnout counts use tabular numerals so columns remain stable as values change.
- Body and control text is never reduced merely to fit more cells; the compact layout comes primarily from grid structure and restrained spacing.
- Critical status labels remain fully readable and are not encoded solely as narrow initials.
- Font loading must avoid invisible text and disruptive layout shifts; a metrically reasonable sans-serif fallback is permitted while the chosen family loads.

## Approved Visual Character and Colour Direction

- The product should feel like a modern university sports service: calm, trustworthy, reliable, and professional rather than a startup analytics dashboard or consumer fitness product.
- The broad institutional tone may take inspiration from Sydney Uni Sport's clear service navigation and timetable-oriented information architecture, but the application must not copy its logo, protected brand assets, page composition, or claim official endorsement.
- Deep navy, slate, white, and light grey form the structural palette. Bright or highly saturated colour is avoided unless needed for usability.
- Accent colour is reserved for system meaning: blue for information, green for open/confirmed states, amber for pending, expiring, or crowded warnings, red for unavailable or error states, and violet for administrator or special-event context.
- Every state includes a visible text label and an icon; timetable states additionally use shape, border, or restrained pattern where needed. Colour never carries meaning alone.
- `Open for play` may use green, but the interface never calls a court `available` in a way that implies it can be reserved.
- Turnout styling remains subordinate to its text bands: `Empty`, `Players attending`, `Good turnout`, and `Crowded`. Crowded uses warning emphasis without resembling a booking conflict.
- Surfaces use subtle borders, restrained corner radii, generous whitespace outside the dense grid, and light shadows only where they communicate elevation such as panels, sheets, popovers, and dialogs.
- Readability and WCAG contrast take priority over decorative flair or strict visual similarity to either reference site.

### Approved surface composition

- The desktop and mobile timetable is one continuous bordered grid, not a collection of day or interval cards.
- Rounded card surfaces are reserved for supporting regions such as `Your plans`, location/contact information, and administrator summaries.
- Light shadows are reserved for genuinely elevated layers: side panels, bottom sheets, popovers, and dialogs. Static timetable and information regions rely primarily on borders and spacing.
- Supporting cards use restrained radii and enough whitespace for comprehension without pushing the timetable below unnecessary dashboard chrome.

## Approved Keyboard Timetable Navigation

- The timetable implements a single roving tab stop rather than placing every 30-minute interval in the page Tab sequence.
- Arrow Up and Arrow Down move between time intervals. Arrow Left and Arrow Right move between dates while retaining the closest matching time.
- Home and End move to the first and last interval in the current day; documented modified Home/End shortcuts may move to the first or last date.
- Enter or Space opens the focused interval's attendance details. Escape closes the resulting popover or sheet and restores focus to that interval.
- Tab and Shift+Tab leave the grid for the next or previous page control.
- The currently focused interval has a high-contrast focus indicator that remains distinct from `Your plan`, selected, today, turnout, and court-status treatments.
- Screen-reader instructions are announced once when focus enters the grid, not repeated on every interval.
- Mobile and touch use the same underlying interval names and relationships even when the layout becomes a single-day view.

## Approved At-a-Glance Turnout Content

- Every playable interval shows the exact venue-wide attendee count, its turnout band, a people icon, and a compact preview of attending members.
- The band label remains visible; member previews do not replace `Empty`, `Players attending`, `Good turnout`, or `Crowded`.
- The preview uses only public display-name information already approved for the shared dashboard and never exposes immutable usernames, player codes, membership status, or PIN-related information.
- The grid limits the number of visible member previews and expresses overflow as a count so dense sessions do not expand row height.
- The interval detail popover or sheet remains the authoritative place to read the complete attendee list and attendance intervals.

### Approved member-preview treatment

- The grid displays up to three overlapping circular initial tokens derived from public display names, followed by a compact overflow value such as `+4`.
- Tokens are typographic placeholders, not profile photographs, and do not introduce image upload into the MVP.
- Initial tokens use a restrained neutral treatment so they do not compete with turnout or court-status colour.
- The token group has one accessible name that states the previewed display names and total attendance; individual decorative circles are hidden from assistive technology.
- Duplicate initials are acceptable in the compact preview because activating the interval exposes the complete display-name list.
- `Your plan` continues to use its approved explicit text marker and outline; a member must not identify their plan solely by finding their initials.

## Approved Signed-Out Attendance Journey

- Signed-out visitors may inspect the complete timetable and attendance details through the shared link.
- Activating `Add my attendance` preserves the selected date and time, then opens authentication in a desktop panel or mobile full-screen sheet.
- The authentication surface offers `Sign in` and `Create account` without navigating away from the timetable context.
- Successful sign-in or self-service account creation returns the member to the attendance flow with the original valid date and time intact.
- If the schedule changed during authentication, the approved conflict state preserves the intent, explains the change, and asks the member to adjust it.
- Closing authentication makes no attendance change and restores focus to the initiating control.
- Account errors remain within the authentication surface and never clear the pending attendance intent unless the member explicitly cancels it.

## Approved Timetable Density

- Desktop 30-minute rows target approximately 36–40px in height, subject to text zoom and content fitting without clipping.
- Mobile rows are at least 48px high so the full interval trigger is a comfortable touch target.
- Row height is consistent within a rendered timetable; member-preview overflow never expands an individual row.
- The interface does not include compact/comfortable density preferences in the MVP.
- At 200% browser zoom, content may reflow to the single-day layout rather than compressing labels or introducing two-dimensional page scrolling.

## Approved Official-Social-Day Treatment

- Tuesday, Friday, and Sunday receive an `Official social` text badge in the day header when the authoritative schedule marks them as official sessions.
- Official-social columns do not receive a background tint or saturated colour treatment.
- The badge includes a restrained calendar or group icon but does not compete with turnout, `Your plan`, or court-status treatments.
- Mobile date-strip items use the same short text concept, with an accessible full label; colour alone never identifies an official social day.
- If Riley changes the authoritative programme, badges follow the dated schedule rather than assuming fixed weekdays forever.

## Approved Success Feedback

- Adding, changing, or removing attendance immediately updates the timetable and `Your plans`, then shows a temporary success toast.
- The toast uses concise action-specific language, such as `Attendance added for Friday, 6:30–8:00 pm`, and is announced through a polite live region.
- It remains visible long enough to read, targets roughly six seconds, pauses while hovered or keyboard-focused, and includes a dismiss control.
- The toast never receives focus automatically and never contains the only copy of a required next action or important result.
- The persistent timetable and `Your plans` update remain the authoritative confirmation after the toast disappears.
- Errors and schedule conflicts are not temporary toasts; they remain adjacent to the action that needs attention.

## Approved Post-Validation Schedule Annotations (Deferred from Release 1)

- Member-facing notices are schedule-related annotations attached to a required affected date or date range.
- An annotation appears in the relevant desktop day header and the selected-day mobile header; a concise marker opens its full text without obscuring timetable status.
- Multiple annotations on one date collapse behind a labelled count and remain keyboard and screen-reader accessible.
- The dashboard has no general notice-board card or undated news strip.
- General committee news remains in WhatsApp and is explicitly outside the application's MVP publishing scope.
- An annotation supplies context only and never changes court operating status by itself.

## Approved Court-Status Rail Treatment

- Court 1 and Court 2 rails retain a white or neutral surface across all operational states.
- Each rail combines a short text label and distinct icon with a narrow status-coloured edge; the full rail is never filled with saturated status colour.
- `Open for play` uses a check icon and green edge; `Lights off` uses a power or moon icon and unavailable treatment; `Competition` uses a trophy icon; `Coaching` uses a coaching-appropriate icon; `Other closure` uses a no-entry icon.
- Unavailable states may add subtle hatching that remains legible in grayscale and does not reduce text contrast.
- Violet is reserved for a special-event or administrator context only when it improves distinction; it does not become generic decoration.
- Icon labels are never removed at narrow widths. The layout reflows before reducing a rail to colour or icon alone.

## Approved Desktop Navigation Frame

- Release 1 desktop uses a persistent left sidebar containing the club name and the primary destinations `Attendance` and `Club information`, followed by account controls. The approved post-validation navigation adds `Past plans`.
- Riley sees `Manage schedule` only after administrator authentication; member navigation does not advertise an unusable admin destination.
- The active destination uses a strong text, shape, and border treatment rather than colour alone.
- The sidebar is restrained and service-oriented: no metric cards, oversized icon tiles, promotional artwork, or nested dashboard menus.
- The attendance timetable receives the remaining horizontal space and defines the page's visual priority.
- The sidebar may compact at intermediate widths but must not squeeze the seven-day grid below its readable minimum; the layout switches to the approved single-day mobile presentation first.

## Approved Mobile Navigation Frame

- Mobile retains a narrow navigation rail beside the single-day timetable rather than moving primary navigation to a header drawer or bottom tabs.
- The Release 1 rail provides destinations for `Attendance` and `Club information`; administrator access remains contextual and account controls remain reachable. The approved post-validation rail adds `Past plans`.
- The active destination has a persistent shape and contrast treatment in addition to its icon.
- The rail and selected-day timetable must fit without horizontal page scrolling; the timetable keeps its aligned time, turnout, Court 1, and Court 2 structure.
- Every icon has a programmatic accessible name and at least a 44px touch target.
- A clearly labelled rail control expands navigation as an overlay to reveal full destination names; selecting a destination or dismissing the overlay collapses it.
- Expansion does not permanently reduce timetable width, traps focus while modal on narrow screens, closes with Escape, and returns focus to the rail control.

## What Already Exists

- No production UI, component library, or prior design system exists.
- `PRODUCT_DESIGN.md` supplies approved product language, workflows, turnout bands, court statuses, and privacy constraints.
- Visual mockups are not yet approved. The gstack designer could not run because its OpenAI API key is not configured.

## Not in Scope

- Court reservations, court allocation, and waitlists — the in-person first-come-first-served system remains authoritative.
- Payments, credits, or membership entitlements — they do not test the turnout-forecast hypothesis.
- Verified arrival, check-in, or proof of actual court use — the MVP records intention only.
- General club-news publishing or automated WhatsApp notifications — WhatsApp remains the club's general and urgent communication channel.
- Google Calendar overlay or synchronisation — the manually maintained schedule is the MVP source of truth.
- Profile photographs, avatar uploads, social reactions, chat, or fitness gamification — compact public display-name previews meet the social-discovery need with less data and complexity.
- Multiple administrator roles, vanity-metric cards, or a general-purpose content-management system — Riley is the sole operational administrator.
- A 24-hour timetable, card-per-slot schedule, forced onboarding tour, or display-density preference — each would weaken scanability or add scope without improving the core decision.
- Real-data production launch during the synthetic implementation phase — committee inputs still gate the affected capabilities and M5 pilot.

## Interaction State Coverage

| Surface | Empty / initial | Loading / updating | Success | Conflict | Error / unavailable |
|---|---|---|---|---|---|
| Attendance timetable | Grid preserved with `Empty`; all-day closure points to next playable date | Geometry-matched skeleton initially; current grid plus `Updating…` during navigation | Updated interval is authoritative; success toast is supplementary | Affected interval refreshes while preserved input is explained | Full timetable region is replaced with error and `Try again`; mutation controls unavailable |
| Interval details | `No one has announced attendance yet` plus eligible CTA | Trigger remains selected; details report loading without moving focus | Complete public display-name list reflects the result | Changed status is explained and the initiating time remains identifiable | Inline retry; close returns focus to the interval |
| Add/change attendance | Preserved date/time or a neutral unselected form | Only submit and affected inputs become busy | Panel/sheet closes; timetable and `Your plans` update; toast announces result | Form stays open, values remain, exact schedule change is shown | Persistent inline summary states that nothing was submitted and offers retry |
| Remove attendance | Existing plan summary supplies context | Destructive control becomes busy once; duplicate submission blocked | Plan disappears from grid and summary; toast confirms removal | If the plan changed elsewhere, refresh and explain that no removal occurred | Plan stays visible; confirmation surface offers `Try again` |
| Authentication | Sign-in and create-account choices share one contextual surface | Pending attendance intent remains preserved | Returns directly to attendance review | Schedule conflict after authentication uses the normal preserved-input flow | Account error stays in the surface and never clears the pending intent |
| Date navigation | Current selection is explicit; limits explain the 14-day boundary | Current timetable remains visible with `aria-busy` | Focus context and scroll position persist | Not applicable | Full-region timetable error retains date controls and context |
| Admin event editor | Selected range supplies initial values | Save control becomes busy; draft remains visible | Timetable updates at the same date and scroll position | Draft remains; overlap or override consequence is identified | Persistent inline error; no published schedule change |
| Recurring event edit | Scope dialog starts with neither choice selected | Affected action alone becomes busy | Series or exception summary confirms effective date | Future exceptions or precedence conflicts are named before save | Existing series stays unchanged and retry remains available |
| Schedule annotations (post-validation) | Deferred in Release 1 | Deferred in Release 1 | Deferred in Release 1 | Deferred in Release 1 | Deferred in Release 1 |

No offline mutation queue is provided. The interface never suggests that an attendance change succeeded until the authoritative server response is rendered.

## User Journey Storyboards

### John chooses a less crowded session

| Step | John does | John feels | Design support |
|---|---|---|---|
| 1 | Opens `Attendance` | Oriented within five seconds | Compact legend and non-booking statement |
| 2 | Compares counts, bands, initials, social badges, and court rails | Informed rather than surprised | One aligned weekly grid with turnout dominant |
| 3 | Opens a quieter Friday interval after seeing crowded Sunday | Socially reassured | Complete attendee list in popover or sheet |
| 4 | Chooses `Add my attendance` and sets arrival/duration | In control | Two-step flow with venue-wide wording and warnings |
| 5 | Confirms | Trusts the result | Grid and `Your plans` update; toast is supplementary |
| 6 | Encounters a schedule change | Guided rather than punished | Preserved input and exact conflict explanation |
| 7 | Changes or removes the plan later | Confident the forecast is accurate | Explicit actions and consequence confirmation |

Emotional arc: uncertainty about turnout → rapid comprehension → confidence in the chosen session → visible control over the plan.

### A new player discovers the club

| Step | Visitor does | Visitor feels | Design support |
|---|---|---|---|
| 1 | Opens the shared link | Welcomed, not gated | Full read access and no forced tutorial |
| 2 | Reads how attendance works | Reassured | Public-name and non-reservation explanation |
| 3 | Checks people, social days, location, contact, and WhatsApp | Socially informed | Timetable plus discoverable club information |
| 4 | Selects a time and creates an account | Momentum is preserved | Contextual authentication retains date/time |
| 5 | Returns to review and confirms | Ready to participate | Direct return to the intended action |

Emotional arc: unfamiliarity → reassurance → social confidence → low-friction participation.

### Riley blocks courts and manages a recurring event

| Step | Riley does | Riley feels | Design support |
|---|---|---|---|
| 1 | Signs in and enters `Manage schedule` | Clearly accountable | Explicit mode separates admin and member actions |
| 2 | Selects a court/time range | Precise | Pointer and keyboard range selection |
| 3 | Adds status, title, recurrence, and dates | Context retained | Highlighted range and side panel |
| 4 | Reviews the recurrence and member view | Informed | Natural-language summary and timetable preview |
| 5 | Resolves overlaps or precedence | Cautious, not blocked blindly | Named conflicts with draft preserved |
| 6 | Saves | Confident | In-place authoritative update and announcement |
| 7 | Later edits an occurrence | Protected from broad mistakes | Neutral scope dialog before editor or delete |

Emotional arc: operational responsibility → precise control → informed caution → confidence that members see the intended schedule.

## Design System

### Foundation tokens

| Token | Approved value / rule | Purpose |
|---|---|---|
| `font-ui` | IBM Plex Sans Condensed, sans-serif fallback | Entire interface |
| `text-xs/sm/md/lg/xl` | 12 / 14 / 16 / 20 / 28px; minimum 16px for form inputs on mobile | Compact but readable hierarchy |
| `line-compact/body/heading` | 1.2 / 1.45 / 1.15 | Stable grid and readable prose |
| `space-1..8` | 4, 8, 12, 16, 24, 32, 40, 48px | Consistent spacing scale |
| `navy-900` | `#102A43` | Primary text and institutional structure |
| `slate-700` | `#334E68` | Secondary structure |
| `slate-600` | `#486581` | Muted text only where contrast passes |
| `canvas` | `#F5F7FA` | Page background |
| `surface` | `#FFFFFF` | Timetable, panels, and cards |
| `border` | `#CBD5E1` | Grid and surface borders |
| `info` / `info-soft` | `#1D4ED8` / `#EFF6FF` | Informational state |
| `positive` / `positive-soft` | `#137333` / `#ECFDF3` | Open for play and confirmed state |
| `warning` / `warning-soft` | `#92400E` / `#FFF7E6` | Crowded, pending, or expiring state |
| `danger` / `danger-soft` | `#B42318` / `#FEF3F2` | Unavailable, destructive, or error state |
| `special` / `special-soft` | `#6D28D9` / `#F5F3FF` | Admin or special-event state only |
| `focus` | 3px `#0B63CE` outline with 2px offset | Keyboard focus, distinct from state borders |
| `radius-sm/md/lg` | 4 / 8 / 12px | Grid details / cards / elevated layers |
| `shadow-raised` | 0 8px 24px rgba(16, 42, 67, 0.14) | Panels, popovers, sheets, dialogs only |
| `motion-fast/base` | 120 / 180ms, disabled for reduced motion | State and overlay transitions only |

Final foreground/background pairs must be checked for WCAG AA in implementation. Soft colours are backgrounds and never replace their darker semantic foregrounds.

### Turnout mapping

| Product band | At-a-glance treatment |
|---|---|
| `Empty` (0) | Neutral surface, `0 · Empty`, people icon, no initial tokens |
| `Players attending` (1–4) | Informational edge/icon, exact count, band, up to three initials |
| `Good turnout` (5–9) | Positive edge/icon, exact count, band, up to three initials and overflow |
| `Crowded` (10+) | Warning edge/icon, exact count, band, initials/overflow; never uses unavailable red |

### Core components

| Component | Required behaviour |
|---|---|
| Desktop sidebar | Persistent labelled destinations, strong active state, contextual admin entry |
| Mobile navigation rail | 44px targets, active shape, labelled expansion overlay, focus trap and restoration |
| Date controls | 14-day bounds, today/selected/social text markers, previous/next controls |
| Timetable grid | Sticky headers/time axis, roving focus, aligned 30-minute rows, semantic relationships |
| Turnout interval | Count, band, people icon, initial tokens, overflow, selected and `Your plan` states |
| Court-status rail | Neutral surface, narrow semantic edge, icon, text, optional unavailable hatch |
| Attendance popover/sheet | Complete names and intervals, turnout summary, eligible CTA, focus restoration |
| Attendance panel/sheet | Edit then review, preserved values, server validation, conflict/error summaries |
| `Your plans` summary | One compact row per plan with visible change/remove actions |
| Confirmation dialog/sheet | Explicit consequence, safe secondary action, destructive action where applicable |
| Toast | Polite live announcement, roughly six seconds, pause/dismiss, nonessential content only |
| Error region | Error heading, plain-language outcome, reference code if useful, `Try again` |
| Admin schedule editor | Direct range selection, side panel, member-facing preview, recurrence summary |
| Scope dialog | Neither choice preselected; `This occurrence` and `This and future` explanations |
| Schedule annotation | Required date range, concise date-header marker, accessible expanded text |

## Responsive and Accessibility Requirements

- Use content-driven layout changes rather than device labels. Switch from the seven-day grid before any column becomes unreadable; validate the practical breakpoint with real content.
- At 320 CSS pixels, the expanded mobile rail overlays rather than displacing the timetable, and no page-level horizontal scroll is permitted.
- Support text reflow and 200% browser zoom without clipped labels, hidden actions, or two-dimensional page scrolling.
- Timetable semantics must expose date, time, turnout, attendee count, personal-plan state, and both court statuses for the focused interval.
- All dialogs, sheets, popovers, and rail overlays have visible headings, predictable dismissal, focus containment where modal, and focus restoration.
- Validation presents an error summary plus field-level association. Status updates use appropriate polite or assertive live regions without duplicate announcements.
- Respect `prefers-reduced-motion`; no interaction depends on animation, drag, hover, swipe, or long press.
- Provide pointer-independent alternatives for range selection and every contextual action.
- Verify high-contrast mode, grayscale distinction, 200% zoom, keyboard-only use, and representative screen-reader flows before release.

## Review Findings and Ratings

| Pass | Initial | Final | Resolution |
|---|---:|---:|---|
| Information architecture | 5/10 | 10/10 | Defined desktop hierarchy, mobile day model, navigation frames, disclosure, and ASCII structure |
| Interaction states | 4/10 | 10/10 | Specified empty, loading, updating, success, conflict, refresh error, and recovery behavior |
| Journey and emotional arc | 5/10 | 10/10 | Storyboarded John, a new player, and Riley from orientation through recovery |
| AI-slop risk | 6/10 | 10/10 | Classified as app UI; rejected card mosaics, gradients, ornamental icons, generic hero copy, and saturated status blocks |
| Design-system alignment | 2/10 | 10/10 | Established typography, colour, spacing, radius, motion, status mapping, and component vocabulary |
| Responsive and accessibility | 5/10 | 9/10 | Defined single-day mobile behavior, roving grid navigation, focus, zoom, touch, screen-reader, and reduced-motion requirements; rail validation is tracked |
| Unresolved decisions | 3/10 | 10/10 | All interactive review decisions were answered; one approved validation task is deferred, not undecided |

App-UI litmus result: product identity is explicit; the timetable is the single visual anchor; each region has one job; cards are limited to supporting interactions; motion is functional; and the design remains coherent without decorative shadows.

## Implementation Tasks

Synthesized from this review's findings. Each task derives from a specific finding above. Run with Claude Code or Codex after the implementation gate; check each item as it ships.

- [ ] **T1 (P1, human: ~1d / CC: ~2h)** — Page shell — Build semantic desktop and mobile navigation frames.
  - Surfaced by: Information architecture — persistent desktop sidebar and expandable mobile rail require intentional focus and layout behavior.
  - Files: `internal/web/`, `web/templates/`, `web/static/`
  - Verify: Keyboard navigation, 320px layout, landmarks, skip link, and focus restoration pass manual checks.
- [ ] **T2 (P1, human: ~2d / CC: ~4h)** — Timetable — Render authoritative weekly and selected-day attendance grids.
  - Surfaced by: Information architecture — turnout must dominate aligned 30-minute rows while both courts remain secondary status rails.
  - Files: `internal/web/`, `web/templates/attendance/`, `web/static/`
  - Verify: Server and rendering tests cover the 14-day window, shared weekly range, official-social badges, and lights-off rows.
- [ ] **T3 (P1, human: ~2d / CC: ~4h)** — Timetable interaction — Add accessible interval navigation and attendance detail disclosure.
  - Surfaced by: Responsive and accessibility — roving focus, sticky context, popover/sheet behavior, and complete accessible interval names are required.
  - Files: `internal/web/`, `web/templates/attendance/`, `web/static/`
  - Verify: Keyboard-only and representative screen-reader flows open and close details with correct focus restoration.
- [ ] **T4 (P1, human: ~2d / CC: ~4h)** — Member attendance — Build preserved-intent authentication and add/change/remove flows.
  - Surfaced by: Journey — signed-out and returning members must retain context through authentication and confirmation.
  - Files: `internal/auth/`, `internal/attendance/`, `internal/web/`, `web/templates/`
  - Verify: Handler/domain tests cover confirmation, removal, one-plan-per-day rules, invalid intervals, and intent preservation.
- [ ] **T5 (P1, human: ~2d / CC: ~4h)** — Interaction states — Implement authoritative loading, success, conflict, and error recovery.
  - Surfaced by: Interaction states — stale data must not be actionable and conflicts must preserve member input.
  - Files: `internal/web/`, `web/templates/components/`, `web/static/`
  - Verify: HTMX and full-page tests exercise skeletons, updating, toast timing, duplicate submission, conflict, and retry paths.
- [ ] **T6 (P1, human: ~3d / CC: ~6h)** — Schedule administration — Build Release 1 range blocking, recurrence, and scopes; dated annotations remain deferred.
  - Surfaced by: Journey — Riley needs direct pointer/keyboard range selection with explicit recurrence consequences.
  - Files: `internal/schedule/`, `internal/admin/`, `internal/web/`, `web/templates/admin/`
  - Verify: Domain and UI tests cover overlaps, closure precedence, weekly splits, exceptions, previews, and scope confirmation.
- [ ] **T7 (P2, human: ~1d / CC: ~2h)** — Design system — Apply approved tokens, icons, turnout bands, and neutral court rails.
  - Surfaced by: Design-system alignment — semantic accents must remain restrained and never communicate alone.
  - Files: `web/static/`, `web/templates/components/`
  - Verify: Automated contrast checks plus grayscale, high-contrast, and visual state review pass for every status.
- [ ] **T8 (P1, human: ~2d / CC: ~4h)** — Accessibility QA — Verify the complete responsive interaction model.
  - Surfaced by: Responsive and accessibility — the dense grid and layered surfaces create keyboard, zoom, and announcement risk.
  - Files: `internal/web/`, `web/templates/`, `web/static/`, test files
  - Verify: Test at 320/390px, 200% zoom, keyboard-only, reduced motion, touch, and representative screen readers.
- [ ] **T9 (P2, human: ~1d / CC: ~2h)** — Real-data validation — Exercise the design with committee schedules and representative crowding.
  - Surfaced by: Remaining external inputs — row density and status combinations cannot be considered final with placeholder schedules alone.
  - Files: fixtures and test data selected during architecture review
  - Verify: Two-court light hours, Tue/Fri/Sun sessions, competition/coaching, closures, and 10+ turnout remain scannable.
- [ ] **T10 (P3, human: ~0.5d / CC: ~1h)** — Mobile navigation — Prototype and member-test the expandable rail.
  - Surfaced by: Responsive and accessibility — the approved unconventional rail remains a usability risk on narrow screens.
  - Files: `TODOS.md`, prototype artifacts
  - Verify: Meet the explicit exit criteria in `TODOS.md` with at least three representative club members.

## Remaining External Inputs

The visual and interaction decisions are approved. Implementation still depends on the committee inputs classified in `PRODUCT_DESIGN.md`, including actual light hours, social-session times, recurring occupations, closures, approved contact/location/WhatsApp copy, privacy terms, retention authority, PIN-reset verification, and operational ownership.

Visual mockups remain unavailable because the optional gstack designer integration has no configured OpenAI API key. This does not change the approved written design direction.

## Completion Summary

```text
+====================================================================+
|         DESIGN PLAN REVIEW — COMPLETION SUMMARY                    |
+====================================================================+
| System Audit         | Written UI plan; no production UI exists   |
| Step 0               | 5/10; desktop timetable + mobile priority  |
| Pass 1  (Info Arch)  | 5/10 → 10/10 after fixes                   |
| Pass 2  (States)     | 4/10 → 10/10 after fixes                   |
| Pass 3  (Journey)    | 5/10 → 10/10 after fixes                   |
| Pass 4  (AI Slop)    | 6/10 → 10/10 after fixes                   |
| Pass 5  (Design Sys) | 2/10 → 10/10 after fixes                   |
| Pass 6  (Responsive) | 5/10 → 9/10 after fixes                    |
| Pass 7  (Decisions)  | 31 resolved, 0 deferred                    |
+--------------------------------------------------------------------+
| NOT in scope         | Written (9 exclusions with rationale)      |
| What already exists  | Written                                    |
| TODOS.md updates     | 1 proposed, 1 approved                     |
| Approved mockups     | 0 generated, 0 approved                    |
| Overall design score | 5/10 → 9/10                                |
+====================================================================+
```

The plan is design-complete. Run a code-level design review after implementation for visual and interaction QA. The `/autoplan` JSONL artifact was not written because `jq` is unavailable and the gstack workflow forbids hand-rolled JSONL.

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | Not run | Product discovery was completed separately |
| Codex Review | `/codex review` | Independent second opinion | 0 | Not run | No current review-log entry |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 0 | Required | Must validate the interaction and persistence architecture before implementation |
| Design Review | `/plan-design-review` | UI/UX gaps | 1 | Clear | Score 5/10 → 9/10; 31 decisions; 0 unresolved |
| DX Review | `/plan-devex-review` | Developer-experience gaps | 0 | Not run | No current review-log entry |

**VERDICT:** DESIGN CLEARED — engineering review required before implementation.

NO UNRESOLVED DECISIONS
