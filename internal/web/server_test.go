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
	if shown.Code != http.StatusOK || !strings.Contains(shown.Body.String(), "alexexample#") || !strings.Contains(shown.Body.String(), "/attendance/new?") {
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
	form := url.Values{"date": {"2026-07-15"}, "start": {"900"}, "end": {"1080"}, "csrf": {session.CSRF}}
	reviewRequest := httptest.NewRequest(http.MethodPost, "/attendance/review", strings.NewReader(form.Encode()))
	reviewRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reviewRequest.AddCookie(cookie)
	review := httptest.NewRecorder()
	server.Handler().ServeHTTP(review, reviewRequest)
	if review.Code != http.StatusOK || !strings.Contains(review.Body.String(), "Review attendance") || !strings.Contains(review.Body.String(), "3 hours") {
		t.Fatalf("review status=%d body=%s", review.Code, review.Body.String())
	}
	confirmRequest := httptest.NewRequest(http.MethodPost, "/attendance", strings.NewReader(form.Encode()))
	confirmRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	confirmRequest.AddCookie(cookie)
	confirm := httptest.NewRecorder()
	server.Handler().ServeHTTP(confirm, confirmRequest)
	if confirm.Code != http.StatusSeeOther {
		t.Fatalf("confirm status=%d body=%s", confirm.Code, confirm.Body.String())
	}
	dashboardRequest := httptest.NewRequest(http.MethodGet, "/?date=2026-07-15", nil)
	dashboardRequest.AddCookie(cookie)
	dashboard := httptest.NewRecorder()
	server.Handler().ServeHTTP(dashboard, dashboardRequest)
	if !strings.Contains(dashboard.Body.String(), "Your plans") || !strings.Contains(dashboard.Body.String(), "Flow Tester") {
		t.Fatalf("dashboard missing saved plan: %s", dashboard.Body.String())
	}
}

func TestAttendanceEditorUsesHiddenDateAndOneGraduatedTimeline(t *testing.T) {
	server := newTestServer(t)
	session, err := server.app.CreateAccount(context.Background(), "Timeline Tester", "7531", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/attendance/new?date=2026-07-15&start=900", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: session.Token})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || strings.Contains(body, `type="date"`) || !strings.Contains(body, `Wednesday 15 July`) || !strings.Contains(body, `<small>Coming</small><small>Leaving</small>`) || !strings.Contains(body, `class="graduated-timeline"`) || !strings.Contains(body, `data-allowed-start="870" data-allowed-end="1320"`) || strings.Count(body, `class="closed"`) == 0 || strings.Count(body, `type="range"`) != 2 || strings.Count(body, `min="600" max="1320"`) != 2 || strings.Count(body, `value="900" data-start-range`) != 1 {
		t.Fatalf("attendance editor has unexpected date/timeline controls: %s", body)
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
	for _, expected := range []string{`class="date-anchor"`, `class="horizontal-calendar"`, `>10:00</time>`, `>13:00</time>`, `>22:00</time>`, `class="turnout-count`, `>Court 1</th>`, `>Court 2</th>`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("horizontal desktop timetable missing %q", expected)
		}
	}
	if strings.Count(body, `href="/?date=`) < 8 {
		t.Fatal("date navigation does not contain eight date choices")
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
	if !strings.Contains(body, "Interval details") {
		t.Fatal("aggregate detail disclosure is missing")
	}
	if strings.Contains(body, "Player 1") || strings.Contains(body, "#0001") {
		t.Fatal("aggregate detail exposed an identity")
	}
}
