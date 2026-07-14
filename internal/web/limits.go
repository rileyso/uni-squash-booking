package web

import (
	"net"
	"strings"
	"sync"
	"time"
)

type attemptWindow struct {
	Count int
	Reset time.Time
}
type attemptLimiter struct {
	mu     sync.Mutex
	values map[string]attemptWindow
	now    func() time.Time
}

func newAttemptLimiter() *attemptLimiter {
	return &attemptLimiter{values: make(map[string]attemptWindow), now: time.Now}
}

func (l *attemptLimiter) loginAllowed(username, address string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count("login:user:"+strings.ToLower(strings.TrimSpace(username)), 15*time.Minute) < 5 && l.count("login:network:"+clientHost(address), 15*time.Minute) < 25
}
func (l *attemptLimiter) loginFailed(username, address string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.increment("login:user:"+strings.ToLower(strings.TrimSpace(username)), 15*time.Minute)
	l.increment("login:network:"+clientHost(address), 15*time.Minute)
}
func (l *attemptLimiter) loginSucceeded(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.values, "login:user:"+strings.ToLower(strings.TrimSpace(username)))
}
func (l *attemptLimiter) creationAllowed(address, userAgent string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	network := "create:network:" + clientHost(address)
	device := "create:device:" + clientHost(address) + ":" + userAgent
	if l.count(network, time.Hour) >= 20 || l.count(device, time.Hour) >= 3 {
		return false
	}
	l.increment(network, time.Hour)
	l.increment(device, time.Hour)
	return true
}
func (l *attemptLimiter) count(key string, window time.Duration) int {
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
}
func clientHost(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return host
	}
	return address
}
