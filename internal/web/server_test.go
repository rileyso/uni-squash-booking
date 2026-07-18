package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rileyso/uni-squash-booking/internal/app"
	"github.com/rileyso/uni-squash-booking/internal/config"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

func TestAccountCreationPreservesAttendanceIntent(t *testing.T) {
	server := newTestServer(t)
	form := url.Values{"display_name": {"Alex Example"}, "pin": {"2468"}, "pin_confirm": {"2468"}, "member_status": {"member"}, "privacy_ack": {"yes"}, "intent_date": {"2026-07-15"}, "intent_start": {"900"}}
	request := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || !strings.HasPrefix(response.Header().Get("Location"), "/account-created?") {
		t.Fatalf("status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
	cookies := response.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("member session cookie missing")
	}
	created := httptest.NewRequest(http.MethodGet, response.Header().Get("Location"), nil)
	for _, cookie := range cookies {
		if cookie.Name == sessionCookie {
			created.AddCookie(cookie)
		}
	}
	shown := httptest.NewRecorder()
	server.Handler().ServeHTTP(shown, created)
	if shown.Code != http.StatusOK || !strings.Contains(shown.Body.String(), "alexexample#") || !strings.Contains(shown.Body.String(), "/?add=900&amp;date=2026-07-15") || !strings.Contains(shown.Body.String(), "floating-window") || !strings.Contains(shown.Body.String(), "auth-app-background") {
		t.Fatalf("account result did not preserve intent: %s", shown.Body.String())
	}
}

func TestAuthenticatedAttendanceReviewAndConfirmation(t *testing.T) {
	server := newTestServer(t)
	session, err := server.app.CreateAccount(context.Background(), "Flow Tester", "8642", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: session.Token}
	form := url.Values{"date": {"2026-07-21"}, "start": {"960"}, "end": {"1080"}, "csrf": {session.CSRF}}
	reviewRequest := httptest.NewRequest(http.MethodPost, "/attendance/review", strings.NewReader(form.Encode()))
	reviewRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reviewRequest.AddCookie(cookie)
	review := httptest.NewRecorder()
	server.Handler().ServeHTTP(review, reviewRequest)
	if review.Code != http.StatusSeeOther || review.Header().Get("Location") != "/?add=960&date=2026-07-21&end=1080&review=1" {
		t.Fatalf("review status=%d location=%q", review.Code, review.Header().Get("Location"))
	}
	confirmRequest := httptest.NewRequest(http.MethodPost, "/attendance", strings.NewReader(form.Encode()))
	confirmRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	confirmRequest.AddCookie(cookie)
	confirm := httptest.NewRecorder()
	server.Handler().ServeHTTP(confirm, confirmRequest)
	if confirm.Code != http.StatusSeeOther {
		t.Fatalf("confirm status=%d body=%s", confirm.Code, confirm.Body.String())
	}
	dashboardRequest := httptest.NewRequest(http.MethodGet, "/?date=2026-07-21", nil)
	dashboardRequest.AddCookie(cookie)
	dashboard := httptest.NewRecorder()
	server.Handler().ServeHTTP(dashboard, dashboardRequest)
	if !strings.Contains(dashboard.Body.String(), "Your plans") || !strings.Contains(dashboard.Body.String(), "Flow Tester") {
		t.Fatalf("dashboard missing saved plan: %s", dashboard.Body.String())
	}
	if !strings.Contains(dashboard.Body.String(), session.Member.FullUsername) || !strings.Contains(dashboard.Body.String(), `class="account-username"`) {
		t.Fatalf("dashboard missing account username: %s", dashboard.Body.String())
	}
	if !strings.Contains(dashboard.Body.String(), `class="plan-marker" aria-label="Your planned attendance"`) {
		t.Fatalf("dashboard missing saved attendance marker: %s", dashboard.Body.String())
	}
}

func TestAnonymousAttendanceFlowPromptsForSignInOnlyAfterConfirmation(t *testing.T) {
	server := newTestServer(t)
	page := httptest.NewRecorder()
	server.Handler().ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/?date=2026-07-21&add=960", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Add my attendance") || !strings.Contains(page.Body.String(), "Arrival slider") {
		t.Fatalf("anonymous attendance page status=%d body=%s", page.Code, page.Body.String())
	}
	form := url.Values{"date": {"2026-07-21"}, "start": {"960"}, "end": {"1080"}}
	reviewRequest := httptest.NewRequest(http.MethodPost, "/attendance/review", strings.NewReader(form.Encode()))
	reviewRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	review := httptest.NewRecorder()
	server.Handler().ServeHTTP(review, reviewRequest)
	if review.Code != http.StatusSeeOther || review.Header().Get("Location") != "/?add=960&date=2026-07-21&end=1080&review=1" {
		t.Fatalf("anonymous review status=%d location=%q", review.Code, review.Header().Get("Location"))
	}
	confirmRequest := httptest.NewRequest(http.MethodPost, "/attendance", strings.NewReader(form.Encode()))
	confirmRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	confirm := httptest.NewRecorder()
	server.Handler().ServeHTTP(confirm, confirmRequest)
	location := confirm.Header().Get("Location")
	if confirm.Code != http.StatusSeeOther || !strings.HasPrefix(location, "/sign-in?") || !strings.Contains(location, "intent_end=1080") {
		t.Fatalf("anonymous confirm status=%d location=%q", confirm.Code, location)
	}
}

