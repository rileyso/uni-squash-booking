package web

import (
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

type attemptWindow struct {
	Count int
	Reset time.Time
}

// attemptLimiter is deliberately process-local, but it is bounded and fully
// clock-injected so expiry and eviction are deterministic.
type attemptLimiter struct {
	mu         sync.Mutex
	values     map[string]attemptWindow
	now        func() time.Time
	maxEntries int
}

func newAttemptLimiter() *attemptLimiter {
	return &attemptLimiter{values: make(map[string]attemptWindow), now: time.Now, maxEntries: 4096}
}

func (l *attemptLimiter) loginAllowed(username, address string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanup()
	user := normalizeLimiterUsername(username)
	network := clientHost(address)
	return l.count("login:user:"+user) < 5 &&
		l.count("login:network:"+network) < 25 &&
		l.count("login:spray:"+network) < 10
}

func (l *attemptLimiter) loginFailed(username, address string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanup()
	user := normalizeLimiterUsername(username)
	network := clientHost(address)
	l.increment("login:user:"+user, 15*time.Minute)
	l.increment("login:network:"+network, 15*time.Minute)
	// One entry per network/account pair makes distinct-account spray visible.
	pair := "login:pair:" + network + ":" + user
	if l.count(pair) == 0 {
		l.increment("login:spray:"+network, 15*time.Minute)
	}
	l.increment(pair, 15*time.Minute)
}

func (l *attemptLimiter) loginSucceeded(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.values, "login:user:"+normalizeLimiterUsername(username))
}

func (l *attemptLimiter) creationAllowed(address, deviceID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanup()
	network := "create:network:" + clientHost(address)
	device := "create:device:" + deviceID
	if deviceID == "" || l.count(network) >= 20 || l.count(device) >= 3 {
		return false
	}
	l.increment(network, time.Hour)
	l.increment(device, time.Hour)
	return true
}

func (l *attemptLimiter) count(key string) int {
	value, ok := l.values[key]
	if !ok || !l.now().Before(value.Reset) {
		delete(l.values, key)
		return 0
	}
	return value.Count
}

func (l *attemptLimiter) increment(key string, window time.Duration) {
	value, ok := l.values[key]
	if !ok || !l.now().Before(value.Reset) {
		value = attemptWindow{Reset: l.now().Add(window)}
	}
	value.Count++
	l.values[key] = value
	l.evict()
}

func (l *attemptLimiter) cleanup() {
	now := l.now()
	for key, value := range l.values {
		if !now.Before(value.Reset) {
			delete(l.values, key)
		}
	}
}

func (l *attemptLimiter) evict() {
	if l.maxEntries < 1 {
		l.maxEntries = 1
	}
	for len(l.values) > l.maxEntries {
		keys := make([]string, 0, len(l.values))
		for key := range l.values {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			a, b := l.values[keys[i]], l.values[keys[j]]
			if a.Reset.Equal(b.Reset) {
				return keys[i] < keys[j]
			}
			return a.Reset.Before(b.Reset)
		})
		delete(l.values, keys[0])
	}
}

func normalizeLimiterUsername(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func clientHost(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return host
	}
	return address
}
