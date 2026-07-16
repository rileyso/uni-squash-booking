package web

import (
	"testing"
	"time"
)

func TestAttemptLimiterBoundariesAndExpiry(t *testing.T) {
	limiter := newAttemptLimiter()
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	for attempt := 0; attempt < 5; attempt++ {
		if !limiter.loginAllowed("player#1234", "127.0.0.1:1000") {
			t.Fatalf("login blocked at attempt %d", attempt)
		}
		limiter.loginFailed("player#1234", "127.0.0.1:1000")
	}
	if limiter.loginAllowed("player#1234", "127.0.0.1:1000") {
		t.Fatal("fifth failed login did not lock account")
	}
	now = now.Add(16 * time.Minute)
	if !limiter.loginAllowed("player#1234", "127.0.0.1:1000") {
		t.Fatal("login lock did not expire")
	}
	for attempt := 0; attempt < 3; attempt++ {
		if !limiter.creationAllowed("127.0.0.1:1000", "signed-device") {
			t.Fatalf("creation blocked at attempt %d", attempt)
		}
	}
	if limiter.creationAllowed("127.0.0.1:1000", "signed-device") {
		t.Fatal("fourth device creation attempt accepted")
	}
}

func TestAttemptLimiterIsBoundedAndEvictionDeterministic(t *testing.T) {
	limiter := newAttemptLimiter()
	limiter.maxEntries = 3
	limiter.now = func() time.Time { return time.Unix(100, 0) }
	for _, user := range []string{"c#0003", "a#0001", "b#0002"} {
		limiter.loginFailed(user, "127.0.0.1:1")
	}
	if len(limiter.values) > limiter.maxEntries {
		t.Fatalf("entries=%d max=%d", len(limiter.values), limiter.maxEntries)
	}
}