func TestAttendanceGETRedirectsToCanonicalTimetable(t *testing.T) {
	server := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/attendance?date=2026-07-21&time=960", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != "/?date=2026-07-21&time=960" {
		t.Fatalf("status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
}

func TestAttendanceSliderUsesContinuousOpenRange(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?date=2026-07-22&add=840", nil))
	body := response.Body.String()
	for _, expected := range []string{
		`data-min-duration="60"`,
		`data-open-ranges="870-1320"`,
		`input type="range" min="870" max="1320" step="30" value="870" data-start-range`,
		`input type="range" min="870" max="1320" step="30" value="930" data-end-range`,
		`<output data-start-label>2:30 pm</output>`,
		`<output data-end-label>3:30 pm</output>`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("attendance slider missing %q", expected)
		}
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	configuration, err := config.Load(func(key string) string {
		values := map[string]string{"APP_ENV": "test", "DATABASE_PATH": filepath.Join(t.TempDir(), "test.sqlite")}
		return values[key]
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := sqlite.Open(context.Background(), configuration.DatabasePath, configuration.RecoveryGeneration)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	application, err := app.New(configuration, store)
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(application)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func TestFoundationRoutesAndSecurityHeaders(t *testing.T) {
	server := newTestServer(t)
	for _, path := range []string{"/", "/healthz", "/readyz"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code >= 400 {
			t.Fatalf("GET %s status = %d", path, response.Code)
		}
		if response.Header().Get("Content-Security-Policy") == "" {
			t.Fatalf("GET %s missing CSP", path)
		}
	}
}

func TestSyntheticMarkerAndIdentityRoutes(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(response.Body.String(), "Synthetic development data only") {
		t.Fatal("synthetic marker missing")
	}
	for _, path := range []string{"/accounts/new", "/sign-in"} {
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, response.Code)
		}
		if !strings.Contains(response.Body.String(), "floating-window") {
			t.Fatalf("GET %s missing floating auth window", path)
		}
		if !strings.Contains(response.Body.String(), "auth-app-background") || !strings.Contains(response.Body.String(), "horizontal-calendar") {
			t.Fatalf("GET %s missing background calendar", path)
		}
	}
	for _, path := range []string{"/attendees", "/admin"} {
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", path, response.Code)
		}
	}
}

func TestSignInCanRememberOnlyUsername(t *testing.T) {
	server := newTestServer(t)
	page := httptest.NewRecorder()
	server.Handler().ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/sign-in", nil))
	for _, expected := range []string{`src="/static/auth.js"`, `data-sign-in-form`, `data-remember-username`, `data-remember-toggle`, "Remember username on this browser"} {
		if !strings.Contains(page.Body.String(), expected) {
			t.Fatalf("sign-in page missing %q", expected)
		}
	}
	form := url.Values{"username": {"john#1111"}, "pin": {"9999"}}
	request := httptest.NewRequest(http.MethodPost, "/sign-in", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	body := response.Body.String()
	if !strings.Contains(body, `value="john#1111"`) || strings.Contains(body, `value="9999"`) {
		t.Fatalf("failed sign-in did not preserve only username: %s", body)
	}
}

func TestAuthenticationHTMXSurfaceAndRedirectContract(t *testing.T) {
	server := newTestServer(t)
	fragmentRequest := httptest.NewRequest(http.MethodGet, "/sign-in?intent_date=2026-07-21&intent_time=960", nil)
	fragmentRequest.Header.Set("HX-Request", "true")
	fragment := httptest.NewRecorder()
	server.Handler().ServeHTTP(fragment, fragmentRequest)
	body := fragment.Body.String()
	if fragment.Code != http.StatusOK || !strings.Contains(body, `class="form-page floating-page auth-surface-overlay"`) || strings.Contains(body, "<!doctype html>") || strings.Contains(body, "auth-app-background") {
		t.Fatalf("contextual auth status=%d body=%s", fragment.Code, body)
	}
	if fragment.Header().Get("Cache-Control") != "private, no-store" || !strings.Contains(fragment.Header().Get("Vary"), "HX-Request") {
		t.Fatalf("contextual auth headers=%v", fragment.Header())
	}

	failureForm := url.Values{"username": {"john#1111"}, "pin": {"9999"}, "intent_date": {"2026-07-21"}, "intent_time": {"960"}}
	failureRequest := httptest.NewRequest(http.MethodPost, "/sign-in", strings.NewReader(failureForm.Encode()))
	failureRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	failureRequest.Header.Set("HX-Request", "true")
	failure := httptest.NewRecorder()
	server.Handler().ServeHTTP(failure, failureRequest)
	if failure.Code != http.StatusUnauthorized || !strings.Contains(failure.Body.String(), `value="john#1111"`) || strings.Contains(failure.Body.String(), `value="9999"`) || strings.Contains(failure.Body.String(), "<!doctype html>") {
		t.Fatalf("contextual failure status=%d body=%s", failure.Code, failure.Body.String())
	}

	successForm := url.Values{"username": {"john#1111"}, "pin": {"1111"}, "intent_date": {"2026-07-21"}, "intent_time": {"960"}}
	successRequest := httptest.NewRequest(http.MethodPost, "/sign-in", strings.NewReader(successForm.Encode()))
	successRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	successRequest.Header.Set("HX-Request", "true")
	success := httptest.NewRecorder()
	server.Handler().ServeHTTP(success, successRequest)
	if success.Code != http.StatusOK || success.Header().Get("HX-Redirect") != "/?date=2026-07-21&time=960" {
		t.Fatalf("contextual success status=%d HX-Redirect=%q", success.Code, success.Header().Get("HX-Redirect"))
	}
}

func TestAnonymousAttendFormDirectlyTargetsLoginSurface(t *testing.T) {
	server := newTestServer(t)
	pageRequest := httptest.NewRequest(http.MethodGet, "/?date=2026-07-21&time=960", nil)
	pageRequest.Header.Set("HX-Request", "true")
	page := httptest.NewRecorder()
	server.Handler().ServeHTTP(page, pageRequest)
	for _, expected := range []string{`method="get" action="/sign-in"`, `hx-get="/sign-in"`, `hx-target="#auth-surface-root"`, `hx-select="unset"`, `hx-swap="innerHTML"`, `hx-push-url="false"`, `name="intent_date" value="2026-07-21"`, `name="intent_time" value="960"`, `>Attend</button>`} {
		if !strings.Contains(page.Body.String(), expected) {
			t.Fatalf("anonymous Attend form missing %q: %s", expected, page.Body.String())
		}
	}

	form := url.Values{"date": {"2026-07-21"}, "range_start": {"960"}, "range_end": {"1020"}}
	request := httptest.NewRequest(http.MethodPost, "/attendance", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `class="form-page floating-page auth-surface-overlay"`) || !strings.Contains(body, `name="intent_time" value="960"`) || strings.Contains(body, "<!doctype html>") {
		t.Fatalf("Attend did not return contextual login status=%d body=%s", response.Code, body)
	}
}

func TestAnonymousTimetableContainsStatusTextAndNoNames(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, expected := range []string{"Manning Squash Courts (A24)", "Attendance does not reserve a court", "Open for play", `href="/static/forms.css"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %q", expected)
		}
	}
	if strings.Contains(body, "Player 1") || strings.Contains(body, "synthetic0001") {
		t.Fatal("anonymous timetable exposed an identity")
	}
	if strings.Contains(body, "Official social") {
		t.Fatal("official social label is rendered")
	}
	if strings.Contains(body, "> C1 Open for play<") || strings.Contains(body, "> C2 Open for play<") {
		t.Fatal("mobile court rails repeat verbose court labels")
	}
	if strings.Index(body, `/static/forms.css`) > strings.Index(body, `/static/detail.css`) {
		t.Fatal("detail styles must load after shared form styles so the contextual login remains fixed over the calendar")
	}
}

func TestDesktopTimetableUsesSelectedDayHorizontalLayout(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, expected := range []string{`class="date-anchor"`, `class="horizontal-calendar"`, `>10:00</time>`, `>13:00</time>`, `>22:00</time>`, `class="turnout-count`, `class="fa-users-icon"`, `>Court 1</th>`, `>Court 2</th>`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("horizontal desktop timetable missing %q", expected)
		}
	}
	if strings.Count(body, `href="/?date=`) < 8 {
		t.Fatal("date navigation does not contain eight date choices")
	}
}

func TestDashboardIncludesAccessibleExpandableMobileNavigation(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, expected := range []string{
		`src="/static/mobile-nav.js"`,
		`class="nav-toggle"`,
		`aria-expanded="false"`,
		`aria-controls="club-navigation"`,
		`id="club-navigation"`,
		`<span class="nav-label">Attendance</span>`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("mobile navigation missing %q", expected)
		}
	}
}

