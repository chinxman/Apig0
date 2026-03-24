package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type sessionEntry struct {
	user    string
	expires time.Time
}

var (
	mu         sync.Mutex
	challenges = map[string]sessionEntry{}
	sessions   = map[string]sessionEntry{}
)

// SessionTTL returns the configured session lifetime.
// Reads APIG0_SESSION_TTL as a Go duration string (e.g. "8h", "30m", "24h").
// Defaults to 8 hours if unset or unparseable.
func SessionTTL() time.Duration {
	if raw := os.Getenv("APIG0_SESSION_TTL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return 8 * time.Hour
}

// NewChallenge stores a short-lived (5 min) token tied to user after password is validated.
func NewChallenge(user string) string {
	tok := randHex(32)
	mu.Lock()
	challenges[tok] = sessionEntry{user: user, expires: time.Now().Add(5 * time.Minute)}
	mu.Unlock()
	return tok
}

// PeekChallenge returns the user for a challenge without consuming it.
// Returns ("", false) if missing or expired.
func PeekChallenge(id string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	e, ok := challenges[id]
	if !ok || time.Now().After(e.expires) {
		delete(challenges, id)
		return "", false
	}
	return e.user, true
}

// ConsumeChallenge deletes the challenge — call this only after successful TOTP.
func ConsumeChallenge(id string) {
	mu.Lock()
	delete(challenges, id)
	mu.Unlock()
}

// NewSession creates a session token valid for SessionTTL() and returns it.
func NewSession(user string) string {
	tok := randHex(32)
	mu.Lock()
	sessions[tok] = sessionEntry{user: user, expires: time.Now().Add(SessionTTL())}
	mu.Unlock()
	return tok
}

// ValidateSession returns the user for a valid, non-expired session token.
func ValidateSession(tok string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	e, ok := sessions[tok]
	if !ok || time.Now().After(e.expires) {
		delete(sessions, tok)
		return "", false
	}
	return e.user, true
}

// DeleteSession removes a session immediately (logout).
func DeleteSession(tok string) {
	mu.Lock()
	delete(sessions, tok)
	mu.Unlock()
}

// SessionMiddleware rejects requests without a valid apig0_session cookie.
// Sets "session_user" in the Gin context on success.
func SessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := c.Cookie("apig0_session")
		if err != nil || tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		user, ok := ValidateSession(tok)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
			return
		}
		c.Set("session_user", user)
		c.Next()
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
