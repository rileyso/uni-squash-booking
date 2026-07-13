# Sydney University Squash Club Booking System

I want to design and build a court-attendance web application for the Sydney University Squash Club. Right now the focus is just the single web page that is a calendar-sytle dashboard

The application will be built with:

- Go
- Standard `net/http` where practical
- HTMX
- Server-rendered HTML templates
- CSS without a heavy JavaScript framework
- PostgreSQL or SQLite, depending on what we decide during architecture planning

The club currently serves approximately 231 members.

## Reference design

Use this website as a visual and interaction reference:

https://australia-badminton-development-centre.yepbooking.com.au/


Do not copy its logo, name, text, branding, or proprietary assets. Create an original Sydney University Squash Club interface that is strongly inspired by its general visual system and booking experience.

The reference design has:

- design to match a larger ecosystem resembling https://susf.com.au/
- Rounded date-selection controls
- A prominent weekly booking schedule
- Courts displayed as rows
- Times displayed as columns
- Colour-coded occupied and available cells
- Location and booking-category filters
- Previous-day and next-day navigation
- A visible selected date
- A second schedule area for training or organised sessions
- A compact, sports-club-oriented visual style
- Desktop-first presentation with dense scheduling information



For the squash application, adapt this into an original design using University of Sydney Squash Club branding. Do not use official university trademarks or logos unless assets are already provided and their use is authorised.

## Important working style

Start in **architect mode**.

Do not immediately generate the whole application.

First:

1. Inspect the existing repository, if one exists.
2. Summarise what is already present.
3. Identify missing information.
4. Discuss the major design and product decisions with me.
5. Present sensible options and recommendations.
6. Record accepted decisions as lightweight Architecture Decision Records.
7. Produce an implementation roadmap.
8. Wait for my approval before implementing substantial code.

You may create a small planning document, but do not scaffold the entire project until the architecture discussion is complete.

## Product goal

Members should be able to quickly see court availability, choose a time, reserve a squash court, and manage their bookings. The user should be able to click a day with a large 'select button', it will then prompt them to provide a name and then confirm their attendance

Club administrators should be able to manage courts, opening hours, unavailable periods, bookings, members, club sessions, and booking rules. Allow for synchronisation with a google calendar api to pull data and fill into the dashboard

The system should feel lightweight and fast rather than like a large commercial booking platform.

## Start the architecture discussion with these decisions

Work through the following topics with me. For every topic:

- Explain the decision in plain language.
- Give two or three reasonable options.
- State your recommendation.
- Explain the trade-offs.
- Ask for my approval where the choice materially affects the system.

### 1. Users and access

Discuss whether the initial release should support:

- Public visitors
- Registered squash-club members
- Committee members
- Administrators
- Coaches or event organisers

Discuss whether club membership should be:

- Manually approved
- Imported from a CSV
- Connected to an existing university membership list
- Managed entirely inside this application

Do not assume that University of Sydney single sign-on is available.

### 2. Authentication
For simplicity, I don't want to setup account creation, upon sign up, they are prompted to select a drop down box of previously registered names,
or they can create a new name

### 3. Booking rules

Discuss and recommend defaults for:

- Booking duration
- Whether bookings use fixed slots or arbitrary start and end times
- Maximum booking length
- How far ahead members can book
- Maximum active future bookings per member
- Cancellation deadlines
- No-show handling
- Consecutive bookings
- Guest access
- Peak and off-peak rules
- Whether committee members can override restrictions
- Whether recurring bookings are allowed
- Buffer time between bookings
- Handling court closures and maintenance

Treat booking-rule configuration as an important part of the design rather than hard-coding every rule.

### 4. Court and schedule model

The main booking screen should display:

- One row per squash court
- Time slots as columns
- A selected date
- Previous and next date controls
- A seven-day date selector
- Available slots
- Booked slots
- The current member’s own bookings
- Club training or organised sessions
- Court closures
- Past slots
- Peak and off-peak periods

- 30-minute slots

Discuss the best way to represent booking status accessibly. Colour must not be the only indicator.

### 5. Booking interaction

Discuss the HTMX interaction flow.

A likely flow is:

1. Member selects a date.
2. HTMX replaces the schedule grid.
3. Member selects an available cell.
4. A booking panel or modal is returned by the server.
5. The member confirms the reservation.
6. The server validates availability inside a database transaction.
7. The affected grid cell and the member’s booking summary are updated.
8. A confirmation message is displayed.

Discuss whether a side panel, modal, or dedicated confirmation page is best.