func TestIndexHTMXReturnsTimetableInteractionFragment(t *testing.T) {
	server := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/?date=2026-07-21&time=960", nil)
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(body, `id="timetable-interaction"`) || !strings.Contains(body, "Planned attendance after adding") {
		t.Fatalf("HTMX fragment missing authoritative timetable state: %s", body)
	}
	if strings.Contains(body, "<!doctype html>") || strings.Contains(body, "<head>") {
		t.Fatal("HTMX response rendered the full page")
	}
	if !strings.Contains(response.Header().Get("Vary"), "HX-Request") {
		t.Fatalf("Vary = %q", response.Header().Get("Vary"))
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}

	fallbackRequest := httptest.NewRequest(http.MethodGet, "/?date=2026-07-21", nil)
	fallbackRequest.Header.Set("HX-Request", "invalid")
	fallback := httptest.NewRecorder()
	server.Handler().ServeHTTP(fallback, fallbackRequest)
	fallbackBody := fallback.Body.String()
	if !strings.Contains(fallbackBody, "<!doctype html>") {
		t.Fatal("malformed HTMX header did not fall back to the full page")
	}
	for _, expected := range []string{`src="/static/htmx.min.js"`, `hx-swap="outerHTML show:none"`, `hx-push-url="true"`} {
		if !strings.Contains(fallbackBody, expected) {
			t.Fatalf("full page missing HTMX enhancement %q", expected)
		}
	}
}

