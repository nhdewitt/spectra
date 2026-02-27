package server

import (
	"fmt"
	"sync"
	"time"
)

// Account lockout

type loginTracker struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

type loginAttempt struct {
	count    int
	lockedAt time.Time
}

func newLoginTracker() *loginTracker {
	return &loginTracker{
		attempts: make(map[string]*loginAttempt),
	}
}

// check returns an error of the IP is currently locked out.
func (lt *loginTracker) check(ip string) error {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	a, ok := lt.attempts[ip]
	if !ok {
		return nil
	}

	if a.count >= maxLoginAttempts && time.Since(a.lockedAt) < lockoutDuration {
		remaining := lockoutDuration - time.Since(a.lockedAt)
		return fmt.Errorf("account locked, try again in %s", remaining.Round(time.Second))
	}

	if a.count >= maxLoginAttempts {
		delete(lt.attempts, ip)
	}

	return nil
}

// recordFailure increments the failure count for an IP.
func (lt *loginTracker) recordFailure(ip string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	now := time.Now()

	a, ok := lt.attempts[ip]
	if !ok {
		lt.attempts[ip] = &loginAttempt{count: 1, lockedAt: now}
		return
	}

	a.count++
	a.lockedAt = now
}

// recordSuccess clears the failure count for an IP.
func (lt *loginTracker) recordSuccess(ip string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	delete(lt.attempts, ip)
}
