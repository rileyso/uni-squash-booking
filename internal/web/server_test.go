package web

import (
	"context"
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
	created.AddCookie(cookies[0])
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

func TestAnonymousTimetableContainsStatusTextAndNoNames(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, expected := range []string{"Manning Squash Courts (A24)", "Attendance does not reserve a court", "Open for play"} {
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
	for _, expected := range []string{"Selected planned attendance", "12:00 pm-2:00 pm", "4:00 pm-5:00 pm", "Total selected", "3 hours"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("selected attendance panel missing %q", expected)
		}
	}
	for _, expected := range []string{`name="range_start" value="720"`, `name="range_end" value="840"`, `name="range_start" value="960"`, `name="range_end" value="1020"`, `Confirm attendance`} {
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
	if !strings.Contains(body, "Selected planned attendance") {
		t.Fatal("selected attendance disclosure is missing")
	}
	if strings.Contains(body, "Player 1") || strings.Contains(body, "#0001") {
		t.Fatal("aggregate detail exposed an identity")
	}
}