func TestDesktopIntervalSelectionStaysHighlightedUntilResetOrDateChange(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?date=2026-07-21&time=720,780,960", nil))
	body := response.Body.String()
	if strings.Count(body, `class="selected-interval"`) != 9 {
		t.Fatalf("selected desktop court cells not highlighted: %d", strings.Count(body, `class="selected-interval"`))
	}
	if !strings.Contains(body, `class="reset-selection" href="/?date=2026-07-21"`) {
		t.Fatal("selected interval reset link missing")
	}
	if strings.Contains(body, ` at 16:00</h2>`) {
		t.Fatal("interval detail heading still includes the selected time")
	}
	for _, expected := range []string{"Planned attendance after adding", "12:00 pm-2:00 pm", "4:00 pm-5:00 pm"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("selected attendance panel missing %q", expected)
		}
	}
	if strings.Contains(body, "Total planned") {
		t.Fatal("total planned duration is still shown")
	}
	for _, expected := range []string{`name="range_start" value="720"`, `name="range_end" value="840"`, `name="range_start" value="960"`, `name="range_end" value="1020"`, `Attend`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("selected attendance confirmation missing %q", expected)
		}
	}
	if strings.Contains(body, "Split selections cannot be confirmed") {
		t.Fatal("split selections are still blocked")
	}
	if !strings.Contains(body, `href="/?date=2026-07-22"`) || strings.Contains(body, `href="/?date=2026-07-22&amp;time=`) {
		t.Fatal("date navigation does not clear selected interval")
	}
}

