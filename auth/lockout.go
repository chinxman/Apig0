package auth

import (
	"sync"
	"time"
)

const (
	maxFailedAttempts = 5
	lockoutDuration   = 15 * time.Minute
)

type lockoutEntry struct {
	failures int
	lockedAt time.Time // zero if not locked
}

var (
	lockoutMu sync.Mutex
	lockouts  = map[string]*lockoutEntry{}
)

func init() {
	// Sweep expired lockout entries every 5 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			sweepLockouts()
		}
	}()
}

// IsLockedOut returns true if the account is currently locked.
func IsLockedOut(username string) bool {
	lockoutMu.Lock()
	defer lockoutMu.Unlock()
	e, ok := lockouts[username]
	if !ok {
		return false
	}
	if !e.lockedAt.IsZero() && time.Since(e.lockedAt) < lockoutDuration {
		return true
	}
	// Lockout expired — reset
	if !e.lockedAt.IsZero() {
		delete(lockouts, username)
	}
	return false
}

// RecordFailure increments the failure count and locks the account if threshold is reached.
func RecordFailure(username string) {
	lockoutMu.Lock()
	defer lockoutMu.Unlock()
	e, ok := lockouts[username]
	if !ok {
		e = &lockoutEntry{}
		lockouts[username] = e
	}
	e.failures++
	if e.failures >= maxFailedAttempts {
		e.lockedAt = time.Now()
	}
}

// ClearFailures resets the failure count after a successful login.
func ClearFailures(username string) {
	lockoutMu.Lock()
	defer lockoutMu.Unlock()
	delete(lockouts, username)
}

func sweepLockouts() {
	lockoutMu.Lock()
	defer lockoutMu.Unlock()
	for user, e := range lockouts {
		if !e.lockedAt.IsZero() && time.Since(e.lockedAt) >= lockoutDuration {
			delete(lockouts, user)
		}
	}
}
