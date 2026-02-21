// Package security provides account lockout and per-user rate limiting.
package security

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
)

// ── Account Lockout ──────────────────────────────────────────

// LoginAttemptTracker tracks failed login attempts per email.
type LoginAttemptTracker struct {
	mu       sync.Mutex
	attempts map[string]*attemptInfo
	maxFails int
	lockDur  time.Duration
}

type attemptInfo struct {
	failCount int
	lockedAt  time.Time
}

// NewLoginAttemptTracker creates a tracker with configurable limits.
// maxFails: number of failed attempts before lockout.
// lockDuration: how long the account stays locked.
func NewLoginAttemptTracker(maxFails int, lockDuration time.Duration) *LoginAttemptTracker {
	t := &LoginAttemptTracker{
		attempts: make(map[string]*attemptInfo),
		maxFails: maxFails,
		lockDur:  lockDuration,
	}
	// Background cleanup of stale entries every 10 minutes
	go t.cleanup()
	return t
}

// IsLocked checks whether the given email is currently locked out.
func (t *LoginAttemptTracker) IsLocked(email string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	info, ok := t.attempts[email]
	if !ok {
		return false
	}

	// Check if lock has expired
	if !info.lockedAt.IsZero() && time.Since(info.lockedAt) > t.lockDur {
		delete(t.attempts, email)
		return false
	}

	return info.failCount >= t.maxFails
}

// RecordFailure records a failed login attempt. Returns true if now locked.
func (t *LoginAttemptTracker) RecordFailure(email string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	info, ok := t.attempts[email]
	if !ok {
		info = &attemptInfo{}
		t.attempts[email] = info
	}

	info.failCount++
	if info.failCount >= t.maxFails {
		info.lockedAt = time.Now()
		return true
	}
	return false
}

// RecordSuccess clears the failure counter on successful login.
func (t *LoginAttemptTracker) RecordSuccess(email string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, email)
}

// RemainingAttempts returns how many attempts are left before lockout.
func (t *LoginAttemptTracker) RemainingAttempts(email string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	info, ok := t.attempts[email]
	if !ok {
		return t.maxFails
	}
	rem := t.maxFails - info.failCount
	if rem < 0 {
		return 0
	}
	return rem
}

func (t *LoginAttemptTracker) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		t.mu.Lock()
		now := time.Now()
		for email, info := range t.attempts {
			if !info.lockedAt.IsZero() && now.Sub(info.lockedAt) > t.lockDur {
				delete(t.attempts, email)
			}
		}
		t.mu.Unlock()
	}
}

// ── Per-User Rate Limiting ───────────────────────────────────

// UserRateLimiter applies rate limiting on a per-user basis using
// a sliding window counter.
type UserRateLimiter struct {
	mu      sync.Mutex
	users   map[string]*userWindow
	maxReqs int
	window  time.Duration
}

type userWindow struct {
	count   int
	resetAt time.Time
}

// NewUserRateLimiter creates a per-user rate limiter.
// maxRequests: maximum requests allowed within the window.
// window: sliding window duration.
func NewUserRateLimiter(maxRequests int, window time.Duration) *UserRateLimiter {
	rl := &UserRateLimiter{
		users:   make(map[string]*userWindow),
		maxReqs: maxRequests,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

// Middleware returns a Gin middleware that enforces per-user rate limits.
// It reads the user_id set by the Auth middleware.
func (rl *UserRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			// No user context — skip (public routes use the global limiter)
			c.Next()
			return
		}

		key := userID.(interface{ String() string }).String()

		rl.mu.Lock()
		w, ok := rl.users[key]
		if !ok || time.Now().After(w.resetAt) {
			w = &userWindow{count: 0, resetAt: time.Now().Add(rl.window)}
			rl.users[key] = w
		}
		w.count++
		allowed := w.count <= rl.maxReqs
		remaining := rl.maxReqs - w.count
		if remaining < 0 {
			remaining = 0
		}
		rl.mu.Unlock()

		c.Header("X-RateLimit-Limit", http.StatusText(rl.maxReqs))
		c.Header("X-RateLimit-Remaining", http.StatusText(remaining))

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":      "per-user rate limit exceeded",
				"error_code": apperrors.CodeRateLimited,
			})
			return
		}

		c.Next()
	}
}

func (rl *UserRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, w := range rl.users {
			if now.After(w.resetAt) {
				delete(rl.users, key)
			}
		}
		rl.mu.Unlock()
	}
}