func TestToggleTimeHrefAddsAndRemovesSelectedIntervals(t *testing.T) {
	if got := toggleTimeHref("2026-07-14", "960", 1080); got != "/?date=2026-07-14&time=960%2C1080" {
		t.Fatalf("add toggle href = %q", got)
	}
	if got := toggleTimeHref("2026-07-14", "960,1080", 960); got != "/?date=2026-07-14&time=1080" {
		t.Fatalf("remove toggle href = %q", got)
	}
	if got := toggleTimeHref("2026-07-14", "960", 960); got != "/?date=2026-07-14" {
		t.Fatalf("last remove toggle href = %q", got)
	}
}

func TestConfirmingSplitSelectionSavesMultipleIntervals(t *testing.T) {
	server := newTestServer(t)
	session, err := server.app.CreateAccount(context.Background(), "Split Tester", "8642", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{
		"date":        {"2026-07-21"},
		"range_start": {"720", "960"},
		"range_end":   {"840", "1080"},
		"csrf":        {session.CSRF},
	}
	request := httptest.NewRequest(http.MethodPost, "/attendance", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: session.Token})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("confirm status=%d body=%s", response.Code, response.Body.String())
	}
	plans, err := server.app.Plans(context.Background(), session.Member)
	if err != nil || len(plans) != 2 || plans[0].StartMinute != 720 || plans[1].StartMinute != 960 {
		t.Fatalf("split plans not saved: %#v err=%v", plans, err)
	}
}

func TestExistingPlannedCellCannotBeSelectedAndAdjacentAdditionMerges(t *testing.T) {
	server := newTestServer(t)
	session, err := server.app.CreateAccount(context.Background(), "Additive Player", "2468", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.app.SaveAttendance(context.Background(), session.Member, "2026-07-21", 960, 1080); err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: session.Token}

	existingRequest := httptest.NewRequest(http.MethodGet, "/?date=2026-07-21&time=960", nil)
	existingRequest.AddCookie(cookie)
	existing := httptest.NewRecorder()
	server.Handler().ServeHTTP(existing, existingRequest)
	if strings.Contains(existing.Body.String(), "Planned attendance after adding") {
		t.Fatal("existing planned cell was accepted as a new selection")
	}

	additionRequest := httptest.NewRequest(http.MethodGet, "/?date=2026-07-21&time=1080", nil)
	additionRequest.AddCookie(cookie)
	addition := httptest.NewRecorder()
	server.Handler().ServeHTTP(addition, additionRequest)
	for _, expected := range []string{"Planned attendance after adding", "4:00 pm-7:00 pm", "3 hours", "Attend"} {
		if !strings.Contains(addition.Body.String(), expected) {
			t.Fatalf("additive summary missing %q", expected)
		}
	}

	form := url.Values{"date": {"2026-07-21"}, "range_start": {"1080"}, "range_end": {"1140"}, "csrf": {session.CSRF}}
	request := httptest.NewRequest(http.MethodPost, "/attendance", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	plans, err := server.app.Plans(context.Background(), session.Member)
	if err != nil || len(plans) != 1 || plans[0].StartMinute != 960 || plans[0].EndMinute != 1140 {
		t.Fatalf("adjacent addition did not merge: plans=%#v err=%v", plans, err)
	}
}

func TestProfileAndPINHTTPFlow(t *testing.T) {
	server := newTestServer(t)
	session, err := server.app.CreateAccount(context.Background(), "HTTP Profile", "2468", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: session.Token}
	pageRequest := httptest.NewRequest(http.MethodGet, "/account/profile", nil)
	pageRequest.AddCookie(cookie)
	page := httptest.NewRecorder()
	server.Handler().ServeHTTP(page, pageRequest)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Your profile") || !strings.Contains(page.Body.String(), session.Member.FullUsername) {
		t.Fatalf("profile page status=%d body=%s", page.Code, page.Body.String())
	}

	profileForm := url.Values{"display_name": {"HTTP Updated"}, "member_status": {"visitor"}, "current_pin": {"2468"}, "csrf": {session.CSRF}}
	profileRequest := httptest.NewRequest(http.MethodPost, "/account/profile", strings.NewReader(profileForm.Encode()))
	profileRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	profileRequest.AddCookie(cookie)
	profile := httptest.NewRecorder()
	server.Handler().ServeHTTP(profile, profileRequest)
	if profile.Code != http.StatusOK || !strings.Contains(profile.Body.String(), "Profile updated") || !strings.Contains(profile.Body.String(), "HTTP Updated") {
		t.Fatalf("profile update status=%d body=%s", profile.Code, profile.Body.String())
	}

	pinForm := url.Values{"current_pin": {"2468"}, "new_pin": {"8642"}, "new_pin_confirm": {"8642"}, "csrf": {session.CSRF}}
	pinRequest := httptest.NewRequest(http.MethodPost, "/account/pin", strings.NewReader(pinForm.Encode()))
	pinRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	pinRequest.AddCookie(cookie)
	pin := httptest.NewRecorder()
	server.Handler().ServeHTTP(pin, pinRequest)
	if pin.Code != http.StatusOK || !strings.Contains(pin.Body.String(), "Other signed-in devices have been signed out") {
		t.Fatalf("PIN update status=%d body=%s", pin.Code, pin.Body.String())
	}
}