The server must be the source of truth. Client-side state must not determine whether a booking succeeds.

### 6. Preventing double bookings

Design this carefully.

Discuss:

- Database transactions
- Unique constraints
- Overlapping time-range protection
- Optimistic versus pessimistic locking
- What happens when two members select the same slot simultaneously
- How the user sees a conflict response
- How an administrator override works


### 7. Database

Compare PostgreSQL and SQLite for this club-sized application.

Consider:

- Around 231 members
- Deployment simplicity
- Concurrent bookings
- Backups
- Migrations
- Future growth
- Ease of local development

Recommend one database for production and explain why.

Possible entities include:

- User
- MemberProfile
- Role
- Court
- Venue
- Booking
- BookingParticipant
- BookingRule
- OpeningHours
- CourtClosure
- ClubSession
- SessionRegistration
- AuditLog
- Notification

Refine this model rather than accepting it blindly.

### 8. Payments and credits

Do not assume payments are required.

Discuss whether the MVP should be:

- Free bookings for members
- Membership-entitlement based

Keep payment processing outside the MVP unless there is a strong reason to include it.

Design the domain so payment or credit support could be added later without rewriting the booking system.

### 10. Administration

The admin interface may need:

- Court creation and editing
- Opening-hours management
- Court closures
- Booking search
- Booking creation and cancellation
- Member management
- Role management
- Club-session creation
- CSV member import
- Booking-rule configuration
- Audit history
- Basic utilisation reporting

Separate essential MVP tools from later improvements.

### 11. Mobile design

The supplied reference is desktop-oriented, but this application must work well on phones.

Discuss possible mobile layouts for the court schedule:

- Horizontally scrollable grid
- Court-by-court cards
- Time-first list
- A responsive combination of these approaches

Recommend a desktop and mobile strategy while preserving fast booking.

### 12. Accessibility

Plan for:

- Keyboard navigation
- Visible focus states
- Semantic tables where appropriate
- Screen-reader labels
- Sufficient contrast
- Error summaries
- Non-colour booking-status indicators
- Reduced-motion preferences
- Touch-friendly controls

### 13. Deployment

Discuss a simple deployment model for a student or university club project.

Compare options such as:

- A single Go binary with embedded templates and static assets
- Docker deployment
- Managed PostgreSQL
- Reverse proxy with HTTPS
- Environment-variable configuration

The system should be easy for future committee members to maintain.

### 14. Privacy and security

Plan for:

- Minimal collection of member data
- Authorisation checks on every protected action
- Rate limiting on login and booking actions
- CSRF protection
- Input validation
- Secure session cookies
- Structured audit logs
- Backup and restore procedures
- Avoiding sensitive personal information in application logs
- Administrator-action auditing

Do not provide legal conclusions, but flag Australian privacy considerations that the club should investigate.

## Proposed visual direction

Create an original design system with the following direction:

### Colours

- Primary colour: Sydney Uni navy blue (#0B4F8A style)
- White backgrounds for content sections
- Light grey section backgrounds to separate page content
- Dark navy navigation bar
- Blue used consistently for buttons and interactive elements
- Minimal accent colours outside the primary brand palette
- A restrained red for unavailable or blocked slots
- A distinct green, blue, or outlined treatment for available slots
- A separate treatment for the signed-in member’s own bookings

Do not rely only on red versus green.

### Layout

Desktop:

- Compact top utility bar
- Club logo or wordmark on the left
- Account controls on the right
- Main navigation underneath
- Venue and booking-category controls
- Date-navigation panel
- Wide court-availability grid
- “My upcoming bookings” area
- Club training and organised sessions below the main grid

Mobile:

- Simplified header
- Horizontally scrollable date selector
- Sticky selected-date bar
- Court cards or a scrollable timetable
- Large touch targets
- Booking confirmation in a bottom sheet or dedicated page

### Booking-cell states

Define visually distinct states for:

- Lights On - Available
- Booked by the current member
- Attendance
- Club session
- Lights Off Court closure
- In the past
- Pending interaction

Each state should have:

- Colour or border treatment
- Text or icon
- Accessible label
- Hover and focus behaviour where relevant

## Go architecture preferences

Prefer a conventional, readable Go application over excessive abstraction.

Consider a project structure similar to:

```text
cmd/
  web/
    main.go

internal/
  auth/
  booking/
  court/
  member/
  session/
  admin/
  platform/
    database/
    email/
    web/

templates/
  layouts/
  pages/
  partials/
  components/

static/
  css/
  js/
  images/

migrations/