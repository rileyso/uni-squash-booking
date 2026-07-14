# Design Follow-ups

## Validate the expandable mobile navigation rail

- **What:** Prototype the persistent expandable navigation rail at 320px and 390px widths and test it with at least three representative club members before the visual implementation is considered final.
- **Why:** A side rail on a narrow sports timetable is unconventional and may reduce scan width or make destination labels harder to discover.
- **Pros:** Validates label discovery, touch comfort, timetable width, and the overlay expansion behavior before the pattern becomes expensive to change.
- **Cons:** Requires a focused prototype and member-testing session; findings may require revisiting an approved navigation decision.
- **Context:** The design review selected a persistent icon rail over a header menu or bottom navigation. The rail expands as an overlay to show full labels and must work without page-level horizontal scrolling.
- **Depends on / blocked by:** Representative timetable content, a responsive prototype, and access to at least three regular or new club members.
- **Exit criteria:** At 320px and 390px, all destinations are discoverable, targets are at least 44px, the expanded rail is keyboard and screen-reader operable, and the selected-day timetable remains readable without horizontal page scrolling.