func TestAdministratorAuthenticationSearchAndAttestedReset(t *testing.T) {
	server := newTestServer(t)
	memberSession, err := server.app.CreateAccount(context.Background(), "Recovery Target", "2468", "member", true)
	if err != nil {
		t.Fatal(err)
	}

	signInForm := url.Values{"username": {"riley"}, "password": {"synthetic-admin-password"}}
	signInRequest := httptest.NewRequest(http.MethodPost, "/admin/sign-in", strings.NewReader(signInForm.Encode()))
	signInRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	signIn := httptest.NewRecorder()
	server.Handler().ServeHTTP(signIn, signInRequest)
	if signIn.Code != http.StatusSeeOther || signIn.Header().Get("Location") != "/admin/accounts" {
		t.Fatalf("admin sign-in status=%d location=%q", signIn.Code, signIn.Header().Get("Location"))
	}
	var adminCookie *http.Cookie
	for _, cookie := range signIn.Result().Cookies() {
		if cookie.Name == adminSessionCookie {
			adminCookie = cookie
		}
	}
	if adminCookie == nil || adminCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("admin cookie=%#v", adminCookie)
	}

	searchRequest := httptest.NewRequest(http.MethodGet, "/admin/accounts?q="+url.QueryEscape(memberSession.Member.FullUsername), nil)
	searchRequest.AddCookie(adminCookie)
	search := httptest.NewRecorder()
	server.Handler().ServeHTTP(search, searchRequest)
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), "Recovery Target") || !strings.Contains(search.Body.String(), memberSession.Member.FullUsername) {
		t.Fatalf("admin search status=%d body=%s", search.Code, search.Body.String())
	}
	csrf, err := server.app.AdminForToken(context.Background(), adminCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	resetForm := url.Values{"csrf": {csrf}, "q": {memberSession.Member.FullUsername}, "identity_check_attested": {"yes"}}
	resetRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/accounts/%d/reset-pin", memberSession.Member.ID), strings.NewReader(resetForm.Encode()))
	resetRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resetRequest.AddCookie(adminCookie)
	reset := httptest.NewRecorder()
	server.Handler().ServeHTTP(reset, resetRequest)
	if reset.Code != http.StatusOK || !strings.Contains(reset.Body.String(), "Temporary PIN — display once") {
		t.Fatalf("admin reset status=%d body=%s", reset.Code, reset.Body.String())
	}
	if _, _, err := server.app.MemberForToken(context.Background(), memberSession.Token); err == nil {
		t.Fatal("member session survived administrator PIN reset")
	}
}

