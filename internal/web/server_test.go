package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rileyso/uni-squash-booking/internal/app"
	"github.com/rileyso/uni-squash-booking/internal/config"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

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

func TestSyntheticMarkerAndNoIdentityRoutes(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(response.Body.String(), "Synthetic development data only") {
		t.Fatal("synthetic marker missing")
	}
	for _, path := range []string{"/accounts/new", "/attendance", "/attendees", "/admin"} {
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
	for _, expected := range []string{"Planned attendance", "Attendance does not reserve a court", "Open for play", "Official social"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %q", expected)
		}
	}
	if strings.Contains(body, "Player 1") || strings.Contains(body, "synthetic0001") {
		t.Fatal("anonymous timetable exposed an identity")
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
	if !strings.Contains(body, "Interval details") || !strings.Contains(body, "No one has announced attendance yet") {
		t.Fatal("aggregate detail disclosure is missing")
	}
	if strings.Contains(body, "Player 1") || strings.Contains(body, "#0001") {
		t.Fatal("aggregate detail exposed an identity")
	}
}