func TestAnonymousSplitSelectionPreservesSelectedCellsThroughSignIn(t *testing.T) {
	server := newTestServer(t)
	form := url.Values{
		"date":        {"2026-07-21"},
		"range_start": {"720", "960"},
		"range_end":   {"840", "1080"},
	}
	request := httptest.NewRequest(http.MethodPost, "/attendance", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	location := response.Header().Get("Location")
	if response.Code != http.StatusSeeOther || !strings.HasPrefix(location, "/sign-in?") || !strings.Contains(location, "intent_time=720%2C780%2C960%2C1020") {
		t.Fatalf("anonymous split confirm status=%d location=%q", response.Code, location)
	}

	signIn := httptest.NewRecorder()
	server.Handler().ServeHTTP(signIn, httptest.NewRequest(http.MethodGet, location, nil))
	if !strings.Contains(signIn.Body.String(), `name="intent_time" value="720,780,960,1020"`) {
		t.Fatalf("sign in did not preserve selected cells: %s", signIn.Body.String())
	}
}

func TestMobileTimetableContainsThirteenHourlyIntervals(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	start := strings.Index(body, `class="mobile-timetable"`)
	if start < 0 {
		t.Fatal("mobile timetable markup missing")
	}
	end := strings.Index(body[start:], `class="calendar-key"`)
	if end < 0 {
		t.Fatal("mobile timetable boundary missing")
	}
	mobile := body[start : start+end]
	if strings.Count(mobile, "<li>") != 13 || !strings.Contains(mobile, ">10:00 am</time>") || !strings.Contains(mobile, ">10:00 pm</time>") {
		t.Fatalf("mobile timetable does not contain 13 hourly cells from 10 am through 10 pm")
	}
	if !strings.Contains(mobile, `<strong>Court 1</strong><strong>Court 2</strong>`) {
		t.Fatal("mobile timetable is missing aligned court headings")
	}
}

func TestMobileClosedRowsAreNotLinks(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?date=2026-07-15", nil))
	body := response.Body.String()
	start := strings.Index(body, `class="mobile-timetable"`)
	if start < 0 {
		t.Fatal("mobile timetable markup missing")
	}
	end := strings.Index(body[start:], `class="calendar-key"`)
	if end < 0 {
		t.Fatal("mobile timetable boundary missing")
	}
	mobile := body[start : start+end]
	if strings.Contains(mobile, `href="/?date=2026-07-15&amp;time=600"`) {
		t.Fatal("closed mobile row links to interval details")
	}
	if !strings.Contains(mobile, `class="turnout empty turnout-disabled"`) {
		t.Fatal("closed mobile row is not rendered as disabled turnout")
	}
	if !strings.Contains(mobile, `href="/?date=2026-07-15&amp;time=840"`) {
		t.Fatal("open mobile row no longer links to interval details")
	}
}

func TestOpenCourtCellsShowIntervalDetailsWithoutPlusVisual(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?date=2026-07-14", nil))
	body := response.Body.String()
	if !strings.Contains(body, `class="court-status open" href="/?date=2026-07-14&amp;time=960"`) {
		t.Fatal("open court cell does not link to interval details")
	}
	if strings.Contains(body, `class="attendance-add"`) {
		t.Fatal("blue plus attendance visual is still present")
	}
}

func TestAnonymousIntervalDetailUsesAggregateOnly(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?time=1080", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, "Planned attendance after adding") {
		t.Fatal("selected attendance disclosure is missing")
	}
	if strings.Contains(body, "Player 1") || strings.Contains(body, "#0001") {
		t.Fatal("aggregate detail exposed an identity")
	}
}

func TestParticipantNamesUseSeparateCapabilityGatedEndpoint(t *testing.T) {
	server := newTestServer(t)
	session, err := server.app.CreateAccount(context.Background(), "Hover Participant", "2468", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.app.SaveAttendance(context.Background(), session.Member, "2026-07-21", 960, 1080); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/attendance/participants?date=2026-07-21&start=960&end=1020", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Hover Participant") {
		t.Fatalf("participant endpoint status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("participant endpoint cache policy=%q", response.Header().Get("Cache-Control"))
	}
	past := httptest.NewRecorder()
	server.Handler().ServeHTTP(past, httptest.NewRequest(http.MethodGet, "/attendance/participants?date=2026-07-13&start=960&end=1020", nil))
	if past.Code != http.StatusBadRequest {
		t.Fatalf("past participant lookup status=%d", past.Code)
	}
}
